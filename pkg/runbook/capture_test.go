package runbook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTrailer_Bash(t *testing.T) {
	trailer := buildTrailer("bash", []string{"A", "B"}, false)
	assert.Equal(t, "echo \"__SYNACLAB_CAP__A=${A}\"\necho \"__SYNACLAB_CAP__B=${B}\"\n", trailer)
}

func TestBuildTrailer_BashWithSetCwd(t *testing.T) {
	trailer := buildTrailer("bash", nil, true)
	assert.Equal(t, "echo \"__SYNACLAB_CWD__$PWD\"\n", trailer)
}

func TestBuildTrailer_Python(t *testing.T) {
	trailer := buildTrailer("python", []string{"A"}, false)
	assert.Equal(t, "import os\nprint(f\"__SYNACLAB_CAP__A={os.environ.get('A','')}\")\n", trailer)
}

func TestBuildTrailer_PythonWithSetCwdOnly(t *testing.T) {
	trailer := buildTrailer("python", nil, true)
	assert.Equal(t, "import os\nprint(f\"__SYNACLAB_CWD__{os.getcwd()}\")\n", trailer)
}

func TestBuildTrailer_NoCaptureNoCwdIsEmpty(t *testing.T) {
	assert.Empty(t, buildTrailer("bash", nil, false))
	assert.Empty(t, buildTrailer("python", nil, false))
}

func TestParseCaptureOutput_ExtractsAndStripsSentinelLines(t *testing.T) {
	stdout := "hello\n__SYNACLAB_CAP__PUBLIC_IP=1.2.3.4\nworld\n"

	cleaned, captured, cwd, cwdFound := parseCaptureOutput(stdout)

	assert.Equal(t, "hello\nworld\n", cleaned)
	assert.Equal(t, map[string]string{"PUBLIC_IP": "1.2.3.4"}, captured)
	assert.False(t, cwdFound)
	assert.Empty(t, cwd)
}

func TestParseCaptureOutput_UnsetVarIsEmptyString(t *testing.T) {
	stdout := "__SYNACLAB_CAP__UNSET=\n"

	_, captured, _, _ := parseCaptureOutput(stdout)

	assert.Equal(t, map[string]string{"UNSET": ""}, captured)
}

func TestParseCaptureOutput_CwdSentinelParsed(t *testing.T) {
	stdout := "some output\n__SYNACLAB_CWD__/tmp/foo\n"

	cleaned, _, cwd, cwdFound := parseCaptureOutput(stdout)

	assert.Equal(t, "some output\n", cleaned)
	assert.True(t, cwdFound)
	assert.Equal(t, "/tmp/foo", cwd)
}

func TestParseCaptureOutput_NoSentinelsLeavesOutputUntouched(t *testing.T) {
	stdout := "plain output\nno sentinels here\n"

	cleaned, captured, cwd, cwdFound := parseCaptureOutput(stdout)

	assert.Equal(t, stdout, cleaned)
	assert.Empty(t, captured)
	assert.False(t, cwdFound)
	assert.Empty(t, cwd)
}
