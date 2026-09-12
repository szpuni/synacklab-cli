package runbook

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type blockView struct {
	Kind  BlockKind `json:"kind"`
	Prose string    `json:"prose,omitempty"`
	Step  *stepView `json:"step,omitempty"`
}

type stepView struct {
	Name    string   `json:"name"`
	Lang    string   `json:"lang"`
	Source  string   `json:"source"`
	Input   []string `json:"input,omitempty"`
	Capture []string `json:"capture,omitempty"`
	Confirm bool     `json:"confirm"`
	Cwd     string   `json:"cwd,omitempty"`
	SetCwd  bool     `json:"set_cwd"`
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
	blocks := make([]blockView, len(s.doc.Blocks))
	for i, b := range s.doc.Blocks {
		bv := blockView{Kind: b.Kind, Prose: b.Prose}
		if b.Step != nil {
			bv.Step = &stepView{
				Name: b.Step.Name, Lang: b.Step.Lang, Source: b.Step.Source,
				Input: b.Step.Input, Capture: b.Step.Capture, Confirm: b.Step.Confirm,
				Cwd: b.Step.Cwd, SetCwd: b.Step.SetCwd,
			}
		}
		blocks[i] = bv
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"blocks":  blocks,
		"session": toSessionView(s.store.Get()),
	})
}

func (s *Server) handleGetSession(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, toSessionView(s.store.Get()))
}

func (s *Server) handlePostSessionReset(w http.ResponseWriter, _ *http.Request) {
	s.store.Reset(s.doc.Dir)
	writeJSON(w, http.StatusOK, toSessionView(s.store.Get()))
}

type runRequest struct {
	Inputs    map[string]string `json:"inputs"`
	Confirmed bool              `json:"confirmed"`
}

func (s *Server) handlePostStepRun(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	step, ok := s.doc.Steps[name]
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

	if err := CheckConfirmation(step, s.doc.Frontmatter.DangerPatterns, req.Confirmed); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	timeout := EffectiveTimeout(step, s.doc.Frontmatter.DefaultTimeout)
	_, events, err := s.engine.Run(r.Context(), step, req.Inputs, timeout, s.store)
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
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
