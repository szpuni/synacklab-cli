package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"synacklab/pkg/config"
	rblog "synacklab/pkg/log"
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

	source, err := os.ReadFile(docPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", docPath, err)
	}

	doc, err := (&runbook.GoldmarkParser{}).Parse(source, docPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", docPath, err)
	}

	sets, err := parseSetFlags(runSetFlags)
	if err != nil {
		return err
	}

	absCwd, err := filepath.Abs(doc.Dir)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	logger, err := buildLogger(cfg)
	if err != nil {
		return err
	}

	store := runbook.NewSessionStore(runbook.NewID(), docPath, absCwd)
	engine := runbook.NewEngine(runbook.NewFileLogWriter(".synacklab"))

	// SIGINT/SIGTERM cancels the currently-running step's process group
	// instead of leaving it orphaned (it runs in its own group — see
	// executor.go's timeout-kill mechanism — so the default "just die"
	// behavior on an unhandled signal would otherwise not reach it).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return executeNonInteractive(ctx, doc, sets, store, engine, os.Stdout, logger)
}

// executeNonInteractive runs every Step in document order, stopping at the
// first failure (Requirement 11.4). A step with an unmet input=, or one
// requiring confirmation, fails before any process is spawned for it
// (Requirements 11.2, 11.3). ctx is the parent for every step's execution —
// canceling it (e.g. via SIGINT/SIGTERM) kills the currently-running step's
// process group rather than orphaning it.
func executeNonInteractive(ctx context.Context, doc *runbook.Document, sets map[string]string, store runbook.SessionStore, engine runbook.Engine, out io.Writer, logger *rblog.Logger) error {
	for _, block := range doc.Blocks {
		if block.Kind != runbook.BlockStep {
			continue
		}
		step := block.Step

		inputs := map[string]string{}
		var missing []string
		for _, name := range step.Input {
			v, ok := sets[name]
			if !ok {
				missing = append(missing, name)
				continue
			}
			inputs[name] = v
		}
		if len(missing) > 0 {
			return fmt.Errorf("step %q: missing required --set for input(s): %s", step.Name, strings.Join(missing, ", "))
		}

		if required, reason := runbook.RequiresConfirmation(step, doc.Frontmatter.DangerPatterns); required {
			return fmt.Errorf("step %q requires confirmation (%s); non-interactive mode has no way to confirm, so it fails closed — run it via `synacklab serve` instead", step.Name, reason)
		}

		logger.Info("running step %q", step.Name)

		timeout := runbook.EffectiveTimeout(step, doc.Frontmatter.DefaultTimeout)
		_, events, _, err := engine.Run(ctx, step, inputs, timeout, store)
		if err != nil {
			logger.Error("step %q: %s", step.Name, err)
			return fmt.Errorf("step %q: %w", step.Name, err)
		}

		var done runbook.Event
		for ev := range events {
			switch ev.Type {
			case "stdout", "stderr":
				fmt.Fprintln(out, ev.Data)
			case "done":
				done = ev
			}
		}

		if done.TimedOut {
			logger.Warn("step %q timed out", step.Name)
			return fmt.Errorf("step %q timed out", step.Name)
		}
		if done.ExitCode != 0 {
			logger.Warn("step %q exited with code %d", step.Name, done.ExitCode)
			return fmt.Errorf("step %q exited with code %d", step.Name, done.ExitCode)
		}
		logger.Info("step %q finished (duration=%s)", step.Name, done.Duration)
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
