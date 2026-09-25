package runbook

import "sync"

// SessionStore guards concurrent access to a single Session.
type SessionStore interface {
	Get() *Session
	SetVar(key, value string)
	SetCwd(path string)
	AppendHistory(e Execution)
	Reset()
}

type memorySessionStore struct {
	mu         sync.Mutex
	sess       Session
	defaultCwd string
}

// NewSessionStore creates a Session starting in defaultCwd, which is also
// where Reset puts it back.
func NewSessionStore(id, docPath, defaultCwd string) SessionStore {
	return &memorySessionStore{
		defaultCwd: defaultCwd,
		sess: Session{
			ID:      id,
			DocPath: docPath,
			Cwd:     defaultCwd,
			Vars:    map[string]string{},
		},
	}
}

func (s *memorySessionStore) Get() *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	vars := make(map[string]string, len(s.sess.Vars))
	for k, v := range s.sess.Vars {
		vars[k] = v
	}
	history := make([]Execution, len(s.sess.History))
	copy(history, s.sess.History)

	snapshot := s.sess
	snapshot.Vars = vars
	snapshot.History = history
	return &snapshot
}

func (s *memorySessionStore) SetVar(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess.Vars[key] = value
}

func (s *memorySessionStore) SetCwd(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess.Cwd = path
}

func (s *memorySessionStore) AppendHistory(e Execution) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess.History = append(s.sess.History, e)
}

// Reset clears vars and puts cwd back to the session's default. Execution
// history is left intact — it's an audit trail, and each execution is
// already logged to disk independently of session state (Requirement 4).
func (s *memorySessionStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess.Vars = map[string]string{}
	s.sess.Cwd = s.defaultCwd
}
