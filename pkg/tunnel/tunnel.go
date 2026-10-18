// Package tunnel opens TCP tunnels to private network targets through a
// short-lived socat pod and the Kubernetes port-forward API.
package tunnel

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
}

// Forwarder forwards 127.0.0.1:localPort to podPort on the named pod until ctx
// is done or the stream breaks. It closes ready once listening.
type Forwarder func(ctx context.Context, namespace, pod string, localPort, podPort int, ready chan<- struct{}) error

// Open runs a tunnel pod for target and forwards opts.LocalPort to it. It
// blocks until the tunnel ends and always deletes the pod it created.
func Open(ctx context.Context, client kubernetes.Interface, forward Forwarder, target Target, opts Options) error {
	if err := ValidateHost(target.Host); err != nil {
		return err
	}
	if err := ValidatePort(target.Port); err != nil {
		return err
	}

	pod, err := client.CoreV1().Pods(opts.Namespace).Create(ctx, buildPod(target, opts, SanitizeUser(opts.User)), metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tunnel pod in namespace %q: %w", opts.Namespace, err)
	}
	defer deletePod(client, opts.Namespace, pod.Name)

	if err := waitReady(ctx, client, opts.Namespace, pod.Name); err != nil {
		return err
	}

	return forward(ctx, opts.Namespace, pod.Name, opts.LocalPort, podPort, make(chan struct{}))
}

// deletePod uses its own context: by teardown the command's context is
// usually already cancelled.
func deletePod(client kubernetes.Interface, namespace, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
