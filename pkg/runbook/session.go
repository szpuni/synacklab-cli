package runbook

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SessionOptions configures how OpenSession sets up a Session.
type SessionOptions struct {
	// Cwd overrides the session's default working directory (where it
	// starts, and where Reset puts it back). Empty means the document's own
	// directory.
	Cwd string
	// Logs persists each execution's full output to disk (Requirement 4).
	// Nil skips disk logging.
	Logs LogWriter
}

// Session is one open runbook: its parsed Document plus the vars, cwd and
// history accumulated by running its steps. It is safe for concurrent use.
type Session struct {
	doc        *Document
	logs       LogWriter
	defaultCwd string

	mu    sync.Mutex
	state SessionState
}

// OpenSession reads and parses the runbook at path and starts a fresh
// Session for it. Paths are made absolute, so the document stays comparable
// to an absolute workspace root. A read failure is an ErrorTypeNotFound
// *Error; a malformed document is an ErrorTypeParse one.
func OpenSession(path string, opts SessionOptions) (*Session, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("failed to resolve %s: %v", path, err), Cause: err}
	}
	source, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("failed to read %s: %v", path, err), Cause: err}
	}
	doc, err := Parse(source, absPath)
	if err != nil {
		return nil, err
	}

	cwd := doc.Dir
	if opts.Cwd != "" {
		if cwd, err = filepath.Abs(opts.Cwd); err != nil {
			return nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("failed to resolve working directory %s: %v", opts.Cwd, err), Cause: err}
		}
	}

	return &Session{
		doc:        doc,
		logs:       opts.Logs,
		defaultCwd: cwd,
		state:      SessionState{ID: NewID(), DocPath: absPath, Cwd: cwd, Vars: map[string]string{}},
	}, nil
}

// Document returns the parsed runbook this session runs.
func (s *Session) Document() *Document {
	return s.doc
}

// State returns an independent snapshot of the session's vars, cwd and
// history; mutating it does not affect the session.
func (s *Session) State() *SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := s.state
	snapshot.Vars = make(map[string]string, len(s.state.Vars))
	for k, v := range s.state.Vars {
		snapshot.Vars[k] = v
	}
	snapshot.History = append([]Execution(nil), s.state.History...)
	return &snapshot
}

// Reset clears vars and puts cwd back to the session's default. Execution
// history is left intact — it's an audit trail, and each execution is
// already logged to disk independently of session state (Requirement 4).
func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Vars = map[string]string{}
	s.state.Cwd = s.defaultCwd
}

// record commits one finished execution: its captured vars (under both
// "<step>.<VAR>" and bare "<VAR>"), its cwd if the step declared set_cwd,
// its disk log, and its history entry — all under one lock, so the log's
// index always matches the execution's position in History.
func (s *Session) record(step *Step, exec Execution, captured *captureResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, value := range captured.vars {
		s.state.Vars[step.Name+"."+name] = value
		s.state.Vars[name] = value
	}
	if step.SetCwd && captured.cwdFound {
		s.state.Cwd = captured.cwd
	}
	if s.logs != nil {
		if path, err := s.logs.Write(s.state.DocPath, s.state.ID, len(s.state.History)+1, exec); err == nil {
			exec.LogPath = path
		}
	}
	s.state.History = append(s.state.History, exec)
}
