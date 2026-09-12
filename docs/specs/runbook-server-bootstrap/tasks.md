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

- [x] 15. Implement `synacklab fmt`
  - Create `internal/cmd/runbook_fmt.go` and canonical-attribute-order
    rewriting in `pkg/runbook/fmt.go` (sibling to parser.go, sharing its
    fence-scanning/`parseFenceAttrs` helpers rather than duplicating them)
  - Write unit tests: canonical key order/spacing applied, prose/code/attr
    values unchanged, non-runnable fences and frontmatter untouched, parse
    error returns nil+error (so the CLI layer never writes a partial file),
    idempotent on a second run, original fence marker length preserved
  - `FormatDocument` operates on the raw parsed attribute map (which fence
    text is copied from) rather than reconstructing fences from the typed
    `Step` struct — the struct can't distinguish "explicitly `confirm=false`"
    from "omitted", so reconstructing from it would risk inventing or
    dropping attributes; rewriting only key order/spacing in place avoids that
  - _Requirements: 12.1, 12.2, 12.3_

- [x] 16. Integration tests
  - `pkg/runbook/integration_test.go`: fixture runbook driven through
    `serve`'s real HTTP+WS API as a browser client would — run step one,
    read its captured value back over the WebSocket, feed it in as step
    two's declared `input=`, confirm session history has both in order
  - Same file: timeout fixture driven through the same async-goroutine path
    the real API handler uses (not a direct `Engine.Run` call), `ps aux`
    confirms no orphaned `sleep` process survives the kill
  - `internal/cmd/runbook_run_integration_test.go`: drives the actual `run`
    RunE entrypoint end-to-end (file read, parse, `--set`, real
    `FileLogWriter` under `.synacklab`) rather than the narrower
    `executeNonInteractive` unit tests from task 14, which use a nil log
    writer — confirms the on-disk log actually gets written by the real CLI
    command, not just by the engine in isolation
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 6.4, 9.1, 9.2, 11.1–11.5_

- [x] 17. Documentation and example runbook
  - Add a `docs/` (or README) section covering `serve`/`run`/`fmt`, the fence
    attribute table, and an explicit security note on `{{...}}` template
    substitution vs. `input=`/`capture=` (per project-brief.md §13)
  - Add an example runbook `.md` fixture demonstrating input, capture,
    set_cwd, confirm, and danger_patterns
  - `docs/runbook.md` (linked from `docs/README.md`) +
    `examples/runbook-demo.md`
  - **Manually verified against the real binary** while writing these: the
    doc's non-interactive example command turned out to be misleading as
    first written — the fixture's `confirm_before_running` step correctly
    fails the non-interactive run closed (Requirement 11.3), so I rewrote
    both the doc and the fixture's own header to say so accurately instead
    of implying a clean run through all five steps
  - **Bug found and fixed along the way**: `--non-interactive`'s flag help
    text used backtick-quoted `` `synacklab serve` `` in its description;
    pflag treats backticks in a flag's usage string as a value-type
    placeholder for `-h` output, so it was rendering as a garbled type name
    instead of the intended prose. Removed the backticks.
  - _Requirements: 10.4, 13.2_

- [x] 18. Full-suite verification
  - `go build ./...`, `go vet ./...`: clean
  - `go test ./... -race`: all packages pass, including `pkg/runbook` and
    `internal/cmd`
  - `golangci-lint run ./...`: 0 issues in anything this feature touched.
    One pre-existing, unrelated finding remains
    (`internal/auth/errors.go:89`, staticcheck QF1012) — left alone per the
    project's own don't-fix-what-you-didn't-break convention, flagged here
    instead
  - File sizes: largest new file is `executor.go` at 250 lines — no split
    needed, well under the 500-line limit
  - `make build` succeeds; `go mod tidy` leaves `go.mod`/`go.sum` unchanged
    (already tidy)
  - _Requirements: all_

## Post-review follow-ups

Real user feedback from trying the built binary surfaced two CLI-ergonomics
gaps the original spec didn't cover:

