package tunnel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/tunnel"
)

// fakeCluster stands in for the real tunnel: it prints like Open, reports
// ready, then holds the tunnel open until cancelled, or fails if failWith is set.
type fakeCluster struct {
	failWith error
	started  chan tunnel.Request
	stopped  chan struct{}
}

func newFakeCluster() *fakeCluster {
	return &fakeCluster{started: make(chan tunnel.Request, 10), stopped: make(chan struct{}, 10)}
}

func (f *fakeCluster) start(ctx context.Context, req tunnel.Request, _ tunnel.Target, out io.Writer, ready func(string)) error {
	f.started <- req
	defer func() { f.stopped <- struct{}{} }()
	_, _ = fmt.Fprintf(out, "✓ Tunnel pod synacklab-tunnel-x ready (namespace %s)\n", req.Namespace)
	if f.failWith != nil {
		return f.failWith
	}
	ready("synacklab-tunnel-x")
	<-ctx.Done()
	_, _ = fmt.Fprintln(out, "pod deleted")
	return nil
}

func resolveLikeTheCLI(req tunnel.Request) (tunnel.Target, error) {
	switch {
	case req.Name == "orders":
		return tunnel.Target{Kind: "postgres", Host: "orders.internal", Port: 5432}, nil
	case req.Host != "":
		return tunnel.Target{Kind: req.Engine, Host: req.Host, Port: 3306}, nil
	}
	return tunnel.Target{}, fmt.Errorf("unknown database %q; configured databases: orders", req.Name)
}

func startTunnelServer(t *testing.T, cluster *fakeCluster) (*httptest.Server, *tunnel.Server, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	srv := tunnel.NewServer(ctx, tunnel.ServerConfig{
		Databases: []tunnel.Database{{Name: "orders", Engine: "postgres", Host: "orders.internal", Port: 5432}},
		Contexts:  []string{"dev", "prod"},
		Defaults:  tunnel.Request{Context: "prod", Namespace: "tunnels"},
		Resolve:   resolveLikeTheCLI,
		Start:     cluster.start,
	})
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(func() { cancel(); srv.Wait(); ts.Close() })
	return ts, srv, cancel
}

func postTunnel(t *testing.T, ts *httptest.Server, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/tunnels", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

type tunnelView struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	Context   string `json:"context"`
	Target    string `json:"target"`
	LocalAddr string `json:"local_addr"`
	Hint      string `json:"hint"`
	Error     string `json:"error"`
}

