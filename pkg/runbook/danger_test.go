package runbook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesDanger_Match(t *testing.T) {
	matched, pattern := MatchesDanger("rm -rf /var/lib/data", []string{"terraform destroy", "rm -rf"})
	assert.True(t, matched)
	assert.Equal(t, "rm -rf", pattern)
}

func TestMatchesDanger_NoMatch(t *testing.T) {
	matched, pattern := MatchesDanger("echo hello", []string{"rm -rf"})
	assert.False(t, matched)
	assert.Empty(t, pattern)
}

func TestRequiresConfirmation_ConfirmTrueAlwaysRequires(t *testing.T) {
	step := &Step{Name: "s", Source: "echo hi", Confirm: true}

	required, _ := RequiresConfirmation(step, nil)
	assert.True(t, required)
}

func TestRequiresConfirmation_DangerPatternOverridesConfirmFalse(t *testing.T) {
	step := &Step{Name: "s", Source: "rm -rf /data", Confirm: false}

	required, reason := RequiresConfirmation(step, []string{"rm -rf"})
	assert.True(t, required)
	assert.Contains(t, reason, "rm -rf")
}

func TestRequiresConfirmation_NeitherMeansNotRequired(t *testing.T) {
	step := &Step{Name: "s", Source: "echo hi", Confirm: false}

	required, reason := RequiresConfirmation(step, []string{"rm -rf"})
	assert.False(t, required)
	assert.Empty(t, reason)
}

func TestCheckConfirmation_RejectsWhenRequiredAndNotConfirmed(t *testing.T) {
	step := &Step{Name: "danger", Source: "rm -rf /data", Confirm: false}

	err := CheckConfirmation(step, []string{"rm -rf"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "danger")
}

func TestCheckConfirmation_AllowsWhenConfirmedTrue(t *testing.T) {
	step := &Step{Name: "danger", Source: "rm -rf /data", Confirm: false}

	err := CheckConfirmation(step, []string{"rm -rf"}, true)
	assert.NoError(t, err)
}

func TestCheckConfirmation_AllowsWhenNotRequired(t *testing.T) {
	step := &Step{Name: "safe", Source: "echo hi", Confirm: false}

	err := CheckConfirmation(step, []string{"rm -rf"}, false)
	assert.NoError(t, err)
}