- [x] `make lint`/`make test` fixes
  - `make lint` (plain `golangci-lint run`, no path scoping) was failing on
    a pre-existing, unrelated `staticcheck` finding in
    `internal/auth/errors.go:89` that task 18's scoped lint runs didn't
    surface. Fixed it (one-line `fmt.Fprintf` instead of
    `sb.WriteString(fmt.Sprintf(...))`) rather than leaving `make lint` red
    for an unrelated reason.
  - `make test` was already passing; confirmed unaffected.

- [x] `serve`/`run`/`fmt` moved under a `runbook` command group, and their
      path argument made optional with file/directory/cwd-default resolution
  - Top-level `synacklab --help` was cluttered with `serve`/`run`/`fmt`
    sitting alongside `auth`/`github`/etc. Added `internal/cmd/runbook.go`
    (`runbookCmd`, mirroring the existing `githubCmd` group pattern exactly:
    subcommands self-register via their own `init()`, the group registers
    on `rootCmd` in `root.go`) and moved all three under it:
    `synacklab runbook serve|run|fmt`.
  - Every command required an exact file path (`accepts 1 arg(s), received
    0`; `is a directory` when given one) — bad UX for a tool meant to be run
    from a runbook's own directory. Added `internal/cmd/runbook_path.go`
    (`resolveRunbookPath`, TDD'd): the path argument is now optional
    (`cobra.MaximumNArgs(1)`) and may be a file, a directory, or omitted
    entirely (defaults to `.`). A directory resolves to `RUNBOOK.md`, then
    `runbook.md`, then its only `*.md` file if exactly one exists; otherwise
    a clear, actionable error (listing the ambiguous files, or naming the
    expected convention) instead of a crash or a raw filesystem error.
  - `--port`/`--bind 127.0.0.1`/cwd-from-invocation-directory defaults were
    already in place from task 13 — the actual blocker was the mandatory
    exact-file argument, not missing defaults.
  - Manually verified against the real binary: top-level `--help` now shows
    a single `runbook` entry; `runbook fmt <dir-with-RUNBOOK.md>` resolves
    and runs; a directory with two ambiguous `*.md` files and an empty
    directory each produce the intended clear error; `runbook serve` with
    zero arguments, run from a directory containing only `RUNBOOK.md`,
    starts correctly and serves it (confirmed via `curl /api/doc`).
  - Updated `docs/runbook.md` and `examples/runbook-demo.md` to the new
    `synacklab runbook <subcommand>` invocation and the path-resolution
    rules. `requirements.md`/`design.md` still show the original
    `synacklab serve <file.md>`-style examples from the initial spec review
    — left as the historical record of what was originally planned, per
    this file's own account of what actually shipped.

- [x] `serve` gains a directory-mode file-browser sidebar; visual redesign
  - Follow-up to the previous entry: file/dir/cwd-default resolution for
    `serve` still forced an ambiguous directory (no `RUNBOOK.md`, more than
    one `*.md` file) to an error — real feedback was that this should still
    *start*, with a way to pick a file from the browser instead of failing
    at the command line.
  - `pkg/runbook`: added `workspace.go` (`ListMarkdownFiles` — a pruned
    directory tree containing only `*.md` files and the dirs that
    transitively hold them, hidden entries skipped; `GET /api/files`) and
    `POST /api/open` (re-parses a chosen file, builds a fresh
    `SessionStore` for it — switching documents mid-session resets vars/
    cwd/history for the new one). Path-traversal is neutralized by cleaning
    the client-supplied relative path as if rooted at `/` before joining it
    to root (a leading `..` can't escape `/`, so it can't escape root
    either) — TDD'd, plus an end-to-end check against the real binary.
  - `Server.doc`/`.store` became mutable (`activeDoc`, a small
    `sync.RWMutex`-guarded holder) since `/api/open` can now swap them at
    runtime; every handler reads through `s.active.get()` instead of a
    fixed field. Both may be nil (nothing opened yet) — handlers treat that
    as a normal state (`GET /api/doc` returns `{"active":false}`, running a
    step 409s with a message pointing at the file list), not an error path.
  - `EnableWorkspace(root, defaultCwd)` is now unconditional in `serve`
    (`internal/cmd/runbook_serve.go`'s `resolveServeTarget` never errors on
    ambiguity — an unresolved directory just means nothing is preselected,
    the frontend's sidebar picks from there) — `run`/`fmt` keep the old
    hard-fail `resolveRunbookPath` since they have no way to prompt.
  - Frontend rewrite (`pkg/runbook/web/`): added a dark sidebar file tree
    (directories collapsible, only `.md` files clickable/highlighted,
    responsive collapse under 800px) and restyled everything else —
    Inter font, an indigo accent/design-token system, rounded
    (`border-radius: full`) buttons, card-style step blocks — modeled on
    the palette/typography/button system in `~/code/platform-console/
    frontend/src/styles/globals.css` per the user's explicit reference.
  - Manually verified against the real binary: a directory with no
    `RUNBOOK.md` and two ambiguous `.md` files (nested under a subdirectory)
    now starts `serve` successfully with nothing preselected;
    `GET /api/files` correctly excludes a `.txt` file placed alongside them;
    `POST /api/open` switches the active document and `GET /api/doc`
    reflects it; opening a non-`.md` file and a `../../../etc/passwd.md`
    traversal attempt both correctly fail (400 and 404) against the live
    server, not just in unit tests.
  - **Not verified**: actual visual rendering in a browser — no browser
    tooling is available in this environment. Checked instead: every CSS
    class referenced from `app.js`/`index.html` exists in `app.css` (no
    typo'd class producing unstyled markup), and the full HTTP/JSON
    contract end-to-end via `curl`. Genuine in-browser visual confirmation
    (does it actually look good, not just "does it reference real classes")
    is still outstanding and worth doing once you have a chance to open it.

- [x] Console logging (error/warn/info), level controlled from config
  - Feedback: `serve`'s console showed nothing after the startup banner —
    all activity happens via the browser, so there was no way to see what
    the server was doing without opening dev tools. Added leveled logging
    (`error`/`warn`/`info`) and a `log_level` setting in
    `~/.synacklab/config.yaml` (default `info`, showing all three) to
    control it, per the request that this live in synacklab's main config
    rather than a per-command flag.
  - New `pkg/log` package (TDD'd): a minimal `Logger` (`Error`/`Warn`/`Info`,
    timestamped, mutex-guarded for concurrent use) filtering by a configured
    threshold; `log.Discard()` for tests/callers that want silence.
  - `pkg/config.Config` gained `LogLevel string \`yaml:"log_level,omitempty"\``.
  - `pkg/runbook.Server` gained `SetLogger` (default: `log.Discard()`, so
    existing/unrelated tests stay quiet) and now logs: every HTTP request
    (method/path/status/duration, via a new logging middleware in
    `Routes()`), each step's start/finish/duration, non-zero exit codes and
    timeouts at `Warn`, and every error response via a shared
    `writeError` (client failures at `Warn`, 5xx at `Error`) — so error
    logging is automatic at every call site rather than sprinkled by hand.
  - **Bug found and fixed by this change's own tests**: wrapping
    `http.ResponseWriter` in a `statusRecorder` (to capture the status code
    for the request-logging line) broke the WebSocket upgrade — gorilla's
    `Upgrade` needs the `http.Hijacker` interface, which embedding only the
    `http.ResponseWriter` *interface* silently hides. Existing WS tests
    caught this immediately (`websocket: bad handshake`). Fixed by adding a
    `Hijack()` method that forwards to the underlying writer.
  - **Race found and fixed by this change's own tests**: `executionRegistry`
    closed `rec.done` *before* logging that execution's completion, so a
    caller (or a test) waiting on `done` could read the log buffer before
    the write happened — a real ordering bug, not just a flaky test.
    Fixed by logging first, then closing `done`.
  - `internal/cmd`: new `buildLogger(cfg)` shared by `serve` and `run`;
    `run`'s existing `"==> stepname"` boundary print became a leveled
    `logger.Info`, with `Warn`/`Error` added for its failure paths — the
    actual step stdout/stderr passthrough is untouched (always shown,
    regardless of level, since that's real command output, not a log).
  - Manually verified against the real binary: `log_level: info` shows
    per-request and per-step lines exactly as designed; `log_level: error`
    shows only the startup banner (everything else correctly suppressed);
    an invalid value (`log_level: verbose`) fails fast with a clear error
    naming the bad value; `run --non-interactive` shows the same leveled
    step lifecycle around its unfiltered stdout passthrough.
  - **Unrelated but worth recording**: fixing this surfaced that an earlier
    `brew install gh` attempt (from the PR-creation task) had left the
    system's `go` command broken — Homebrew had unlinked the working 1.26.6
    before failing on outdated Command Line Tools, and stray build
    processes kept re-breaking the link for a while after. Resolved by
    killing the stray processes, removing the incomplete partial-upgrade
    keg, and relinking 1.26.6. Flagged to the user directly; not otherwise
    related to this feature.

- [x] Signal handling (Ctrl+C/SIGTERM) for `serve` and `run`
  - Feedback: Ctrl+C stopped working against `serve` — required killing the
    process from another shell. Root cause: nothing installed a signal
    handler, and every executed step spawns in its **own process group**
    (`Setpgid: true` in `executor.go`, needed so a per-step timeout can kill
    the whole group without recursively killing the parent). That isolation
    is exactly what stops a plain, unhandled SIGINT from reaching an
    in-flight step's process when the parent dies — so even where a bare
    `^C` did kill the top-level process, any step running via the browser at
    that moment would be orphaned rather than cleaned up.
  - `pkg/runbook.Server` gained `SetBaseContext(ctx)`: the parent context
    every step execution runs under (default `context.Background()`).
    `handlePostStepRun` now runs steps under it instead of a bare
    `context.Background()`, so canceling it kills any in-flight execution's
    process group via the exact same mechanism a timeout already uses
    (`Engine.Run` derives its per-step timeout context from whatever parent
    it's given — this needed no engine changes at all).
  - `internal/cmd/runbook_serve.go`: `serve` now binds its own
    `net.Listener` up front (so a bind failure surfaces before printing
    "Serving..."), derives a `signal.NotifyContext(... os.Interrupt,
    syscall.SIGTERM)`, passes it to `srv.SetBaseContext`, and drives the
    HTTP server through a new `serveHTTP(ctx, ln, handler, logger)` that
    shuts down gracefully (bounded 5s) on cancellation instead of blocking
    forever in `http.ListenAndServe`.
  - `internal/cmd/runbook_run.go`: `executeNonInteractive` gained a `ctx`
    parameter (first arg) threaded into `engine.Run` in place of
    `context.Background()`; `runRunbookRun` derives the same kind of
    signal-cancelable context and passes it through, so Ctrl+C during
    `run --non-interactive` kills the current step instead of orphaning it.
  - TDD'd at both layers: `pkg/runbook`'s
    `TestHandlePostStepRun_CancelingBaseContextKillsInFlightExecution` and
    `internal/cmd`'s `TestExecuteNonInteractive_ContextCancellationKillsInFlightStep`
    each start a 5s-sleep step, cancel the context after ~100ms, and assert
    both prompt return (not the full 5s) and — via `ps aux` — no orphaned
    `sleep` process; `internal/cmd`'s `TestServeHTTP_*` tests cover the
    graceful-shutdown wrapper directly against a real ephemeral-port
    listener.
  - **Manually verified against the real binary** (not just unit tests):
    sent a real `SIGINT` (Python `subprocess` + `send_signal`, avoiding the
    shell-`&`-backgrounding artifact that made an earlier ad hoc repro
    attempt misleading — backgrounding a job sets SIGINT to ignored per
    POSIX shell semantics, which isn't the user's actual foreground-Ctrl+C
    scenario) to a running `serve` process — three repeated runs all exited
    0 with "shutting down..." logged; sent it again while a step was
    actively mid-`sleep 5` — exited 0 immediately with no orphaned `sleep`
    process; same mid-step signal against `run --non-interactive` — exited
    non-zero immediately (not after the full 5s), again with no orphan.
