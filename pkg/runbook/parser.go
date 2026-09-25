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

// Parse turns runbook Markdown source into a Document. Everything outside a
// runnable fence — headings, prose, lists, non-runnable code fences — is
// copied verbatim as Prose, so parsing never has to reconstruct Markdown
// from the AST.
func Parse(source []byte, docPath string) (*Document, error) {
	fm, source, err := parseFrontmatter(source)
	if err != nil {
		return nil, err
	}

	fences, err := runnableFences(source)
	if err != nil {
		return nil, err
	}

	doc := &Document{
		Path:        docPath,
		Dir:         filepath.Dir(docPath),
		Frontmatter: fm,
		Steps:       map[string]*Step{},
	}

	appendProse := func(raw []byte) {
		if strings.TrimSpace(string(raw)) == "" {
			return
		}
		doc.Blocks = append(doc.Blocks, Block{Kind: BlockProse, Prose: string(raw)})
	}

	autoIndex := 0
	cursor := 0
	for _, f := range fences {
		appendProse(source[cursor:f.start])

		step, err := stepFromFence(f, &autoIndex)
		if err != nil {
			return nil, err
		}
		if existing, dup := doc.Steps[step.Name]; dup {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf(
				"duplicate step name %q (already used by an earlier step with source %q)", step.Name, existing.Source)}
		}
		doc.Steps[step.Name] = step
		doc.Blocks = append(doc.Blocks, Block{Kind: BlockStep, Step: step})

		cursor = f.stop
	}
	appendProse(source[cursor:])

	return doc, nil
}

// runnableFence is one top-level, non-empty bash/python fenced code block —
// the only kind of fence that becomes a Step.
type runnableFence struct {
	lang  string
	attrs map[string]string
	// code is the block's content with fence indentation removed.
	code string
	// start/stop span the whole fence including its delimiter lines;
	// contentStart/contentStop span only the raw content between them.
	start, stop, contentStart, contentStop int
}

// runnableFences finds every runnable fence in source, in document order,
// with its attributes parsed.
func runnableFences(source []byte) ([]runnableFence, error) {
	root := goldmark.New().Parser().Parse(text.NewReader(source))

	var fences []runnableFence
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

		info := ""
		if fcb.Info != nil {
			info = string(fcb.Info.Segment.Value(source))
		}
		attrs, err := parseFenceAttrs(info)
		if err != nil {
			return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf("malformed fence attributes %q: %s", info, err)}
		}

		contentStart, contentStop := lines.At(0).Start, lines.At(lines.Len()-1).Stop
		start, stop := expandFence(source, contentStart, contentStop)
		fences = append(fences, runnableFence{
			lang: lang, attrs: attrs, code: string(lines.Value(source)),
			start: start, stop: stop, contentStart: contentStart, contentStop: contentStop,
		})
	}
	return fences, nil
}

func stepFromFence(f runnableFence, autoIndex *int) (*Step, error) {
	attrs := f.attrs
	step := &Step{
		Lang:   f.lang,
		Source: f.code,
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
	if v := attrs["sensitive"]; v != "" {
		step.Sensitive = splitTrim(v)
		for _, name := range step.Sensitive {
			if !containsStr(step.Input, name) {
				return nil, &Error{Type: ErrorTypeParse, Message: fmt.Sprintf(
					"step %q: sensitive=%q is not declared in input=", step.Name, name)}
			}
		}
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

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
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
