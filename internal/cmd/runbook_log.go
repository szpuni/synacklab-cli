package cmd

import (
	"fmt"
	"os"

	"synacklab/pkg/config"
	rblog "synacklab/pkg/log"
)

// buildLogger constructs the console logger for runbook commands. Verbosity
// is controlled by log_level in ~/.synacklab/config.yaml (error/warn/info,
// default info) — see docs/runbook.md.
func buildLogger(cfg *config.Config) (*rblog.Logger, error) {
	level, err := rblog.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log_level %q in ~/.synacklab/config.yaml: %w", cfg.LogLevel, err)
	}
	return rblog.New(os.Stdout, level), nil
}
