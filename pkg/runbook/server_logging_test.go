package runbook

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rblog "synacklab/pkg/log"
)

// syncBuffer is a bytes.Buffer safe to read while the server's goroutines
// are still writing log lines to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func newLoggingTestServer(t *testing.T, src string) (*Server, *syncBuffer) {
	t.Helper()
	srv := newTestServer(t, src)
	buf := &syncBuffer{}
	srv.SetLogger(rblog.New(buf, rblog.LevelInfo))
	return srv, buf
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

	runViaAPI(t, serve(t, srv), "hello")

	out := buf.String()
	assert.Contains(t, out, `"hello"`)
	assert.Contains(t, out, "started")
	assert.Contains(t, out, "finished")
}

func TestServer_LogsNonZeroExitAtWarn(t *testing.T) {
	srv, buf := newLoggingTestServer(t, "```bash {name=fails}\nexit 3\n```\n")

	runViaAPI(t, serve(t, srv), "fails")

	out := buf.String()
	assert.Contains(t, out, "WARN")
	assert.Contains(t, out, "exited with code 3")
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
	srv := NewServer(nil, SessionOptions{})
	srv.EnableWorkspace(root)
	buf := &syncBuffer{}
	srv.SetLogger(rblog.New(buf, rblog.LevelInfo))

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
