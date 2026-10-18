// Package tunnel opens TCP tunnels to private network targets through a
// short-lived socat pod and the Kubernetes port-forward API.
package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// DefaultNamespace is where tunnel pods run unless configured otherwise.
	DefaultNamespace = "synacklab-tunnels"
	// DefaultImage is a socat image pinned by its multi-arch digest.
	DefaultImage = "alpine/socat:1.8.1.1@sha256:7f9a06753033f2b7de18edc2353f2c15153413d95a039163c6db270fc7a6c3b0"
	// DefaultMaxDuration is how long a tunnel pod may live before Kubernetes kills it.
	DefaultMaxDuration = 8 * time.Hour
)

// Target is the remote TCP endpoint the tunnel pod connects to.
type Target struct {
	Kind string // e.g. "postgres", "mysql": used for labels and the client hint
	Host string
	Port int
}

// Options controls where and how the tunnel pod runs.
type Options struct {
	Namespace    string
	Image        string
	MaxDuration  time.Duration
	NodeSelector map[string]string
	LocalPort    int
	User         string // local OS username; sanitized before use
	Out, Err     io.Writer
}

// Forwarder forwards 127.0.0.1:localPort to podPort on the named pod until ctx
// is done or the stream breaks. It closes ready once listening.
type Forwarder func(ctx context.Context, namespace, pod string, localPort, podPort int, ready chan struct{}) error

var (
	// retryInterval paces re-forwarding and retried API calls.
	retryInterval = time.Second
	// podCheckTimeout bounds how long a dropped forward waits for the API
	// server (e.g. restarting) to report the pod's state.
	podCheckTimeout = time.Minute
	// deleteTimeout bounds teardown retries.
	deleteTimeout = 10 * time.Second
	// podWatchInterval paces checks that the pod is still running while
	// forwarding; an idle forward does not notice the pod dying by itself.
	podWatchInterval = 5 * time.Second
)

// Open runs a tunnel pod for target and forwards opts.LocalPort to it. It
// blocks until ctx is cancelled (returning nil) or the tunnel fails, and
// always deletes the pod it created.
func Open(ctx context.Context, client corev1client.CoreV1Interface, forward Forwarder, target Target, opts Options) error {
	return withRBACHint(run(ctx, client.Pods(opts.Namespace), forward, target, opts), opts.Namespace)
}

func run(ctx context.Context, pods corev1client.PodInterface, forward Forwarder, target Target, opts Options) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if err := checkLocalPortFree(opts.LocalPort); err != nil {
		return err
	}

	pod, err := createPod(ctx, pods, target, opts)
	if err != nil {
		if ctx.Err() != nil && !apierrors.IsAlreadyExists(err) {
			// Cancelled mid-request: the API server may have created the pod anyway.
			deletePod(pods, pod.Name, opts)
			return nil
		}
		return fmt.Errorf("failed to create tunnel pod in namespace %q: %w", opts.Namespace, err)
	}
	defer deletePod(pods, pod.Name, opts)

	if err := waitReady(ctx, pods, pod.Name); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	runCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer stop()
	wg.Go(func() { streamLogs(runCtx, pods, pod.Name, opts) })

	connected := false
	for attempt := 0; ; attempt++ {
		ready := make(chan struct{})
		if attempt == 0 {
			wg.Go(func() {
				select {
				case <-ready:
				case <-runCtx.Done():
					// Still announce a forward that became ready just before shutdown.
					select {
					case <-ready:
					default:
						return
					}
				}
				printConnected(pod.Name, target, opts)
			})
		}

		forwardCtx, stopForward := context.WithCancel(ctx)
		wg.Go(func() { stopWhenPodStops(forwardCtx, stopForward, pods, pod.Name) })
		err := forward(forwardCtx, opts.Namespace, pod.Name, opts.LocalPort, podPort, ready)
		stopForward()
		if ctx.Err() != nil {
			return nil
		}
		select {
		case <-ready:
			connected = true
		default:
		}
		// Retrying cannot fix a denied request, nor a forward that never worked.
		if apierrors.IsForbidden(err) || !connected {
			return fmt.Errorf("port-forward to tunnel pod %s failed: %w", pod.Name, err)
		}

		current, getErr := getPodRetrying(ctx, pods, pod.Name)
		if ctx.Err() != nil {
			return nil
		}
		if apierrors.IsNotFound(getErr) {
			return fmt.Errorf("tunnel pod %s was deleted", pod.Name)
		}
		if getErr != nil {
			return fmt.Errorf("port-forward to tunnel pod %s ended (%v) and the pod could not be checked: %w", pod.Name, err, getErr)
		}
		if current.Status.Phase != corev1.PodRunning {
			return fmt.Errorf("tunnel pod %s stopped (%s)", pod.Name, describe(current))
		}
		_, _ = fmt.Fprintf(opts.Err, "⚠️  Connection to tunnel pod lost (%v); reconnecting...\n", err)

		select {
		case <-time.After(retryInterval):
		case <-ctx.Done():
			return nil
		}
	}
}

