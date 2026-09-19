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

func newWorkspaceServer(t *testing.T, root string) *Server {
	t.Helper()
	srv := NewServer(nil, nil, NewEngine(nil))
	srv.EnableWorkspace(root, "")
	return srv
}

func TestHandleGetFiles_ReturnsTreeWhenWorkspaceEnabled(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "a.md"))
	srv := newWorkspaceServer(t, root)

	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "a.md")
}

func TestHandleGetFiles_404WhenWorkspaceDisabled(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlePostOpen_SwitchesActiveDocument(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "deploy.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	doc, store := srv.active.get()
	require.NotNil(t, doc)
	require.NotNil(t, store)
	assert.Contains(t, doc.Steps, "hello")
}

func TestHandlePostOpen_DefaultsSessionCwdToFilesOwnDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "deploy.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "sub/deploy.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	_, store := srv.active.get()
	require.NotNil(t, store)
	assert.Equal(t, filepath.Join(root, "sub"), store.Get().Cwd)
}

func TestHandlePostOpen_UsesConfiguredDefaultCwdOverride(t *testing.T) {
	root := t.TempDir()
	cwdOverride := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := NewServer(nil, nil, NewEngine(nil))
	srv.EnableWorkspace(root, cwdOverride)

	body, _ := json.Marshal(map[string]string{"file": "deploy.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	_, store := srv.active.get()
	require.NotNil(t, store)
	assert.Equal(t, cwdOverride, store.Get().Cwd)
}

func TestHandlePostOpen_RejectsNonMdFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hi"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "notes.txt"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	doc, _ := srv.active.get()
	assert.Nil(t, doc)
}

func TestHandlePostOpen_PathTraversalStaysWithinRoot(t *testing.T) {
	root := t.TempDir()
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "../../../etc/passwd.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	// The traversal is neutralized to a path under root, which doesn't
	// exist there, so this must 404 — not read a real /etc/passwd.
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlePostOpen_404WhenWorkspaceDisabled(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	body, _ := json.Marshal(map[string]string{"file": "x.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGetDoc_NoActiveDocumentInWorkspaceMode(t *testing.T) {
	root := t.TempDir()
	srv := newWorkspaceServer(t, root)

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Active    bool `json:"active"`
		Workspace bool `json:"workspace"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.False(t, got.Active)
	assert.True(t, got.Workspace)
}

func TestHandlePostStepRun_NoActiveDocumentReturnsConflict(t *testing.T) {
	root := t.TempDir()
	srv := newWorkspaceServer(t, root)

	req := httptest.NewRequest(http.MethodPost, "/api/steps/anything/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestHandlePostOpen_ReopeningActiveFileIsNoOpAndKeepsSession(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "deploy.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	activeStore(srv).SetVar("A", "1")

	req2 := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var got struct {
		Unchanged bool `json:"unchanged"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &got))
	assert.True(t, got.Unchanged)
	assert.Equal(t, "1", activeStore(srv).Get().Vars["A"])
}

func TestHandlePostOpen_SwitchingAwayFromNonEmptySessionRequiresForce(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "a.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	activeStore(srv).SetVar("A", "1")

	switchBody, _ := json.Marshal(map[string]string{"file": "b.md"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(switchBody))
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusConflict, rec2.Code)
	var got struct {
		RequiresConfirmation bool `json:"requires_confirmation"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &got))
	assert.True(t, got.RequiresConfirmation)
	assert.Equal(t, "1", activeStore(srv).Get().Vars["A"], "session must not be discarded without confirmation")
}

func TestHandlePostOpen_ForceSwitchesAndReplacesSession(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "a.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	activeStore(srv).SetVar("A", "1")

	switchBody, _ := json.Marshal(map[string]any{"file": "b.md", "force": true})
	req2 := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(switchBody))
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Empty(t, activeStore(srv).Get().Vars)
}

func TestHandlePostOpen_SwitchingAwayFromEmptySessionNeedsNoConfirmation(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "a.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	switchBody, _ := json.Marshal(map[string]string{"file": "b.md"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(switchBody))
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestHandlePostOpen_RefusesSwitchWhileExecutionInFlightRegardlessOfForce(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.md"), []byte("```bash {name=slow}\nsleep 5\n```\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	body, _ := json.Marshal(map[string]string{"file": "a.md"})
	req := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	runReq := httptest.NewRequest(http.MethodPost, "/api/steps/slow/run", bytes.NewReader([]byte(`{}`)))
	runRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(runRec, runReq)
	require.Equal(t, http.StatusAccepted, runRec.Code)

	switchBody, _ := json.Marshal(map[string]any{"file": "b.md", "force": true})
	req2 := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(switchBody))
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusConflict, rec2.Code)
}

func TestHandleGetDoc_IncludesActiveFileAfterOpen(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "deploy.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))
	srv := newWorkspaceServer(t, root)

	openBody, _ := json.Marshal(map[string]string{"file": "sub/deploy.md"})
	openReq := httptest.NewRequest(http.MethodPost, "/api/open", bytes.NewReader(openBody))
	openRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(openRec, openReq)
	require.Equal(t, http.StatusOK, openRec.Code)

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	var got struct {
		ActiveFile string `json:"active_file"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "sub/deploy.md", got.ActiveFile)
}
