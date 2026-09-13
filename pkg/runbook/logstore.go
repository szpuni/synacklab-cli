package runbook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LogWriter persists one execution's full output to disk regardless of
// capture= (Requirement 4).
type LogWriter interface {
	Write(docPath, sessionID string, index int, exec Execution) (path string, err error)
}

// FileLogWriter writes logs under <BaseDir>/<doc-slug>/<sessionID>/steps/.
type FileLogWriter struct {
	BaseDir string
}

func NewFileLogWriter(baseDir string) *FileLogWriter {
	return &FileLogWriter{BaseDir: baseDir}
}

func (w *FileLogWriter) Write(docPath, sessionID string, index int, exec Execution) (string, error) {
	dir := filepath.Join(w.BaseDir, slugifyDocName(docPath), sessionID, "steps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", &Error{Type: ErrorTypeExecution, Message: "failed to create log directory", Cause: err}
	}

	path := filepath.Join(dir, fmt.Sprintf("%d-%s.log", index, exec.StepName))
	if err := os.WriteFile(path, []byte(formatLog(exec)), 0o644); err != nil {
		return "", &Error{Type: ErrorTypeExecution, Message: "failed to write log file", Cause: err}
	}
	return path, nil
}

func formatLog(exec Execution) string {
	var b strings.Builder
	fmt.Fprintf(&b, "step: %s\nstarted: %s\nduration: %s\nexit_code: %d\ntimed_out: %v\n\n",
		exec.StepName, exec.Started.Format("2006-01-02T15:04:05Z07:00"), exec.Duration, exec.ExitCode, exec.TimedOut)
	b.WriteString("=== stdout ===\n")
	b.WriteString(exec.Stdout)
	b.WriteString("=== stderr ===\n")
	b.WriteString(exec.Stderr)
	return b.String()
}

var nonSlugChars = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

// slugifyDocName turns a document path's base filename into a filesystem-safe slug.
func slugifyDocName(docPath string) string {
	base := filepath.Base(docPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	slug := nonSlugChars.ReplaceAllString(base, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "runbook"
	}
	return slug
}
