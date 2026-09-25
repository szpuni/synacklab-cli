package runbook

import "net/http"

func (s *Server) handleWSExecution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, ok := s.registry.get(id)
	if !ok {
		http.Error(w, "unknown execution "+id, http.StatusNotFound)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, buffered := rec.subscribe()

	finished := false
	for _, ev := range buffered {
		if writeErr := conn.WriteJSON(toWSEvent(ev)); writeErr != nil {
			return
		}
		if ev.Type == EventDone {
			finished = true
		}
	}

	if finished {
		return
	}
	for ev := range ch {
		if writeErr := conn.WriteJSON(toWSEvent(ev)); writeErr != nil {
			return
		}
		if ev.Type == EventDone {
			return
		}
	}
}

// wsEvent mirrors project-brief.md §9.2's wire format: stdout/stderr events
// carry only data, the terminal done event carries exit_code/duration_ms/captured.
type wsEvent struct {
	Type       EventType         `json:"type"`
	Data       string            `json:"data,omitempty"`
	ExitCode   *int              `json:"exit_code,omitempty"`
	DurationMS *int64            `json:"duration_ms,omitempty"`
	TimedOut   *bool             `json:"timed_out,omitempty"`
	Canceled   *bool             `json:"canceled,omitempty"`
	Captured   map[string]string `json:"captured,omitempty"`
}

func toWSEvent(ev Event) wsEvent {
	out := wsEvent{Type: ev.Type, Data: ev.Data}
	if ev.Type == EventDone {
		exitCode, durationMS, timedOut, canceled := ev.ExitCode, ev.Duration.Milliseconds(), ev.TimedOut, ev.Canceled
		out.ExitCode = &exitCode
		out.DurationMS = &durationMS
		out.TimedOut = &timedOut
		out.Canceled = &canceled
		out.Captured = ev.Captured
	}
	return out
}

// handleCancelExecution stops a specific in-flight execution's process group
// (Requirement 4.4). Canceling an unknown id is a 404; canceling an
// already-finished execution is a no-op 200 rather than an error, since
// there is nothing left to cancel (Requirement 4.6).
func (s *Server) handleCancelExecution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, ok := s.registry.get(id)
	if !ok {
		s.writeError(w, http.StatusNotFound, "unknown execution "+id)
		return
	}
	if rec.isDone() {
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "already finished"})
		return
	}
	rec.cancel()
	s.writeJSON(w, http.StatusAccepted, map[string]string{"status": "canceling"})
}
