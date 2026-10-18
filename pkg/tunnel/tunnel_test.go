package tunnel_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"synacklab/pkg/tunnel"
)

var pgTarget = tunnel.Target{Kind: "postgres", Host: "orders.internal", Port: 5432}

func testOptions() tunnel.Options {
	return tunnel.Options{
		Namespace:    "tunnels",
		Image:        "registry.corp/socat:1",
		MaxDuration:  2 * time.Hour,
		NodeSelector: map[string]string{"subnet": "app"},
		LocalPort:    15432,
		User:         "Wojtek",
		Out:          io.Discard,
		Err:          io.Discard,
	}
}

var runningAndReady = corev1.PodStatus{
	Phase:             corev1.PodRunning,
	ContainerStatuses: []corev1.ContainerStatus{{Name: "socat", Ready: true}},
}

// clusterWhere returns a fake cluster in which every created pod immediately
// reports status, standing in for the scheduler and kubelet.
func clusterWhere(status corev1.PodStatus) *fake.Clientset {
	client := fake.NewClientset()
	client.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).Status = status
		return false, nil, nil
	})
	return client
}

// forwardCall records what Open asked the forwarder to do, and the pod as it
// existed in the cluster at that moment.
type forwardCall struct {
	namespace, pod     string
	localPort, podPort int
	podAtForward       *corev1.Pod
}

// closeOnFirstForward is a Forwarder that records the call, then simulates
// the user pressing Ctrl-C.
func closeOnFirstForward(client *fake.Clientset, cancel context.CancelFunc, calls *[]forwardCall) tunnel.Forwarder {
	return func(ctx context.Context, namespace, pod string, localPort, podPort int, ready chan struct{}) error {
		got, err := client.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return err
		}
		*calls = append(*calls, forwardCall{namespace, pod, localPort, podPort, got})
		close(ready)
		cancel()
		return nil
	}
}

func TestOpen_RunsALockedDownSocatPodPointedAtTheTarget(t *testing.T) {
	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	err := tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions())
	require.NoError(t, err)
	require.Len(t, calls, 1)

	call := calls[0]
	assert.Equal(t, "tunnels", call.namespace)
	assert.Equal(t, 15432, call.localPort)
	assert.Equal(t, 8080, call.podPort)

	pod := call.podAtForward
	assert.Equal(t, call.pod, pod.Name)
	assert.True(t, strings.HasPrefix(pod.Name, "synacklab-tunnel-wojtek-"), pod.Name)
	assert.Len(t, strings.TrimPrefix(pod.Name, "synacklab-tunnel-wojtek-"), 5)
	assert.Equal(t, map[string]string{
		"app.kubernetes.io/managed-by": "synacklab",
		"synacklab.io/user":            "wojtek",
		"synacklab.io/target":          "postgres",
	}, pod.Labels)

	spec := pod.Spec
	assert.Equal(t, corev1.RestartPolicyNever, spec.RestartPolicy)
	assert.Equal(t, new(false), spec.AutomountServiceAccountToken)
	assert.Equal(t, new(int64(7200)), spec.ActiveDeadlineSeconds)
	assert.Equal(t, map[string]string{"subnet": "app"}, spec.NodeSelector)
	assert.Equal(t, &corev1.PodSecurityContext{
		RunAsNonRoot:   new(true),
		RunAsUser:      new(int64(65534)),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}, spec.SecurityContext)

	require.Len(t, spec.Containers, 1)
	c := spec.Containers[0]
	assert.Equal(t, "registry.corp/socat:1", c.Image)
	assert.Empty(t, c.Command, "image entrypoint is socat; no shell")
	assert.Equal(t, []string{"TCP-LISTEN:8080,fork,reuseaddr", "TCP:orders.internal:5432,connect-timeout=10"}, c.Args)
	assert.Equal(t, []corev1.ContainerPort{{ContainerPort: 8080, Protocol: corev1.ProtocolTCP}}, c.Ports)
	// A probe would make socat open a real database connection every period.
	assert.Nil(t, c.ReadinessProbe)
	assert.Nil(t, c.LivenessProbe)
	assert.Nil(t, c.StartupProbe)
	assert.Equal(t, &corev1.SecurityContext{
		AllowPrivilegeEscalation: new(false),
		ReadOnlyRootFilesystem:   new(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}, c.SecurityContext)
	assert.Equal(t, corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("16Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
	}, c.Resources)
}

func TestOpen_BracketsIPv6TargetsForSocat(t *testing.T) {
	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	target := tunnel.Target{Kind: "mysql", Host: "fd00::15", Port: 3306}
	require.NoError(t, tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), target, testOptions()))

	require.Len(t, calls, 1)
	assert.Equal(t, "TCP:[fd00::15]:3306,connect-timeout=10", calls[0].podAtForward.Spec.Containers[0].Args[1])
}

func TestOpen_DeletesThePodWhenTheTunnelCloses(t *testing.T) {
	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	require.NoError(t, tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions()))

	pods, err := client.CoreV1().Pods("tunnels").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, pods.Items)
}

