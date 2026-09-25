package runbook

import (
	"fmt"
	"strings"
)

const (
	capturePrefix = "__SYNACLAB_CAP__"
	cwdPrefix     = "__SYNACLAB_CWD__"
)

// buildTrailer generates the sentinel-emitting lines appended after a step's
// script (project-brief.md §7.4/§7.5). It runs in the same process as the
// user script, so bash sees whatever the script exported and python sees
// whatever it wrote to os.environ.
func buildTrailer(lang string, capture []string, setCwd bool) string {
	var b strings.Builder

	switch lang {
	case "bash":
		for _, name := range capture {
			fmt.Fprintf(&b, "echo \"%s%s=${%s}\"\n", capturePrefix, name, name)
		}
		if setCwd {
			fmt.Fprintf(&b, "echo \"%s$PWD\"\n", cwdPrefix)
		}
	case "python":
		if len(capture) > 0 || setCwd {
			b.WriteString("import os\n")
		}
		for _, name := range capture {
			fmt.Fprintf(&b, "print(f\"%s%s={os.environ.get('%s','')}\")\n", capturePrefix, name, name)
		}
		if setCwd {
			fmt.Fprintf(&b, "print(f\"%s{os.getcwd()}\")\n", cwdPrefix)
		}
	}

	return b.String()
}

// captureResult accumulates what a step exported through its trailer's
// sentinel lines: captured vars, and its final cwd if set_cwd asked for it.
type captureResult struct {
	vars     map[string]string
	cwd      string
	cwdFound bool
}

func newCaptureResult() *captureResult {
	return &captureResult{vars: map[string]string{}}
}

// consume records line if it is a sentinel and reports whether it was one,
// so the caller keeps sentinel lines out of the step's visible output.
func (c *captureResult) consume(line string) bool {
	if rest, ok := strings.CutPrefix(line, capturePrefix); ok {
		if name, value, found := strings.Cut(rest, "="); found {
			c.vars[name] = value
		}
		return true
	}
	if rest, ok := strings.CutPrefix(line, cwdPrefix); ok {
		c.cwd, c.cwdFound = rest, true
		return true
	}
	return false
}
