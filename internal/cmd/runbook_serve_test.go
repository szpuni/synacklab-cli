package cmd

import (
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

func TestBuildRunbookServer_ParsesDocAndDefaultsCwdToDocDir(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {name=hello}\necho hi\n```\n"), 0o644))

	srv, resolvedPath, err := buildRunbookServer(docPath, "")
	require.NoError(t, err)
	assert.Equal(t, docPath, resolvedPath)
	require.NotNil(t, srv)
}

func TestBuildRunbookServer_MissingFileIsError(t *testing.T) {
	_, _, err := buildRunbookServer("/nonexistent/does-not-exist.md", "")
	assert.Error(t, err)
}

func TestBuildRunbookServer_ParseErrorIsError(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "bad.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {name=x\necho hi\n```\n"), 0o644))

	_, _, err := buildRunbookServer(docPath, "")
	assert.Error(t, err)
}
