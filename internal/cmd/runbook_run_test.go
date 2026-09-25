package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSetFlags_Valid(t *testing.T) {
	got, err := parseSetFlags([]string{"A=1", "B=two"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "1", "B": "two"}, got)
}

func TestParseSetFlags_InvalidFormatIsError(t *testing.T) {
	_, err := parseSetFlags([]string{"no-equals-sign"})
	assert.Error(t, err)
}

func TestRunRunbookRun_MissingInputPointsAtSetFlag(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "deploy.md")
	require.NoError(t, os.WriteFile(docPath, []byte("```bash {name=greet, input=NAME}\necho hello $NAME\n```\n"), 0o644))
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	origNonInteractive, origSets := runNonInteractive, runSetFlags
	runNonInteractive, runSetFlags = true, nil
	t.Cleanup(func() { runNonInteractive, runSetFlags = origNonInteractive, origSets })

	err = runRunbookRun(nil, []string{docPath})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NAME")
	assert.Contains(t, err.Error(), "--set")
	_, statErr := os.Stat(filepath.Join(dir, ".synacklab"))
	assert.True(t, os.IsNotExist(statErr), "no step may have run, so nothing is logged")
}
