package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"synacklab/pkg/runbook"
)

var runbookFmtCmd = &cobra.Command{
	Use:   "fmt [path]",
	Short: "Canonicalize a runbook's fence attribute formatting",
	Long: `fmt rewrites each runnable fence's attribute string into a canonical key
order and spacing, without altering prose, code content, or attribute
values/semantics. On a parse error the file is left untouched.

path may be a runbook file, a directory (RUNBOOK.md/runbook.md, or its
only *.md file, is used), or omitted entirely to use the current directory.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRunbookFmt,
}

func init() {
	runbookCmd.AddCommand(runbookFmtCmd)
}

func runRunbookFmt(_ *cobra.Command, args []string) error {
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

	formatted, err := runbook.FormatDocument(source)
	if err != nil {
		return fmt.Errorf("failed to format %s: %w", docPath, err)
	}

	if bytes.Equal(source, formatted) {
		fmt.Printf("%s already formatted\n", docPath)
		return nil
	}

	perm := os.FileMode(0o644)
	if info, statErr := os.Stat(docPath); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(docPath, formatted, perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", docPath, err)
	}

	fmt.Printf("formatted %s\n", docPath)
	return nil
}
