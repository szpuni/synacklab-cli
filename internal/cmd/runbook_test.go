package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunbookCmd_IsRegisteredUnderRoot(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "runbook" {
			found = true
		}
		assert.NotContains(t, []string{"serve", "run", "fmt"}, c.Name(),
			"serve/run/fmt must be subcommands of runbook, not top-level")
	}
	assert.True(t, found, "runbook command group must be registered on rootCmd")
}

func TestRunbookCmd_HasServeRunFmtSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range runbookCmd.Commands() {
		names[c.Name()] = true
	}
	assert.True(t, names["serve"])
	assert.True(t, names["run"])
	assert.True(t, names["fmt"])
}
