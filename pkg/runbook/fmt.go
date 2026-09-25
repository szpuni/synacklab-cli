package runbook

import (
	"bytes"
	"sort"
	"strings"
)

// canonicalAttrOrder matches the attribute table in project-brief.md §5.1.
var canonicalAttrOrder = []string{"name", "input", "capture", "sensitive", "confirm", "cwd", "set_cwd", "timeout"}

// FormatDocument rewrites every runnable fence's attribute string into
// canonical key order and spacing (Requirement 12.1). Prose, code content,
// non-runnable fences, frontmatter, and attribute values are left
// byte-for-byte unchanged (Requirement 12.2). On a parse error it returns
// nil and the error, so a caller never writes a partial result to disk
// (Requirement 12.3).
func FormatDocument(source []byte) ([]byte, error) {
	_, body, err := parseFrontmatter(source)
	if err != nil {
		return nil, err
	}
	frontmatterLen := len(source) - len(body)
	frontmatterRaw := source[:frontmatterLen]

	formattedBody, err := formatFences(body)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(frontmatterRaw)+len(formattedBody))
	out = append(out, frontmatterRaw...)
	out = append(out, formattedBody...)
	return out, nil
}

func formatFences(source []byte) ([]byte, error) {
	fences, err := runnableFences(source)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	cursor := 0
	for _, f := range fences {
		out.Write(source[cursor:f.start])

		marker := fenceMarker(source, f.start)
		out.WriteString(marker)
		out.WriteString(f.lang)
		if canonical := canonicalAttrString(f.attrs); canonical != "" {
			out.WriteString(" {")
			out.WriteString(canonical)
			out.WriteString("}")
		}
		out.WriteString("\n")
		out.Write(source[f.contentStart:f.contentStop])
		out.WriteString(marker)
		out.WriteString("\n")

		cursor = f.stop
	}
	out.Write(source[cursor:])

	return out.Bytes(), nil
}

// canonicalAttrString formats attrs in canonicalAttrOrder, appending any
// unrecognized keys (sorted, for determinism) after the known ones.
func canonicalAttrString(attrs map[string]string) string {
	seen := make(map[string]bool, len(attrs))
	parts := make([]string, 0, len(attrs))

	for _, key := range canonicalAttrOrder {
		if v, ok := attrs[key]; ok {
			parts = append(parts, key+"="+v)
			seen[key] = true
		}
	}

	var extra []string
	for k := range attrs {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		parts = append(parts, k+"="+attrs[k])
	}

	return strings.Join(parts, ", ")
}

// fenceMarker returns the run of fence characters (``` or ~~~, any length
// >=3) starting at fenceStart, so a rewritten fence keeps its original
// marker length/style instead of always collapsing to ``` .
func fenceMarker(source []byte, fenceStart int) string {
	c := source[fenceStart]
	i := fenceStart
	for i < len(source) && source[i] == c {
		i++
	}
	return string(source[fenceStart:i])
}
