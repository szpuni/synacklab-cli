package runbook

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rblog "synacklab/pkg/log"
)

func newLoggingTestServer(t *testing.T, src string) (*Server, *bytes.Buffer) {
	t.Helper()
	srv, _ := newTestServer(t, src)
	var buf bytes.Buffer
	srv.SetLogger(rblog.New(&buf, rblog.LevelInfo))
	return srv, &buf
}

func TestServer_LogsHTTPRequestsAtInfo(t *testing.T) {
	srv, buf := newLoggingTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	out := buf.String()
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "GET")
	assert.Contains(t, out, "/api/doc")
}

func TestServer_LogsStepStartAndSuccessfulFinishAtInfo(t *testing.T) {
	srv, buf := newLoggingTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/steps/hello/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	waitForNoRunningExecutions(t, srv)

	out := buf.String()
	assert.Contains(t, out, `"hello"`)
	assert.Contains(t, out, "started")
	assert.Contains(t, out, "finished")
}

func TestServer_LogsNonZeroExitAtWarn(t *testing.T) {
	srv, buf := newLoggingTestServer(t, "```bash {name=fails}\nexit 3\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/steps/fails/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	waitForNoRunningExecutions(t, srv)

	out := buf.String()
	assert.Contains(t, out, "WARN")
	assert.Contains(t, out, "exit")
}

func TestServer_LogsValidationFailureAtWarn(t *testing.T) {
	srv, buf := newLoggingTestServer(t, "```bash {name=greet, input=NAME}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/steps/greet/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	assert.Contains(t, buf.String(), "WARN")
}

func TestServer_LogsOpenSuccessAtInfoAndFailureAtWarn(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := NewServer(nil, nil, NewEngine(nil))
	srv.EnableWorkspace(root, "")
	var buf bytes.Buffer
	srv.SetLogger(rblog.New(&buf, rblog.LevelInfo))

	okReq := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader([]byte(`{"file":"deploy.md"}`)))
	okRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(okRec, okReq)
	require.Equal(t, http.StatusOK, okRec.Code)
	assert.Contains(t, buf.String(), "INFO")
	assert.Contains(t, buf.String(), "deploy.md")

	buf.Reset()
	badReq := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader([]byte(`{"file":"missing.md"}`)))
	badRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(badRec, badReq)
	require.Equal(t, http.StatusNotFound, badRec.Code)
	assert.Contains(t, buf.String(), "WARN")
}

// waitForNoRunningExecutions polls until every registered execution's done
// channel is closed, so tests can assert on log output written by the
// registry's background drain goroutine without a fixed sleep.
func waitForNoRunningExecutions(t *testing.T, srv *Server) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		srv.registry.mu.Lock()
		allDone := true
		for _, rec := range srv.registry.entries {
			select {
			case <-rec.done:
			default:
				allDone = false
			}
		}
		srv.registry.mu.Unlock()
		if allDone {
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for executions to finish")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
