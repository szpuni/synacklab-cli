package runbook

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileLogWriter_WritesExpectedPathAndContent(t *testing.T) {
	base := t.TempDir()
	w := NewFileLogWriter(base)

	exec := Execution{
		StepName: "get_ip",
		Started:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Duration: 842 * time.Millisecond,
		ExitCode: 0,
		Stdout:   "1.2.3.4\n",
		Stderr:   "",
	}

	path, err := w.Write("/home/user/runbooks/deploy.md", "sess-1", 1, exec)
	require.NoError(t, err)

	expectedPath := filepath.Join(base, "deploy", "sess-1", "steps", "1-get_ip.log")
	assert.Equal(t, expectedPath, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "1.2.3.4")
	assert.Contains(t, string(content), "exit_code: 0")
	assert.Contains(t, string(content), "842ms")
}

func TestFileLogWriter_IncludesStderrAndTimeoutFlag(t *testing.T) {
	base := t.TempDir()
	w := NewFileLogWriter(base)

	exec := Execution{
		StepName: "slow",
		ExitCode: -1,
		TimedOut: true,
		Stdout:   "partial output\n",
		Stderr:   "some warning\n",
	}

	path, err := w.Write("/tmp/runbook.md", "sess-2", 3, exec)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "partial output")
	assert.Contains(t, string(content), "some warning")
	assert.Contains(t, string(content), "timed_out: true")
}

func TestFileLogWriter_WrittenEvenWithoutCapture(t *testing.T) {
	base := t.TempDir()
	w := NewFileLogWriter(base)

	path, err := w.Write("/tmp/runbook.md", "sess-3", 1, Execution{StepName: "no-capture", Stdout: "hi\n"})
	require.NoError(t, err)

	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func TestFileLogWriter_SequentialNamingPreservesOrder(t *testing.T) {
	base := t.TempDir()
	w := NewFileLogWriter(base)

	p1, err := w.Write("/tmp/runbook.md", "sess-4", 1, Execution{StepName: "first"})
	require.NoError(t, err)
	p2, err := w.Write("/tmp/runbook.md", "sess-4", 2, Execution{StepName: "second"})
	require.NoError(t, err)

	assert.Equal(t, "1-first.log", filepath.Base(p1))
	assert.Equal(t, "2-second.log", filepath.Base(p2))
}
