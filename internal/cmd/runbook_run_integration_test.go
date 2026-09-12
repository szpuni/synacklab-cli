package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunRunbookRun_EndToEndWithSetAndDiskLogging drives the actual `run`
// RunE entrypoint (file read, parse, --set, real FileLogWriter under
// .synacklab) rather than the narrower executeNonInteractive unit tests,
// which use a nil log writer (Requirements 11.1, 11.2, 11.5).
func TestRunRunbookRun_EndToEndWithSetAndDiskLogging(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	require.NoError(t, os.WriteFile(docPath, []byte(
		"```bash {name=greet, input=NAME}\necho hello $NAME\n```\n"), 0o644))

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	origNonInteractive, origSets := runNonInteractive, runSetFlags
	runNonInteractive = true
	runSetFlags = []string{"NAME=world"}
	t.Cleanup(func() { runNonInteractive, runSetFlags = origNonInteractive, origSets })

	err = runRunbookRun(nil, []string{docPath})
	require.NoError(t, err)

	entries, readErr := os.ReadDir(findSessionDir(t, filepath.Join(dir, ".synacklab", "deploy")))
	require.NoError(t, readErr)
	require.Len(t, entries, 1)
	assert.Equal(t, "1-greet.log", entries[0].Name())
}

func findSessionDir(t *testing.T, docSlugDir string) string {
	t.Helper()
	entries, err := os.ReadDir(docSlugDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "expected exactly one session directory")
	return filepath.Join(docSlugDir, entries[0].Name(), "steps")
}

func TestRunRunbookRun_MissingNonInteractiveFlagIsError(t *testing.T) {
	origNonInteractive := runNonInteractive
	runNonInteractive = false
	t.Cleanup(func() { runNonInteractive = origNonInteractive })

	err := runRunbookRun(nil, []string{"unused.md"})
	assert.Error(t, err)
}
