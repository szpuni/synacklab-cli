package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		// Keep a denied WebSocket upgrade's Forbidden status: client-go reports
		// SPDY failures as plain text. RBAC checks WebSocket as `get` and SPDY
		// as `create` on pods/portforward, so the SPDY fallback may still work.
		var denied error
		dialer = portforward.NewFallbackDialerForStreaming(websocketDialer, dialer, func(err error) bool {
			var upgradeErr *httpstream.UpgradeFailureError
			if errors.As(err, &upgradeErr) && apierrors.IsForbidden(upgradeErr.Cause) {
				denied = upgradeErr.Cause
			}
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})

		fw, err := portforward.NewOnAddressesForStreamingWithContext(ctx, dialer, []string{"127.0.0.1"},
			[]string{fmt.Sprintf("%d:%d", localPort, podPort)}, ready, io.Discard, errOut)
		if err != nil {
			return fmt.Errorf("failed to set up port-forward: %w", err)
		}
		err = fw.ForwardPorts()
		select {
		case <-ready: // connected, so any denial was overcome by the fallback
			return err
		default:
		}
		if err != nil && denied != nil {
			return fmt.Errorf("%w (SPDY fallback: %v)", denied, err)
		}
		return err
	}
}
