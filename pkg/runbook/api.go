package runbook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/yuin/goldmark"
)

type blockView struct {
	Kind  BlockKind `json:"kind"`
	Prose string    `json:"prose,omitempty"`
	Step  *stepView `json:"step,omitempty"`
}

type stepView struct {
	Name            string   `json:"name"`
	Lang            string   `json:"lang"`
	Source          string   `json:"source"`
	Input           []string `json:"input,omitempty"`
	Capture         []string `json:"capture,omitempty"`
	Sensitive       []string `json:"sensitive,omitempty"`
	Confirm         bool     `json:"confirm"`
	Cwd             string   `json:"cwd,omitempty"`
	SetCwd          bool     `json:"set_cwd"`
	RequiresConfirm bool     `json:"requires_confirm"`
	ConfirmReason   string   `json:"confirm_reason,omitempty"`
}

type sessionView struct {
	Cwd     string            `json:"cwd"`
	Vars    map[string]string `json:"vars"`
	History []execHistoryView `json:"history"`
}

type execHistoryView struct {
	StepName   string            `json:"step_name"`
	StartedAt  time.Time         `json:"started_at"`
	ExitCode   int               `json:"exit_code"`
	DurationMS int64             `json:"duration_ms"`
	TimedOut   bool              `json:"timed_out"`
	LogPath    string            `json:"log_path,omitempty"`
	Captured   map[string]string `json:"captured,omitempty"`
}

func toSessionView(sess *Session) sessionView {
	history := make([]execHistoryView, len(sess.History))
	for i, e := range sess.History {
		history[i] = execHistoryView{
			StepName:   e.StepName,
			StartedAt:  e.Started,
			ExitCode:   e.ExitCode,
			DurationMS: e.Duration.Milliseconds(),
			TimedOut:   e.TimedOut,
			LogPath:    e.LogPath,
			Captured:   e.Captured,
		}
	}
	return sessionView{Cwd: sess.Cwd, Vars: sess.Vars, History: history}
}

func (s *Server) handleGetDoc(w http.ResponseWriter, _ *http.Request) {
	doc, store := s.active.get()
	if doc == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"active": false, "workspace": s.root != ""})
		return
	}

	blocks := make([]blockView, len(doc.Blocks))
	for i, b := range doc.Blocks {
		bv := blockView{Kind: b.Kind}
		if b.Kind == BlockProse {
			bv.Prose = renderProseHTML(b.Prose)
		}
		if b.Step != nil {
			requiresConfirm, reason := RequiresConfirmation(b.Step, doc.Frontmatter.DangerPatterns)
			bv.Step = &stepView{
				Name: b.Step.Name, Lang: b.Step.Lang, Source: b.Step.Source,
				Input: b.Step.Input, Capture: b.Step.Capture, Sensitive: b.Step.Sensitive, Confirm: b.Step.Confirm,
				Cwd: b.Step.Cwd, SetCwd: b.Step.SetCwd,
				RequiresConfirm: requiresConfirm, ConfirmReason: reason,
			}
		}
		blocks[i] = bv
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"active":      true,
		"workspace":   s.root != "",
		"active_file": relativeToRoot(s.root, doc.Path),
		"blocks":      blocks,
		"session":     toSessionView(store.Get()),
	})
}

// renderProseHTML converts a prose block's raw Markdown to HTML server-side
// so the embedded frontend needs no client-side Markdown library.
func renderProseHTML(markdown string) string {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(markdown), &buf); err != nil {
		return markdown
	}
	return buf.String()
}

func (s *Server) handleGetSession(w http.ResponseWriter, _ *http.Request) {
	_, store := s.active.get()
	if store == nil {
		s.writeJSON(w, http.StatusOK, sessionView{Vars: map[string]string{}})
		return
	}
	s.writeJSON(w, http.StatusOK, toSessionView(store.Get()))
}

// handleGetLog serves the contents of a step execution's on-disk log to the
// browser. path must exactly match one of the active session's own
// History[].LogPath values — this is what keeps the endpoint from being
// usable as an arbitrary local file reader even if path is
// attacker-influenced (Requirement 2.3).
func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		s.writeError(w, http.StatusBadRequest, "missing path query parameter")
		return
	}

	_, store := s.active.get()
	if store == nil {
		s.writeError(w, http.StatusNotFound, "no active session")
		return
	}

	found := false
	for _, e := range store.Get().History {
		if e.LogPath == path {
			found = true
			break
		}
	}
	if !found {
		s.writeError(w, http.StatusNotFound, "log not found in the current session's history")
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "failed to read log: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) handlePostSessionReset(w http.ResponseWriter, _ *http.Request) {
	doc, store := s.active.get()
	if doc == nil || store == nil {
		s.writeError(w, http.StatusConflict, "no runbook open")
		return
	}
	store.Reset(doc.Dir)
	s.writeJSON(w, http.StatusOK, toSessionView(store.Get()))
}

type runRequest struct {
	Inputs    map[string]string `json:"inputs"`
	Confirmed bool              `json:"confirmed"`
}

func (s *Server) handlePostStepRun(w http.ResponseWriter, r *http.Request) {
	doc, store := s.active.get()
	if doc == nil {
		s.writeError(w, http.StatusConflict, "no runbook open — select one from the file list")
		return
	}

	name := r.PathValue("name")
	step, ok := doc.Steps[name]
	if !ok {
		s.writeError(w, http.StatusNotFound, "unknown step "+name)
		return
	}

	var req runRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			s.writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
			return
		}
	}

	for _, inputName := range step.Input {
		if _, ok := req.Inputs[inputName]; !ok {
			s.writeError(w, http.StatusBadRequest, "missing required input: "+inputName)
			return
		}
	}

	if running, ok := s.registry.runningInfo(); ok {
		s.writeError(w, http.StatusConflict, fmt.Sprintf("step %q is still running (execution %s)", running.stepName, running.id))
		return
	}

	if err := CheckConfirmation(step, doc.Frontmatter.DangerPatterns, req.Confirmed); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id := NewID()
	if running, ok := s.registry.tryReserve(id, step.Name); !ok {
		s.writeError(w, http.StatusConflict, fmt.Sprintf("step %q is still running (execution %s)", running.stepName, running.id))
		return
	}

	// Detached from the *request* context (which ends as soon as this
	// handler returns the execution_id) but still tied to the server's own
	// baseCtx, so a shutdown can still cancel an in-flight execution.
	timeout := EffectiveTimeout(step, doc.Frontmatter.DefaultTimeout)
	_, events, cancel, err := s.engine.Run(s.baseCtx, step, req.Inputs, timeout, store)
	if err != nil {
		s.registry.releaseReservation(id)
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.registry.start(id, step.Name, cancel, events, s.logger)
	s.writeJSON(w, http.StatusAccepted, map[string]string{"execution_id": id})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeError writes a JSON {"error": message} response and logs it — Warn
// for a client-caused failure (4xx), Error for a server-caused one (5xx) —
// so every error response also shows up on the running server's console.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		s.logger.Error("%s", message)
	} else {
		s.logger.Warn("%s", message)
	}
	s.writeJSON(w, status, map[string]string{"error": message})
}
