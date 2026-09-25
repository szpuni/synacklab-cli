package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLoopbackBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":   true,
		"localhost":   true,
		"::1":         true,
		"0.0.0.0":     false,
		"192.168.1.5": false,
	}
	for bind, want := range cases {
		assert.Equal(t, want, isLoopbackBind(bind), "bind=%s", bind)
	}
}

func TestServeFlags_Defaults(t *testing.T) {
	assert.Equal(t, "4747", runbookServeCmd.Flags().Lookup("port").DefValue)
	assert.Equal(t, "127.0.0.1", runbookServeCmd.Flags().Lookup("bind").DefValue)
	assert.Equal(t, "", runbookServeCmd.Flags().Lookup("cwd").DefValue)
}

func TestResolveServeTarget_ExplicitFileReturnsItsDirAsRootWithFilePreselected(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	writeFile(t, docPath, "```bash\necho hi\n```\n")

	root, initialFile, err := resolveServeTarget(docPath)
	require.NoError(t, err)
	assert.Equal(t, dir, root)
	assert.Equal(t, docPath, initialFile)
}

func TestResolveServeTarget_DirectoryWithUnambiguousFilePreselectsIt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "RUNBOOK.md"), "content")

	root, initialFile, err := resolveServeTarget(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, root)
	assert.Equal(t, filepath.Join(dir, "RUNBOOK.md"), initialFile)
}

func TestResolveServeTarget_AmbiguousDirectoryHasNoErrorAndNoPreselection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "a")
	writeFile(t, filepath.Join(dir, "b.md"), "b")

	root, initialFile, err := resolveServeTarget(dir)
	require.NoError(t, err, "serve must not hard-fail on ambiguity — the sidebar lets the user pick")
	assert.Equal(t, dir, root)
	assert.Empty(t, initialFile)
}

func TestResolveServeTarget_EmptyDirectoryHasNoErrorAndNoPreselection(t *testing.T) {
	dir := t.TempDir()

	root, initialFile, err := resolveServeTarget(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, root)
	assert.Empty(t, initialFile)
}

func TestResolveServeTarget_NonexistentPathIsError(t *testing.T) {
	_, _, err := resolveServeTarget("/nonexistent/does-not-exist")
	assert.Error(t, err)
}

func TestBuildRunbookServer_WithInitialFileParsesAndLabelsIt(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))

	srv, label, err := buildRunbookServer(dir, docPath, "")
	require.NoError(t, err)
	assert.Equal(t, docPath, label)
	require.NotNil(t, srv)
}

func TestBuildRunbookServer_WithoutInitialFileStillBuildsAServer(t *testing.T) {
	dir := t.TempDir()

	srv, label, err := buildRunbookServer(dir, "", "")
	require.NoError(t, err)
	assert.Equal(t, dir, label)
	require.NotNil(t, srv)
}

func TestBuildRunbookServer_MissingInitialFileIsError(t *testing.T) {
	dir := t.TempDir()
	_, _, err := buildRunbookServer(dir, filepath.Join(dir, "does-not-exist.md"), "")
	assert.Error(t, err)
}

func TestBuildRunbookServer_ParseErrorIsError(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "bad.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {name=x\necho hi\n```\n"), 0o644))

	_, _, err := buildRunbookServer(dir, docPath, "")
	assert.Error(t, err)
}

// TestBuildRunbookServer_ActiveFileIsReportedWithRelativeArgs is a
// regression test: root/initialFile as passed on the command line are
// relative to the invocation directory, but the server's internal root is
// absolutized in EnableWorkspace. Parsing the document at a path that
// stays relative while root is absolute broke relativeToRoot (used for
// GET /api/doc's active_file), silently returning "" instead of the
// resolved file name — found by manually exercising a real serve
// invocation, not by an existing test.
func TestBuildRunbookServer_ActiveFileIsReportedWithRelativeArgs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "RUNBOOK.md"), []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	root, initialFile, err := resolveServeTarget("") // relative: "." and "RUNBOOK.md"
	require.NoError(t, err)

	srv, _, err := buildRunbookServer(root, initialFile, "")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	var got struct {
		ActiveFile string `json:"active_file"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "RUNBOOK.md", got.ActiveFile)
}

// TestBuildRunbookServer_ResetRestoresCwdFlag is a regression test: reset
// used to put the session back in the document's own directory, silently
// dropping serve's --cwd override.
func TestBuildRunbookServer_ResetRestoresCwdFlag(t *testing.T) {
	dir := t.TempDir()
	cwdFlag := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))

	srv, _, err := buildRunbookServer(dir, docPath, cwdFlag)
	require.NoError(t, err)
	routes := srv.Routes()

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/session/reset", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var got struct {
		Cwd string `json:"cwd"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, cwdFlag, got.Cwd)
}
