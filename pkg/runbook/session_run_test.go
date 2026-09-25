package runbook

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rblog "synacklab/pkg/log"
)

// openTestSession writes src as deploy.md in a fresh directory and opens it,
// so the session's default cwd is that directory.
func openTestSession(t *testing.T, src string, opts SessionOptions) *Session {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deploy.md")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	sess, err := OpenSession(path, opts)
	require.NoError(t, err)
	return sess
}

func drainEvents(t *testing.T, ch <-chan Event) (stdout, stderr []string, done Event) {
	t.Helper()
	for ev := range ch {
		switch ev.Type {
		case EventStdout:
			stdout = append(stdout, ev.Data)
		case EventStderr:
			stderr = append(stderr, ev.Data)
		case EventDone:
			done = ev
		}
	}
	return
}

// runToCompletion starts step name and waits for its done event.
func runToCompletion(t *testing.T, sess *Session, name string, inputs map[string]string) (stdout, stderr []string, done Event) {
	t.Helper()
	events, _, err := sess.Start(context.Background(), name, inputs, false)
	require.NoError(t, err)
	return drainEvents(t, events)
}

func samePath(t *testing.T, want, got string) {
	t.Helper()
	w, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)
	g, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, w, g)
}

func TestSession_BashStepStreamsStdoutAndStderr(t *testing.T) {
	sess := openTestSession(t, "```bash {name=hello}\necho out\n>&2 echo err\n```\n", SessionOptions{})

	stdout, stderr, done := runToCompletion(t, sess, "hello", nil)

	assert.Equal(t, []string{"out"}, stdout)
	assert.Equal(t, []string{"err"}, stderr)
	assert.Equal(t, 0, done.ExitCode)
}

func TestSession_PythonStepRuns(t *testing.T) {
	sess := openTestSession(t, "```python {name=py}\nprint('hi')\n```\n", SessionOptions{})

	stdout, _, done := runToCompletion(t, sess, "py", nil)

	assert.Equal(t, []string{"hi"}, stdout)
	assert.Equal(t, 0, done.ExitCode)
}

func TestSession_NonZeroExitIsReportedAndRecorded(t *testing.T) {
	sess := openTestSession(t, "```bash {name=fail}\nexit 3\n```\n", SessionOptions{})

	_, _, done := runToCompletion(t, sess, "fail", nil)

	assert.Equal(t, 3, done.ExitCode)
	require.Len(t, sess.State().History, 1)
	assert.Equal(t, 3, sess.State().History[0].ExitCode)
}

func TestSession_StartsInDocumentDirectoryByDefault(t *testing.T) {
	sess := openTestSession(t, "```bash {name=pwd}\npwd\n```\n", SessionOptions{})

	stdout, _, _ := runToCompletion(t, sess, "pwd", nil)

	require.Len(t, stdout, 1)
	samePath(t, sess.Document().Dir, stdout[0])
}

func TestSession_CwdOptionOverridesDocumentDirectory(t *testing.T) {
	override := t.TempDir()
	sess := openTestSession(t, "```bash {name=pwd}\npwd\n```\n", SessionOptions{Cwd: override})

	stdout, _, _ := runToCompletion(t, sess, "pwd", nil)

	require.Len(t, stdout, 1)
	samePath(t, override, stdout[0])
}

