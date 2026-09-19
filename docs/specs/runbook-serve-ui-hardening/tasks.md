# Implementation Plan

- [x] 1. Confirmation-gate reset: shared frontend helper (TDD via manual browser verification, no JS test runner in this repo)
  - Implement `createConfirmGate(button, {armLabel, baseLabel, onConfirmed})`
    in `pkg/runbook/web/app.js`
  - Wire it into `renderStep()`'s Run button, replacing the one-shot
    `let confirmed = ...` closure
  - Reset the gate in `streamExecution()`'s `done` branch, in `runStep()`'s
    pre-execution error path, and in `ws.onerror`
  - Manually verify: arm a `confirm=true` step, run it, confirm the button
    relabels back to unarmed and a second click re-arms rather than firing
  - _Requirements: 1.1, 1.2, 1.3, 1.6_

- [x] 2. Confirmation-gate reset: apply to the reset-session control
  - Wrap `index.html`'s `#reset-btn` handler in `app.js` with the same
    `createConfirmGate` from task 1
  - Manually verify: clicking "Reset session" once arms/relabels it
    ("Confirm reset"), does not call the API; a second click resets and
    restores the base label
  - _Requirements: 1.4, 1.5, 1.6_

- [x] 3. Add execution start time to the session wire format (TDD)
  - Write a test in `pkg/runbook/api_test.go` asserting `toSessionView`
    populates a `started_at` field on each history entry from
    `Execution.Started`
  - Add `StartedAt time.Time` to `execHistoryView` in `pkg/runbook/api.go`
    and populate it in `toSessionView`
  - _Requirements: 2.5_

- [x] 4. Log-content endpoint (TDD)
  - Write `httptest` table-driven tests in `pkg/runbook/api_test.go`:
    `GET /api/logs?path=<value from the active session's own history>` →
    200 + file contents; a path not present in the active session's history
    (including one belonging to a different/prior session) → 404; missing
    `path` query param → 400
  - Implement `handleGetLog` in `pkg/runbook/api.go`, matching `path`
    against `store.Get().History[].LogPath` by exact string match (not a
    prefix/containment check) before reading and serving the file as
    `text/plain`
  - Register `GET /api/logs` in `Routes()` (`pkg/runbook/server.go`)
  - _Requirements: 2.3_

- [x] 5. Frontend: vars chip list and per-step last-run display
  - Add `#vars-panel` container to `pkg/runbook/web/index.html`; add
    `.var-chip`/`.step-last-run` rules to `pkg/runbook/web/app.css`
  - In `pkg/runbook/web/app.js`, extend `refreshSession()` and `renderDoc()`
    to render vars chips from `session.vars` and, per step, the most recent
    matching `session.history` entry (exit code, duration, `started_at`,
    and a "View log" control calling the task 4 endpoint when `log_path` is
    present)
  - Manually verify: run two steps, reload the page, confirm vars and both
    steps' last-run info reappear without re-running anything; click "View
    log" and confirm it shows the on-disk content
  - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [x] 6. Safe reopen: skip no-op reopen of the active file (TDD)
  - Write a test in `pkg/runbook/workspace_api_test.go`: `POST /api/open`
    with the already-active file's path leaves an existing session var
    untouched and does not error
  - Implement the early-return branch in `handlePostOpen`
    (`pkg/runbook/workspace.go`) comparing the resolved `absPath` against
    the active document's `Path`
  - _Requirements: 3.1_

- [x] 7. Safe reopen: confirm-before-discard when switching to a different file (TDD)
  - Write tests in `pkg/runbook/workspace_api_test.go`: switching away from
    a session with non-empty vars/history without `force` → 409 with
    `requires_confirmation: true`; with `force: true` → 200 and the session
    is replaced; switching away from an empty session (no vars, no history)
    without `force` → 200 (no confirmation needed)
  - Add `Force bool` to `openRequest` and implement the check in
    `handlePostOpen`
  - _Requirements: 3.2, 3.4, 3.5_

- [x] 8. Frontend: switch-file confirmation dialog
  - In `pkg/runbook/web/app.js`'s `openFile()`, handle a 409 response
    carrying `requires_confirmation: true` by prompting the user (using
    already-loaded vars/history counts) and, on confirmation, retrying with
    `{file, force: true}`
  - Manually verify: capture a var, click a different file in the sidebar,
    confirm the browser prompt appears and canceling it leaves the original
    session and document active
  - _Requirements: 3.3_

