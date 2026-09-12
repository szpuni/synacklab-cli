# Requirements Document

## Introduction

The Runbook Server extends `synacklab` with a `serve` command that turns a Markdown
file into an interactive, human-in-the-loop runbook. It starts a local Go web
server that parses the document's fenced `bash`/`python` code blocks into
individually runnable "steps," serves a small embedded single-page frontend,
and executes steps one at a time strictly on explicit user action — carrying
output between steps via session state (env-var capture) or opt-in text
templating. It is explicitly not an automation/agent runner: nothing executes
without a click, and no process outlives its own execution. A companion
`run --non-interactive` command reuses the same parser and execution engine
for CI, and `fmt` canonicalizes fence attribute formatting. Source: `docs/specs/runbook-server-bootstrap/project-brief.md`.

## Requirements

### Requirement 1 — Markdown parsing and fence attributes

**User Story:** As a DevOps engineer, I want to write a runbook as plain Markdown with fenced `bash`/`python` blocks, so that any existing or LLM-authored Markdown file becomes executable with zero required changes.

#### Acceptance Criteria

1. WHEN a fenced code block's info string language is `bash` or `python` THEN the system SHALL parse it as a runnable Step.
2. WHEN a fenced code block's language is anything else THEN the system SHALL render it as static, non-runnable code.
3. WHEN a fence's info string contains curly-brace attributes (`{key=value ...}`) THEN the system SHALL parse `name`, `input`, `capture`, `confirm`, `cwd`, `set_cwd`, and `timeout` per the format in project-brief.md §5.1.
4. IF a Step has no `name` attribute THEN the system SHALL assign it an auto-generated unique name (`step-1`, `step-2`, …).
5. IF two Steps in the same document declare the same explicit `name` THEN the system SHALL fail parsing with an error identifying both locations.
6. WHEN the document contains a YAML frontmatter block THEN the system SHALL parse `default_timeout` and `danger_patterns` per project-brief.md §5.2.
7. IF frontmatter is absent or a field is omitted THEN the system SHALL fall back to documented defaults (120s timeout, no danger patterns).

### Requirement 2 — Frontend document rendering

**User Story:** As a DevOps engineer, I want to see my runbook's prose and steps rendered in a browser in document order, so that I can read and act on it like a normal document.

#### Acceptance Criteria

1. WHEN `synacklab serve <file.md>` starts THEN the system SHALL serve a single-page frontend, embedded in the binary via `go:embed`, requiring no separate frontend build step.
2. WHEN the frontend loads THEN the system SHALL call `GET /api/doc` and render prose blocks and Steps in the document's original order.
3. WHEN a Step is runnable (`bash`/`python`) THEN the system SHALL render a Run control for it.
4. WHEN a Step declares `input=` variables THEN the system SHALL render one form field per variable and SHALL NOT enable Run until all declared inputs are filled.
5. WHEN session vars or cwd change after an execution THEN the frontend SHALL reflect the updated state without a full page reload.

### Requirement 3 — On-demand execution

**User Story:** As a DevOps engineer, I want each step to run only when I click it, in its own process, so that nothing executes implicitly and no shell state silently persists between steps.

#### Acceptance Criteria