func TestSession_StepCwdIsRelativeToSessionCwd(t *testing.T) {
	sess := openTestSession(t, "```bash {name=pwd, cwd=sub}\npwd\n```\n", SessionOptions{})
	sub := filepath.Join(sess.Document().Dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	stdout, _, _ := runToCompletion(t, sess, "pwd", nil)

	require.Len(t, stdout, 1)
	samePath(t, sub, stdout[0])
}

func TestSession_SetCwdCarriesIntoLaterSteps(t *testing.T) {
	sess := openTestSession(t, "```bash {name=cd, set_cwd=true}\nmkdir -p sub && cd sub\n```\n\n```bash {name=pwd}\npwd\n```\n", SessionOptions{})

	runToCompletion(t, sess, "cd", nil)
	stdout, _, _ := runToCompletion(t, sess, "pwd", nil)

	require.Len(t, stdout, 1)
	samePath(t, filepath.Join(sess.Document().Dir, "sub"), stdout[0])
}

func TestSession_CapturedVarsReachLaterStepsUnderBothNames(t *testing.T) {
	sess := openTestSession(t, "```bash {name=get_ip, capture=IP}\nexport IP=1.2.3.4\n```\n\n"+
		"```bash {name=use}\necho \"$IP\"\n```\n", SessionOptions{})

	runToCompletion(t, sess, "get_ip", nil)
	stdout, _, _ := runToCompletion(t, sess, "use", nil)

	assert.Equal(t, []string{"1.2.3.4"}, stdout)
	vars := sess.State().Vars
	assert.Equal(t, "1.2.3.4", vars["IP"])
	assert.Equal(t, "1.2.3.4", vars["get_ip.IP"])
}

func TestSession_InputTakesPrecedenceOverCapturedVar(t *testing.T) {
	sess := openTestSession(t, "```bash {name=set, capture=FOO}\nexport FOO=session-value\n```\n\n"+
		"```bash {name=use, input=FOO}\necho $FOO\n```\n", SessionOptions{})

	runToCompletion(t, sess, "set", nil)
	stdout, _, _ := runToCompletion(t, sess, "use", map[string]string{"FOO": "input-value"})

	assert.Equal(t, []string{"input-value"}, stdout)
}

func TestSession_UndeclaredInputsDoNotReachTheStep(t *testing.T) {
	sess := openTestSession(t, "```bash {name=use}\necho \"[$SNEAKY]\"\n```\n", SessionOptions{})

	stdout, _, _ := runToCompletion(t, sess, "use", map[string]string{"SNEAKY": "x"})

	assert.Equal(t, []string{"[]"}, stdout)
}

func TestSession_MissingInputFailsBeforeSpawning(t *testing.T) {
	sess := openTestSession(t, "```bash {name=greet, input=NAME, sensitive=NAME}\necho hi $NAME\n```\n", SessionOptions{})

	_, _, err := sess.Start(context.Background(), "greet", nil, false)

	var rbErr *Error
	require.ErrorAs(t, err, &rbErr)
	assert.Equal(t, ErrorTypeValidation, rbErr.Type)
	assert.Contains(t, err.Error(), "NAME")
	assert.Empty(t, sess.State().History)
}

func TestSession_UnknownStepIsNotFound(t *testing.T) {
	sess := openTestSession(t, "```bash {name=hello}\necho hi\n```\n", SessionOptions{})

	_, _, err := sess.Start(context.Background(), "missing", nil, false)

	var rbErr *Error
	require.ErrorAs(t, err, &rbErr)
	assert.Equal(t, ErrorTypeNotFound, rbErr.Type)
}

func TestSession_ConfirmationGate(t *testing.T) {
	src := "---\ndanger_patterns:\n  - \"rm -rf\"\n---\n" +
		"```bash {name=wipe, confirm=false}\nrm -rf ./nothing-here\n```\n\n" +
		"```bash {name=asks, confirm=true}\necho hi\n```\n\n" +
		"```bash {name=safe}\necho hi\n```\n"

	cases := []struct {
		step       string
		confirmed  bool
		wantErr    bool
		wantReason string
	}{
		{step: "wipe", confirmed: false, wantErr: true, wantReason: "rm -rf"},
		{step: "wipe", confirmed: true},
		{step: "asks", confirmed: false, wantErr: true, wantReason: "confirm=true"},
		{step: "asks", confirmed: true},
		{step: "safe", confirmed: false},
	}
	for _, tc := range cases {
		sess := openTestSession(t, src, SessionOptions{})
		events, _, err := sess.Start(context.Background(), tc.step, nil, tc.confirmed)
		if tc.wantErr {
			require.Error(t, err, "%s confirmed=%v", tc.step, tc.confirmed)
			assert.Contains(t, err.Error(), tc.wantReason)
			assert.Empty(t, sess.State().History, "a refused step must not run")
			continue
		}
		require.NoError(t, err, "%s confirmed=%v", tc.step, tc.confirmed)
		drainEvents(t, events)
	}
}

func TestSession_StepTimeoutKillsProcessPromptly(t *testing.T) {
	sess := openTestSession(t, "```bash {name=slow, timeout=200ms}\nsleep 5\n```\n", SessionOptions{})

	start := time.Now()
	_, _, done := runToCompletion(t, sess, "slow", nil)

	assert.True(t, done.TimedOut)
	assert.False(t, done.Canceled)
	assert.Less(t, time.Since(start), 4*time.Second, "process group should be killed promptly on timeout")
}

func TestSession_DocumentDefaultTimeoutApplies(t *testing.T) {
	sess := openTestSession(t, "---\ndefault_timeout: 200ms\n---\n```bash {name=slow}\nsleep 5\n```\n", SessionOptions{})

	_, _, done := runToCompletion(t, sess, "slow", nil)

	assert.True(t, done.TimedOut)
}

func TestSession_CancelStopsStepAndRecordsIt(t *testing.T) {
	sess := openTestSession(t, "```bash {name=slow}\nsleep 30\n```\n", SessionOptions{})

	events, cancel, err := sess.Start(context.Background(), "slow", nil, false)
	require.NoError(t, err)
	start := time.Now()
	cancel()
	_, _, done := drainEvents(t, events)

	assert.Less(t, time.Since(start), 4*time.Second, "cancel should stop the process group promptly, not wait for the timeout")
	assert.True(t, done.Canceled)
	assert.False(t, done.TimedOut)
	require.Len(t, sess.State().History, 1)
	assert.True(t, sess.State().History[0].Canceled)
}

func TestSession_TemplateReferencesResolveBeforeSpawning(t *testing.T) {
	sess := openTestSession(t, "```bash {name=first, capture=X}\necho out1\nexport X=from-vars\n```\n\n"+
		"```bash {name=second}\necho '{{vars.X}}' {{steps.first.exit_code}}\n```\n", SessionOptions{})

	runToCompletion(t, sess, "first", nil)
	stdout, _, _ := runToCompletion(t, sess, "second", nil)

	assert.Equal(t, []string{"from-vars 0"}, stdout)
}

func TestSession_UnresolvedTemplateReferenceFailsBeforeSpawning(t *testing.T) {
	sess := openTestSession(t, "```bash {name=tmpl}\necho {{vars.missing}}\n```\n", SessionOptions{})

	_, _, err := sess.Start(context.Background(), "tmpl", nil, false)

	var rbErr *Error
	require.ErrorAs(t, err, &rbErr)
	assert.Equal(t, ErrorTypeTemplate, rbErr.Type)
	assert.Empty(t, sess.State().History)
}

func TestSession_EveryExecutionIsLoggedToDiskInOrder(t *testing.T) {
	logDir := t.TempDir()
	sess := openTestSession(t, "```bash {name=a}\necho out\n>&2 echo err\n```\n\n```bash {name=b}\necho two\n```\n",
		SessionOptions{Logs: NewFileLogWriter(logDir)})

	runToCompletion(t, sess, "a", nil)
	runToCompletion(t, sess, "b", nil)

	history := sess.State().History
	require.Len(t, history, 2)
	assert.Equal(t, "1-a.log", filepath.Base(history[0].LogPath))
	assert.Equal(t, "2-b.log", filepath.Base(history[1].LogPath))
	content, err := os.ReadFile(history[0].LogPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "out")
	assert.Contains(t, string(content), "err")
}

func TestSession_ResetClearsVarsAndCwdButKeepsHistory(t *testing.T) {
	override := t.TempDir()
	sess := openTestSession(t, "```bash {name=cd, capture=A, set_cwd=true}\nexport A=1\nmkdir -p sub && cd sub\n```\n",
		SessionOptions{Cwd: override})
	runToCompletion(t, sess, "cd", nil)
	require.NotEmpty(t, sess.State().Vars)

	sess.Reset()

	state := sess.State()
	assert.Empty(t, state.Vars)
	assert.Equal(t, override, state.Cwd, "reset must return to the configured default cwd")
	assert.Len(t, state.History, 1, "reset should not clear execution history")
}

func TestSession_StateIsAnIndependentSnapshot(t *testing.T) {
	sess := openTestSession(t, "```bash {name=set, capture=A}\nexport A=1\n```\n", SessionOptions{})
	runToCompletion(t, sess, "set", nil)

	snapshot := sess.State()
	snapshot.Vars["A"] = "mutated"
	snapshot.History[0].StepName = "mutated"

	assert.Equal(t, "1", sess.State().Vars["A"])
	assert.Equal(t, "set", sess.State().History[0].StepName)
}

func TestOpenSession_MissingFileIsNotFound(t *testing.T) {
	_, err := OpenSession(filepath.Join(t.TempDir(), "missing.md"), SessionOptions{})

	var rbErr *Error
	require.ErrorAs(t, err, &rbErr)
	assert.Equal(t, ErrorTypeNotFound, rbErr.Type)
}

func TestOpenSession_MalformedDocumentIsParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.md")
	require.NoError(t, os.WriteFile(path, []byte("```bash {name=x\necho hi\n```\n"), 0o644))

	_, err := OpenSession(path, SessionOptions{})

	var rbErr *Error
	require.ErrorAs(t, err, &rbErr)
	assert.Equal(t, ErrorTypeParse, rbErr.Type)
}

