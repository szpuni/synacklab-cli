package tunnel

import (
	"context"
	"sync"
)

const (
	stateStarting  = "starting"
	stateConnected = "connected"
	stateClosing   = "closing"
	stateClosed    = "closed"
	stateFailed    = "failed"
)

// tunnelRun is one tunnel opened from the page: its state and everything it
// printed, which page reloads replay.
type tunnelRun struct {
	id     string
	req    Request
	target Target
	cancel context.CancelFunc

	mu      sync.Mutex
	state   string
	pod     string
	err     string
	output  []byte
	changed chan struct{} // closed and replaced on every change
}

type tunnelView struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Pod       string `json:"pod,omitempty"`
	Namespace string `json:"namespace"`
	Context   string `json:"context,omitempty"`
	Target    string `json:"target"`
	LocalAddr string `json:"local_addr"`
	Hint      string `json:"hint,omitempty"`
	Error     string `json:"error,omitempty"`
}

func newTunnelRun(id string, req Request, target Target, cancel context.CancelFunc) *tunnelRun {
	return &tunnelRun{id: id, req: req, target: target, cancel: cancel, state: stateStarting, changed: make(chan struct{})}
}

// Write collects the tunnel's output (Options.Out and Err).
func (t *tunnelRun) Write(p []byte) (int, error) {
	t.update(func() { t.output = append(t.output, p...) })
	return len(p), nil
}

func (t *tunnelRun) connected(pod string) {
	t.update(func() {
		t.pod = pod
		if t.state == stateStarting {
			t.state = stateConnected
		}
	})
}

func (t *tunnelRun) close() {
	t.update(func() {
		if t.state == stateStarting || t.state == stateConnected {
			t.state = stateClosing
		}
	})
	t.cancel()
}

// finish records how Start returned: nil means closed on request.
func (t *tunnelRun) finish(err error) {
	t.update(func() {
		if err != nil {
			t.state, t.err = stateFailed, err.Error()
		} else {
			t.state = stateClosed
		}
	})
}

func (t *tunnelRun) finished() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == stateClosed || t.state == stateFailed
}

// since returns the output after the first offset bytes, the current view,
// and a channel closed on the next change.
func (t *tunnelRun) since(offset int) (string, tunnelView, <-chan struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.output[offset:]), t.viewLocked(), t.changed
}

func (t *tunnelRun) view() tunnelView {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.viewLocked()
}

func (t *tunnelRun) viewLocked() tunnelView {
	return tunnelView{
		ID: t.id, State: t.state, Pod: t.pod, Namespace: t.req.Namespace, Context: t.req.Context,
		Target: t.target.String(), LocalAddr: localAddr(t.req.LocalPort),
		Hint: ClientHint(t.target.Kind, t.req.LocalPort), Error: t.err,
	}
}

func (t *tunnelRun) update(change func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	change()
	close(t.changed)
	t.changed = make(chan struct{})
}
