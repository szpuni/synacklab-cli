package runbook

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// canonicalAttrOrder matches the attribute table in project-brief.md §5.1.
var canonicalAttrOrder = []string{"name", "input", "capture", "confirm", "cwd", "set_cwd", "timeout"}

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
	md := goldmark.New()
	root := md.Parser().Parse(text.NewReader(source))

	var out bytes.Buffer
	cursor := 0

	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		fcb, ok := n.(*gast.FencedCodeBlock)
		if !ok {
			continue
		}
		lang := string(fcb.Language(source))
		if lang != "bash" && lang != "python" {
			continue
		}

		lines := fcb.Lines()
		if lines.Len() == 0 {
			continue
		}
		contentStart, contentStop := lines.At(0).Start, lines.At(lines.Len()-1).Stop
		fenceStart, fenceStop := expandFence(source, contentStart, contentStop)

		out.Write(source[cursor:fenceStart])

		info := ""
		if fcb.Info != nil {
			info = string(fcb.Info.Segment.Value(source))
		}
		attrs, err := parseFenceAttrs(info)
		if err != nil {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("malformed fence attributes %q: %s", info, err)}
		}

		marker := fenceMarker(source, fenceStart)
		out.WriteString(marker)
		out.WriteString(lang)
		if canonical := canonicalAttrString(attrs); canonical != "" {
			out.WriteString(" {")
			out.WriteString(canonical)
			out.WriteString("}")
		}
		out.WriteString("\n")
		out.Write(source[contentStart:contentStop])
		out.WriteString(marker)
		out.WriteString("\n")

		cursor = fenceStop
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
