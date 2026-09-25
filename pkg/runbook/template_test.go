package runbook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstitute_VarsReference(t *testing.T) {
	sess := &SessionState{Vars: map[string]string{"PUBLIC_IP": "1.2.3.4"}}

	out, err := Substitute("echo {{vars.PUBLIC_IP}}", sess)
	require.NoError(t, err)
	assert.Equal(t, "echo 1.2.3.4", out)
}

func TestSubstitute_StepsStdoutAndExitCode(t *testing.T) {
	sess := &SessionState{History: []Execution{
		{StepName: "get_ip", Stdout: "1.2.3.4\n", ExitCode: 0},
	}}

	out, err := Substitute("ip={{steps.get_ip.stdout}} code={{steps.get_ip.exit_code}}", sess)
	require.NoError(t, err)
	assert.Equal(t, "ip=1.2.3.4\n code=0", out)
}

func TestSubstitute_UsesMostRecentExecutionOfStep(t *testing.T) {
	sess := &SessionState{History: []Execution{
		{StepName: "x", Stdout: "old"},
		{StepName: "x", Stdout: "new"},
	}}

	out, err := Substitute("{{steps.x.stdout}}", sess)
	require.NoError(t, err)
	assert.Equal(t, "new", out)
}

func TestSubstitute_UnknownStepReferenceIsError(t *testing.T) {
	sess := &SessionState{}

	_, err := Substitute("{{steps.missing.stdout}}", sess)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestSubstitute_UnknownVarReferenceIsError(t *testing.T) {
	sess := &SessionState{Vars: map[string]string{}}

	_, err := Substitute("{{vars.missing}}", sess)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestSubstitute_NoShellEscaping(t *testing.T) {
	sess := &SessionState{Vars: map[string]string{"X": `$(rm -rf /); echo "pwned"`}}

	out, err := Substitute("run {{vars.X}}", sess)
	require.NoError(t, err)
	assert.Equal(t, `run $(rm -rf /); echo "pwned"`, out)
}

func TestSubstitute_NoTemplateReferencesIsNoop(t *testing.T) {
	sess := &SessionState{}

	out, err := Substitute("echo plain script\n", sess)
	require.NoError(t, err)
	assert.Equal(t, "echo plain script\n", out)
}
