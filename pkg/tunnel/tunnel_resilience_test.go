package tunnel_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"synacklab/pkg/tunnel"
)

// Tests for how Open survives and ends: dropped forwards, API server
// outages, the pod stopping, Ctrl-C, and teardown.

// fastRetries shrinks retry/watch pacing and the teardown deadline for one test.
func fastRetries(t *testing.T) {
	t.Helper()
	interval, deleteTimeout, watch := *tunnel.RetryInterval, *tunnel.DeleteTimeout, *tunnel.PodWatchInterval
	*tunnel.RetryInterval, *tunnel.DeleteTimeout, *tunnel.PodWatchInterval = time.Millisecond, 50*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() {
		*tunnel.RetryInterval, *tunnel.DeleteTimeout, *tunnel.PodWatchInterval = interval, deleteTimeout, watch
	})
}

func TestOpen_ReconnectsWhileThePodIsStillRunning(t *testing.T) {
	fastRetries(t)

	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())
	errOut := &syncBuffer{}
	opts := testOptions()
	opts.Err = errOut

	var ports []int
	forward := func(_ context.Context, _, _ string, localPort, _ int, ready chan struct{}) error {
		ports = append(ports, localPort)
		close(ready)
		if len(ports) == 1 {
			return errors.New("lost connection to pod")
		}
		cancel()
		return nil
	}

	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), forward, pgTarget, opts))
	assert.Equal(t, []int{15432, 15432}, ports)
	assert.Contains(t, errOut.String(), "reconnecting")
}

func TestOpen_FailsWithThePodReasonWhenThePodStops(t *testing.T) {
	client := clusterWhere(runningAndReady)
	forward := func(ctx context.Context, namespace, pod string, _, _ int, ready chan struct{}) error {
		close(ready)
		// Kubernetes enforces activeDeadlineSeconds.
		p, err := client.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		require.NoError(t, err)
		p.Status = corev1.PodStatus{Phase: corev1.PodFailed, Reason: "DeadlineExceeded", Message: "Pod was active on the node longer than the specified deadline"}
		_, err = client.CoreV1().Pods(namespace).UpdateStatus(ctx, p, metav1.UpdateOptions{})
		require.NoError(t, err)
		return errors.New("lost connection to pod")
	}

	err := tunnel.Open(t.Context(), client.CoreV1(), forward, pgTarget, testOptions())
	assert.ErrorContains(t, err, "DeadlineExceeded")
}

func TestOpen_WarnsWhenThePodCannotBeDeleted(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)
	client.PrependReactor("delete", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall
	errOut := &syncBuffer{}
	opts := testOptions()
	opts.Err = errOut

	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), closeOnFirstForward(client, cancel, &calls), pgTarget, opts))
	assert.Contains(t, errOut.String(), calls[0].pod)
	assert.Contains(t, errOut.String(), "connection refused")
	assert.Contains(t, errOut.String(), "max_duration")
}

func TestOpen_ReconnectsThroughABriefAPIServerOutage(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)
	ctx, cancel := context.WithCancel(t.Context())

	// After the first forward drops, the API server answers the next two pod
	// lookups with errors, as it does while restarting.
	var dropped bool
	failures := 2
	client.PrependReactor("get", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "" { // pod logs, streamed concurrently
			return false, nil, nil
		}
		if dropped && failures > 0 {
			failures--
			return true, nil, apierrors.NewForbidden(corev1.Resource("pods"), "", errors.New("RBAC: not ready"))
		}
		return false, nil, nil
	})

	forwards := 0
	forward := func(_ context.Context, _, _ string, _, _ int, ready chan struct{}) error {
		forwards++
		close(ready)
		if forwards == 1 {
			dropped = true
			return errors.New("lost connection to pod")
		}
		cancel()
		return nil
	}

	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), forward, pgTarget, testOptions()))
	assert.Equal(t, 2, forwards)
	assert.Zero(t, failures)
}

func TestOpen_RetriesDeletingThePodThroughABriefAPIServerOutage(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)
	failures := 2
	client.PrependReactor("delete", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		if failures > 0 {
			failures--
			return true, nil, errors.New("connection refused")
		}
		return false, nil, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall
	errOut := &syncBuffer{}
	opts := testOptions()
	opts.Err = errOut

	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), closeOnFirstForward(client, cancel, &calls), pgTarget, opts))

	pods, err := client.CoreV1().Pods("tunnels").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, pods.Items)
	assert.Empty(t, errOut.String())
}

func TestOpen_NoticesThePodStoppingWhileTheTunnelIsIdle(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)

	// Like the real forwarder, block until stopped: an idle forward never
	// sees the pod die on its own.
	forward := func(ctx context.Context, namespace, pod string, _, _ int, ready chan struct{}) error {
		close(ready)
		p, err := client.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		require.NoError(t, err)
		p.Status = corev1.PodStatus{Phase: corev1.PodFailed, Reason: "DeadlineExceeded"}
		_, err = client.CoreV1().Pods(namespace).UpdateStatus(ctx, p, metav1.UpdateOptions{})
		require.NoError(t, err)
		<-ctx.Done()
		return nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := tunnel.Open(ctx, client.CoreV1(), forward, pgTarget, testOptions())
	assert.ErrorContains(t, err, "DeadlineExceeded")
}

func TestOpen_CtrlCWhileWaitingForThePodExitsCleanly(t *testing.T) {
	client := clusterWhere(corev1.PodStatus{Phase: corev1.PodPending})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	var calls []forwardCall

	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions()))
	assert.Empty(t, calls)

	pods, err := client.CoreV1().Pods("tunnels").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, pods.Items)
}

func TestOpen_NoticesThePodBeingDeletedWhileTheTunnelIsIdle(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)

	forward := func(ctx context.Context, namespace, pod string, _, _ int, ready chan struct{}) error {
		close(ready)
		require.NoError(t, client.CoreV1().Pods(namespace).Delete(ctx, pod, metav1.DeleteOptions{}))
		<-ctx.Done()
		return nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := tunnel.Open(ctx, client.CoreV1(), forward, pgTarget, testOptions())
	assert.ErrorContains(t, err, "was deleted")
}

func TestOpen_RetriesWithAnotherNameWhenThePodNameIsTaken(t *testing.T) {
	client := clusterWhere(runningAndReady)
	collisions := 1
	client.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if collisions > 0 {
			collisions--
			name := action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).Name
			return true, nil, apierrors.NewAlreadyExists(corev1.Resource("pods"), name)
		}
		return false, nil, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions()))
	assert.Len(t, calls, 1)
}
