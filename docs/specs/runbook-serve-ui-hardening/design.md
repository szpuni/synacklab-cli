# Design Document

## Overview

This is a hardening pass over the shipped `pkg/runbook` package and its
embedded frontend — no new packages, no architectural change. Every change
below lands inside the existing files named in `docs/specs/
runbook-server-bootstrap/design.md`'s Package Structure (`pkg/runbook/*.go`
and `pkg/runbook/web/*`), following that design's existing conventions:
`*Error` types with an `ErrorType` constant, `SessionStore`/`Engine` as the
seams for new behavior, and the existing `stepView`/`sessionView` JSON
mapping layer in `api.go` for anything new that crosses the wire. Two small
new REST endpoints are added (a log-content endpoint for Requirement 2 and a
cancel endpoint for Requirement 4); everything else is a change to existing
handlers, existing types, or the frontend alone. File-size discipline (500
lines/file) is re-checked per file below; `api.go` (196 lines) and
`executor.go` (250 lines) have the most headroom used up and are the ones to
watch if a future pass adds more to them.

## Requirement 1 — Confirmation gate reset (steps + reset button)

**Files:** `pkg/runbook/web/app.js`, `pkg/runbook/web/index.html` only. No
backend change: whether a step requires confirmation is already computed
server-side (`RequiresConfirmation` in `danger.go`, surfaced as `stepView.
RequiresConfirm`/`ConfirmReason`); this is purely a frontend state-machine
bug plus reuse of the same pattern for the reset control.

- Extract a small reusable helper, e.g. `createConfirmGate(button, {armLabel,
  baseLabel, onConfirmed})`, used by both `renderStep()`'s Run button and the
  reset button's click handler (`index.html`'s `#reset-btn`). It owns a
  single `armed` boolean and exposes `handleClick()` (arm-or-fire) and
  `reset()` (force back to unarmed + restore `baseLabel`).
- In `renderStep()`, replace the current one-shot `let confirmed = ...`
  closure variable with a `createConfirmGate` instance. Call `gate.reset()`
  in the `done` branch of `streamExecution()`'s `ws.onmessage` handler
  (currently just `runBtn.disabled = false`) and in the pre-execution error
  paths in `runStep()` (`if (!res.ok) { ... }`) and `streamExecution()`'s
  `ws.onerror`, everywhere the existing code already re-enables `runBtn`.
- For the reset button (`index.html`'s `#reset-btn`, wired in `app.js`'s
  bottom `document.getElementById("reset-btn").addEventListener(...)`),
  wrap the existing handler with the same `createConfirmGate`: first click
  arms and relabels the button text to "Confirm reset" (mirroring the
  step Run control's "Confirm & Run"), second click calls the existing
  `POST /api/session/reset` + `loadDoc()` logic, and the gate resets to
  unarmed once that completes (success or failure).
- No new wire format: this requirement is entirely about frontend event
  sequencing.

## Requirement 2 — Session state visibility (vars, history, logs)

**Files:** `pkg/runbook/api.go`, `pkg/runbook/types.go`, new
`pkg/runbook/logstore.go` addition, `pkg/runbook/web/{app.js,app.css,
index.html}`.

- `execHistoryView` (`api.go`) is missing a start timestamp even though
  `Execution.Started` (`types.go`) already has one. Add `StartedAt
  time.Time \`json:"started_at"\`` to `execHistoryView` and populate it from
  `e.Started` in `toSessionView`.
- New read-only endpoint to serve a log's contents to the browser, since
  `LogPath` is an absolute filesystem path the browser cannot fetch
  directly: `GET /api/logs?path=<log_path>`. Handler
  (`handleGetLog` in `api.go`) validates that the requested path (i) is one
  of the current session's own `History[].LogPath` values (looked up by
  exact match, not just a directory-prefix check, to avoid exposing any
  other file in `.synacklab/`) and (ii) exists, then serves it as
  `text/plain`. Rejecting anything not literally present in the session's
  own history means this endpoint cannot be used as an arbitrary local file
  reader even if `path` is attacker-influenced — it can only ever return a
  log this same session already told the client about via `GET /api/session`
  or `GET /api/doc`.
