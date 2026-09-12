package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"synacklab/pkg/runbook"
)

var runbookFmtCmd = &cobra.Command{
	Use:   "fmt <file.md>",
	Short: "Canonicalize a runbook's fence attribute formatting",
	Long: `fmt rewrites each runnable fence's attribute string into a canonical key
order and spacing, without altering prose, code content, or attribute
values/semantics. On a parse error the file is left untouched.`,
	Args: cobra.ExactArgs(1),
	RunE: runRunbookFmt,
}

func init() {
	rootCmd.AddCommand(runbookFmtCmd)
}

func runRunbookFmt(_ *cobra.Command, args []string) error {
	docPath := args[0]
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
