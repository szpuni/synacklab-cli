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

// parseCaptureOutput strips __SYNACLAB_CAP__/__SYNACLAB_CWD__ sentinel lines
// from stdout, returning the cleaned output plus the captured vars and cwd.
func parseCaptureOutput(stdout string) (cleaned string, captured map[string]string, cwd string, cwdFound bool) {
	captured = map[string]string{}
	lines := strings.Split(stdout, "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, capturePrefix):
			rest := line[len(capturePrefix):]
			if idx := strings.Index(rest, "="); idx >= 0 {
				captured[rest[:idx]] = rest[idx+1:]
			}
		case strings.HasPrefix(line, cwdPrefix):
			cwd = line[len(cwdPrefix):]
			cwdFound = true
		default:
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "\n"), captured, cwd, cwdFound
}
