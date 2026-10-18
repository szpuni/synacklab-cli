package tunnel_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"synacklab/pkg/tunnel"
)

// Tests for what the user sees when their Kubernetes RBAC denies a step.

func forbidden(resource, verb string) error {
	return apierrors.NewForbidden(corev1.Resource(resource), "",
		fmt.Errorf(`User "jdoe" cannot %s resource %q in API group "" in the namespace "tunnels"`, verb, resource))
}

func TestOpen_PointsToTheRBACSetupWhenPodCreationIsDenied(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("create", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, forbidden("pods", "create")
	})
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	err := tunnel.Open(ctx, client.CoreV1(), closeOnFirstForward(client, cancel, &calls), pgTarget, testOptions())
	assert.True(t, apierrors.IsForbidden(err), "got %v", err)
	assert.ErrorContains(t, err, `cannot create resource "pods"`)
	assert.ErrorContains(t, err, "docs/tunnel.md")
}

func TestOpen_FailsInsteadOfReconnectingWhenPortForwardIsDenied(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)

	forwards := 0
	forward := func(context.Context, string, string, int, int, chan struct{}) error {
		forwards++
		return fmt.Errorf("port-forward: %w", forbidden("pods/portforward", "create"))
	}

	err := tunnel.Open(t.Context(), client.CoreV1(), forward, pgTarget, testOptions())
	assert.Equal(t, 1, forwards)
	assert.ErrorContains(t, err, `cannot create resource "pods/portforward"`)
	assert.ErrorContains(t, err, "docs/tunnel.md")
}

func TestOpen_FailsWhenTheFirstForwardNeverConnects(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)

	forwards := 0
	forward := func(context.Context, string, string, int, int, chan struct{}) error {
		forwards++
		return errors.New("error upgrading connection: unable to upgrade connection: 403 Forbidden")
	}

	err := tunnel.Open(t.Context(), client.CoreV1(), forward, pgTarget, testOptions())
	assert.Equal(t, 1, forwards, "nothing was ever connected, so there is nothing to reconnect")
	assert.ErrorContains(t, err, "unable to upgrade connection")
}

func TestOpen_PointsToTheRBACSetupWhenLogsOrDeleteAreDenied(t *testing.T) {
	fastRetries(t)
	client := clusterWhere(runningAndReady)
	client.PrependReactor("get", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "log" {
			return true, nil, forbidden("pods/log", "get")
		}
		return false, nil, nil
	})
	client.PrependReactor("delete", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, forbidden("pods", "delete")
	})
	ctx, cancel := context.WithCancel(t.Context())
	errOut := &syncBuffer{}
	opts := testOptions()
	opts.Err = errOut

	forward := func(_ context.Context, _, _ string, _, _ int, ready chan struct{}) error {
		close(ready)
		assert.Eventually(t, func() bool { return strings.Contains(errOut.String(), "pods/log") }, time.Second, 10*time.Millisecond)
		cancel()
		return nil
	}
	require.NoError(t, tunnel.Open(ctx, client.CoreV1(), forward, pgTarget, opts))

	got := errOut.String()
	assert.Contains(t, got, `cannot get resource "pods/log"`)
	assert.Contains(t, got, `cannot delete resource "pods"`)
	assert.Equal(t, 2, strings.Count(got, "docs/tunnel.md"), got)
}
