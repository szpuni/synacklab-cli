# Implementation Plan

- [x] 1. Set up `pkg/runbook` package skeleton and core types
  - Create `pkg/runbook/types.go` with `Document`, `Block`, `Step`, `Frontmatter`
  - Create `pkg/runbook/errors.go` with `ErrorType` consts and `*Error`
  - `goldmark`/`gorilla/websocket` deferred to tasks 2 and 11 — `go mod tidy`
    strips unused requires, so they're added alongside the code that imports them
  - _Requirements: 1.1, 1.2, 1.3_

- [x] 2. Implement Markdown + fence-attribute parsing (TDD)
  - Write table-driven tests first: all attrs present, missing attrs, duplicate
    explicit names, malformed attribute syntax, non-bash/python fences,
    auto-generated names, document-order preservation
  - Implement `pkg/runbook/parser.go` (goldmark wrapper + custom fence-info
    parser) to make the tests pass
  - Prose is copied verbatim by cutting only at runnable-fence boundaries
    (not reconstructed from the AST) — goldmark's `Lines()` drops ATX heading
    markers, so AST reconstruction of prose was lossy; raw byte-range slicing
    around fence cut points isn't
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 3. Implement YAML frontmatter parsing (TDD)
  - Write tests: `default_timeout`/`danger_patterns` present, absent, partial
  - Implement `pkg/runbook/frontmatter.go` using existing `gopkg.in/yaml.v3`
  - Wired into `GoldmarkParser.Parse`: frontmatter is stripped before
    goldmark sees the source, `Document.Frontmatter` populated
  - _Requirements: 1.6, 1.7_

- [x] 4. Implement in-memory session store (TDD)
  - Write tests: `SetVar` last-write-wins, `SetCwd`, `AppendHistory`, `Reset`,
    concurrent access under `go test -race`
  - Implement `pkg/runbook/session.go`
  - `Reset` clears vars/cwd but leaves History intact (brief only specifies
    clearing vars+cwd; each execution is already logged to disk independently)
  - _Requirements: 14.1, 14.2, 14.3, 14.4_

- [x] 5. Implement capture-trailer generation and parsing (TDD)
  - Write tests: bash trailer emits `__SYNACLAB_CAP__` lines matching
    `capture=` names, python trailer equivalent, unset var captured as empty
    string, `__SYNACLAB_CWD__` line generation/parsing for `set_cwd=true`,
    stripping of sentinel lines from displayed/logged output
  - Implement `pkg/runbook/capture.go`
  - _Requirements: 6.1, 6.2, 6.3, 7.2, 7.4_

- [x] 6. Implement template substitution (TDD)
  - Write tests: `{{steps.x.stdout}}`/`{{steps.x.exit_code}}`/`{{vars.x}}`
    substitution, missing-reference error (no spawn side effect), explicit
    "no shell-escaping" assertion test
  - Implement `pkg/runbook/template.go`
  - Added `Execution.Stdout` (types.go) — kept in memory alongside the
    on-disk log so `{{steps.x.stdout}}` can resolve without reading the log
  - _Requirements: 10.1, 10.2, 10.3_

- [x] 7. Implement danger-pattern matching and confirm gating (TDD)
  - Write tests: pattern match/no-match, `confirm=true` always requires
    confirmation, danger pattern forces confirmation regardless of
    `confirm=false`
  - Implement `pkg/runbook/danger.go`
  - _Requirements: 8.1, 8.2, 8.4_

