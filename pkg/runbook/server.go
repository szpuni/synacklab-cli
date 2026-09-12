package runbook

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
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

// Server wires the REST/WebSocket API around a parsed Document, its Session,
// and an Engine. One Server per running `serve` process (Requirement 14.1).
type Server struct {
	doc      *Document
	store    SessionStore
	engine   Engine
	registry *executionRegistry
	upgrader websocket.Upgrader
}

func NewServer(doc *Document, store SessionStore, engine Engine) *Server {
	return &Server{
		doc: doc, store: store, engine: engine, registry: newExecutionRegistry(),
		// CheckOrigin is permissive: serve binds 127.0.0.1 by default and v1
		// has no auth story at all (Requirement 13.3), so origin-checking
		// wouldn't add real protection over what a local tool already accepts.
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/doc", s.handleGetDoc)
	mux.HandleFunc("GET /api/session", s.handleGetSession)
	mux.HandleFunc("POST /api/session/reset", s.handlePostSessionReset)
	mux.HandleFunc("POST /api/steps/{name}/run", s.handlePostStepRun)
	mux.HandleFunc("GET /ws/executions/{id}", s.handleWSExecution)
	mux.Handle("GET /", http.FileServer(webRoot()))
	return mux
}

// executionRecord buffers a running/completed execution's events and fans
// them out to subscribers (WebSocket connections), so a client that
// connects late — or after a fast execution already finished — still gets
// the full event history replayed before any live events.
type executionRecord struct {
	mu          sync.Mutex
	events      []Event
	subscribers []chan Event
	done        chan struct{}
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

type executionRegistry struct {
	mu      sync.Mutex
	entries map[string]*executionRecord
}

func newExecutionRegistry() *executionRegistry {
	return &executionRegistry{entries: map[string]*executionRecord{}}
}

// start records a new execution under a fresh id and drains its event
// channel into the record in the background.
func (r *executionRegistry) start(events <-chan Event) string {
	id := newExecutionID()
	rec := &executionRecord{done: make(chan struct{})}

	r.mu.Lock()
	r.entries[id] = rec
	r.mu.Unlock()

	go func() {
		for ev := range events {
			rec.publish(ev)
			if ev.Type == "done" {
				close(rec.done)
			}
		}
	}()

	return id
}

func (r *executionRegistry) get(id string) (*executionRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.entries[id]
	return rec, ok
}

func newExecutionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
