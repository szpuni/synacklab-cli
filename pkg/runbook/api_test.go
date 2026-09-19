package runbook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, src string) (*Server, string) {
	t.Helper()
	doc, err := (&GoldmarkParser{}).Parse([]byte(src), "/tmp/deploy.md")
	require.NoError(t, err)

	store := NewSessionStore("sess-1", doc.Path, t.TempDir())
	srv := NewServer(doc, store, NewEngine(nil))
	return srv, doc.Path
}

// activeStore returns the Server's current SessionStore, for tests that
// need to inspect/mutate session state directly.
func activeStore(s *Server) SessionStore {
	_, store := s.active.get()
	return store
}

func TestHandleGetDoc_ReturnsBlocksAndSession(t *testing.T) {
	srv, _ := newTestServer(t, "# Title\n\n```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "\"hello\"")
	assert.Contains(t, body, "<h1>Title</h1>", "prose should be server-rendered to HTML so the frontend needs no markdown library")
}

func TestHandleGetDoc_ExposesConfirmationRequirement(t *testing.T) {
	src := "---\ndanger_patterns:\n  - \"rm -rf\"\n---\n```bash {name=wipe}\nrm -rf /tmp/x\n```\n\n```bash {name=safe}\necho hi\n```\n"
	srv, _ := newTestServer(t, src)

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	var got struct {
		Blocks []struct {
			Step *struct {
				Name            string `json:"name"`
				RequiresConfirm bool   `json:"requires_confirm"`
				ConfirmReason   string `json:"confirm_reason,omitempty"`
			} `json:"step,omitempty"`
		} `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	require.Len(t, got.Blocks, 2)
	assert.True(t, got.Blocks[0].Step.RequiresConfirm)
	assert.Contains(t, got.Blocks[0].Step.ConfirmReason, "rm -rf")
	assert.False(t, got.Blocks[1].Step.RequiresConfirm)
}

func TestToSessionView_PopulatesStartedAtFromExecution(t *testing.T) {
	started := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	sess := &Session{Vars: map[string]string{}, History: []Execution{{StepName: "hello", Started: started}}}

	view := toSessionView(sess)

	require.Len(t, view.History, 1)
	assert.True(t, started.Equal(view.History[0].StartedAt))
}

func TestHandleGetDoc_IncludesSensitiveOnStep(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=creds, input=TOKEN, sensitive=TOKEN}\necho $TOKEN\n```\n\n```bash {name=plain}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	var got struct {
		Blocks []struct {
			Step *struct {
				Name      string   `json:"name"`
				Sensitive []string `json:"sensitive,omitempty"`
			} `json:"step,omitempty"`
		} `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	require.Len(t, got.Blocks, 2)
	assert.Equal(t, []string{"TOKEN"}, got.Blocks[0].Step.Sensitive)
	assert.Empty(t, got.Blocks[1].Step.Sensitive)
}

func TestHandleGetLog_ReturnsContentForPathInSessionHistory(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	logPath := filepath.Join(t.TempDir(), "1-hello.log")
	require.NoError(t, os.WriteFile(logPath, []byte("log contents"), 0o644))
	activeStore(srv).AppendHistory(Execution{StepName: "hello", LogPath: logPath})

	req := httptest.NewRequest(http.MethodGet, "/api/logs?path="+logPath, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "log contents", rec.Body.String())
}

func TestHandleGetLog_404ForPathNotInSessionHistory(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	otherPath := filepath.Join(t.TempDir(), "other.log")
	require.NoError(t, os.WriteFile(otherPath, []byte("secret"), 0o644))

	req := httptest.NewRequest(http.MethodGet, "/api/logs?path="+otherPath, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGetLog_400WhenPathMissing(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGetSession_ReturnsVarsCwdHistory(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Cwd  string            `json:"cwd"`
		Vars map[string]string `json:"vars"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.NotEmpty(t, got.Cwd)
}

func TestHandlePostSessionReset_ClearsVars(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	activeStore(srv).SetVar("A", "1")

	req := httptest.NewRequest(http.MethodPost, "/api/session/reset", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, activeStore(srv).Get().Vars)
}

func TestHandlePostStepRun_MissingInputReturns400(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=greet, input=NAME}\necho hi $NAME\n```\n")

	body, _ := json.Marshal(map[string]any{"inputs": map[string]string{}, "confirmed": false})
	req := httptest.NewRequest(http.MethodPost, "/api/steps/greet/run", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, activeStore(srv).Get().History)
}

func TestHandlePostStepRun_UnconfirmedDangerStepReturns400(t *testing.T) {
	src := "---\ndanger_patterns:\n  - \"rm -rf\"\n---\n```bash {name=wipe}\nrm -rf /tmp/x\n```\n"
	srv, _ := newTestServer(t, src)

	body, _ := json.Marshal(map[string]any{"confirmed": false})
	req := httptest.NewRequest(http.MethodPost, "/api/steps/wipe/run", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "rm -rf")
}

func TestHandlePostStepRun_UnknownStepReturns404(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/steps/missing/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlePostStepRun_RejectsConcurrentRunWith409(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=slow}\nsleep 5\n```\n\n```bash {name=other}\necho hi\n```\n")

	req1 := httptest.NewRequest(http.MethodPost, "/api/steps/slow/run", bytes.NewReader([]byte(`{}`)))
	rec1 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusAccepted, rec1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/api/steps/other/run", bytes.NewReader([]byte(`{}`)))
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusConflict, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "slow")

	// Cancel the in-flight execution so a subsequent request is accepted
	// again once it completes.
	var got struct {
		ExecutionID string `json:"execution_id"`
	}
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &got))
	rec, ok := srv.registry.get(got.ExecutionID)
	require.True(t, ok)
	rec.cancel()
	select {
	case <-rec.done:
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not complete in time")
	}

	req3 := httptest.NewRequest(http.MethodPost, "/api/steps/other/run", bytes.NewReader([]byte(`{}`)))
	rec3 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusAccepted, rec3.Code)
}

func TestHandlePostStepRun_ValidRunReturnsAcceptedAndExecutesAsync(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=greet, capture=OUT}\nexport OUT=hi\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/steps/greet/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
	var got struct {
		ExecutionID string `json:"execution_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotEmpty(t, got.ExecutionID)

	rec2, ok := srv.registry.get(got.ExecutionID)
	require.True(t, ok)
	select {
	case <-rec2.done:
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not complete in time")
	}

	assert.Equal(t, "hi", activeStore(srv).Get().Vars["OUT"])
}
