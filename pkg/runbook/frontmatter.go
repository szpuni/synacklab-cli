package runbook

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type frontmatterYAML struct {
	DefaultTimeout string   `yaml:"default_timeout"`
	DangerPatterns []string `yaml:"danger_patterns"`
}

// parseFrontmatter strips a leading "---\n...\n---\n" YAML block from source,
// returning the parsed Frontmatter and the remaining document bytes. If
// source has no leading "---" delimiter, it returns a zero-value Frontmatter
// and source unchanged.
func parseFrontmatter(source []byte) (Frontmatter, []byte, error) {
	const delim = "---"

	s := string(source)
	if !strings.HasPrefix(s, delim+"\n") {
		return Frontmatter{}, source, nil
	}

	lines := strings.SplitAfter(s, "\n")
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == delim {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return Frontmatter{}, nil, &Error{Type: ErrorTypeParse, Message: "unclosed frontmatter block: no closing \"---\" found"}
	}

	raw := strings.Join(lines[1:closeIdx], "")
	rest := []byte(strings.Join(lines[closeIdx+1:], ""))

	var parsed frontmatterYAML
	if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
		return Frontmatter{}, nil, &Error{Type: ErrorTypeParse, Message: "malformed frontmatter YAML", Cause: err}
	}

	fm := Frontmatter{DangerPatterns: parsed.DangerPatterns}
	if parsed.DefaultTimeout != "" {
		d, err := time.ParseDuration(parsed.DefaultTimeout)
		if err != nil {
			return Frontmatter{}, nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("invalid default_timeout %q", parsed.DefaultTimeout), Cause: err}
		}
		fm.DefaultTimeout = d
	}

	return fm, rest, nil
}