- `app.js`: `refreshSession()` currently reads only `session.cwd`. Extend it
  (and the initial `renderDoc()` path, which already receives
  `doc.session`) to also render:
  - A vars chip list (new `#vars-panel` element in `index.html`'s topbar or
    a new strip under it; new `.var-chip` rule in `app.css`), one chip per
    `session.vars` entry, `name=value`.
  - Per-step last-run info: when rendering a step card in `renderStep()`,
    look up the most recent `session.history` entry with matching
    `step_name` (last one wins — history is append-only in order) and, if
    found, render a small `.step-last-run` line (exit code, duration,
    relative/absolute time from the new `started_at` field, and — if
    `log_path` is set — a "View log" control that `fetch`es
    `/api/logs?path=...` and shows the result in a simple modal/expandable
    block rather than navigating away).
  - Both of the above are driven from `doc.session` on `renderDoc()` (covers
    initial load and `POST /api/open` switches) and from the response of
    `GET /api/session` after `refreshSession()` (covers post-execution
    updates) — satisfying Acceptance Criteria 4's "rebuilt from server
    responses, not accumulated client state."
- `app.css`: `.var-chip` (pill, monospace value, sits well against
  `--bg-surface`), `.step-last-run` (small muted text row above the step's
  output panel).

## Requirement 3 — Safe reopen / switch confirmation

**Files:** `pkg/runbook/workspace.go`, `pkg/runbook/web/app.js`.

