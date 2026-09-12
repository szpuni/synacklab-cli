package runbook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestHandleGetDoc_ReturnsBlocksAndSession(t *testing.T) {
	srv, _ := newTestServer(t, "# Title\n\n```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "\"hello\"")
	assert.Contains(t, body, "Title")
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
	srv.store.SetVar("A", "1")

	req := httptest.NewRequest(http.MethodPost, "/api/session/reset", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, srv.store.Get().Vars)
}

func TestHandlePostStepRun_MissingInputReturns400(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=greet, input=NAME}\necho hi $NAME\n```\n")

	body, _ := json.Marshal(map[string]any{"inputs": map[string]string{}, "confirmed": false})
	req := httptest.NewRequest(http.MethodPost, "/api/steps/greet/run", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, srv.store.Get().History)
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

	assert.Equal(t, "hi", srv.store.Get().Vars["OUT"])
}
