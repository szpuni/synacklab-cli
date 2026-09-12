package runbook

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
)

// Server wires the REST/WebSocket API around a parsed Document, its Session,
// and an Engine. One Server per running `serve` process (Requirement 14.1).
type Server struct {
	doc      *Document
	store    SessionStore
	engine   Engine
	registry *executionRegistry
}

func NewServer(doc *Document, store SessionStore, engine Engine) *Server {
	return &Server{doc: doc, store: store, engine: engine, registry: newExecutionRegistry()}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/doc", s.handleGetDoc)
	mux.HandleFunc("GET /api/session", s.handleGetSession)
	mux.HandleFunc("POST /api/session/reset", s.handlePostSessionReset)
	mux.HandleFunc("POST /api/steps/{name}/run", s.handlePostStepRun)
	return mux
}

// executionRecord buffers a running/completed execution's events so a
// WebSocket client that connects late (or a fast execution that finishes
// before the client upgrades) still gets the full event history replayed.
type executionRecord struct {
	mu     sync.Mutex
	events []Event
	done   chan struct{}
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
			rec.mu.Lock()
			rec.events = append(rec.events, ev)
			rec.mu.Unlock()
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
