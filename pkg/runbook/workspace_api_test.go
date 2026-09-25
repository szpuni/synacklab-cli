package runbook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureA is a runbook whose one step leaves the session non-empty.
const captureA = "```bash {name=set, capture=A}\nexport A=1\n```\n"

func newWorkspaceServer(t *testing.T, root string, opts SessionOptions) *Server {
	t.Helper()
	srv := NewServer(nil, opts)
	srv.EnableWorkspace(root)
	return srv
}

func writeRunbook(t *testing.T, path, src string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
}

func postOpen(t *testing.T, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(raw)))
	return rec
}

type docJSON struct {
	Active     bool   `json:"active"`
	Workspace  bool   `json:"workspace"`
	ActiveFile string `json:"active_file"`
	Blocks     []struct {
		Step *struct {
			Name string `json:"name"`
		} `json:"step"`
	} `json:"blocks"`
}

func getDoc(t *testing.T, h http.Handler) docJSON {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/doc", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var got docJSON
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return got
}

func TestHandleGetFiles_ReturnsTreeWhenWorkspaceEnabled(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "a.md"))
	srv := newWorkspaceServer(t, root, SessionOptions{})

	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "a.md")
}

func TestHandleGetFiles_404WhenWorkspaceDisabled(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlePostOpen_SwitchesActiveDocument(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "deploy.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "deploy.md"})

	require.Equal(t, http.StatusOK, rec.Code)
	doc := getDoc(t, srv.Routes())
	assert.True(t, doc.Active)
	require.Len(t, doc.Blocks, 1)
	assert.Equal(t, "hello", doc.Blocks[0].Step.Name)
}

func TestHandlePostOpen_DefaultsSessionCwdToFilesOwnDirectory(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "sub", "deploy.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})

	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "sub/deploy.md"}).Code)

	assert.Equal(t, filepath.Join(root, "sub"), getSession(t, srv.Routes()).Cwd)
}

func TestHandlePostOpen_UsesConfiguredDefaultCwdOverride(t *testing.T) {
	root := t.TempDir()
	cwdOverride := t.TempDir()
	writeRunbook(t, filepath.Join(root, "deploy.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{Cwd: cwdOverride})

	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "deploy.md"}).Code)

	assert.Equal(t, cwdOverride, getSession(t, srv.Routes()).Cwd)
}

func TestHandlePostOpen_ResetRestoresConfiguredDefaultCwd(t *testing.T) {
	root := t.TempDir()
	cwdOverride := t.TempDir()
	writeRunbook(t, filepath.Join(root, "deploy.md"), "```bash {name=cd, set_cwd=true}\ncd /\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{Cwd: cwdOverride})
	ts := serve(t, srv)
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "deploy.md"}).Code)
	runViaAPI(t, ts, "cd")
	require.Equal(t, "/", getSession(t, srv.Routes()).Cwd)

	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/session/reset", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, cwdOverride, getSession(t, srv.Routes()).Cwd)
}

func TestHandlePostOpen_RejectsNonMdFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hi"), 0o644))
	srv := newWorkspaceServer(t, root, SessionOptions{})

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "notes.txt"})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, getDoc(t, srv.Routes()).Active)
}

func TestHandlePostOpen_MalformedRunbookIs400(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "bad.md"), "```bash {name=x\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "bad.md"})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, getDoc(t, srv.Routes()).Active)
}

func TestHandlePostOpen_DirectoryIs404(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir.md"), 0o755))
	srv := newWorkspaceServer(t, root, SessionOptions{})

	assert.Equal(t, http.StatusNotFound, postOpen(t, srv.Routes(), map[string]string{"file": "dir.md"}).Code)
}

func TestHandlePostOpen_PathTraversalStaysWithinRoot(t *testing.T) {
	root := t.TempDir()
	srv := newWorkspaceServer(t, root, SessionOptions{})

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "../../../etc/passwd.md"})

	// The traversal is neutralized to a path under root, which doesn't
	// exist there, so this must 404 — not read a real /etc/passwd.
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlePostOpen_404WhenWorkspaceDisabled(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	assert.Equal(t, http.StatusNotFound, postOpen(t, srv.Routes(), map[string]string{"file": "x.md"}).Code)
}

