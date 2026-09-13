package cmd

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rblog "synacklab/pkg/log"
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

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.Discard())

	require.NoError(t, err)
	assert.Contains(t, out.String(), "one")
	assert.Contains(t, out.String(), "two")
	assert.Len(t, store.Get().History, 2)
}

func TestExecuteNonInteractive_LogsStepLifecycleAtInfo(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=first}\necho one\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out, logs bytes.Buffer

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.New(&logs, rblog.LevelInfo))

	require.NoError(t, err)
	assert.Contains(t, logs.String(), `"first"`)
	assert.Contains(t, logs.String(), "running step")
	assert.Contains(t, logs.String(), "finished")
}

func TestExecuteNonInteractive_LogsNonZeroExitAtWarn(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=fails}\nexit 1\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out, logs bytes.Buffer

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.New(&logs, rblog.LevelInfo))

	require.Error(t, err)
	assert.Contains(t, logs.String(), "WARN")
	assert.Contains(t, logs.String(), "exited with code 1")
}

func TestExecuteNonInteractive_MissingSetFailsBeforeExecution(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=greet, input=NAME}\necho hi $NAME\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.Discard())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NAME")
	assert.Empty(t, store.Get().History)
}

func TestExecuteNonInteractive_ConfirmStepFailsClosed(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=danger, confirm=true}\necho hi\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.Discard())

	require.Error(t, err)
	assert.Empty(t, store.Get().History)
}

func TestExecuteNonInteractive_DangerPatternFailsClosed(t *testing.T) {
	doc := parseTestDoc(t, "---\ndanger_patterns:\n  - \"rm -rf\"\n---\n```bash {name=wipe}\nrm -rf /tmp/x\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.Discard())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rm -rf")
	assert.Empty(t, store.Get().History)
}

func TestExecuteNonInteractive_NonZeroExitStopsRemainingSteps(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=fails}\nexit 1\n```\n\n```bash {name=never}\necho should-not-run\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	err := executeNonInteractive(context.Background(), doc, nil, store, runbook.NewEngine(nil), &out, rblog.Discard())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fails")
	assert.Len(t, store.Get().History, 1, "second step must not have run")
	assert.NotContains(t, out.String(), "should-not-run")
}

// TestExecuteNonInteractive_ContextCancellationKillsInFlightStep is a
// regression test for orphaned processes: previously every step ran under
// context.Background(), so nothing about the CLI's own lifecycle (e.g. a
// SIGINT the RunE wires up to cancel this context) could stop a
// currently-running step, which executes in its own process group.
func TestExecuteNonInteractive_ContextCancellationKillsInFlightStep(t *testing.T) {
	doc := parseTestDoc(t, "```bash {name=slow}\nsleep 5\n```\n")
	store := runbook.NewSessionStore("s1", doc.Path, t.TempDir())
	var out bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := executeNonInteractive(ctx, doc, nil, store, runbook.NewEngine(nil), &out, rblog.Discard())
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Less(t, elapsed, 4*time.Second, "canceling ctx should kill the running step promptly, not wait out its own sleep")

	psOut, psErr := exec.Command("ps", "aux").Output()
	require.NoError(t, psErr)
	assert.NotContains(t, string(psOut), "sleep 5", "no orphaned process should remain after ctx is canceled")
}