1. WHEN a user clicks Run on a Step THEN the system SHALL send `POST /api/steps/{name}/run` and SHALL NOT execute anything before this explicit call.
2. WHEN executing a Step THEN the system SHALL spawn exactly one process for that execution (`bash -c` or `python3 -c`) and SHALL NOT keep any process alive after it exits.
3. WHEN the process is spawned THEN the system SHALL set `Dir` from the session cwd (or the Step's `cwd=` override, resolved relative to session cwd) and `Env` from the merged session-vars + declared `input=` map.
4. WHEN a Step is currently executing THEN the system SHALL stream stdout and stderr to the frontend live over `GET /ws/executions/{execution_id}` as `{"type":"stdout"|"stderr","data":...}` events.
5. WHEN the process exits THEN the system SHALL send a terminal `{"type":"done","exit_code":...,"duration_ms":...,"captured":{...}}` event.

### Requirement 4 — Execution logging

**User Story:** As a DevOps engineer, I want every execution logged to disk regardless of whether I captured its output, so that I have a complete audit trail of what actually ran.

#### Acceptance Criteria

1. WHEN any Step execution completes (success, failure, or timeout) THEN the system SHALL write a log file under `.synacklab/<doc-slug>/<session-id>/steps/` containing stdout, stderr, exit code, and timing.
2. WHEN a Step has no `capture=` attribute THEN the system SHALL still write its full execution log.
3. WHEN a log file is written THEN the system SHALL name it to preserve execution order (e.g. `<n>-<step-name>.log`).

### Requirement 5 — Input variables

**User Story:** As a DevOps engineer, I want to supply per-run values to a step from the UI, so that a step can be parameterized without editing the Markdown.

#### Acceptance Criteria

1. WHEN a Step declares `input=A,B` THEN the system SHALL require the run request to supply values for `A` and `B` in `inputs`.
2. WHEN a run request is missing a declared `input=` value THEN the system SHALL reject the request with a validation error and SHALL NOT spawn a process.
3. WHEN a run request supplies input values THEN the system SHALL inject them into the spawned process's environment, merged with existing session vars (declared inputs take precedence for that execution).

### Requirement 6 — Output capture between steps

**User Story:** As a DevOps engineer, I want a step's output to become available to a later step, so that I can chain steps (e.g. fetch an IP, then use it) without manual copy/paste.

#### Acceptance Criteria

1. WHEN a Step declares `capture=A,B` THEN the system SHALL append a capture trailer to the script (per project-brief.md §7.4) that emits `__SYNACLAB_CAP__<name>=<value>` lines for each declared variable.
2. WHEN parsing execution output THEN the system SHALL strip `__SYNACLAB_CAP__` lines from stdout before display/logging and SHALL store the values in session state, keyed by `<step-name>.<var-name>` and, if globally unique, by `<var-name>` alone.
3. IF a declared `capture=` variable was never exported/assigned by the script THEN the system SHALL store it as an empty string and SHALL NOT fail the execution solely for that reason.
4. WHEN session vars are captured THEN the system SHALL make them available as `input=` sources and as `{{vars.<name>}}`/`{{steps.<name>.stdout}}` template targets for subsequent executions.

### Requirement 7 — Working directory management

**User Story:** As a DevOps engineer, I want an explicit way to change the runbook's working directory going forward, so that directory state is predictable across independently-spawned processes.

#### Acceptance Criteria

1. WHEN a session starts THEN the system SHALL default session cwd to the document's containing directory (or `--cwd` if given to `serve`).
2. WHEN a Step declares `cwd=` THEN the system SHALL use that path (resolved relative to session cwd) as `Dir` for that execution only, and SHALL NOT persist it to session state.
3. IF a Step's script changes directory internally (e.g. `cd`) and does not declare `set_cwd=true` THEN the system SHALL NOT reflect that change in session cwd for later steps.
4. WHEN a Step declares `set_cwd=true` THEN the system SHALL append a cwd-capture trailer, parse the emitted `__SYNACLAB_CWD__` line, and update session cwd for subsequent executions.

### Requirement 8 — Confirmation and danger patterns

**User Story:** As a DevOps engineer, I want destructive-looking commands to require explicit confirmation, so that I don't accidentally run something dangerous while stepping through an AI-generated runbook.

#### Acceptance Criteria

1. WHEN a Step declares `confirm=true` THEN the system SHALL reject a run request lacking `confirmed: true` in the body.
2. WHEN document frontmatter declares `danger_patterns` and a Step's source matches any pattern THEN the system SHALL require confirmation for that Step regardless of its own `confirm=` value.
3. WHEN a Step matches a danger pattern THEN the frontend SHALL surface which pattern matched before the user confirms.
4. WHEN `confirm` is unset and no danger pattern matches THEN the system SHALL allow the run to proceed without a confirmation step (default `false`).

### Requirement 9 — Timeouts

**User Story:** As a DevOps engineer, I want long-running or hung steps to be killed automatically, so that a stuck command can't block the runbook indefinitely.

#### Acceptance Criteria

1. WHEN a Step executes THEN the system SHALL run it under a `context.WithTimeout` derived from (in precedence order) the Step's `timeout=`, the document's `default_timeout`, or the server default of 120s.
2. WHEN the timeout elapses before the process exits THEN the system SHALL kill the process group and mark the execution `timed_out`.
3. WHEN an execution times out THEN the system SHALL still write its partial output to the execution log per Requirement 4.

### Requirement 10 — Template substitution

**User Story:** As a DevOps engineer, I want an opt-in way to inline a prior step's exact output into a later step's source, so that I have a more direct option than env-var capture when I explicitly choose to use it.

#### Acceptance Criteria

1. WHEN a Step's source contains `{{steps.<name>.stdout}}`, `{{steps.<name>.exit_code}}`, or `{{vars.<name>}}` THEN the system SHALL substitute the literal value as plain text before writing the script to disk/spawning.
2. WHEN template substitution is performed THEN the system SHALL NOT apply any shell-escaping to the substituted value.
3. WHEN a referenced step name or var name does not exist in session state at substitution time THEN the system SHALL fail the run request with a clear error identifying the missing reference, before spawning any process.
4. WHEN documentation for this feature is written THEN it SHALL state plainly that template substitution is a command-injection risk for untrusted/unpredictable output and that `input=`/`capture=` is the recommended default.

### Requirement 11 — Non-interactive CI execution

**User Story:** As a DevOps engineer, I want to run a runbook top-to-bottom without a browser, so that I can validate it in CI using the same parsing and execution logic as interactive mode.

#### Acceptance Criteria

1. WHEN `synacklab run <file.md> --non-interactive` is invoked THEN the system SHALL parse the document with the same parser used by `serve` and execute every runnable Step in document order.
2. WHEN a Step declares `input=` and no matching `--set VAR=value` was provided THEN the system SHALL fail with a clear error before execution begins (no interactive prompt).
3. WHEN a Step declares `confirm=true` or matches a `danger_patterns` entry THEN non-interactive mode SHALL fail closed (refuse to run that step) unless a future flag explicitly opts in — v1 has no such flag, so these steps SHALL cause a non-zero exit with an explanatory message.
4. WHEN any Step exits non-zero THEN the system SHALL stop execution of subsequent steps and exit non-zero, unless a future `--continue-on-error` flag is added (out of scope for v1).
5. WHEN `run --non-interactive` completes THEN the system SHALL write the same per-step logs as `serve` (Requirement 4).

### Requirement 12 — Fence attribute formatting

**User Story:** As a DevOps engineer, I want a formatter for runbook fence attributes, so that hand-written or LLM-generated attribute strings are normalized without changing behavior.

#### Acceptance Criteria

1. WHEN `synacklab fmt <file.md>` is invoked THEN the system SHALL rewrite each runnable fence's attribute string into a canonical key order and spacing.
2. WHEN formatting a document THEN the system SHALL NOT alter prose, code content, or attribute values/semantics.
3. WHEN formatting a document with a parse error THEN the system SHALL fail without modifying the file.

### Requirement 13 — Network binding and safety

**User Story:** As a DevOps engineer, I want the server to be safe by default, so that I don't accidentally expose arbitrary local command execution to my network.

#### Acceptance Criteria

1. WHEN `synacklab serve` starts without `--bind` THEN the system SHALL bind to `127.0.0.1` only.
2. WHEN `--bind 0.0.0.0` (or any non-loopback address) is passed THEN the system SHALL print an explicit warning that anyone reaching the port can execute arbitrary commands as the local user, before starting.
3. WHEN the server starts THEN the system SHALL implement no authentication in v1, consistent with the documented non-goal.

### Requirement 14 — Session state model

**User Story:** As a DevOps engineer, I want one in-memory session per running server, so that state is simple, inspectable, and never silently persists across restarts.

#### Acceptance Criteria

1. WHEN `serve` starts THEN the system SHALL create exactly one Session for the running process, holding vars (flat, last-write-wins), cwd, and execution history.
2. WHEN `serve` is restarted THEN the system SHALL start with a fresh Session (no cross-restart persistence in v1).
3. WHEN `POST /api/session/reset` is called THEN the system SHALL clear vars and reset cwd to the document default, without affecting already-written log files.
4. WHEN `GET /api/session` is called THEN the system SHALL return current vars, cwd, and an execution history summary.
