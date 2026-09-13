package cmd

import (
	"github.com/spf13/cobra"
)

var runbookCmd = &cobra.Command{
	Use:   "runbook",
	Short: "Interactive, human-in-the-loop Markdown runbooks",
	Long: `runbook turns a Markdown file's fenced bash/python code blocks into an
interactive runbook: execute steps one at a time from a browser (serve),
drive them top to bottom for CI (run), or canonicalize their fence
attribute formatting (fmt).

Each command accepts a file, a directory, or no argument at all (defaults
to the current directory). When given a directory, it looks for
RUNBOOK.md or runbook.md, falling back to a single *.md file if the
directory contains exactly one.

Available commands:
  serve - Start an interactive web server for a runbook
  run   - Execute a runbook top to bottom without a browser (for CI)
  fmt   - Canonicalize a runbook's fence attribute formatting`,
}

func init() {
	// Subcommands are added in their respective files
}
