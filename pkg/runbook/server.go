package runbook

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	rblog "synacklab/pkg/log"
)

//go:embed web
var webFS embed.FS

// webRoot strips the "web" prefix so the embedded files serve at "/", "/app.js", etc.
func webRoot() http.FileSystem {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // web/ is embedded at build time; a missing dir is a build-time bug, not a runtime one
	}
	return http.FS(sub)
}

// Server wires the REST/WebSocket API around the active Session. One Server
// per running `serve` process (Requirement 14.1).
type Server struct {
	active   *activeRunbook
	opts     SessionOptions // how a runbook opened via /api/open is set up
	upgrader websocket.Upgrader
	root     string // "" disables the file-browser endpoints (/api/files, /api/open)
	logger   *rblog.Logger
	baseCtx  context.Context // parent for every step execution; canceling it kills any in-flight step
}

// NewServer creates a Server around sess, which may be nil (nothing open
// yet) — pair with EnableWorkspace so the frontend's file browser can open
// one. opts sets up every runbook opened later via /api/open.
func NewServer(sess *Session, opts SessionOptions) *Server {
	return &Server{
		active: newActiveRunbook(sess), opts: opts,
		// CheckOrigin is permissive: serve binds 127.0.0.1 by default and v1
		// has no auth story at all (Requirement 13.3), so origin-checking
		// wouldn't add real protection over what a local tool already accepts.
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
		// Silent by default so existing/unrelated tests don't gain console
		// noise; real callers (the CLI) call SetLogger with a real one.
		logger:  rblog.Discard(),
		baseCtx: context.Background(),
	}
}

// SetLogger replaces the Server's logger. Call before Routes() serves
// traffic; the default is silent (log.Discard()).
func (s *Server) SetLogger(l *rblog.Logger) {
	s.logger = l
}

// SetBaseContext sets the parent context every step execution runs under.
// Canceling it (e.g. the CLI canceling its own shutdown context on
// SIGINT/SIGTERM) kills any in-flight execution's process group via the
// same mechanism a per-step timeout uses — so a running step is never
// orphaned by the server process it belongs to exiting. Defaults to
// context.Background(). Call before Routes() serves traffic.
func (s *Server) SetBaseContext(ctx context.Context) {
	s.baseCtx = ctx
}

// EnableWorkspace turns on the left-pane file browser rooted at root: GET
// /api/files lists its *.md files and POST /api/open switches the active
// document to one of them. Call before Routes().
func (s *Server) EnableWorkspace(root string) {
	s.root = root
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/doc", s.handleGetDoc)
	mux.HandleFunc("GET /api/session", s.handleGetSession)
	mux.HandleFunc("GET /api/logs", s.handleGetLog)
	mux.HandleFunc("POST /api/session/reset", s.handlePostSessionReset)
	mux.HandleFunc("POST /api/steps/{name}/run", s.handlePostStepRun)
	mux.HandleFunc("GET /ws/executions/{id}", s.handleWSExecution)
	mux.HandleFunc("POST /api/executions/{id}/cancel", s.handleCancelExecution)
	mux.HandleFunc("GET /api/files", s.handleGetFiles)
	mux.HandleFunc("POST /api/open", s.handlePostOpen)
	mux.Handle("GET /", http.FileServer(webRoot()))
	return s.withRequestLogging(mux)
}

// withRequestLogging logs every request at Info, so a running `serve`
// process's own console shows activity the browser client causes.
func (s *Server) withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying ResponseWriter, since embedding only the
// http.ResponseWriter interface would otherwise hide it — required for the
// WebSocket upgrade, which hijacks the connection.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hj.Hijack()
}

// NewID generates a random id, used for both execution ids and (by CLI
// callers) session ids.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