- [x] 8. Implement the execution engine core (TDD)
  - Write tests using short-lived real `bash`/`python3` subprocesses: env
    merge precedence (declared input wins over session var), `Dir` resolution
    (step `cwd=` vs session cwd), timeout kill + process-group cleanup (no
    orphaned children — verified manually with `ps` after the timeout test),
    non-timeout exit code propagation, capture/set_cwd updating the session,
    template substitution applied (and its errors surfaced) before spawn
  - Implement `pkg/runbook/executor.go` wiring capture.go + template.go into
    `Engine.Run`; danger.go/confirm gating stays a caller-side check (task 10)
    since it's a request-validation concern, not an execution-mechanics one
  - Design adjustments from design.md's sketch: `Engine.Run` takes a
    `SessionStore` (not a raw `*Session`) for thread-safe reads/writes, plus
    an explicit `timeout time.Duration` argument — `EffectiveTimeout(step,
    docDefault)` resolves the step/frontmatter/120s precedence at the call
    site, keeping the engine decoupled from `Document`/frontmatter
  - `cmd.Cancel` overridden to `syscall.Kill(-pid, SIGKILL)` so a timeout
    kills the whole process group, not just the shell/interpreter
  - _Requirements: 3.2, 3.3, 5.1, 5.2, 5.3, 7.1, 7.3, 9.1, 9.2, 9.3_

- [x] 9. Implement per-execution disk logging (TDD)
  - Write tests: log file created under `.synacklab/<doc-slug>/<session-id>/steps/`
    with correct `<n>-<name>.log` naming, contains stdout/stderr/exit
    code/timing, written even when `capture=` is absent, written on timeout
  - Implement `pkg/runbook/logstore.go`, wire into `Engine.Run`
  - Added `Execution.Stderr` (types.go) alongside the existing `Stdout` field
    so the log can include both streams; `NewEngine` now takes a `LogWriter`
    (nil disables logging, used by engine tests that don't care about that
    side effect — real `serve`/`run` call sites always pass one)
  - _Requirements: 4.1, 4.2, 4.3, 9.3_

- [x] 10. Implement REST API handlers (TDD)
  - Write `httptest`-based table-driven tests per handler: `GET /api/doc`,
    `GET /api/session`, `POST /api/session/reset`, `POST /api/steps/{name}/run`
    (missing input → 400, unconfirmed danger/confirm step → 400, valid → 202 +
    `execution_id`, unknown step → 404)
  - Implement `pkg/runbook/server.go` (`Server`, route wiring via Go 1.22+
    `http.ServeMux` method+path patterns — no external router needed) and
    `pkg/runbook/api.go` (handlers + JSON view types)
  - `server.go` also introduces `executionRegistry`: `POST .../run` starts
    the engine and returns `execution_id` immediately (202), a background
    goroutine drains that execution's event channel into a buffered record
    keyed by id — this is what task 11's WS handler will replay from, so a
    fast execution finishing before a client upgrades isn't lost
  - Execution ids are generated with `crypto/rand` rather than adding a UUID
    dependency for something this small
  - _Requirements: 2.2, 5.2, 8.1, 8.2, 14.3, 14.4_

- [x] 11. Implement WebSocket execution streaming (TDD)
  - Write tests: stdout/stderr events arrive in order, terminal `done` event
    carries `exit_code`/`duration_ms`/`captured`, late-connecting client still
    receives buffered events for a fast/already-finished execution, unknown
    execution id → 404 on the upgrade
  - Implement `pkg/runbook/ws.go`; reworked `executionRecord` in `server.go`
    into a proper publish/subscribe fan-out (`subscribe`/`publish`) so a
    late WS connection replays the buffered snapshot then keeps streaming
    live events, atomically (no events lost or duplicated at the join point)
  - **Bug found by this task's tests and fixed**: the run handler (task 10)
    passed `r.Context()` into the detached `Engine.Run` goroutine — that
    context is canceled the instant the HTTP handler returns (right after
    the 202), which killed the process almost immediately via the
    timeout/cancel machinery from task 8, before it could produce output.
    Switched to `context.Background()` for the async run.
  - _Requirements: 3.4, 3.5_

- [x] 12. Build the embedded frontend SPA
  - Implement `pkg/runbook/web/{index.html,app.js,app.css}`: render prose +
    steps in order, Run controls, input form fields gating Run, live
    stdout/stderr panel via the WS endpoint, session var/cwd display that
    updates without reload, confirmation prompt showing the matched danger
    pattern
  - Wire `go:embed` in `pkg/runbook/server.go`
  - Added server-side prose rendering (`renderProseHTML` in api.go, via the
    already-imported goldmark) and `stepView.RequiresConfirm`/`ConfirmReason`
    (Requirement 8.3) — both covered by unit tests, since they're backend
    logic; also added `server_test.go` asserting the embedded SPA is served
    correctly (index.html, app.js/app.css content, app.js referencing the
    real API/WS paths) as the closest thing to a smoke test available here
  - **Not done**: manual in-browser verification — no browser tooling is
    available in this environment, and there's no `serve` CLI command yet
    (task 13) to launch against. Flagging honestly per instructions rather
    than claiming it was checked; worth doing once task 13 lands
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 8.3_

