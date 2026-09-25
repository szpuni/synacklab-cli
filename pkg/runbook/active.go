package runbook

import (
	"context"
	"fmt"
	"sync"

	rblog "synacklab/pkg/log"
)

// errSessionNotEmpty refuses switching away from a session that has vars or
// history unless the caller forces it (Requirement 6.4's confirmation).
var errSessionNotEmpty = &Error{Type: ErrorTypeConflict, Message: "switching runbooks will discard the current session's captured variables and history"}

// activeRunbook owns the Server's open Session together with its single
// in-flight execution slot (Requirement 6) and every execution's event
// record. Keeping them under one lock is what makes "one execution at a
// time" and "no switching runbooks mid-run" hold: each operation checks and
// changes them atomically.
type activeRunbook struct {
	mu         sync.Mutex
	sess       *Session // nil until a runbook is opened
	running    *runningExecution
	executions map[string]*executionRecord
}

// runningExecution identifies the execution currently holding the slot.
type runningExecution struct {
	id       string
	stepName string
}

func newActiveRunbook(sess *Session) *activeRunbook {
	return &activeRunbook{sess: sess, executions: map[string]*executionRecord{}}
}

// session returns the open Session, or nil if nothing is open.
func (a *activeRunbook) session() *Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sess
}

// start runs stepName in the open session if no other execution holds the
// slot, returning the new execution's id. Its events are buffered for
// replay to any number of subscribers; the slot is freed before done is
// published. Errors are *Error: ErrorTypeConflict when nothing is open or
// the slot is taken, otherwise whatever Session.Start rejected the run with.
func (a *activeRunbook) start(ctx context.Context, stepName string, inputs map[string]string, confirmed bool, logger *rblog.Logger) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.sess == nil {
		return "", &Error{Type: ErrorTypeConflict, Message: "no runbook open — select one from the file list"}
	}
	if a.running != nil {
		return "", &Error{Type: ErrorTypeConflict, Message: fmt.Sprintf("step %q is still running (execution %s)", a.running.stepName, a.running.id)}
	}

	events, cancel, err := a.sess.Start(ctx, stepName, inputs, confirmed)
	if err != nil {
		return "", err
	}

	id := NewID()
	rec := &executionRecord{done: make(chan struct{}), cancel: cancel}
	a.executions[id] = rec
	a.running = &runningExecution{id: id, stepName: stepName}
	logger.Info("step %q started (execution %s)", stepName, id)

	go a.drain(id, stepName, rec, events, logger)
	return id, nil
}

// drain publishes an execution's events into its record, logging its
// outcome so a running `serve` process's console shows what's happening
// even though execution itself is triggered from the browser.
func (a *activeRunbook) drain(id, stepName string, rec *executionRecord, events <-chan Event, logger *rblog.Logger) {
	for ev := range events {
		if ev.Type != EventDone {
			rec.publish(ev)
			continue
		}
		// Log and free the slot before publishing done, so a client that
		// has seen done can immediately run the next step, and never
		// observes a console missing this step's log line.
		if failure := ev.failure(); failure != "" {
			logger.Warn("step %q %s (execution %s, duration=%s)", stepName, failure, id, ev.Duration)
		} else {
			logger.Info("step %q finished (execution %s, duration=%s)", stepName, id, ev.Duration)
		}
		a.mu.Lock()
		if a.running != nil && a.running.id == id {
			a.running = nil
		}
		a.mu.Unlock()
		rec.publish(ev)
		close(rec.done)
	}
}

// switchTo makes the runbook at absPath the open one, with a fresh session
// set up by opts. It is refused (ErrorTypeConflict) while a step is running
// — force can't override that — and, unless force is set, when the current
// session has vars or history (errSessionNotEmpty). Reopening the already
// open runbook is a no-op that keeps its session (unchanged=true).
func (a *activeRunbook) switchTo(absPath string, force bool, opts SessionOptions) (unchanged bool, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running != nil {
		return false, &Error{Type: ErrorTypeConflict, Message: fmt.Sprintf(
			"step %q is still running (execution %s); cancel or wait for it before switching runbooks", a.running.stepName, a.running.id)}
	}
	if a.sess != nil {
		if a.sess.Document().Path == absPath {
			return true, nil
		}
		if state := a.sess.State(); !force && (len(state.Vars) > 0 || len(state.History) > 0) {
			return false, errSessionNotEmpty
		}
	}

	sess, err := OpenSession(absPath, opts)
	if err != nil {
		return false, err
	}
	a.sess = sess
	return false, nil
}

// execution returns the event record of execution id.
func (a *activeRunbook) execution(id string) (*executionRecord, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rec, ok := a.executions[id]
	return rec, ok
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