func TestHandleGetDoc_NoActiveDocumentInWorkspaceMode(t *testing.T) {
	srv := newWorkspaceServer(t, t.TempDir(), SessionOptions{})

	got := getDoc(t, srv.Routes())

	assert.False(t, got.Active)
	assert.True(t, got.Workspace)
}

func TestHandlePostStepRun_NoActiveDocumentReturnsConflict(t *testing.T) {
	srv := newWorkspaceServer(t, t.TempDir(), SessionOptions{})

	req := httptest.NewRequest(http.MethodPost, "/api/steps/anything/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestHandlePostOpen_ReopeningActiveFileIsNoOpAndKeepsSession(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "deploy.md"), captureA)
	srv := newWorkspaceServer(t, root, SessionOptions{})
	ts := serve(t, srv)
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "deploy.md"}).Code)
	runViaAPI(t, ts, "set")

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "deploy.md"})

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Unchanged bool `json:"unchanged"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.Unchanged)
	assert.Equal(t, "1", getSession(t, srv.Routes()).Vars["A"])
}

func TestHandlePostOpen_SwitchingAwayFromNonEmptySessionRequiresForce(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "a.md"), captureA)
	writeRunbook(t, filepath.Join(root, "b.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})
	ts := serve(t, srv)
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "a.md"}).Code)
	runViaAPI(t, ts, "set")

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "b.md"})

	require.Equal(t, http.StatusConflict, rec.Code)
	var got struct {
		RequiresConfirmation bool `json:"requires_confirmation"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.True(t, got.RequiresConfirmation)
	assert.Equal(t, "a.md", getDoc(t, srv.Routes()).ActiveFile)
	assert.Equal(t, "1", getSession(t, srv.Routes()).Vars["A"], "session must not be discarded without confirmation")
}

func TestHandlePostOpen_ForceSwitchesAndReplacesSession(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "a.md"), captureA)
	writeRunbook(t, filepath.Join(root, "b.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})
	ts := serve(t, srv)
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "a.md"}).Code)
	runViaAPI(t, ts, "set")

	rec := postOpen(t, srv.Routes(), map[string]any{"file": "b.md", "force": true})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "b.md", getDoc(t, srv.Routes()).ActiveFile)
	got := getSession(t, srv.Routes())
	assert.Empty(t, got.Vars)
	assert.Empty(t, got.History)
}

func TestHandlePostOpen_SwitchingAwayFromEmptySessionNeedsNoConfirmation(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "a.md"), "```bash {name=hello}\necho hi\n```\n")
	writeRunbook(t, filepath.Join(root, "b.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "a.md"}).Code)

	rec := postOpen(t, srv.Routes(), map[string]string{"file": "b.md"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "b.md", getDoc(t, srv.Routes()).ActiveFile)
}

func TestHandlePostOpen_RefusesSwitchWhileExecutionInFlightRegardlessOfForce(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "a.md"), "```bash {name=slow}\nsleep 5\n```\n")
	writeRunbook(t, filepath.Join(root, "b.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})
	ts := serve(t, srv)
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "a.md"}).Code)
	slowID := startRun(t, ts, "slow")
	conn := wsDial(t, ts, slowID)

	rec := postOpen(t, srv.Routes(), map[string]any{"file": "b.md", "force": true})

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "a.md", getDoc(t, srv.Routes()).ActiveFile)

	cancelResp, err := http.Post(ts.URL+"/api/executions/"+slowID+"/cancel", "application/json", nil)
	require.NoError(t, err)
	cancelResp.Body.Close()
	lastEventOf(t, conn)
}

func TestHandleGetDoc_IncludesActiveFileAfterOpen(t *testing.T) {
	root := t.TempDir()
	writeRunbook(t, filepath.Join(root, "sub", "deploy.md"), "```bash {name=hello}\necho hi\n```\n")
	srv := newWorkspaceServer(t, root, SessionOptions{})
	require.Equal(t, http.StatusOK, postOpen(t, srv.Routes(), map[string]string{"file": "sub/deploy.md"}).Code)

	assert.Equal(t, "sub/deploy.md", getDoc(t, srv.Routes()).ActiveFile)
}
