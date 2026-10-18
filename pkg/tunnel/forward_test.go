package tunnel_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"synacklab/pkg/tunnel"
)

// apiServerAnswering stands in for the API server's pods/portforward
// endpoint: GET is the WebSocket upgrade, POST the SPDY fallback.
func apiServerAnswering(t *testing.T, websocket, spdy int) (*rest.Config, corev1client.CoreV1Interface) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, verb := websocket, "get"
		if r.Method == http.MethodPost {
			code, verb = spdy, "create"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		status := metav1.Status{TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}, Status: metav1.StatusFailure, Code: int32(code)}
		if code == http.StatusForbidden {
			status.Reason = metav1.StatusReasonForbidden
			status.Message = `pods "tunnel-pod" is forbidden: User "jdoe" cannot ` + verb + ` resource "pods/portforward"`
		} else {
			status.Message = http.StatusText(code)
		}
		_ = json.NewEncoder(w).Encode(status)
	}))
	t.Cleanup(srv.Close)
	config := &rest.Config{Host: srv.URL}
	client, err := corev1client.NewForConfig(config)
	require.NoError(t, err)
	return config, client
}

func TestPortForwarder_ReportsADenialOnlyWhenTheConnectionWasDenied(t *testing.T) {
	for name, tc := range map[string]struct {
		websocket, spdy int
		wantForbidden   bool
		wantErr         string
	}{
		// The documented Role grants `create` only, so WebSocket (`get`) is
		// always denied; a later SPDY failure is not a denial.
		"SPDY fallback unavailable":                   {websocket: http.StatusForbidden, spdy: http.StatusServiceUnavailable, wantErr: "Service Unavailable"},
		"both denied":                                 {websocket: http.StatusForbidden, spdy: http.StatusForbidden, wantForbidden: true, wantErr: `cannot create resource "pods/portforward"`},
		"SPDY denied after another WebSocket failure": {websocket: http.StatusBadRequest, spdy: http.StatusForbidden, wantForbidden: true, wantErr: `cannot create resource "pods/portforward"`},
	} {
		t.Run(name, func(t *testing.T) {
			config, client := apiServerAnswering(t, tc.websocket, tc.spdy)
			forward := tunnel.PortForwarder(config, client, io.Discard)

			err := forward(t.Context(), "tunnels", "tunnel-pod", 0, 8080, make(chan struct{}))
			require.Error(t, err)
			assert.Equal(t, tc.wantForbidden, apierrors.IsForbidden(err), "got %v", err)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
