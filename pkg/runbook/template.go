package runbook

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	stepsRefRe = regexp.MustCompile(`\{\{\s*steps\.([\w-]+)\.(stdout|exit_code)\s*\}\}`)
	varsRefRe  = regexp.MustCompile(`\{\{\s*vars\.([\w-]+)\s*\}\}`)
)

// Substitute replaces {{steps.<name>.stdout|exit_code}} and {{vars.<name>}}
// references with their literal values as plain text — no shell-escaping.
// This is the documented tradeoff vs. input=/capture= (project-brief.md §13):
// callers must treat referenced values as trusted before using this path.
func Substitute(source string, sess *SessionState) (string, error) {
	var refErr error

	out := stepsRefRe.ReplaceAllStringFunc(source, func(m string) string {
		if refErr != nil {
			return m
		}
		groups := stepsRefRe.FindStringSubmatch(m)
		name, field := groups[1], groups[2]

		exec, found := lastExecution(sess, name)
		if !found {
			refErr = &Error{Type: ErrorTypeTemplate, Message: fmt.Sprintf("unknown step reference %q", name)}
			return m
		}
		if field == "exit_code" {
			return strconv.Itoa(exec.ExitCode)
		}
		return exec.Stdout
	})
	if refErr != nil {
		return "", refErr
	}

	out = varsRefRe.ReplaceAllStringFunc(out, func(m string) string {
		if refErr != nil {
			return m
		}
		name := varsRefRe.FindStringSubmatch(m)[1]
		val, ok := sess.Vars[name]
		if !ok {
			refErr = &Error{Type: ErrorTypeTemplate, Message: fmt.Sprintf("unknown var reference %q", name)}
			return m
		}
		return val
	})
	if refErr != nil {
		return "", refErr
	}

	return out, nil
}

func lastExecution(sess *SessionState, stepName string) (Execution, bool) {
	for i := len(sess.History) - 1; i >= 0; i-- {
		if sess.History[i].StepName == stepName {
			return sess.History[i], true
		}
	}
	return Execution{}, false
}
