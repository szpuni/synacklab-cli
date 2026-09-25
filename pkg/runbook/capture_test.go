package runbook

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapture_BashExportsBecomeCapturedAndHiddenFromOutput(t *testing.T) {
	sess := openTestSession(t, "```bash {name=get, capture=A,B}\necho visible\nexport A=1 B='x=y'\n```\n", SessionOptions{})

	stdout, _, done := runToCompletion(t, sess, "get", nil)

	assert.Equal(t, []string{"visible"}, stdout, "sentinel lines must never reach the output stream")
	assert.Equal(t, map[string]string{"A": "1", "B": "x=y"}, done.Captured)
	require.Len(t, sess.State().History, 1)
	assert.Equal(t, "visible\n", sess.State().History[0].Stdout, "sentinel lines must not leak into {{steps.X.stdout}}")
}

func TestCapture_PythonEnvironWritesBecomeCaptured(t *testing.T) {
	sess := openTestSession(t, "```python {name=get, capture=A}\nimport os\nprint('visible')\nos.environ['A'] = '1'\n```\n", SessionOptions{})

	stdout, _, done := runToCompletion(t, sess, "get", nil)

	assert.Equal(t, []string{"visible"}, stdout)
	assert.Equal(t, map[string]string{"A": "1"}, done.Captured)
}

func TestCapture_UnsetVariableIsCapturedAsEmpty(t *testing.T) {
	sess := openTestSession(t, "```bash {name=get, capture=NEVER_SET_ANYWHERE}\ntrue\n```\n", SessionOptions{})

	_, _, done := runToCompletion(t, sess, "get", nil)

	assert.Equal(t, map[string]string{"NEVER_SET_ANYWHERE": ""}, done.Captured)
}

func TestCapture_PythonSetCwdMovesSessionAndStaysHidden(t *testing.T) {
	sess := openTestSession(t, "```python {name=cd, set_cwd=true}\nimport os\nos.mkdir('sub')\nos.chdir('sub')\n```\n", SessionOptions{})

	stdout, _, _ := runToCompletion(t, sess, "cd", nil)

	assert.Empty(t, stdout)
	samePath(t, filepath.Join(sess.Document().Dir, "sub"), sess.State().Cwd)
}

func TestCapture_WithoutCaptureOrSetCwdOutputIsUntouched(t *testing.T) {
	sess := openTestSession(t, "```bash {name=plain}\necho one\necho two\n```\n", SessionOptions{})

	stdout, _, done := runToCompletion(t, sess, "plain", nil)

	assert.Equal(t, []string{"one", "two"}, stdout)
	assert.Empty(t, done.Captured)
}
