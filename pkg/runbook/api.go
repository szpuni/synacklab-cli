package runbook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

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
		writeJSON(w, http.StatusOK, map[string]any{"active": false, "workspace": s.root != ""})
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
				Input: b.Step.Input, Capture: b.Step.Capture, Confirm: b.Step.Confirm,
				Cwd: b.Step.Cwd, SetCwd: b.Step.SetCwd,
				RequiresConfirm: requiresConfirm, ConfirmReason: reason,
			}
		}
		blocks[i] = bv
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
		writeJSON(w, http.StatusOK, sessionView{Vars: map[string]string{}})
		return
	}
	writeJSON(w, http.StatusOK, toSessionView(store.Get()))
}

func (s *Server) handlePostSessionReset(w http.ResponseWriter, _ *http.Request) {
	doc, store := s.active.get()
	if doc == nil || store == nil {
		writeError(w, http.StatusConflict, "no runbook open")
		return
	}
	store.Reset(doc.Dir)
	writeJSON(w, http.StatusOK, toSessionView(store.Get()))
}

type runRequest struct {
	Inputs    map[string]string `json:"inputs"`
	Confirmed bool              `json:"confirmed"`
}

func (s *Server) handlePostStepRun(w http.ResponseWriter, r *http.Request) {
	doc, store := s.active.get()
	if doc == nil {
		writeError(w, http.StatusConflict, "no runbook open — select one from the file list")
		return
	}

	name := r.PathValue("name")
	step, ok := doc.Steps[name]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown step "+name)
		return
	}

	var req runRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "malformed request body: "+err.Error())
			return
		}
	}

	for _, inputName := range step.Input {
		if _, ok := req.Inputs[inputName]; !ok {
			writeError(w, http.StatusBadRequest, "missing required input: "+inputName)
			return
		}
	}

	if err := CheckConfirmation(step, doc.Frontmatter.DangerPatterns, req.Confirmed); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Detached from the request context: the run must outlive the HTTP
	// handler, which returns as soon as it hands back the execution_id.
	timeout := EffectiveTimeout(step, doc.Frontmatter.DefaultTimeout)
	_, events, err := s.engine.Run(context.Background(), step, req.Inputs, timeout, store)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id := s.registry.start(events)
	writeJSON(w, http.StatusAccepted, map[string]string{"execution_id": id})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
