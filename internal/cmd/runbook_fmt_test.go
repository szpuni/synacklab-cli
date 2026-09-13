package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRunbookFmt_RewritesFileToCanonicalOrder(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "runbook.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {timeout=30s, name=x}\necho hi\n```\n"), 0o644))

	err := runRunbookFmt(nil, []string{docPath})
	require.NoError(t, err)

	got, err := os.ReadFile(docPath)
	require.NoError(t, err)
	assert.Equal(t, "```bash {name=x, timeout=30s}\necho hi\n```\n", string(got))
}

func TestRunRunbookFmt_ParseErrorLeavesFileUnmodified(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "bad.md")
	original := "```bash {name=x\necho hi\n```\n"
	require.NoError(t, os.WriteFile(docPath, []byte(original), 0o644))

	err := runRunbookFmt(nil, []string{docPath})
	require.Error(t, err)

	got, readErr := os.ReadFile(docPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(got))
}

func TestRunRunbookFmt_AlreadyFormattedIsNoOp(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "runbook.md")
	original := "```bash {name=x}\necho hi\n```\n"
	require.NoError(t, os.WriteFile(docPath, []byte(original), 0o644))

	err := runRunbookFmt(nil, []string{docPath})
	require.NoError(t, err)

	got, readErr := os.ReadFile(docPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(got))
}
