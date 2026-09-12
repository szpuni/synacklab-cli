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
		if ev.Type == "done" {
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
		if ev.Type == "done" {
			return
		}
	}
}

// wsEvent mirrors project-brief.md §9.2's wire format: stdout/stderr events
// carry only data, the terminal done event carries exit_code/duration_ms/captured.
type wsEvent struct {
	Type       string            `json:"type"`
	Data       string            `json:"data,omitempty"`
	ExitCode   *int              `json:"exit_code,omitempty"`
	DurationMS *int64            `json:"duration_ms,omitempty"`
	TimedOut   *bool             `json:"timed_out,omitempty"`
	Captured   map[string]string `json:"captured,omitempty"`
}

func toWSEvent(ev Event) wsEvent {
	out := wsEvent{Type: ev.Type, Data: ev.Data}
	if ev.Type == "done" {
		exitCode, durationMS, timedOut := ev.ExitCode, ev.Duration.Milliseconds(), ev.TimedOut
		out.ExitCode = &exitCode
		out.DurationMS = &durationMS
		out.TimedOut = &timedOut
		out.Captured = ev.Captured
	}
	return out
}
