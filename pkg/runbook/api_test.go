package runbook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer serves a runbook written from src, logging executions to a
// temp dir.
func newTestServer(t *testing.T, src string) *Server {
	t.Helper()
	return NewServer(openTestSession(t, src, SessionOptions{Logs: NewFileLogWriter(t.TempDir())}), SessionOptions{})
}

// serve runs srv on a real listener, for tests that need WebSockets.
func serve(t *testing.T, srv *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts
}

// runViaAPI starts step name and returns its done event, read off its
// WebSocket as a browser would.
func runViaAPI(t *testing.T, ts *httptest.Server, name string) map[string]any {
	t.Helper()
	return lastEventOf(t, wsDial(t, ts, startRun(t, ts, name)))
}

type sessionJSON struct {
	Cwd     string            `json:"cwd"`
	Vars    map[string]string `json:"vars"`
	History []struct {
		StepName  string    `json:"step_name"`
		StartedAt time.Time `json:"started_at"`
		LogPath   string    `json:"log_path"`
	} `json:"history"`
}

func getSession(t *testing.T, h http.Handler) sessionJSON {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var got sessionJSON
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func TestHandleGetDoc_ReturnsBlocksAndSession(t *testing.T) {
	srv := newTestServer(t, "# Title\n\n```bash {name=hello}\necho hi\n```\n")

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
	srv := newTestServer(t, src)

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

func TestHandleGetDoc_IncludesSensitiveOnStep(t *testing.T) {
	srv := newTestServer(t, "```bash {name=creds, input=TOKEN, sensitive=TOKEN}\necho $TOKEN\n```\n\n```bash {name=plain}\necho hi\n```\n")

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

func TestHandleGetSession_HistoryRecordsEachRunWithStartTime(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	ts := serve(t, srv)
	before := time.Now()

	runViaAPI(t, ts, "hello")

	got := getSession(t, srv.Routes())
	assert.NotEmpty(t, got.Cwd)
	require.Len(t, got.History, 1)
	assert.Equal(t, "hello", got.History[0].StepName)
	assert.False(t, got.History[0].StartedAt.Before(before.Truncate(time.Second)))
}

func TestHandleGetLog_ServesLogOfAnExecutionInThisSession(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi-from-log\n```\n")
	ts := serve(t, srv)
	runViaAPI(t, ts, "hello")
	logPath := getSession(t, srv.Routes()).History[0].LogPath
	require.NotEmpty(t, logPath)

	resp, err := http.Get(ts.URL + "/api/logs?path=" + url.QueryEscape(logPath))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "hi-from-log")
}

func TestHandleGetLog_404ForPathNotInSessionHistory(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/logs?path=/etc/hosts", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGetLog_400WhenPathMissing(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlePostSessionReset_ClearsVarsKeepsHistory(t *testing.T) {
	srv := newTestServer(t, "```bash {name=set, capture=A}\nexport A=1\n```\n")
	ts := serve(t, srv)
	runViaAPI(t, ts, "set")
	require.Equal(t, "1", getSession(t, srv.Routes()).Vars["A"])

	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/session/reset", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	got := getSession(t, srv.Routes())
	assert.Empty(t, got.Vars)
	assert.Len(t, got.History, 1)
}

func TestHandlePostStepRun_MissingInputReturns400(t *testing.T) {
	srv := newTestServer(t, "```bash {name=greet, input=NAME}\necho hi $NAME\n```\n")

	body, _ := json.Marshal(map[string]any{"inputs": map[string]string{}, "confirmed": false})
	req := httptest.NewRequest(http.MethodPost, "/api/steps/greet/run", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, getSession(t, srv.Routes()).History)
}

func TestHandlePostStepRun_UnconfirmedDangerStepReturns400(t *testing.T) {
	src := "---\ndanger_patterns:\n  - \"rm -rf\"\n---\n```bash {name=wipe}\nrm -rf /tmp/x\n```\n"
	srv := newTestServer(t, src)

	body, _ := json.Marshal(map[string]any{"confirmed": false})
	req := httptest.NewRequest(http.MethodPost, "/api/steps/wipe/run", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "rm -rf")
}

func TestHandlePostStepRun_UnknownStepReturns404(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/steps/missing/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlePostStepRun_RejectedRunDoesNotHoldTheSlot(t *testing.T) {
	srv := newTestServer(t, "```bash {name=greet, input=NAME}\necho hi $NAME\n```\n\n```bash {name=other}\necho hi\n```\n")
	ts := serve(t, srv)

	resp, err := http.Post(ts.URL+"/api/steps/greet/run", "application/json", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	assert.Equal(t, "done", runViaAPI(t, ts, "other")["type"])
}

func TestHandlePostStepRun_RejectsConcurrentRunWith409(t *testing.T) {
	srv := newTestServer(t, "```bash {name=slow}\nsleep 5\n```\n\n```bash {name=other}\necho hi\n```\n")
	ts := serve(t, srv)

	slowID := startRun(t, ts, "slow")
	conn := wsDial(t, ts, slowID)

	resp, err := http.Post(ts.URL+"/api/steps/other/run", "application/json", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, string(body), "slow")

	cancelResp, err := http.Post(ts.URL+"/api/executions/"+slowID+"/cancel", "application/json", nil)
	require.NoError(t, err)
	cancelResp.Body.Close()
	assert.Equal(t, true, lastEventOf(t, conn)["canceled"])

	// The next run is accepted as soon as the previous one reported done.
	assert.Equal(t, "done", runViaAPI(t, ts, "other")["type"])
}

func TestHandlePostStepRun_ValidRunReturnsAcceptedAndCommitsCaptures(t *testing.T) {
	srv := newTestServer(t, "```bash {name=greet, capture=OUT}\nexport OUT=hi\n```\n")
	ts := serve(t, srv)

	done := runViaAPI(t, ts, "greet")

	assert.Equal(t, map[string]any{"OUT": "hi"}, done["captured"])
	assert.Equal(t, "hi", getSession(t, srv.Routes()).Vars["OUT"], "session must be updated by the time done is delivered")
}
