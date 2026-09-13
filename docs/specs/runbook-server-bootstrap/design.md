# Design Document

## Overview

The Runbook Server adds a new `pkg/runbook` package plus three thin `internal/cmd`
wrappers (`serve`, `run`, `fmt`) to synacklab. It follows the existing
architecture split used by `pkg/github`: parsing/config models, a core engine
(here: parser + session + executor instead of reconciler), and a client-facing
layer (here: an HTTP/WebSocket server instead of a GitHub API client). Nothing
in this feature talks to AWS, EKS, or GitHub — it is fully independent of the
other `pkg/` packages and only shares `internal/cmd` conventions.

Nothing executes without an explicit request from the frontend or the
non-interactive runner; the server holds only data (session state) between
requests, never a live shell.

## Architecture

```
                     ┌─────────────────────────┐
   browser  <──WS──> │   Go HTTP/WS server      │
   (SPA,             │  ┌────────────────────┐  │
    embedded         │  │ Markdown parser     │  │
    via go:embed)    │  │ (goldmark + custom  │  │
                      │  │  fence-attr hook)   │  │
                      │  └────────────────────┘  │
                      │  ┌────────────────────┐  │
                      │  │ Session store       │  │
                      │  │ (in-memory map,     │  │
                      │  │  mutex-guarded)     │  │
                      │  └────────────────────┘  │
                      │  ┌────────────────────┐  │
                      │  │ Execution engine    │  │
                      │  │ (spawns bash/python │  │
                      │  │  per request only)  │  │
                      │  └────────────────────┘  │
                      └─────────────────────────┘
                                  │
                         .synacklab/<session>/
                         step logs, session.json
```

`synacklab run --non-interactive` reuses the Parser and Execution Engine
directly, driving them from a CLI loop instead of the HTTP layer — no server,
no WebSocket.

## Package Structure

```
internal/cmd/
├── runbook_serve.go        # `synacklab serve` — starts HTTP/WS server
├── runbook_run.go          # `synacklab run --non-interactive`
├── runbook_fmt.go          # `synacklab fmt`

pkg/runbook/
├── types.go                # Document, Step, Session, Execution, Attributes
├── parser.go                # goldmark wrapper + fence-attribute parsing
├── frontmatter.go           # YAML frontmatter (default_timeout, danger_patterns)
├── session.go                # Session store, mutex-guarded var/cwd/history state
├── executor.go               # process spawn, Dir/Env/timeout wiring (bash+python)
├── capture.go                 # capture-trailer generation + __SYNACLAB_CAP__/__SYNACLAB_CWD__ parsing
├── template.go                 # {{steps.x.stdout}} / {{vars.x}} substitution
├── danger.go                    # danger_patterns matching, confirm gating
├── logstore.go                   # per-execution log files under .synacklab/
├── server.go                      # http.Handler wiring, routes -> handlers
├── api.go                          # REST handlers (/api/doc, /api/session, /api/session/reset)
├── ws.go                            # /ws/executions/{id} handler, event streaming
├── errors.go                        # ErrorType consts + *Error (parse/exec/validation)
└── web/                             # embedded SPA (go:embed)
    ├── index.html
    ├── app.js
    └── app.css
```

Each file is expected to stay under the repo's 500-line limit; `executor.go`
and `api.go` are the most likely candidates to need an early split
(`executor_bash.go`/`executor_python.go`, `api_steps.go`) — call that out in
review rather than pre-splitting speculatively.

## Components and Interfaces

### Parser

```go
type Parser interface {
    Parse(source []byte, docPath string) (*Document, error)
}

type Document struct {
    Path      string
    Dir       string
    Frontmatter Frontmatter
    Blocks    []Block   // prose + steps, in document order
    Steps     map[string]*Step // by name, for O(1) lookup
}

type Block struct {
    Kind BlockKind // BlockProse | BlockStep
    Prose string    // set if BlockProse
    Step  *Step      // set if BlockStep
}

type Step struct {
    Name     string
    Lang     string // "bash" | "python"
    Source   string
    Input    []string
    Capture  []string
    Confirm  bool
    Cwd      string
    SetCwd   bool
    Timeout  time.Duration // resolved: step -> frontmatter -> 120s default
}

type Frontmatter struct {
    DefaultTimeout time.Duration
    DangerPatterns []string
}
```

