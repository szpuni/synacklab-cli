package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestResolveRunbookPath_ExplicitFileIsReturnedUnchanged(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	writeFile(t, docPath, "```bash\necho hi\n```\n")

	got, err := resolveRunbookPath(docPath)
	require.NoError(t, err)
	assert.Equal(t, docPath, got)
}

func TestResolveRunbookPath_DirectoryPrefersUppercaseRUNBOOKmd(t *testing.T) {
	dir := t.TempDir()
	if !isCaseSensitiveFS(t, dir) {
		t.Skip("RUNBOOK.md/runbook.md precedence can't be tested as two distinct files on a case-insensitive filesystem")
	}
	writeFile(t, filepath.Join(dir, "RUNBOOK.md"), "upper")
	writeFile(t, filepath.Join(dir, "runbook.md"), "lower")

	got, err := resolveRunbookPath(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "RUNBOOK.md"), got)
}

func TestResolveRunbookPath_DirectoryFallsBackToLowercaseRunbookmd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "runbook.md"), "lower")

	got, err := resolveRunbookPath(dir)
	require.NoError(t, err)
	content, readErr := os.ReadFile(got)
	require.NoError(t, readErr)
	assert.Equal(t, "lower", string(content))
}

// isCaseSensitiveFS reports whether dir's filesystem treats "a" and "A" as
// distinct files (true on most Linux filesystems, false on default macOS
// APFS/HFS+).
func isCaseSensitiveFS(t *testing.T, dir string) bool {
	t.Helper()
	lower := filepath.Join(dir, "case-check")
	upper := filepath.Join(dir, "CASE-CHECK")
	writeFile(t, lower, "x")
	_, err := os.Stat(upper)
	sensitive := os.IsNotExist(err)
	require.NoError(t, os.Remove(lower))
	return sensitive
}

func TestResolveRunbookPath_DirectoryWithSingleMarkdownFileFallsBack(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "deploy.md"), "content")

	got, err := resolveRunbookPath(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "deploy.md"), got)
}

func TestResolveRunbookPath_DirectoryWithMultipleMarkdownFilesIsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "a")
	writeFile(t, filepath.Join(dir, "b.md"), "b")

	_, err := resolveRunbookPath(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.md")
	assert.Contains(t, err.Error(), "b.md")
}

func TestResolveRunbookPath_DirectoryWithNoMarkdownFilesIsError(t *testing.T) {
	dir := t.TempDir()

	_, err := resolveRunbookPath(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNBOOK.md")
}

func TestResolveRunbookPath_EmptyArgDefaultsToCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "RUNBOOK.md"), "content")

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	got, err := resolveRunbookPath("")
	require.NoError(t, err)
	assert.Equal(t, "RUNBOOK.md", got)
}

func TestResolveRunbookPath_NonexistentPathIsError(t *testing.T) {
	_, err := resolveRunbookPath("/nonexistent/path/does-not-exist")
	assert.Error(t, err)
}
