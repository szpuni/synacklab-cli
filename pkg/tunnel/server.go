package tunnel

import (
	"cmp"
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"io"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"

	"synacklab/pkg/webui"
)

//go:embed web
var webFS embed.FS

// Request is what the page asks for, mirroring `tunnel db`'s choices: a
// configured database by Name, or an ad hoc Host/Engine/Port.
type Request struct {
	Name      string `json:"name,omitempty"`
	Host      string `json:"host,omitempty"`
	Engine    string `json:"engine,omitempty"`
	Port      int    `json:"port,omitempty"`
	LocalPort int    `json:"local_port,omitempty"` // 0 = the database port
	Context   string `json:"context,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// Database is a configured database as the page lists it.
type Database struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

// ServerConfig wires the server to the CLI's own config and target rules.
type ServerConfig struct {
	Databases   []Database
	DatabaseErr string // set when tunnel.databases is invalid; ad hoc targets still work
	Contexts    []string
	Defaults    Request // Context and Namespace preselected in the form
	// Resolve validates req and resolves its target exactly as `tunnel db`
	// does, so its errors are the CLI's.
	Resolve func(req Request) (Target, error)
	// Start runs the tunnel until ctx is done, like Open: out receives what
	// the CLI prints, ready is Options.Ready.
	Start func(ctx context.Context, req Request, target Target, out io.Writer, ready func(pod string)) error
}

// maxActive is how many tunnels may run at once. ponytail: one, like the
// CLI; raising it is all that running several needs on the server side.
const maxActive = 1

// Server is the HTTP API behind `tunnel serve`.
type Server struct {
	cfg      ServerConfig
	baseCtx  context.Context // canceling it closes every tunnel
	upgrader websocket.Upgrader
	wg       sync.WaitGroup

	mu      sync.Mutex
	tunnels map[string]*tunnelRun
}

// NewServer creates a Server whose tunnels all close when ctx is canceled.
func NewServer(ctx context.Context, cfg ServerConfig) *Server {
	return &Server{
		cfg: cfg, baseCtx: ctx, tunnels: make(map[string]*tunnelRun),
		// webui.Serve has already rejected foreign origins.
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// Wait blocks until every tunnel has finished, including deleting its pod.
func (s *Server) Wait() {
	s.wg.Wait()
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("GET /api/tunnels", s.handleListTunnels)
	mux.HandleFunc("POST /api/tunnels", s.handleOpenTunnel)
	mux.HandleFunc("DELETE /api/tunnels/{id}", s.handleCloseTunnel)
	mux.HandleFunc("GET /ws/tunnels/{id}", s.handleWatchTunnel)
	mux.Handle("GET /ui/", http.StripPrefix("/ui", webui.Assets()))
	mux.Handle("GET /", webui.FileServer(webFS, "web"))
	return mux
}

type engineView struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	var engineList []engineView
	for _, name := range slices.Sorted(maps.Keys(engines)) {
		engineList = append(engineList, engineView{name, engines[name].port})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"databases":      orEmpty(s.cfg.Databases),
		"database_error": s.cfg.DatabaseErr,
		"contexts":       orEmpty(s.cfg.Contexts),
		"engines":        engineList,
		"defaults":       s.cfg.Defaults,
	})
}

func (s *Server) handleListTunnels(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	views := []tunnelView{}
	for _, run := range s.tunnels {
		views = append(views, run.view())
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleOpenTunnel(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	target, err := s.cfg.Resolve(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.LocalPort = cmp.Or(req.LocalPort, target.Port)
	if err := ValidatePort(req.LocalPort); err != nil {
		writeError(w, http.StatusBadRequest, "local "+err.Error())
		return
	}
	req.Context = cmp.Or(req.Context, s.cfg.Defaults.Context)
	req.Namespace = cmp.Or(req.Namespace, s.cfg.Defaults.Namespace)

	s.mu.Lock()
	active := 0
	for id, run := range s.tunnels {
		if run.finished() {
			delete(s.tunnels, id) // the page has shown its result; free the slot
		} else {
			active++
		}
	}
	if active >= maxActive {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "a tunnel is already open; close it first")
		return
	}
	ctx, cancel := context.WithCancel(s.baseCtx)
	run := newTunnelRun(rand.Text(), req, target, cancel)
	s.tunnels[run.id] = run
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		defer cancel()
		run.finish(s.cfg.Start(ctx, req, target, run, run.connected))
	}()
	writeJSON(w, http.StatusCreated, map[string]string{"id": run.id})
}

func (s *Server) handleCloseTunnel(w http.ResponseWriter, r *http.Request) {
	run, ok := s.tunnel(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "unknown tunnel "+r.PathValue("id"))
		return
	}
	run.close()
	writeJSON(w, http.StatusAccepted, run.view())
}

// handleWatchTunnel sends the tunnel's output so far and its state, then
// each change, until the tunnel ends or the page goes away.
func (s *Server) handleWatchTunnel(w http.ResponseWriter, r *http.Request) {
	run, ok := s.tunnel(r.PathValue("id"))
	if !ok {
		http.Error(w, "unknown tunnel "+r.PathValue("id"), http.StatusNotFound)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	sent, lastState := 0, tunnelState("")
	for {
		output, view, changed := run.since(sent)
		sent += len(output)
		if output != "" {
			if conn.WriteJSON(map[string]string{"type": "output", "text": output}) != nil {
				return
			}
		}
		if view.State != lastState {
			if conn.WriteJSON(map[string]any{"type": "state", "tunnel": view}) != nil {
				return
			}
			lastState = view.State
		}
		if view.State.done() {
			return
		}
		select {
		case <-changed:
		case <-gone:
			return
		}
	}
}

func (s *Server) tunnel(id string) (*tunnelRun, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.tunnels[id]
	return run, ok
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// orEmpty makes a nil slice encode as JSON [] rather than null.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func localAddr(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}