- [x] 9. Output panel scroll containment (CSS + JS)
  - Add `max-height`/`overflow-y: auto` to `.output` in
    `pkg/runbook/web/app.css`
  - In `pkg/runbook/web/app.js`'s `streamExecution()`, pin-to-bottom
    auto-scroll on new line append when already scrolled to bottom
  - Manually verify with a step that prints hundreds of lines: the step
    card stays a fixed height, the Run button stays visible, and the panel
    auto-scrolls while pinned but stays put once scrolled up
  - _Requirements: 4.1, 4.2_

- [x] 10. Engine: expose a per-execution cancel function (TDD)
  - Write tests in `pkg/runbook/executor_test.go`: calling the returned
    `context.CancelFunc` on a long-running step kills its process group
    (assert no orphaned child, mirroring the existing timeout test), sets
    `Execution.Canceled = true` and `Execution.TimedOut = false` on the
    resulting execution, and the terminal `Event` carries `Canceled: true`
  - Add `Canceled bool` to `Execution` and `Event` in `pkg/runbook/types.go`
  - Change `Engine.Run`'s signature to also return a `context.CancelFunc`;
    update `ProcessEngine.Run` (`pkg/runbook/executor.go`) to return the
    existing internal `cancel` instead of only deferring it, and update
    `wait()` to distinguish `context.Canceled` from
    `context.DeadlineExceeded` on `runCtx.Err()`
  - Update all existing callers/tests of `Engine.Run`'s signature
    (`pkg/runbook/api.go`'s `handlePostStepRun`, `internal/cmd/
    runbook_run.go`'s non-interactive runner, and any test fixtures) to
    match the new return signature
  - _Requirements: 4.4, 4.5_

- [x] 11. Cancel endpoint and registry wiring (TDD)
  - Write tests in `pkg/runbook/ws_test.go` (or a new `ws_cancel_test.go`):
    `POST /api/executions/{id}/cancel` on a running execution returns 202
    and the execution's eventual terminal event has `canceled: true`; on an
    already-finished execution returns 200 without error; on an unknown id
    returns 404
  - Add `cancel context.CancelFunc` to `executionRecord` and a `cancel
    context.CancelFunc` parameter to `executionRegistry.start`
    (`pkg/runbook/server.go`); update `handlePostStepRun`
    (`pkg/runbook/api.go`) to pass the `Engine.Run` cancel func through
  - Implement `handleCancelExecution` in `pkg/runbook/ws.go`; register
    `POST /api/executions/{id}/cancel` in `Routes()`
    (`pkg/runbook/server.go`)
  - Add `Canceled *bool` to `wsEvent` (`pkg/runbook/ws.go`) and populate it
    in `toWSEvent` from `Execution`/`Event.Canceled`
  - _Requirements: 4.4, 4.5, 4.6_

- [x] 12. Frontend: stop control
  - In `pkg/runbook/web/app.js`'s `renderStep()`/`runStep()`, show a "Stop"
    button in `.step-actions` while an execution is outstanding; wire its
    click to `POST /api/executions/{execution_id}/cancel`; render a
    distinct "canceled" status line in `streamExecution()` on a `done` event
    with `canceled: true`
  - Manually verify: start a `sleep 30`-style step, click Stop, confirm the
    UI shows "canceled" within a second or two and no orphaned process
    remains (`ps aux` on the host)
  - _Requirements: 4.3, 4.5_

- [x] 13. Parser: `sensitive=` fence attribute (TDD)
  - Write tests in `pkg/runbook/parser_test.go`: `sensitive=A` on a step
    with `input=A,B` parses into `Step.Sensitive == ["A"]`; `sensitive=C` on
    a step whose `input=` does not include `C` is a parse error naming the
    step and `C`; `sensitive` absent leaves `Step.Sensitive` nil
  - Add `Sensitive []string` to `Step` in `pkg/runbook/types.go`; implement
    parsing and the input-superset validation in `stepFromFence`
    (`pkg/runbook/parser.go`)
  - _Requirements: 5.1, 5.2_

- [x] 14. Formatter: include `sensitive` in canonical attribute order (TDD)
  - Write a test in `pkg/runbook/fmt_test.go`: a fence with attributes in
    arbitrary order including `sensitive=` is rewritten with `sensitive`
    positioned per the new canonical order, values unchanged
  - Add `"sensitive"` to `canonicalAttrOrder` in `pkg/runbook/fmt.go`
    (after `"capture"`)
  - _Requirements: 5.6_

- [x] 15. API: surface `sensitive` in the step view (TDD)
  - Write a test in `pkg/runbook/api_test.go`: `GET /api/doc` includes
    `sensitive` on a step that declares it and omits the field otherwise
  - Add `Sensitive []string \`json:"sensitive,omitempty"\`` to `stepView`
    and populate it in `handleGetDoc` (`pkg/runbook/api.go`)
  - _Requirements: 5.1_

- [x] 16. Frontend: masked input fields with reveal toggle
  - In `pkg/runbook/web/app.js`'s `renderStep()`, render `type="password"`
    + `autocomplete="off"` for input fields listed in `step.sensitive`,
    with a reveal-toggle button; add `.reveal-toggle` styling to
    `pkg/runbook/web/app.css`
  - Manually verify: a step with `input=TOKEN {sensitive=TOKEN}` renders a
    masked field, browser autofill does not offer to save it, and the
    reveal toggle shows/hides the typed value
  - _Requirements: 5.3, 5.4_

- [x] 17. Frontend: mask sensitive values in the vars chip list
  - Extend task 5's vars-chip rendering in `pkg/runbook/web/app.js` to
    cross-reference `doc.blocks[].step.sensitive` against `session.vars`
    keys and mask matching chips by default, with the same reveal toggle
    from task 16
  - Manually verify: capture a sensitive var, confirm its chip shows masked
    and the toggle reveals it
  - _Requirements: 5.5_

- [x] 18. Server: single in-flight execution guard (TDD)
  - Write `httptest` tests in `pkg/runbook/api_test.go`: while one step's
    execution is in flight, a second `POST /api/steps/{name}/run` (same or
    different step) returns 409 naming the running execution; once that
    execution's terminal event has fired, a subsequent run request succeeds
  - Implement the in-flight tracking (mutex-guarded current-execution-id
    field on `Server` or `executionRegistry`) in `pkg/runbook/server.go`,
    set in `handlePostStepRun` before calling `Engine.Run` and cleared in
    `executionRegistry.start`'s existing event-drain loop on the `"done"`
    event
  - _Requirements: 6.1, 6.2_

- [x] 19. Server: refuse document switch while an execution is in flight (TDD)
  - Write a test in `pkg/runbook/workspace_api_test.go`: `POST /api/open`
    (with or without `force`) while an execution is in flight returns 409
  - Add the in-flight check to `handlePostOpen`
    (`pkg/runbook/workspace.go`), checked before the task 7
    vars/history-confirmation check
  - _Requirements: 6.4_

- [x] 20. Frontend: disable all other controls while a step runs
  - In `pkg/runbook/web/app.js`, track an in-flight flag across all step
    cards; while set, disable every Run button (other than the one already
    disabled by the current execution) and the reset control; re-enable
    together on the terminal event
  - Manually verify: start a slow step, confirm every other step's Run
    button and the reset button are disabled until it finishes, then
    re-enabled
  - _Requirements: 6.3_

- [x] 21. Contrast token fixes
  - In `pkg/runbook/web/app.css`'s `:root`, change `--text-muted` from
    `#94a3b8` to `#64748b`; add `--sidebar-text-muted: rgba(255, 255, 255,
    0.58)` and use it in place of the hardcoded `rgba(255, 255, 255, 0.45)`
    on `.tree-dir > .tree-label` and `rgba(255, 255, 255, 0.40)` on
    `.tree-empty`
  - Manually verify with a contrast-checking tool (browser devtools or an
    extension) against the live page: `.cwd`, `.tree-dir > .tree-label`,
    and `.tree-empty` each measure at least 4.5:1 against their respective
    backgrounds; spot-check `.empty-state` (also using `--text-muted`) and
    `.tree-dir > .tree-label:hover` still pass
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

- [x] 22. Full-suite verification
  - `go build ./...`, `go vet ./...`: clean
  - `go test ./... -race`: all packages pass, including every new/changed
    test in `pkg/runbook`
  - `golangci-lint run`: 0 new issues
  - File sizes: confirm `api.go`, `executor.go`, `server.go`, `ws.go`,
    `workspace.go` remain under the repo's 500-line limit after these
    changes; split (e.g. `api.go` → `api.go`/`api_logs.go`) if any exceeds
    it, per the pattern used for `pkg/github/errors.go`
  - `make build` succeeds
  - _Requirements: all_
