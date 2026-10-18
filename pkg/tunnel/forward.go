package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/streaming/pkg/httpstream"
)

// PortForwarder returns the real Forwarder: the Kubernetes pods/portforward
// API over WebSockets, falling back to SPDY like kubectl. It listens on
// 127.0.0.1 only and writes per-connection errors to errOut.
//
// RBAC checks the WebSocket upgrade as `get` and SPDY as `create`, so a denied
// WebSocket upgrade still falls back. The outcome is SPDY's: only its denial
// is reported as Forbidden.
func PortForwarder(config *rest.Config, client corev1client.CoreV1Interface, errOut io.Writer) Forwarder {
	return func(ctx context.Context, namespace, pod string, localPort, podPort int, ready chan struct{}) error {
		url := client.RESTClient().Post().
			Resource("pods").Namespace(namespace).Name(pod).SubResource("portforward").URL()

		transport, upgrader, err := spdy.RoundTripperFor(config)
		if err != nil {
			return fmt.Errorf("failed to build port-forward transport: %w", err)
		}
		spdyTransport := &deniedTransport{RoundTripper: transport}
		dialer := spdy.NewDialerForStreaming(upgrader, &http.Client{Transport: spdyTransport}, http.MethodPost, url)
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
		if err := fw.ForwardPorts(); spdyTransport.denied == nil {
			return err
		}
		return spdyTransport.denied
	}
}

// deniedTransport records a Forbidden SPDY upgrade response as a StatusError:
// client-go reports upgrade failures as plain text, losing the status.
type deniedTransport struct {
	http.RoundTripper
	denied error
}

func (t *deniedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusForbidden {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	var status metav1.Status
	if err == nil && json.Unmarshal(body, &status) == nil && status.Reason == metav1.StatusReasonForbidden {
		t.denied = &apierrors.StatusError{ErrStatus: status}
	}
	return resp, nil
}
