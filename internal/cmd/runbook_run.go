package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	"synacklab/pkg/runbook"
)

var (
	runNonInteractive bool
	runSetFlags       []string
)

var runbookRunCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Execute a runbook top to bottom without a browser (for CI)",
	Long: `run drives the same parser and execution engine as serve, but top to
bottom with no browser and no per-step confirmation prompt. A step that
declares input= must have a matching --set, and a step requiring
confirmation (confirm=true or a danger_patterns match) fails the run closed
— run it interactively via 'synacklab runbook serve' instead.

path may be a runbook file, a directory (RUNBOOK.md/runbook.md, or its
only *.md file, is used), or omitted entirely to use the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRunbookRun,
}

func init() {
	runbookRunCmd.Flags().BoolVar(&runNonInteractive, "non-interactive", false,
		"execute the full document top to bottom (required — interactive execution is synacklab runbook serve's job)")
	runbookRunCmd.Flags().StringArrayVar(&runSetFlags, "set", nil, "VAR=value for a step's input= (repeatable)")
	runbookCmd.AddCommand(runbookRunCmd)
}

func runRunbookRun(_ *cobra.Command, args []string) error {
	if !runNonInteractive {
		return fmt.Errorf("run requires --non-interactive (interactive execution is synacklab runbook serve's job)")
	}

	pathArg := ""
	if len(args) > 0 {
		pathArg = args[0]
	}
	docPath, err := resolveRunbookPath(pathArg)
	if err != nil {
		return err
	}

	sess, err := runbook.OpenSession(docPath, runbook.SessionOptions{Logs: runbook.NewFileLogWriter(".synacklab")})
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", docPath, err)
	}

	sets, err := parseSetFlags(runSetFlags)
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	logger, err := buildLogger(cfg)
	if err != nil {
		return err
	}

	// SIGINT/SIGTERM cancels the currently-running step's process group
	// instead of leaving it orphaned (it runs in its own group — see
	// process.go's timeout-kill mechanism — so the default "just die"
	// behavior on an unhandled signal would otherwise not reach it).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := sess.RunAll(ctx, sets, os.Stdout, logger); err != nil {
		var rbErr *runbook.Error
		if errors.As(err, &rbErr) && rbErr.Type == runbook.ErrorTypeValidation {
			return fmt.Errorf("%w — pass inputs with --set; steps that need confirmation fail closed here, run them via 'synacklab runbook serve'", err)
		}
		return err
	}
	return nil
}

func parseSetFlags(sets []string) (map[string]string, error) {
	result := map[string]string{}
	for _, s := range sets {
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --set value %q, expected VAR=value", s)
		}
		result[k] = v
	}
	return result, nil
}