func TestSession_RunAllRunsStepsInOrder(t *testing.T) {
	sess := openTestSession(t, "```bash {name=first}\necho one\n```\n\n```bash {name=second, input=WHO}\necho two $WHO\n```\n", SessionOptions{})
	var out, logs bytes.Buffer

	err := sess.RunAll(context.Background(), map[string]string{"WHO": "you", "UNUSED": "x"}, &out, rblog.New(&logs, rblog.LevelInfo))

	require.NoError(t, err)
	assert.Equal(t, "one\ntwo you\n", out.String())
	assert.Len(t, sess.State().History, 2)
	assert.Contains(t, logs.String(), `running step "first"`)
	assert.Contains(t, logs.String(), `step "second" finished`)
}

func TestSession_RunAllStopsAtFirstFailure(t *testing.T) {
	sess := openTestSession(t, "```bash {name=fails}\nexit 1\n```\n\n```bash {name=never}\necho should-not-run\n```\n", SessionOptions{})
	var out, logs bytes.Buffer

	err := sess.RunAll(context.Background(), nil, &out, rblog.New(&logs, rblog.LevelInfo))

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"fails" exited with code 1`)
	assert.Len(t, sess.State().History, 1, "second step must not have run")
	assert.NotContains(t, out.String(), "should-not-run")
	assert.Contains(t, logs.String(), "WARN")
}

func TestSession_RunAllFailsClosedOnConfirmationAndMissingInput(t *testing.T) {
	for _, src := range []string{
		"```bash {name=danger, confirm=true}\necho hi\n```\n",
		"---\ndanger_patterns:\n  - \"rm -rf\"\n---\n```bash {name=wipe}\nrm -rf ./nothing-here\n```\n",
		"```bash {name=greet, input=NAME}\necho hi $NAME\n```\n",
	} {
		sess := openTestSession(t, src, SessionOptions{})

		err := sess.RunAll(context.Background(), nil, &bytes.Buffer{}, rblog.Discard())

		var rbErr *Error
		require.ErrorAs(t, err, &rbErr, src)
		assert.Equal(t, ErrorTypeValidation, rbErr.Type, src)
		assert.Empty(t, sess.State().History, "nothing may run: %s", src)
	}
}

// TestSession_RunAllContextCancellationKillsInFlightStep is a regression
// test for orphaned processes: every step runs in its own process group, so
// only ctx (wired to SIGINT/SIGTERM by the CLI) can stop it.
func TestSession_RunAllContextCancellationKillsInFlightStep(t *testing.T) {
	sess := openTestSession(t, "```bash {name=slow}\nsleep 7\n```\n", SessionOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := sess.RunAll(ctx, nil, &bytes.Buffer{}, rblog.Discard())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
	assert.Less(t, time.Since(start), 4*time.Second, "canceling ctx should kill the running step promptly")
	psOut, psErr := exec.Command("ps", "aux").Output()
	require.NoError(t, psErr)
	assert.NotContains(t, string(psOut), "sleep 7", "no orphaned process should remain after ctx is canceled")
}