- [x] 13. Implement `synacklab serve` command
  - Create `internal/cmd/runbook_serve.go`: `--port 4747`, `--bind 127.0.0.1`,
    `--cwd path` flags; loopback-only default; explicit warning banner when
    `--bind` is non-loopback
  - Write unit tests for flag parsing/defaults and the bind-warning trigger
  - Exported `runbook.NewID()` (renamed from the execution-registry-only
    `newExecutionID`) for the CLI to generate a session id
  - **Manually verified end-to-end** (real binary, not just httptest): built
    `./bin/synacklab`, ran `serve` against a fixture runbook in the
    background, then drove it for real — `curl` on `/`, `/app.js`,
    `/api/doc`; `POST /api/steps/get_ip/run`; a small Python WebSocket client
    reading `/ws/executions/{id}` to completion; `GET /api/session` showing
    the captured var; and the `.synacklab/.../steps/1-get_ip.log` file on
    disk. All matched expected behavior. (Hit and fixed my own test-harness
    mistake along the way — a shell `&` background attempt outlived its
    subshell and held the port for the next attempt; not a product bug.)
  - _Requirements: 13.1, 13.2, 13.3_

- [x] 14. Implement `synacklab run --non-interactive`
  - Create `internal/cmd/runbook_run.go` reusing `pkg/runbook` parser +
    engine directly (no HTTP layer), `--set VAR=value` flags, top-to-bottom
    execution, stop-on-first-nonzero-exit, fail-closed on
    `confirm=true`/danger-pattern steps
  - Write unit tests: missing `--set` for declared `input=` fails before
    execution, non-zero step halts remaining steps, confirm/danger steps
    fail closed, per-step logs written same as `serve` (same `Engine`/
    `FileLogWriter` as `serve`, so this is inherited rather than re-tested)
  - `--non-interactive` is a required flag (errors without it) rather than
    an implicit default — v1 has no interactive terminal prompt path at all,
    so this makes that explicit instead of silently ignoring the missing flag
  - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5_

- [ ] 15. Implement `synacklab fmt`
  - Create `internal/cmd/runbook_fmt.go` and canonical-attribute-order
    rewriting in `pkg/runbook/parser.go` (or a small `fmt.go` sibling)
  - Write unit tests: canonical key order/spacing applied, prose/code/attr
    values unchanged, parse error leaves file untouched
  - _Requirements: 12.1, 12.2, 12.3_

- [ ] 16. Integration tests
  - End-to-end fixture runbook driven through `serve`'s HTTP+WS API
    (input→capture chaining across two steps), assert final session state
  - `run --non-interactive` fixture with `--set`, assert exit code + logs
  - Timeout fixture (`timeout=1s` sleeping longer), assert `timed_out` +
    no orphaned child process
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 6.4, 9.1, 9.2, 11.1–11.5_

- [ ] 17. Documentation and example runbook
  - Add a `docs/` (or README) section covering `serve`/`run`/`fmt`, the fence
    attribute table, and an explicit security note on `{{...}}` template
    substitution vs. `input=`/`capture=` (per project-brief.md §13)
  - Add an example runbook `.md` fixture demonstrating input, capture,
    set_cwd, confirm, and danger_patterns
  - _Requirements: 10.4, 13.2_

- [ ] 18. Full-suite verification
  - `go test ./...` (including `-race` for session/executor packages)
  - `golangci-lint run`
  - Confirm no file in `pkg/runbook/` or `internal/cmd/` exceeds 500 lines;
    split (e.g. `executor_bash.go`/`executor_python.go`, `api_steps.go`) if so
  - _Requirements: all_