Parsing uses `goldmark` for the Markdown/CommonMark structure and a custom
fence-info-string parser (regex-based, not a goldmark extension) to split
` ```bash {name=x, capture=y} ` into language + `key=value` attribute map.
Duplicate explicit names and malformed attribute syntax are parse errors
(Requirement 1.5).

### Session Store

```go
type SessionStore interface {
    Get() *Session
    SetVar(key, value string)
    SetCwd(path string)
    AppendHistory(e Execution)
    Reset(defaultCwd string)
}

type Session struct {
    ID      string
    DocPath string
    Cwd     string
    Vars    map[string]string // flat, last-write-wins; keyed "<step>.<var>" + short alias if unique
    History []Execution
    mu      sync.Mutex
}

type Execution struct {
    StepName string
    Started  time.Time
    Duration time.Duration
    ExitCode int
    TimedOut bool
    LogPath  string
    Captured map[string]string
}
```

One `Session` per `serve` process (Requirement 14); `run --non-interactive`
constructs an equivalent in-memory session that is never exposed over HTTP.

### Execution Engine

```go
type Engine interface {
    Run(ctx context.Context, step *Step, inputs map[string]string, sess *Session) (*Execution, <-chan Event, error)
}

type Event struct {
    Type     string // "stdout" | "stderr" | "done"
    Data     string
    ExitCode int
    Duration time.Duration
    Captured map[string]string
}
```

`Run` performs, in order: template substitution (if the source contains
`{{...}}`, resolved against `sess`; Requirement 10) → env merge (session vars
+ declared `input=`, declared inputs win) → append capture/cwd trailer
(Requirement 6, 7) → resolve `Dir` (step `cwd=` or session cwd) → spawn under
`context.WithTimeout` (Requirement 9) → stream stdout/stderr lines to the
returned channel → on exit, strip `__SYNACLAB_CAP__`/`__SYNACLAB_CWD__` lines,
update `sess`, write the log (Requirement 4), and emit a final `done` event.

Bash: `exec.CommandContext(ctx, "bash", "-c", wrapped)`.
Python: `exec.CommandContext(ctx, "python3", "-c", wrapped)`.
Both set `SysProcAttr` to start a new process group so timeout kills the whole
group, not just the shell/interpreter (Requirement 9.2).

### HTTP/WS Server

Routes (`net/http` + `ServeMux`, no new router dependency needed for this
route count):

| Method | Path                          | Handler                                   |
|--------|-------------------------------|--------------------------------------------|
| GET    | `/api/doc`                    | serialize `Document` + current session view |
| GET    | `/api/session`                | serialize `Session` (vars/cwd/history)      |
| POST   | `/api/session/reset`          | `SessionStore.Reset`                        |
| POST   | `/api/steps/{name}/run`       | validate inputs/confirm → `Engine.Run` async, return `{execution_id}` |
| GET    | `/ws/executions/{execution_id}` | upgrade, relay `Engine.Run`'s event channel |
| GET    | `/` and static assets         | embedded SPA (`web/`)                       |

**New dependency decision (flag for review):** the brief doesn't pin a
WebSocket library. Proposing `github.com/gorilla/websocket` — the de facto
standard, actively maintained, used throughout the Go ecosystem for exactly
this pattern. `github.com/coder/websocket` (formerly `nhooyr.io/websocket`) is
a lighter, more modern alternative if preferred; either is a small, low-risk
addition to `go.mod`.

`POST /api/steps/{name}/run` returns `execution_id` immediately (per
project-brief.md §9.1) and the actual run happens in a goroutine; the
WebSocket handler subscribes to that execution's event channel. If the
frontend connects to the WS endpoint after the execution already started, it
still gets buffered events replayed from a small ring buffer keyed by
`execution_id` (avoids a race where fast/trivial commands finish before the
WS upgrade completes — relevant given the ~200ms NFR-4 target).

### Danger pattern gating

```go
func MatchesDanger(source string, patterns []string) (matched bool, pattern string)
```

Applied at run-request validation time, before spawning: if `step.Confirm ||
matched`, the request must carry `confirmed: true` or is rejected
(Requirement 8). The matched pattern is returned to the frontend for display,
not just a boolean, so the UI can show *why* confirmation is required.

### Template substitution

```go
func Substitute(source string, sess *Session) (string, error)
```

Plain `{{...}}` text substitution, no shell-escaping (Requirement 10.2, and
project-brief.md §13's explicit security stance — this is not a bug to fix,
it's the documented tradeoff vs. `input=`/`capture=`). Missing references are
a hard error before spawn (Requirement 10.3).

## Data Models

Log file naming: `<execution-index>-<step-name>.log` under
`.synacklab/<doc-slug>/<session-id>/steps/`, where `doc-slug` is a
filesystem-safe slug of the document's base filename. Log content: stdout,
stderr, exit code, start time, duration — plain text, one section each,
`__SYNACLAB_CAP__`/`__SYNACLAB_CWD__` lines stripped (they're metadata, not
step output).

## Error Handling

Following the `pkg/github/errors.go` pattern: a small `ErrorType` enum plus a
wrapping `*Error`:

```go
type ErrorType string

