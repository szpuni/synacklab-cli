package runbook

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Parser turns runbook Markdown source into a Document.
type Parser interface {
	Parse(source []byte, docPath string) (*Document, error)
}

// GoldmarkParser implements Parser using goldmark to locate top-level fenced
// code blocks and a custom fence-info-string parser for runbook attributes.
// Everything outside a runnable (bash/python) fence — headings, prose, lists,
// non-runnable code fences — is copied verbatim as Prose, so parsing never
// has to reconstruct Markdown from the AST.
type GoldmarkParser struct{}

func (p *GoldmarkParser) Parse(source []byte, docPath string) (*Document, error) {
	md := goldmark.New()
	root := md.Parser().Parse(text.NewReader(source))

	doc := &Document{
		Path:  docPath,
		Dir:   filepath.Dir(docPath),
		Steps: map[string]*Step{},
	}

	autoIndex := 0
	cursor := 0

	appendProse := func(raw []byte) {
		if strings.TrimSpace(string(raw)) == "" {
			return
		}
		doc.Blocks = append(doc.Blocks, Block{Kind: BlockProse, Prose: string(raw)})
	}

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

		appendProse(source[cursor:fenceStart])

		step, err := stepFromFence(fcb, lang, source, &autoIndex)
		if err != nil {
			return nil, err
		}
		if existing, dup := doc.Steps[step.Name]; dup {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf(
				"duplicate step name %q (already used by an earlier step with source %q)", step.Name, existing.Source)}
		}
		doc.Steps[step.Name] = step
		doc.Blocks = append(doc.Blocks, Block{Kind: BlockStep, Step: step})

		cursor = fenceStop
	}
	appendProse(source[cursor:])

	return doc, nil
}

func stepFromFence(fcb *gast.FencedCodeBlock, lang string, source []byte, autoIndex *int) (*Step, error) {
	info := ""
	if fcb.Info != nil {
		info = string(fcb.Info.Segment.Value(source))
	}

	attrs, err := parseFenceAttrs(info)
	if err != nil {
		return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("malformed fence attributes %q: %s", info, err)}
	}

	step := &Step{
		Lang:   lang,
		Source: string(fcb.Lines().Value(source)),
		Cwd:    attrs["cwd"],
	}

	if name := attrs["name"]; name != "" {
		step.Name = name
	} else {
		*autoIndex++
		step.Name = fmt.Sprintf("step-%d", *autoIndex)
	}

	if v := attrs["input"]; v != "" {
		step.Input = splitTrim(v)
	}
	if v := attrs["capture"]; v != "" {
		step.Capture = splitTrim(v)
	}

	if v, ok := attrs["confirm"]; ok {
		b, perr := strconv.ParseBool(v)
		if perr != nil {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("invalid confirm= value %q on step %q", v, step.Name)}
		}
		step.Confirm = b
	}

	if v, ok := attrs["set_cwd"]; ok {
		b, perr := strconv.ParseBool(v)
		if perr != nil {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("invalid set_cwd= value %q on step %q", v, step.Name)}
		}
		step.SetCwd = b
	}

	if v, ok := attrs["timeout"]; ok {
		d, perr := time.ParseDuration(v)
		if perr != nil {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("invalid timeout= value %q on step %q", v, step.Name)}
		}
		step.Timeout = d
	}

	return step, nil
}

// parseFenceAttrs parses "name=x, capture=A,B" style attribute strings.
// Attributes are comma-separated key=value pairs; a token without '=' is
// treated as a continuation of the previous key's (comma-separated) value,
// which lets input=/capture= carry multiple names without ambiguity.
func parseFenceAttrs(info string) (map[string]string, error) {
	open := strings.Index(info, "{")
	if open == -1 {
		return map[string]string{}, nil
	}
	closeIdx := strings.LastIndex(info, "}")
	if closeIdx == -1 || closeIdx < open {
		return nil, fmt.Errorf("unbalanced braces")
	}

	attrs := map[string]string{}
	lastKey := ""
	for _, tok := range strings.Split(info[open+1:closeIdx], ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if idx := strings.Index(tok, "="); idx >= 0 {
			key := strings.TrimSpace(tok[:idx])
			val := strings.TrimSpace(tok[idx+1:])
			if key == "" {
				return nil, fmt.Errorf("empty attribute key in %q", tok)
			}
			attrs[key] = val
			lastKey = key
		} else {
			if lastKey == "" {
				return nil, fmt.Errorf("attribute value %q has no key", tok)
			}
			attrs[lastKey] += "," + tok
		}
	}
	return attrs, nil
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// expandFence widens a fenced-code-block's content span to include its
// opening (```lang) and closing (```) delimiter lines, since Lines() covers
// only the code content between them.
func expandFence(source []byte, start, stop int) (int, int) {
	if start > 0 {
		j := start - 1
		for j > 0 && source[j-1] != '\n' {
			j--
		}
		start = j
	}
	j := stop
	for j < len(source) && source[j] != '\n' {
		j++
	}
	if j < len(source) {
		j++
	}
	return start, j
}
