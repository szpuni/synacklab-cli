package tunnel

import (
	"fmt"
	"maps"
	"slices"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient connects using standard kubeconfig loading rules ($KUBECONFIG,
// else ~/.kube/config, exec credential plugins included). An empty
// contextName uses the kubeconfig's current context. Only the core/v1 client
// is built: the full clientset adds ~9 MB of unused API groups to the binary.
func NewClient(contextName string) (*corev1client.CoreV1Client, *rest.Config, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	client, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return client, config, nil
}

// Contexts lists the kubeconfig's context names, sorted, and its current
// context, using the same loading rules as NewClient.
func Contexts() (names []string, current string, err error) {
	raw, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	return slices.Sorted(maps.Keys(raw.Contexts)), raw.CurrentContext, nil
}