const (
    ErrorTypeParse       ErrorType = "parse"        // malformed fence attrs, duplicate names
    ErrorTypeValidation  ErrorType = "validation"    // missing input=, missing confirm
    ErrorTypeTemplate    ErrorType = "template"      // unresolved {{...}} reference
    ErrorTypeExecution   ErrorType = "execution"     // spawn failure, non-timeout process error
    ErrorTypeTimeout     ErrorType = "timeout"
)

type Error struct {
    Type    ErrorType
    Message string
    Cause   error
}
func (e *Error) Unwrap() error { return e.Cause }
```

REST handlers map these to HTTP status: `ErrorTypeParse`/`ErrorTypeValidation`/
`ErrorTypeTemplate` → 400, `ErrorTypeExecution` → 500, `ErrorTypeTimeout` is
not itself an HTTP error (it's a terminal execution state delivered over the
`done` WS event with `timed_out: true`).

## Testing Strategy

Matches the repo's existing `pkg/github` parity bar (`go test ./...` +
`golangci-lint run` before considering any task done).

### Unit Testing

1. **Parser**: fence-attribute parsing (all attrs, missing attrs, duplicate
   names, malformed syntax), frontmatter parsing, non-bash/python fences
   left static, document-order preservation.
2. **Session**: var last-write-wins, cwd default/override, reset behavior,
   concurrent access (mutex correctness under `-race`).
3. **Executor**: capture-trailer output parsing (bash + python), `set_cwd`
   trailer parsing, env merge precedence (input over session var), timeout
   kill (use a deliberately slow test command), Dir resolution (`cwd=` vs
   session cwd).
4. **Template substitution**: literal substitution correctness, missing-ref
   error, no shell-escaping (explicit test asserting raw substitution to
   guard the documented security tradeoff).
5. **Danger patterns**: match/no-match, confirm-required precedence over
   `confirm=false`.
6. **API handlers**: table-driven `httptest` cases per Requirement (missing
   inputs → 400, unconfirmed danger step → 400, valid run → 200 + execution_id).

### Integration Testing

1. End-to-end: parse a fixture `.md` with input→capture chaining, drive it
   through `serve`'s HTTP+WS API with a test client, assert final session
   vars match expected captured values.
2. `run --non-interactive`: fixture document, `--set` flags, assert exit code
   and per-step log files.
3. Timeout: fixture step with `timeout=1s` sleeping longer, assert
   `timed_out` and process-group cleanup (no orphaned child).

### Testing Approach

`testify` for assertions (existing convention). Table-driven tests for
parser/executor attribute matrices. `httptest.Server` for API/WS integration
tests — no live browser needed. Fixture runbooks live under
`pkg/runbook/testdata/`.

## Dependencies to add

- `github.com/yuin/goldmark` — Markdown/CommonMark parsing (explicitly named
  in project-brief.md §6).
- `github.com/gorilla/websocket` (proposed, see above) — WS streaming.
- Frontmatter YAML reuses the existing `gopkg.in/yaml.v3` dependency; no new
  addition needed there.
