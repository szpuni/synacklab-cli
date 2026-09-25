package runbook

import (
	"fmt"
	"time"
)

type BlockKind string

const (
	BlockProse BlockKind = "prose"
	BlockStep  BlockKind = "step"
)

// Block is a document element in document order: either prose or a Step.
type Block struct {
	Kind  BlockKind
	Prose string
	Step  *Step
}

// Step is a fenced bash/python code block and its parsed fence attributes.
type Step struct {
	Name      string
	Lang      string
	Source    string
	Input     []string
	Capture   []string
	Sensitive []string
	Confirm   bool
	Cwd       string
	SetCwd    bool
	Timeout   time.Duration
}

// Frontmatter is the document-level YAML config block (project-brief.md §5.2).
type Frontmatter struct {
	DefaultTimeout time.Duration
	DangerPatterns []string
}

// Document is a parsed runbook: ordered blocks plus a name-keyed step index.
type Document struct {
	Path        string
	Dir         string
	Frontmatter Frontmatter
	Blocks      []Block
	Steps       map[string]*Step
}

// SessionState is a snapshot of a Session's accumulated state: captured
// vars (a flat, last-write-wins namespace), cwd, and execution history.
type SessionState struct {
	ID      string
	DocPath string
	Cwd     string
	Vars    map[string]string
	History []Execution
}

// Execution records the result of one Step run. Stdout is kept in memory
// (in addition to the on-disk log) so {{steps.<name>.stdout}} template
// references can resolve without re-reading the log file.
type Execution struct {
	StepName string
	Started  time.Time
	Duration time.Duration
	ExitCode int
	TimedOut bool
	Canceled bool
	LogPath  string
	Stdout   string
	Stderr   string
	Captured map[string]string
}

// EventType distinguishes a step's output lines from its terminal event.
type EventType string

const (
	EventStdout EventType = "stdout"
	EventStderr EventType = "stderr"
	EventDone   EventType = "done"
)

// Event is a single message streamed from a running step: an output line,
// or the terminal EventDone carrying the result.
type Event struct {
	Type     EventType
	Data     string
	ExitCode int
	Duration time.Duration
	TimedOut bool
	Canceled bool
	Captured map[string]string
}

// failure describes why a done Event counts as a failed step — "canceled",
// "timed out", or "exited with code N" — or returns "" for success.
func (e Event) failure() string {
	switch {
	case e.Canceled:
		return "canceled"
	case e.TimedOut:
		return "timed out"
	case e.ExitCode != 0:
		return fmt.Sprintf("exited with code %d", e.ExitCode)
	default:
		return ""
	}
}
