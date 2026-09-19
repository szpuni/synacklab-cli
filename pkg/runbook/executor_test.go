package runbook

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func drainEvents(t *testing.T, ch <-chan Event) (stdout, stderr []string, done Event) {
	t.Helper()
	for ev := range ch {
		switch ev.Type {
		case "stdout":
			stdout = append(stdout, ev.Data)
		case "stderr":
			stderr = append(stderr, ev.Data)
		case "done":
			done = ev
		}
	}
	return
}

func runStep(t *testing.T, step *Step, inputs map[string]string, timeout time.Duration, store SessionStore) (stdout, stderr []string, done Event) {
	t.Helper()
	engine := NewEngine(nil)
	_, events, _, err := engine.Run(context.Background(), step, inputs, timeout, store)
	require.NoError(t, err)
	return drainEvents(t, events)
}

func TestEngine_Run_BashEchoSucceeds(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "echo", Lang: "bash", Source: "echo hello\n"}

	stdout, _, done := runStep(t, step, nil, time.Second, store)

	assert.Equal(t, []string{"hello"}, stdout)
	assert.Equal(t, 0, done.ExitCode)
}

func TestEngine_Run_PythonPrintSucceeds(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "py", Lang: "python", Source: "print('hi')\n"}

	stdout, _, done := runStep(t, step, nil, time.Second, store)

	assert.Equal(t, []string{"hi"}, stdout)
	assert.Equal(t, 0, done.ExitCode)
}

func TestEngine_Run_ExitCodePropagation(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "fail", Lang: "bash", Source: "exit 3\n"}

	_, _, done := runStep(t, step, nil, time.Second, store)

	assert.Equal(t, 3, done.ExitCode)
}

func TestEngine_Run_InputTakesPrecedenceOverSessionVar(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	store.SetVar("FOO", "session-value")
	step := &Step{Name: "echo-foo", Lang: "bash", Source: "echo $FOO\n"}

	stdout, _, _ := runStep(t, step, map[string]string{"FOO": "input-value"}, time.Second, store)

	assert.Equal(t, []string{"input-value"}, stdout)
}

func TestEngine_Run_StepCwdOverridesSessionCwd(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	store := NewSessionStore("s1", "/tmp/runbook.md", tmp)
	step := &Step{Name: "pwd", Lang: "bash", Source: "pwd\n", Cwd: "sub"}

	stdout, _, _ := runStep(t, step, nil, time.Second, store)

	require.Len(t, stdout, 1)
	resolved, err := filepath.EvalSymlinks(stdout[0])
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(sub)
	require.NoError(t, err)
	assert.Equal(t, expected, resolved)
}

func TestEngine_Run_TimeoutMarksExecutionTimedOut(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "slow", Lang: "bash", Source: "sleep 5\n"}

	start := time.Now()
	_, _, done := runStep(t, step, nil, 200*time.Millisecond, store)
	elapsed := time.Since(start)

	assert.True(t, done.TimedOut)
	assert.Less(t, elapsed, 4*time.Second, "process group should be killed promptly on timeout")
}

func TestEngine_Run_CaptureUpdatesSessionVars(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "get_ip", Lang: "bash", Source: "export OUT=42\n", Capture: []string{"OUT"}}

	_, _, done := runStep(t, step, nil, time.Second, store)

	assert.Equal(t, map[string]string{"OUT": "42"}, done.Captured)
	sess := store.Get()
	assert.Equal(t, "42", sess.Vars["get_ip.OUT"])
	assert.Equal(t, "42", sess.Vars["OUT"])
}

func TestEngine_Run_SetCwdUpdatesSession(t *testing.T) {
	tmp := t.TempDir()
	store := NewSessionStore("s1", "/tmp/runbook.md", tmp)
	step := &Step{Name: "cd", Lang: "bash", Source: "mkdir -p sub && cd sub\n", SetCwd: true}

	runStep(t, step, nil, time.Second, store)

	resolved, err := filepath.EvalSymlinks(store.Get().Cwd)
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(filepath.Join(tmp, "sub"))
	require.NoError(t, err)
	assert.Equal(t, expected, resolved)
}

func TestEngine_Run_TemplateSubstitutionAppliedBeforeSpawn(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	store.SetVar("X", "substituted-value")
	step := &Step{Name: "tmpl", Lang: "bash", Source: "echo {{vars.X}}\n"}

	stdout, _, _ := runStep(t, step, nil, time.Second, store)

	assert.Equal(t, []string{"substituted-value"}, stdout)
}

func TestEngine_Run_UnresolvedTemplateReferenceFailsBeforeSpawn(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "tmpl", Lang: "bash", Source: "echo {{vars.missing}}\n"}

	engine := NewEngine(nil)
	_, events, cancel, err := engine.Run(context.Background(), step, nil, time.Second, store)

	assert.Error(t, err)
	assert.Nil(t, events)
	assert.Nil(t, cancel)
}

func TestEngine_Run_CancelFuncStopsExecutionAndMarksCanceled(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "slow", Lang: "bash", Source: "sleep 30\n"}

	engine := NewEngine(nil)
	execution, events, cancel, err := engine.Run(context.Background(), step, nil, 10*time.Second, store)
	require.NoError(t, err)
	require.NotNil(t, cancel)

	start := time.Now()
	cancel()
	_, _, done := drainEvents(t, events)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 4*time.Second, "cancel should stop the process group promptly, not wait for the timeout")
	assert.True(t, done.Canceled)
	assert.False(t, done.TimedOut)
	assert.True(t, execution.Canceled)
	assert.False(t, execution.TimedOut)
}

func TestEngine_Run_WritesLogAndSetsLogPath(t *testing.T) {
	logDir := t.TempDir()
	store := NewSessionStore("s1", "/tmp/deploy.md", t.TempDir())
	step := &Step{Name: "logged", Lang: "bash", Source: "echo out\n>&2 echo err\n"}

	engine := NewEngine(NewFileLogWriter(logDir))
	execution, events, _, err := engine.Run(context.Background(), step, nil, time.Second, store)
	require.NoError(t, err)
	drainEvents(t, events)

	require.NotEmpty(t, execution.LogPath)
	content, readErr := os.ReadFile(execution.LogPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "out")
	assert.Contains(t, string(content), "err")

	sess := store.Get()
	require.Len(t, sess.History, 1)
	assert.Equal(t, execution.LogPath, sess.History[0].LogPath)
}
