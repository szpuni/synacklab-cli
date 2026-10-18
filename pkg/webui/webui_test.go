package webui

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rblog "synacklab/pkg/log"
)

func TestValidateBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":   true,
		"127.0.0.2":   true,
		"::1":         true,
		"localhost":   true,
		"0.0.0.0":     false,
		"::":          false,
		"192.168.1.5": false,
		"example.com": false,
	}
	for bind, ok := range cases {
		err := ValidateBind(bind)
		if ok {
			assert.NoError(t, err, "bind=%s", bind)
		} else {
			assert.ErrorContains(t, err, "local-only", "bind=%s", bind)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startServe runs Serve on a loopback port with a handler answering 200 "ok",
// returning the listener address and the log output.
func startServe(t *testing.T) (string, *syncBuffer) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	go func() {
		_ = Serve(ctx, ln, h, rblog.New(logs, rblog.LevelInfo))
		close(done)
	}()
	t.Cleanup(func() { cancel(); <-done })
	return ln.Addr().String(), logs
}

func get(t *testing.T, addr, host string, header http.Header) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/x", nil)
	require.NoError(t, err)
	req.Host = host
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestServe_AllowsOnlyLocalHostsAndOrigins(t *testing.T) {
	addr, logs := startServe(t)
	_, port, _ := net.SplitHostPort(addr)
	wsUpgrade := http.Header{"Connection": {"Upgrade"}, "Upgrade": {"websocket"}}

	cases := []struct {
		name   string
		host   string
		header http.Header
		want   int
	}{
		{"listener address", addr, nil, http.StatusOK},
		{"localhost", "localhost:" + port, nil, http.StatusOK},
		{"same origin", addr, http.Header{"Origin": {"http://" + addr}}, http.StatusOK},
		{"localhost origin", addr, http.Header{"Origin": {"http://localhost:" + port}}, http.StatusOK},
		{"rebinding host", "evil.example:" + port, nil, http.StatusForbidden},
		{"other port", "localhost:1", nil, http.StatusForbidden},
		{"foreign origin", addr, http.Header{"Origin": {"https://evil.example"}}, http.StatusForbidden},
		{"foreign origin on websocket", addr, http.Header{
			"Origin": {"https://evil.example"}, "Connection": wsUpgrade["Connection"], "Upgrade": wsUpgrade["Upgrade"],
		}, http.StatusForbidden},
		{"null origin", addr, http.Header{"Origin": {"null"}}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, get(t, addr, tc.host, tc.header))
		})
	}
	assert.Contains(t, logs.String(), "WARN")
	assert.Contains(t, logs.String(), "evil.example")
}

func TestServe_LogsRequestsAtInfo(t *testing.T) {
	addr, logs := startServe(t)
	get(t, addr, addr, nil)
	out := logs.String()
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "GET /x 200")
}

func TestServe_ShutsDownGracefullyWhenContextCanceled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- Serve(ctx, ln, http.NewServeMux(), rblog.Discard()) }()

	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}
}

func TestServe_ServesUntilCanceled(t *testing.T) {
	addr, _ := startServe(t)
	resp, err := http.Get("http://" + addr + "/x")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
}