// createPod retries with a new random name suffix if the name is taken. On
// error it returns the pod it last tried to create.
func createPod(ctx context.Context, pods corev1client.PodInterface, target Target, opts Options) (*corev1.Pod, error) {
	for attempt := 1; ; attempt++ {
		pod := buildPod(target, opts, SanitizeUser(opts.User))
		created, err := pods.Create(ctx, pod, metav1.CreateOptions{})
		if err == nil {
			return created, nil
		}
		if !apierrors.IsAlreadyExists(err) || attempt == 3 {
			return pod, err
		}
	}
}

func checkLocalPortFree(port int) error {
	if err := ValidatePort(port); err != nil {
		return fmt.Errorf("local %w", err)
	}
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("local port %d is already in use; choose another with --local-port: %w", port, err)
	}
	return l.Close()
}

func printConnected(podName string, target Target, opts Options) {
	_, _ = fmt.Fprintf(opts.Out, "✓ Tunnel pod %s ready (namespace %s)\n", podName, opts.Namespace)
	_, _ = fmt.Fprintf(opts.Out, "→ %s %s is available at 127.0.0.1:%d\n",
		target.Kind, net.JoinHostPort(target.Host, strconv.Itoa(target.Port)), opts.LocalPort)
	if e, ok := engines[target.Kind]; ok {
		_, _ = fmt.Fprintf(opts.Out, "  "+e.clientHint+"\n", opts.LocalPort)
	}
	_, _ = fmt.Fprintln(opts.Out, "  Press Ctrl-C to close the tunnel")
}

// streamLogs copies socat's output (e.g. refused upstream connections) to
// opts.Err until ctx is done.
func streamLogs(ctx context.Context, pods corev1client.PodInterface, podName string, opts Options) {
	logs, err := pods.GetLogs(podName, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
	if err != nil {
		if ctx.Err() == nil {
			_, _ = fmt.Fprintf(opts.Err, "⚠️  Could not stream tunnel pod logs: %v\n", withRBACHint(err, opts.Namespace))
		}
		return
	}
	defer func() { _ = logs.Close() }()
	_, _ = io.Copy(opts.Err, logs)
}

// stopWhenPodStops calls stop once the pod leaves the Running phase (e.g.
// killed at max_duration) or is deleted. Other lookup errors are ignored;
// the next check retries.
func stopWhenPodStops(ctx context.Context, stop context.CancelFunc, pods corev1client.PodInterface, name string) {
	_ = wait.PollUntilContextCancel(ctx, podWatchInterval, false, func(ctx context.Context) (bool, error) {
		pod, err := pods.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return err == nil && pod.Status.Phase != corev1.PodRunning, nil
	})
	stop()
}

// getPodRetrying retries the lookup through transient API errors, such as
// the brief Forbidden responses of an API server whose RBAC cache is warming up.
// NotFound is final: the pod was deleted.
func getPodRetrying(ctx context.Context, pods corev1client.PodInterface, name string) (*corev1.Pod, error) {
	var pod *corev1.Pod
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, retryInterval, podCheckTimeout, true, func(ctx context.Context) (bool, error) {
		pod, lastErr = pods.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(lastErr) {
			return false, lastErr
		}
		return lastErr == nil, nil
	})
	if err != nil && lastErr != nil {
		return nil, lastErr
	}
	return pod, err
}

// deletePod uses its own context: by teardown the command's context is
// usually already cancelled. It retries through transient API errors.
func deletePod(pods corev1client.PodInterface, name string, opts Options) {
	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()
	var err error
	_ = wait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		err = pods.Delete(ctx, name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			err = nil
		}
		return err == nil, nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(opts.Err, "⚠️  Failed to delete tunnel pod %s in namespace %s: %v\n"+
			"   Kubernetes will remove it when max_duration (%s) expires.\n", name, opts.Namespace, withRBACHint(err, opts.Namespace), opts.MaxDuration)
	}
}

// withRBACHint points a denied user at the RBAC setup. err stays matchable
// with apierrors.IsForbidden.
func withRBACHint(err error, namespace string) error {
	if !apierrors.IsForbidden(err) {
		return err
	}
	return fmt.Errorf("%w\nyour Kubernetes user is not allowed to run tunnels in namespace %q; "+
		"ask a cluster admin to apply the RBAC setup in docs/tunnel.md", err, namespace)
}
