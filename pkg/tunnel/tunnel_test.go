package tunnel_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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
	}
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
	return func(ctx context.Context, namespace, pod string, localPort, podPort int, ready chan<- struct{}) error {
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
	client := fake.NewClientset()
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
	assert.Equal(t, []string{"TCP-LISTEN:8080,fork,reuseaddr", "TCP:orders.internal:5432"}, c.Args)
	assert.Equal(t, []corev1.ContainerPort{{ContainerPort: 8080, Protocol: corev1.ProtocolTCP}}, c.Ports)
	require.NotNil(t, c.ReadinessProbe)
	require.NotNil(t, c.ReadinessProbe.TCPSocket)
	assert.Equal(t, 8080, c.ReadinessProbe.TCPSocket.Port.IntValue())
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
	client := fake.NewClientset()
	ctx, cancel := context.WithCancel(t.Context())
	var calls []forwardCall

	target := tunnel.Target{Kind: "mysql", Host: "fd00::15", Port: 3306}
	require.NoError(t, tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), target, testOptions()))

	require.Len(t, calls, 1)
	assert.Equal(t, "TCP:[fd00::15]:3306", calls[0].podAtForward.Spec.Containers[0].Args[1])
}

func TestOpen_DeletesThePodWhenTheTunnelCloses(t *testing.T) {
	client := fake.NewClientset()
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
			client := fake.NewClientset()
			ctx, cancel := context.WithCancel(t.Context())
			var calls []forwardCall

			err := tunnel.Open(ctx, client, closeOnFirstForward(client, cancel, &calls), target, testOptions())
			assert.Error(t, err)
			assert.Empty(t, client.Actions())
		})
	}
}
