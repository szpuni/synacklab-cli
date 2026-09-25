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
	"sync"
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
	active   *activeDoc
	opts     SessionOptions // how a runbook opened via /api/open is set up
	registry *executionRegistry
	upgrader websocket.Upgrader
	root     string // "" disables the file-browser endpoints (/api/files, /api/open)
	logger   *rblog.Logger
	baseCtx  context.Context // parent for every step execution; canceling it kills any in-flight step
}

// activeDoc holds the currently-open Session, swappable at runtime via
// /api/open when a Server is in workspace (directory) mode. It may be nil,
// meaning nothing is open yet.
type activeDoc struct {
	mu   sync.RWMutex
	sess *Session
}

func (a *activeDoc) get() *Session {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sess
}

func (a *activeDoc) set(sess *Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sess = sess
}

// NewServer creates a Server around sess, which may be nil (nothing open
// yet) — pair with EnableWorkspace so the frontend's file browser can open
// one. opts sets up every runbook opened later via /api/open.
func NewServer(sess *Session, opts SessionOptions) *Server {
	return &Server{
		active: &activeDoc{sess: sess}, opts: opts, registry: newExecutionRegistry(),
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

// executionRecord buffers a running/completed execution's events and fans
// them out to subscribers (WebSocket connections), so a client that
// connects late — or after a fast execution already finished — still gets
// the full event history replayed before any live events. cancel stops the
// specific execution this record tracks (Requirement 4.4).
type executionRecord struct {
	mu          sync.Mutex
	events      []Event
	subscribers []chan Event
	done        chan struct{}
	cancel      context.CancelFunc
}

// isDone reports whether this execution's terminal event has already been
// published, without blocking — used by the cancel endpoint to treat
// canceling an already-finished execution as a no-op (Requirement 4.6).
func (r *executionRecord) isDone() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// subscribe registers a new listener and returns the events already
// published, atomically with registration so none are missed or duplicated.
func (r *executionRecord) subscribe() (<-chan Event, []Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan Event, 32)
	r.subscribers = append(r.subscribers, ch)
	snapshot := make([]Event, len(r.events))
	copy(snapshot, r.events)
	return ch, snapshot
}

func (r *executionRecord) publish(ev Event) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	subs := make([]chan Event, len(r.subscribers))
	copy(subs, r.subscribers)
	r.mu.Unlock()

	for _, ch := range subs {
		ch <- ev
	}
}

// runningExecution identifies the single execution currently permitted to be
// in flight (Requirement 6).
type runningExecution struct {
	id       string
	stepName string
}

type executionRegistry struct {
	mu      sync.Mutex
	entries map[string]*executionRecord
	running *runningExecution
}

func newExecutionRegistry() *executionRegistry {
	return &executionRegistry{entries: map[string]*executionRecord{}}
}

// tryReserve atomically claims the single in-flight execution slot for id/
// stepName. It fails (ok=false) if another execution is already running,
// returning that execution so the caller can report it (409 Conflict).
func (r *executionRegistry) tryReserve(id, stepName string) (running *runningExecution, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running != nil {
		return r.running, false
	}
	r.running = &runningExecution{id: id, stepName: stepName}
	return nil, true
}

// releaseReservation clears a reservation made by tryReserve without ever
// calling start — used when starting the underlying process itself fails.
func (r *executionRegistry) releaseReservation(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running != nil && r.running.id == id {
		r.running = nil
	}
}

// runningInfo reports the currently in-flight execution, if any.
func (r *executionRegistry) runningInfo() (*runningExecution, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running, r.running != nil
}

// start records a new execution under id (already reserved via tryReserve)
// and drains its event channel into the record in the background, logging
// its lifecycle so a running `serve` process's console shows what's
// happening even though execution itself is triggered from the browser.
func (r *executionRegistry) start(id, stepName string, cancel context.CancelFunc, events <-chan Event, logger *rblog.Logger) {
	rec := &executionRecord{done: make(chan struct{}), cancel: cancel}

	r.mu.Lock()
	r.entries[id] = rec
	r.mu.Unlock()

	logger.Info("step %q started (execution %s)", stepName, id)

	go func() {
		for ev := range events {
			if ev.Type != EventDone {
				rec.publish(ev)
				continue
			}
			// Log and free the slot before publishing done, so a client
			// that has seen done can immediately run the next step, and
			// never observes a console missing this step's log line.
			if failure := ev.failure(); failure != "" {
				logger.Warn("step %q %s (execution %s, duration=%s)", stepName, failure, id, ev.Duration)
			} else {
				logger.Info("step %q finished (execution %s, duration=%s)", stepName, id, ev.Duration)
			}
			r.mu.Lock()
			if r.running != nil && r.running.id == id {
				r.running = nil
			}
			r.mu.Unlock()
			rec.publish(ev)
			close(rec.done)
		}
	}()
}

func (r *executionRegistry) get(id string) (*executionRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.entries[id]
	return rec, ok
}

// NewID generates a random id, used for both execution ids and (by CLI
// callers) session ids.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
