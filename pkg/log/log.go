// Package log provides a minimal leveled logger (error/warn/info) for
// console output, with its threshold controlled by synacklab's own
// configuration (log_level in ~/.synacklab/config.yaml).
package log

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Level is a log severity. Lower values are more severe and always shown;
// a Logger's configured Level is the most verbose level it will print.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
)

func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelWarn:
		return "WARN"
	default:
		return "INFO"
	}
}

// ParseLevel parses a config value into a Level. An empty string defaults
// to LevelInfo (matching synacklab's documented default).
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error":
		return LevelError, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "info", "":
		return LevelInfo, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q (expected error, warn, or info)", s)
	}
}

// Logger writes leveled lines to out, filtering anything more verbose than
// its configured Level.
type Logger struct {
	mu    sync.Mutex
	out   io.Writer
	level Level
	now   func() time.Time
}

// New creates a Logger writing to out, showing level and anything more severe.
func New(out io.Writer, level Level) *Logger {
	return &Logger{out: out, level: level, now: time.Now}
}

// Discard returns a Logger that produces no output, for tests and other
// callers that don't want console noise.
func Discard() *Logger {
	return New(io.Discard, LevelInfo)
}

func (l *Logger) Error(format string, args ...any) { l.write(LevelError, format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.write(LevelWarn, format, args...) }
func (l *Logger) Info(format string, args ...any)  { l.write(LevelInfo, format, args...) }

func (l *Logger) write(level Level, format string, args ...any) {
	if level > l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.out, "%s [%-5s] %s\n", l.now().Format(time.RFC3339), level, fmt.Sprintf(format, args...))
}
