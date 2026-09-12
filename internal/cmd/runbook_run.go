package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"synacklab/pkg/runbook"
)

var (
	runNonInteractive bool
	runSetFlags       []string
)

var runbookRunCmd = &cobra.Command{
	Use:   "run <file.md>",
	Short: "Execute a runbook top to bottom without a browser (for CI)",
	Long: `run drives the same parser and execution engine as serve, but top to
bottom with no browser and no per-step confirmation prompt. A step that
declares input= must have a matching --set, and a step requiring
confirmation (confirm=true or a danger_patterns match) fails the run closed
— run it interactively via 'synacklab serve' instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runRunbookRun,
}

func init() {
	runbookRunCmd.Flags().BoolVar(&runNonInteractive, "non-interactive", false,
		"execute the full document top to bottom (required — interactive execution is synacklab serve's job)")
	runbookRunCmd.Flags().StringArrayVar(&runSetFlags, "set", nil, "VAR=value for a step's input= (repeatable)")
	rootCmd.AddCommand(runbookRunCmd)
}

func runRunbookRun(_ *cobra.Command, args []string) error {
	if !runNonInteractive {
		return fmt.Errorf("run requires --non-interactive (interactive execution is `synacklab serve`)")
	}

	docPath := args[0]
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

	store := runbook.NewSessionStore(runbook.NewID(), docPath, absCwd)
	engine := runbook.NewEngine(runbook.NewFileLogWriter(".synacklab"))

	return executeNonInteractive(doc, sets, store, engine, os.Stdout)
}

// executeNonInteractive runs every Step in document order, stopping at the
// first failure (Requirement 11.4). A step with an unmet input=, or one
// requiring confirmation, fails before any process is spawned for it
// (Requirements 11.2, 11.3).
func executeNonInteractive(doc *runbook.Document, sets map[string]string, store runbook.SessionStore, engine runbook.Engine, out io.Writer) error {
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

		fmt.Fprintf(out, "==> %s\n", step.Name)

		timeout := runbook.EffectiveTimeout(step, doc.Frontmatter.DefaultTimeout)
		_, events, err := engine.Run(context.Background(), step, inputs, timeout, store)
		if err != nil {
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
			return fmt.Errorf("step %q timed out", step.Name)
		}
		if done.ExitCode != 0 {
			return fmt.Errorf("step %q exited with code %d", step.Name, done.ExitCode)
		}
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
