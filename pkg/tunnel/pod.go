package tunnel

import (
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// podPort is where socat listens inside the pod. It is fixed and unprivileged
// whatever the target port, so the pod never needs NET_BIND_SERVICE.
const podPort = 8080

// nobody is the UID socat runs as; the default image would otherwise run as root.
const nobody = 65534

// buildPod returns the tunnel pod for target. The name is chosen here rather
// than via generateName so callers know it before the create call returns.
func buildPod(target Target, opts Options, user string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "synacklab-tunnel-" + user + "-" + utilrand.String(5),
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "synacklab",
				"synacklab.io/user":            user,
				"synacklab.io/target":          target.Kind,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			AutomountServiceAccountToken: new(false),
			ActiveDeadlineSeconds:        new(int64(opts.MaxDuration.Seconds())),
			NodeSelector:                 opts.NodeSelector,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   new(true),
				RunAsUser:      new(int64(nobody)),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:  "socat",
				Image: opts.Image,
				Args: []string{
					"TCP-LISTEN:" + strconv.Itoa(podPort) + ",fork,reuseaddr",
					"TCP:" + net.JoinHostPort(target.Host, strconv.Itoa(target.Port)),
				},
				Ports: []corev1.ContainerPort{{ContainerPort: podPort, Protocol: corev1.ProtocolTCP}},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler:  corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(podPort)}},
					PeriodSeconds: 2,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("16Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: new(false),
					ReadOnlyRootFilesystem:   new(true),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}
}