func listTunnels(t *testing.T, ts *httptest.Server) []tunnelView {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/tunnels")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var out []tunnelView
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

func waitForState(t *testing.T, ts *httptest.Server, state string) tunnelView {
	t.Helper()
	var last []tunnelView
	require.Eventually(t, func() bool {
		last = listTunnels(t, ts)
		return len(last) == 1 && last[0].State == state
	}, 2*time.Second, 10*time.Millisecond, "want state %s, have %+v", state, last)
	return last[0]
}

func TestServer_OffersTheSameChoicesAsTheCLI(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())

	resp, err := http.Get(ts.URL + "/api/config")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var cfg struct {
		Databases []tunnel.Database `json:"databases"`
		Contexts  []string          `json:"contexts"`
		Engines   []struct {
			Name string `json:"name"`
			Port int    `json:"port"`
		} `json:"engines"`
		Defaults tunnel.Request `json:"defaults"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cfg))

	assert.Equal(t, "orders", cfg.Databases[0].Name)
	assert.Equal(t, []string{"dev", "prod"}, cfg.Contexts)
	assert.Len(t, cfg.Engines, 2)
	assert.Equal(t, "mysql", cfg.Engines[0].Name)
	assert.Equal(t, 3306, cfg.Engines[0].Port)
	assert.Equal(t, "prod", cfg.Defaults.Context)
	assert.Equal(t, "tunnels", cfg.Defaults.Namespace)
}

func TestServer_OpensATunnelAndShowsHowToConnect(t *testing.T) {
	cluster := newFakeCluster()
	ts, _, _ := startTunnelServer(t, cluster)

	code, body := postTunnel(t, ts, `{"name":"orders"}`)
	require.Equal(t, http.StatusCreated, code, body)

	req := <-cluster.started
	assert.Equal(t, 5432, req.LocalPort, "local port defaults to the database port")
	assert.Equal(t, "tunnels", req.Namespace, "namespace defaults like the CLI")
	assert.Equal(t, "prod", req.Context)

	tv := waitForState(t, ts, "connected")
	assert.Equal(t, body["id"], tv.ID)
	assert.Equal(t, "synacklab-tunnel-x", tv.Pod)
	assert.Equal(t, "tunnels", tv.Namespace)
	assert.Equal(t, "postgres orders.internal:5432", tv.Target)
	assert.Equal(t, "127.0.0.1:5432", tv.LocalAddr)
	assert.Equal(t, "psql -h 127.0.0.1 -p 5432 -U <user> <database>", tv.Hint)
}

func TestServer_RejectsWhatTheCLIWouldRejectBeforeStarting(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())

	code, body := postTunnel(t, ts, `{"name":"nope"}`)

	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, `unknown database "nope"; configured databases: orders`, body["error"])
	assert.Empty(t, listTunnels(t, ts))
}

func TestServer_RunsOneTunnelAtATime(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())
	code, _ := postTunnel(t, ts, `{"name":"orders"}`)
	require.Equal(t, http.StatusCreated, code)

	code, body := postTunnel(t, ts, `{"host":"db.internal","engine":"mysql","local_port":13306}`)

	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, body["error"], "already open")
}

func TestServer_ClosingATunnelStopsIt(t *testing.T) {
	cluster := newFakeCluster()
	ts, _, _ := startTunnelServer(t, cluster)
	_, body := postTunnel(t, ts, `{"name":"orders"}`)
	waitForState(t, ts, "connected")

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tunnels/"+body["id"].(string), nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	<-cluster.stopped
	waitForState(t, ts, "closed")

	code, _ := postTunnel(t, ts, `{"name":"orders"}`)
	assert.Equal(t, http.StatusCreated, code, "a closed tunnel frees the slot")
}

func TestServer_ClosingAnUnknownTunnelIsNotFound(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tunnels/nope", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_ShowsWhyATunnelFailed(t *testing.T) {
	cluster := newFakeCluster()
	cluster.failWith = errors.New("pods is forbidden\nask a cluster admin")
	ts, _, _ := startTunnelServer(t, cluster)
	postTunnel(t, ts, `{"name":"orders"}`)

	tv := waitForState(t, ts, "failed")
	assert.Equal(t, "pods is forbidden\nask a cluster admin", tv.Error)
}

func TestServer_StreamsOutputSoFarThenLiveUpdates(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())
	_, body := postTunnel(t, ts, `{"name":"orders"}`)
	id := body["id"].(string)
	waitForState(t, ts, "connected")

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/ws/tunnels/"+id, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var output bytes.Buffer
	var states []string
	read := func() {
		var ev struct {
			Type   string     `json:"type"`
			Text   string     `json:"text"`
			Tunnel tunnelView `json:"tunnel"`
		}
		require.NoError(t, conn.ReadJSON(&ev))
		if ev.Type == "output" {
			output.WriteString(ev.Text)
		} else {
			states = append(states, ev.Tunnel.State)
		}
	}
	read()
	read()
	assert.Contains(t, output.String(), "Tunnel pod synacklab-tunnel-x ready")
	assert.Equal(t, []string{"connected"}, states)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tunnels/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	for len(states) == 0 || states[len(states)-1] != "closed" {
		read()
	}
	assert.Contains(t, output.String(), "pod deleted")
}

func TestServer_WaitBlocksUntilTheTunnelHasCleanedUp(t *testing.T) {
	cluster := newFakeCluster()
	ts, srv, cancel := startTunnelServer(t, cluster)
	postTunnel(t, ts, `{"name":"orders"}`)
	waitForState(t, ts, "connected")

	cancel() // the server's own context: SIGINT
	srv.Wait()

	select {
	case <-cluster.stopped:
	default:
		t.Fatal("Wait returned before the tunnel finished")
	}
}

func TestServer_PageUsesTheKitAndRealEndpoints(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())
	get := func(path string) string {
		resp, err := http.Get(ts.URL + path)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	html := get("/")
	assert.Contains(t, html, `href="/ui/tokens.css"`)
	assert.Contains(t, html, `href="/ui/components.css"`)
	assert.Contains(t, html, `<script type="module" src="app.js">`)
	get("/app.css")
	get("/ui/components.js")
	js := get("/app.js")
	assert.Contains(t, js, `from "/ui/components.js"`)
	for _, endpoint := range []string{"/api/config", "/api/tunnels", "/ws/tunnels/"} {
		assert.Contains(t, js, endpoint)
	}
}

func TestServer_PageGetsEmptyListsNotNullWithoutContextsOrDatabases(t *testing.T) {
	srv := tunnel.NewServer(t.Context(), tunnel.ServerConfig{})
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	assert.Contains(t, rec.Body.String(), `"contexts":[]`)
	assert.Contains(t, rec.Body.String(), `"databases":[]`)
}

func TestServer_RejectsAnInvalidLocalPortBeforeStarting(t *testing.T) {
	ts, _, _ := startTunnelServer(t, newFakeCluster())

	code, body := postTunnel(t, ts, `{"name":"orders","local_port":70000}`)

	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "local invalid port 70000: must be between 1 and 65535", body["error"], "same message as tunnel db")
	assert.Empty(t, listTunnels(t, ts))
}
