package runbook

import "time"

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
	Name    string
	Lang    string
	Source  string
	Input   []string
	Capture []string
	Confirm bool
	Cwd     string
	SetCwd  bool
	Timeout time.Duration
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

// Session is one running server's state: captured vars, cwd, execution history.
// Vars is a flat, last-write-wins namespace; SessionStore guards concurrent access.
type Session struct {
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
	LogPath  string
	Stdout   string
	Captured map[string]string
}

// Event is a single message streamed to the frontend over the execution's WebSocket.
type Event struct {
	Type     string
	Data     string
	ExitCode int
	Duration time.Duration
	TimedOut bool
	Captured map[string]string
}
