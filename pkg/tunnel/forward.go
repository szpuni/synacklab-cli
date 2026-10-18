package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"

	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/streaming/pkg/httpstream"
)

// PortForwarder returns the real Forwarder: the Kubernetes pods/portforward
// API over WebSockets, falling back to SPDY like kubectl. It listens on
// 127.0.0.1 only and writes per-connection errors to errOut.
func PortForwarder(config *rest.Config, client corev1client.CoreV1Interface, errOut io.Writer) Forwarder {
	return func(ctx context.Context, namespace, pod string, localPort, podPort int, ready chan struct{}) error {
		url := client.RESTClient().Post().
			Resource("pods").Namespace(namespace).Name(pod).SubResource("portforward").URL()

		transport, upgrader, err := spdy.RoundTripperFor(config)
		if err != nil {
			return fmt.Errorf("failed to build port-forward transport: %w", err)
		}
		dialer := spdy.NewDialerForStreaming(upgrader, &http.Client{Transport: transport}, http.MethodPost, url)
		websocketDialer, err := portforward.NewSPDYOverWebsocketDialerForStreaming(url, config)
		if err != nil {
			return fmt.Errorf("failed to build port-forward dialer: %w", err)
		}
		dialer = portforward.NewFallbackDialerForStreaming(websocketDialer, dialer, func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})

		fw, err := portforward.NewOnAddressesForStreamingWithContext(ctx, dialer, []string{"127.0.0.1"},
			[]string{fmt.Sprintf("%d:%d", localPort, podPort)}, ready, io.Discard, errOut)
		if err != nil {
			return fmt.Errorf("failed to set up port-forward: %w", err)
		}
		return fw.ForwardPorts()
	}
}
