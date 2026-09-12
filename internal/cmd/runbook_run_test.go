package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/runbook"
)

func parseTestDoc(t *testing.T, src string) *runbook.Document {
	t.Helper()
	doc, err := (&runbook.GoldmarkParser{}).Parse([]byte(src), "/tmp/runbook.md")
	require.NoError(t, err)
	return doc
}

func TestParseSetFlags_Valid(t *testing.T) {
	got, err := parseSetFlags([]string{"A=1", "B=two"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"A": "1", "B": "two"}, got)
}

func TestParseSetFlags_InvalidFormatIsError(t *testing.T) {
	_, err := parseSetFlags([]string{"no-equals-sign"})
	assert.Error(t, err)
}

func TestExecuteNonInteractive_RunsStepsInOrderAndSucceeds(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=first}\necho one\n```\n\n```bash {name=second}\necho two\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(doc, nil, store, runbook.NewEngine(nil), &out)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "one")
	assert.Contains(t, out.String(), "two")
	assert.Len(t, store.Get().History, 2)
}

func TestExecuteNonInteractive_MissingSetFailsBeforeExecution(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=greet, input=NAME}\necho hi $NAME\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(doc, nil, store, runbook.NewEngine(nil), &out)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NAME")
	assert.Empty(t, store.Get().History)
}

func TestExecuteNonInteractive_ConfirmStepFailsClosed(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=danger, confirm=true}\necho hi\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(doc, nil, store, runbook.NewEngine(nil), &out)

	require.Error(t, err)
	assert.Empty(t, store.Get().History)
}

func TestExecuteNonInteractive_DangerPatternFailsClosed(t *testing.T) {
	doc := parseTestDoc(t, "---\ndanger_patterns:\n  - \"rm -rf\"\n---\n```bash {name=wipe}\nrm -rf /tmp/x\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(doc, nil, store, runbook.NewEngine(nil), &out)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rm -rf")
	assert.Empty(t, store.Get().History)
}

func TestExecuteNonInteractive_NonZeroExitStopsRemainingSteps(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=fails}\nexit 1\n```\n\n```bash {name=never}\necho should-not-run\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(doc, nil, store, runbook.NewEngine(nil), &out)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fails")
	assert.Len(t, store.Get().History, 1, "second step must not have run")
	assert.NotContains(t, out.String(), "should-not-run")
}