- `handlePostOpen` (`workspace.go`) currently always does `store :=
  NewSessionStore(NewID(), absPath, cwd); s.active.set(doc, store)`. Change
  it to:
  1. Resolve `absPath` as today, then read the currently active doc via
     `s.active.get()`.
  2. If the active doc is non-nil and its `Path == absPath` (already the
     open file): skip creating a new `Document`/`SessionStore` entirely
     and return `200 {"opened": relPath, "unchanged": true}` without
     re-parsing or touching session state (Acceptance Criteria 1). This
     also incidentally avoids an unnecessary re-parse of the same file.
  3. Otherwise, if the active session (from `s.active.get()`'s store) has a
     non-empty `Vars` or non-empty `History`, and the request body's new
     `Force bool` field (added to `openRequest`) is not `true`: respond
     `409 Conflict` with `{"error": "...", "requires_confirmation": true}`
     instead of proceeding (Acceptance Criteria 2).
  4. Otherwise (different file, empty session, or `Force: true`): proceed
     exactly as today — parse, build a fresh `SessionStore`, `s.active.set`.
- `app.js`'s `openFile()`: on a `409` response with `requires_confirmation:
  true` in the body, show a `confirm()` dialog ("Switching runbooks will
  discard N captured variable(s) and M run(s) of history for the current
  session. Continue?", using the current session's `vars`/`history` counts
  already available from the last `loadDoc()`); on confirmation, retry the
  same `POST /api/open` call with `{file, force: true}` in the body.
- Requirement 6.4 layers an additional in-flight-execution check onto this
  same handler (see below) — both checks live in `handlePostOpen`, execution
  in-flight checked first since it's the more urgent conflict.

## Requirement 4 — Output scroll cap and step cancellation

**Files:** `pkg/runbook/web/app.css`, `pkg/runbook/web/app.js`,
`pkg/runbook/executor.go`, `pkg/runbook/server.go`, `pkg/runbook/ws.go`,
`pkg/runbook/types.go`.

### Scroll containment (frontend-only)

- `app.css`'s `.output` rule gains `max-height: 320px; overflow-y: auto;`
  (currently only `min-height`, no cap).
- `app.js`'s `streamExecution()`'s `ws.onmessage`: after appending a line to
  `output`, check whether the panel was scrolled to (or near) its bottom
  before the append, and if so, set `output.scrollTop = output.scrollHeight`
  after — a standard "pinned tail" pattern that still lets a user scroll up
  to read earlier output without being yanked back down.

### Cancellation (backend + frontend)

- `Engine.Run`'s signature changes to return a `context.CancelFunc`
  alongside the existing `(*Execution, <-chan Event, error)`, so a caller
  can cancel a specific in-flight execution instead of only the shared
  `baseCtx`:
  `Run(ctx, step, inputs, timeout, store) (*Execution, <-chan Event,
  context.CancelFunc, error)`. Internally `executor.go` already creates
  `runCtx, cancel := context.WithTimeout(ctx, timeout)` — it is simply
  returned instead of only being deferred internally.
- `Execution` (`types.go`) gains `Canceled bool` alongside the existing
  `TimedOut bool`, and `Event` (`types.go`) gains the same. In `executor.go`'s
  `wait()`, distinguish the three terminal cases from `runCtx.Err()`:
  `nil` → normal exit, `context.DeadlineExceeded` → `TimedOut`,
  `context.Canceled` → `Canceled` (only reachable once something besides the
  timeout calls `cancel()`, i.e. the new cancel endpoint or server shutdown).
- `server.go`'s `executionRecord` gains a `cancel context.CancelFunc` field,
  set when `executionRegistry.start` is called (its signature gains a
  `cancel context.CancelFunc` parameter, passed through from
  `handlePostStepRun`'s new `Engine.Run` return value).
- New endpoint: `POST /api/executions/{id}/cancel`, handler
  `handleCancelExecution` (added to `ws.go`, alongside the other
  execution-id-keyed handler). Looks up the `executionRecord` by id; if
  found and not yet done, calls its stored `cancel()` and returns `202`;
  if already done (`rec.done` closed) or unknown id, returns `200`/`404`
  respectively without erroring the caller for a harmless late cancel
  (Acceptance Criteria 6).
- Wire format: the terminal WS `done` event (`ws.go`'s `wsEvent`) gains a
  `canceled` field (`*bool`, same `omitempty`-via-pointer pattern already
  used for `timed_out`), set from the new `Execution.Canceled`.
- `app.js`: while `output.hidden` is false and no terminal event has
  arrived yet, show a small "Stop" button in the step's `.step-actions` row
  (built in `renderStep()`, alongside the existing Run button); its click
  handler calls `POST /api/executions/{execution_id}/cancel`, and on the
  eventual `done` event with `canceled: true`, render a distinct status
  line ("canceled") instead of the exit-code line.
- No stdin channel to the browser is added — matches the finding's scope
  (cap + cancel only); a step waiting on stdin still has to be canceled
  rather than fed input, and that remains a documented limitation.

## Requirement 5 — Sensitive/masked input fields

**Files:** `pkg/runbook/parser.go`, `pkg/runbook/types.go`,
`pkg/runbook/fmt.go`, `pkg/runbook/api.go`, `pkg/runbook/web/{app.js,
app.css}`.

- `types.go`: `Step` gains `Sensitive []string`, parsed the same way
  `Input`/`Capture` already are.
- `parser.go`'s `stepFromFence`: add
  `if v := attrs["sensitive"]; v != "" { step.Sensitive = splitTrim(v) }`,
  matching the existing `input=`/`capture=` handling exactly (same
  `splitTrim` helper, same comma-separated convention already established
  by `parseFenceAttrs`'s "value continuation" rule — no parser changes
  needed there, `sensitive=A,B` already tokenizes correctly today).
- Validation (Acceptance Criteria 2): after building `step.Sensitive`,
  check each name is present in `step.Input`; if not, return the same
  `&Error{Type: ErrorTypeParse, ...}` pattern used for duplicate step names,
  naming the step and the offending attribute value.
- `fmt.go`: add `"sensitive"` to `canonicalAttrOrder` (currently `[]string{
  "name", "input", "capture", "confirm", "cwd", "set_cwd", "timeout"}`),
  positioned right after `"capture"` since it semantically modifies
  `input=`.
- `api.go`'s `stepView` gains `Sensitive []string \`json:"sensitive,
  omitempty"\``, populated from `b.Step.Sensitive` in `handleGetDoc`.
- `app.js`'s `renderStep()`: when building each input field, check
  `step.sensitive && step.sensitive.includes(name)`. If sensitive: set
  `input.type = "password"; input.autocomplete = "off";` and append a small
  reveal-toggle button (`type="button"`, e.g. an eye glyph) next to the
  field that flips `input.type` between `"password"` and `"text"` on click
  — masked by default (Acceptance Criteria 3–4).
- Requirement 2's vars chip list also consults `sensitive` sets: `app.js`
  tracks which var names (by the step-qualified and short-alias keys
  `session.vars` can contain, per `capture.go`'s existing key convention)
  were ever declared sensitive by their capturing step's `Sensitive` list,
  and renders those chips masked with the same reveal toggle (Acceptance
  Criteria 5). This only needs the already-loaded `doc.blocks[].step.
  sensitive` data cross-referenced against `session.vars` keys — no new API
  surface.
- `app.css`: small `.reveal-toggle` button style (ghost/icon button sized to
  sit inline with `.field-input`).

## Requirement 6 — Execution serialization

**Files:** `pkg/runbook/server.go`, `pkg/runbook/api.go`,
`pkg/runbook/workspace.go`, `pkg/runbook/web/app.js`.

**Decision: server-side reject, not queue**, enforced as the source of
truth, with a frontend mirror for responsiveness. Rejecting (rather than
queuing) is the better fit for this tool's explicit "nothing executes
without a click, at the time of the click" framing (project-brief.md §1,
§7.1): a queued second request would eventually execute without a fresh
click at the moment it actually runs, which is the same class of problem
Requirement 1 exists to close for confirmation. A reject-and-let-the-user-
retry model keeps every execution tied to a live, current user action.
Server-side enforcement (rather than relying on the frontend alone) is
necessary because the frontend-only mitigation in the original design
(each Run button only disables itself) is exactly the gap this finding
identifies, and a second browser tab or a raw `curl` call bypasses any
frontend-only guard entirely.

- `Server` (`server.go`) gains a small in-flight guard: either a
  `sync.Mutex` held for the duration of an execution (simplest, but would
  block the handler goroutine) or, preferably, an `atomic.Pointer[string]`
  (or a mutex-guarded `string`) holding the currently-running execution id,
  set in `handlePostStepRun` right before calling `s.engine.Run` and
  cleared by the same goroutine in `executionRegistry.start`'s existing
  event-draining loop when it observes the `"done"` event (the same place
  that already logs completion) — no new goroutine needed.
- `handlePostStepRun` (`api.go`): before the existing `CheckConfirmation`
  call, check the in-flight guard; if set, respond `409 Conflict` with
  `{"error": "step \"<other>\" is still running (execution <id>)"}` instead
  of calling `s.engine.Run` (Acceptance Criteria 1). No change to
  `Engine`/`ProcessEngine` itself — this is a request-admission check at the
  API layer, consistent with how `CheckConfirmation` is already a
  caller-side check per the original design's task 8 note ("danger.go/
  confirm gating stays a caller-side check ... since it's a request-
  validation concern, not an execution-mechanics one").
- `handlePostOpen` (`workspace.go`): add the same in-flight check ahead of
  the Requirement 3 vars/history check — switching documents while a step
  is running is refused with `409` regardless of `force`, since canceling
  or waiting out that execution is a precondition, not something `force`
  should paper over (Acceptance Criteria 4).
- `app.js`: introduce a single module-level flag (or derive it from
  tracking the currently-rendered Run buttons), set when `runStep()` starts
  and cleared on the terminal WS event; while set, every other step's Run
  button and the reset button are set `disabled = true` (in addition to the
  running step's own button, which is already disabled today) — re-enabled
  together when the flag clears (Acceptance Criteria 3). A rejected `409`
  from a race (e.g. two tabs) surfaces via the existing error-handling path
  in `runStep()` (`if (!res.ok) { ... output.textContent = ... }`), so no
  new error-handling code path is required there, only the extra
  preemptive `disabled` toggling.

## Requirement 7 — Contrast fixes

**Files:** `pkg/runbook/web/app.css` only (token values under `:root`).

Computed contrast ratios (WCAG relative-luminance formula) for the current
values, and the replacements:

| Element | Current value | On background | Current ratio | New value | New ratio |
|---|---|---|---|---|---|
| `.cwd` (`--text-muted`) | `#94a3b8` | `--bg-surface` `#ffffff` | approx 2.56:1 | `#64748b` | approx 4.76:1 |
| `.tree-dir > .tree-label` | `rgba(255,255,255,0.45)` | `--sidebar-bg` `#1e1b4b` | approx 4.29:1 | `rgba(255,255,255,0.58)` | approx 4.96:1 |
| `.tree-empty` | `rgba(255,255,255,0.40)` | `--sidebar-bg` `#1e1b4b` | approx 3.67:1 | `rgba(255,255,255,0.58)` | approx 4.96:1 |

- `--text-muted` changes from `#94a3b8` to `#64748b` (Tailwind's slate-500)
  in `:root`. This token is also used elsewhere in `app.css`
  (`.empty-state`); both uses are against light backgrounds (`--bg-base`/
  `--bg-surface`), so the new value clears 4.5:1 in both places —
  Acceptance Criteria 5's no-regression check passes by inspection (the
  darker replacement can only raise contrast against a light background
  relative to the lighter original).
- A new token `--sidebar-text-muted: rgba(255, 255, 255, 0.58)` is added to
  `:root` and used in place of the two hardcoded rgba literals in
  `.tree-dir > .tree-label` (`color`) and `.tree-empty` (`color`). Using one
  shared token for both (rather than two separate values) means a future
  change only has to preserve one contrast computation.
- `.tree-dir > .tree-label:hover`'s `rgba(255, 255, 255, 0.7)` already
  exceeds 4.5:1 (approx 6.9:1) and is left unchanged.
- No other sidebar or topbar text color is changed; `--text-secondary`
  (`#475569`) and `--text-primary` (`#0f172a`) both already clear 4.5:1
  against `--bg-surface`/`--bg-base` and are out of scope.

## Cross-cutting notes

- Requirements 3 and 6 both add checks to `handlePostOpen`; implement
  Requirement 6's in-flight check first in that handler's body so the two
  new failure modes have a clear precedence (can't-switch-while-running is
  checked before can't-switch-without-confirming-discard).
- Requirement 4's cancellation endpoint is what makes Requirement 6's
  reject-based serialization tolerable in practice — without a way to stop
  a hung step, a strict single-in-flight guard would let one wedged step
  block the whole runbook until its timeout. Implement Requirement 4 before
  or alongside Requirement 6, not after.
- Requirement 5's `Sensitive` field and Requirement 2's vars display are
  independent but interact (masking captured values) — implement
  Requirement 5's parser/type/fmt changes before Requirement 2's frontend
  vars-chip rendering so the masking cross-reference has data to consult.

## Testing Strategy

Matches the original spec's bar: `testify`, table-driven tests for parser/
attribute changes, `httptest`-based handler tests for the two new/changed
endpoints, and `go test ./... -race` + `golangci-lint run` before considering
any task done.

- **Parser (`parser_test.go`, `fmt_test.go`)**: `sensitive=` parsing,
  `sensitive` name not in `input=` → parse error, canonical attribute order
  includes `sensitive`, idempotent formatting unaffected.
- **Session/executor (`session_test.go`, `executor_test.go`)**: `Engine.Run`
  returns a usable `context.CancelFunc`; calling it mid-execution sets
  `Canceled` (not `TimedOut`) on the resulting `Execution`/terminal `Event`,
  kills the process group (assert no orphaned child, same technique the
  existing timeout test already uses), and does not block the caller past a
  short bound.
- **API (`api_test.go`, new `workspace_api_test.go` cases)**: `POST /api/
  steps/{name}/run` returns `409` when another execution is in flight;
  `GET /api/logs?path=...` returns `200` for a path present in the current
  session's history and `404`/`400` for anything else (including a path
  belonging to a *different*, non-active session, and a path-traversal
  attempt); `POST /api/open` returns `200 {"unchanged": true}` for the
  already-active file without constructing a new store (assert via a session
  var set before the call surviving after it); returns `409
  {"requires_confirmation": true}` when switching away from a
  non-empty session without `force`; proceeds with `force: true`; returns
  `409` when an execution is in flight regardless of `force`.
- **WS (`ws_test.go`)**: canceled execution's terminal event carries
  `canceled: true` and not `timed_out: true`; canceling an already-done
  execution id is a no-op, not an error.
- **Frontend**: no JS test runner exists in this repo today (per the
  original spec's testing strategy, frontend verification was manual
  against the real binary); continue that pattern — manually verify against
  `./bin/synacklab runbook serve` in a real browser: confirmation-gate reset
  after a run, vars/history rendering surviving a reload, the reopen/switch
  confirmation dialog, output panel scrolling with a verbose command, the
  stop button actually killing a `sleep`-based step, masked/reveal input
  behavior, second-tab run rejection, and contrast (a contrast-checker
  extension or the browser devtools' built-in one) on the three changed
  elements.
