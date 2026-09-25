package runbook

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapture_BashExportsBecomeCapturedAndHiddenFromOutput(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "get", Lang: "bash", Source: "echo visible\nexport A=1 B='x=y'\n", Capture: []string{"A", "B"}}

	stdout, _, done := runStep(t, step, nil, 5*time.Second, store)

	assert.Equal(t, []string{"visible"}, stdout, "sentinel lines must never reach the output stream")
	assert.Equal(t, map[string]string{"A": "1", "B": "x=y"}, done.Captured)
	require.Len(t, store.Get().History, 1)
	assert.Equal(t, "visible\n", store.Get().History[0].Stdout, "sentinel lines must not leak into {{steps.X.stdout}}")
}

func TestCapture_PythonEnvironWritesBecomeCaptured(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "get", Lang: "python", Source: "import os\nprint('visible')\nos.environ['A'] = '1'\n", Capture: []string{"A"}}

	stdout, _, done := runStep(t, step, nil, 5*time.Second, store)

	assert.Equal(t, []string{"visible"}, stdout)
	assert.Equal(t, map[string]string{"A": "1"}, done.Captured)
}

func TestCapture_UnsetVariableIsCapturedAsEmpty(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "get", Lang: "bash", Source: "true\n", Capture: []string{"NEVER_SET_ANYWHERE"}}

	_, _, done := runStep(t, step, nil, 5*time.Second, store)

	assert.Equal(t, map[string]string{"NEVER_SET_ANYWHERE": ""}, done.Captured)
}

func TestCapture_PythonSetCwdMovesSessionAndStaysHidden(t *testing.T) {
	tmp := t.TempDir()
	store := NewSessionStore("s1", "/tmp/runbook.md", tmp)
	step := &Step{Name: "cd", Lang: "python", Source: "import os\nos.mkdir('sub')\nos.chdir('sub')\n", SetCwd: true}

	stdout, _, _ := runStep(t, step, nil, 5*time.Second, store)

	assert.Empty(t, stdout)
	resolved, err := filepath.EvalSymlinks(store.Get().Cwd)
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(filepath.Join(tmp, "sub"))
	require.NoError(t, err)
	assert.Equal(t, expected, resolved)
}

func TestCapture_WithoutCaptureOrSetCwdOutputIsUntouched(t *testing.T) {
	store := NewSessionStore("s1", "/tmp/runbook.md", t.TempDir())
	step := &Step{Name: "plain", Lang: "bash", Source: "echo one\necho two\n"}

	stdout, _, done := runStep(t, step, nil, 5*time.Second, store)

	assert.Equal(t, []string{"one", "two"}, stdout)
	assert.Empty(t, done.Captured)
}
