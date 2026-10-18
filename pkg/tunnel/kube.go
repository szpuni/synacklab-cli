package tunnel

import (
	"fmt"

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
