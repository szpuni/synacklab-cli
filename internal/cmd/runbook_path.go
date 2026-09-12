package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var defaultRunbookNames = []string{"RUNBOOK.md", "runbook.md"}

// resolveRunbookPath resolves a user-supplied path — a file, a directory,
// or empty (meaning the current directory) — to a single runbook file.
// A directory is searched for RUNBOOK.md, then runbook.md, then (if
// exactly one exists) any other *.md file, so `synacklab runbook serve`
// works without requiring the exact file path every time.
func resolveRunbookPath(path string) (string, error) {
	if path == "" {
		path = "."
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to access %s: %w", path, err)
	}
	if !info.IsDir() {
		return path, nil
	}

	return findRunbookInDir(path)
}

func findRunbookInDir(dir string) (string, error) {
	for _, name := range defaultRunbookNames {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return "", fmt.Errorf("failed to scan %s for runbooks: %w", dir, err)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no runbook found in %s — expected RUNBOOK.md, runbook.md, or exactly one *.md file", dir)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = filepath.Base(m)
		}
		return "", fmt.Errorf("multiple *.md files found in %s and none named RUNBOOK.md/runbook.md — specify one explicitly: %s",
			dir, strings.Join(names, ", "))
	}
}