func TestOpen_RejectsAnInvalidTargetBeforeTouchingTheCluster(t *testing.T) {
	for name, target := range map[string]tunnel.Target{
		"socat option as host": {Kind: "postgres", Host: "-dd", Port: 5432},
		"port out of range":    {Kind: "postgres", Host: "db", Port: 0},
	} {
		t.Run(name, func(t *testing.T) {
			client := clusterWhere(runningAndReady)
			ctx, cancel := context.WithCancel(t.Context())
			var calls []forwardCall

			err := tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), target, testOptions())
			assert.Error(t, err)
			assert.Empty(t, client.Actions())
		})
	}
}

func TestOpen_ReportsWhyThePodCouldNotStartAndCleansUp(t *testing.T) {
	waiting := func(reason, message string) corev1.PodStatus {
		return corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "socat",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message}},
		}}}
	}
	for name, tc := range map[string]struct {
		status  corev1.PodStatus
		wantErr []string
	}{
		"image cannot be pulled": {
			status:  waiting("ImagePullBackOff", `Back-off pulling image "registry.corp/socat:1"`),
			wantErr: []string{"ImagePullBackOff", `Back-off pulling image "registry.corp/socat:1"`},
		},
		"no node matches the node selector": {
			status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
				Reason: corev1.PodReasonUnschedulable, Message: "0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector.",
			}}},
			wantErr: []string{"Unschedulable", "didn't match Pod's node affinity/selector"},
		},
		"socat exits": {
			status: corev1.PodStatus{Phase: corev1.PodFailed, ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "socat",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1}},
			}}},
			wantErr: []string{"Error", "exit code 1"},
		},
		"never ready": {
			status:  corev1.PodStatus{Phase: corev1.PodPending},
			wantErr: []string{"not ready after", "Pending"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			restore := *tunnel.ReadyTimeout
			*tunnel.ReadyTimeout = 50 * time.Millisecond
			t.Cleanup(func() { *tunnel.ReadyTimeout = restore })

			client := clusterWhere(tc.status)
			ctx, cancel := context.WithCancel(t.Context())
			var calls []forwardCall

			err := tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions())
			require.Error(t, err)
			for _, want := range tc.wantErr {
				assert.ErrorContains(t, err, want)
			}
			assert.Empty(t, calls, "must not forward to a pod that is not ready")

			pods, listErr := client.CoreV1().Pods("tunnels").List(t.Context(), metav1.ListOptions{})
			require.NoError(t, listErr)
			assert.Empty(t, pods.Items)
		})
	}
}

// syncBuffer is a bytes.Buffer safe for Open's concurrent writers.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestOpen_RefusesABusyLocalPortBeforeCreatingAPod(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall
	opts := testOptions()
	opts.LocalPort = busy.Addr().(*net.TCPAddr).Port

	err = tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), pgTarget, opts)
	assert.ErrorContains(t, err, "already in use")
	assert.ErrorContains(t, err, "--local-port")
	assert.Empty(t, client.Actions())
}

func TestOpen_TellsTheUserHowToConnectOnceForwarding(t *testing.T) {
	for kind, hint := range map[string]string{
		"postgres": "psql -h 127.0.0.1 -p 15432",
		"mysql":    "mysql -h 127.0.0.1 -P 15432",
	} {
		t.Run(kind, func(t *testing.T) {
			client := clusterWhere(runningAndReady)
			ctx, cancel := context.WithCancel(t.Context())
			var calls []forwardCall
			out := &syncBuffer{}
			opts := testOptions()
			opts.Out = out

			target := tunnel.Target{Kind: kind, Host: "orders.internal", Port: 5432}
			require.NoError(t, tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), target, opts))

			got := out.String()
			assert.Contains(t, got, calls[0].pod)
			assert.Contains(t, got, "tunnels")
			assert.Contains(t, got, kind+" orders.internal:5432")
			assert.Contains(t, got, "127.0.0.1:15432")
			assert.Contains(t, got, hint)
			assert.Contains(t, got, "Ctrl-C")
		})
	}
}

func TestOpen_StreamsPodLogsToStderr(t *testing.T) {
	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())
	errOut := &syncBuffer{}
	opts := testOptions()
	opts.Err = errOut

	forward := func(_ context.Context, _, _ string, _, _ int, ready chan struct{}) error {
		close(ready)
		assert.Eventually(t, func() bool { return strings.Contains(errOut.String(), "fake logs") }, time.Second, 10*time.Millisecond)
		cancel()
		return nil
	}
	require.NoError(t, tunnel.Open(ctx, client, forward, pgTarget, opts))
}

func TestOpen_KeepsForbiddenErrorsRecognisable(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("create", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(corev1.Resource("pods"), "", errors.New(`User "jdoe" cannot create resource "pods"`))
	})
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	err := tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions())
	assert.True(t, apierrors.IsForbidden(err), "got %v", err)
}
