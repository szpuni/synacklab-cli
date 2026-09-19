# Requirements Document

## Introduction

This is a hardening/follow-up pass on the already-shipped `synacklab runbook
serve` feature (see `docs/specs/runbook-server-bootstrap/`). A design review
of `pkg/runbook/{server,api,ws,session,workspace,executor,danger,template,
types}.go` and the embedded frontend (`pkg/runbook/web/{index.html,app.js,
app.css}`) surfaced eight findings: a real safety-defeating bug in the
confirmation gate, several UX/observability gaps that undercut the tool's
own "chain step output, show me what's going on" value proposition, a
missing execution-serialization guarantee, and accessibility (contrast)
defects. This document turns those findings into requirements for a
follow-up implementation pass. It does not introduce new architecture and
does not revisit any of the original spec's scope cuts (v1 remains
single-document-at-a-time, local-only, in-memory session, bash/python only).
No project-brief.md accompanies this spec — the findings below serve as the
brief. Source: internal design review, 2026-09-19.

## Requirements

### Requirement 1 — Confirmation gate resets after every destructive action

**User Story:** As a DevOps engineer, I want a step's (or the session reset's) confirmation arming to expire the moment it's used, so that a single stale click years — or even seconds — later can't fire a destructive action without a fresh, explicit confirmation.

#### Acceptance Criteria

1. WHEN a Step requiring confirmation (`confirm=true` or a `danger_patterns` match) is armed by a first Run click THEN the frontend SHALL relabel its Run control to indicate an armed state (e.g. "Confirm & Run") and SHALL NOT execute until a second, distinct click.
2. WHEN an armed Step's execution completes (success, non-zero exit, or timeout) THEN the frontend SHALL reset that Step's confirmation state back to unarmed and restore the Run control's original label, so a subsequent click re-arms rather than firing immediately.
3. WHEN an armed Step's run request fails before execution starts (e.g. a network error or a 4xx response) THEN the frontend SHALL also reset the confirmation state back to unarmed rather than leaving it armed indefinitely.
4. WHEN the user clicks "Reset session" THEN the frontend SHALL require the same arm-then-confirm interaction used for a `confirm=true` step (first click arms and relabels the control, e.g. to "Confirm reset"; a second click performs the reset) before calling `POST /api/session/reset`.
5. WHEN the reset control's confirmation is armed and the user clicks elsewhere without confirming, then returns later and clicks Reset again THEN the system MAY treat this as a fresh arm-click (no time-based expiry is required) but SHALL NOT have executed the reset from the first click alone.
6. WHEN implementing Acceptance Criteria 1–5 THEN the system SHALL use one shared confirmation-gate mechanism for both per-step Run controls and the reset control, rather than two independent implementations.

### Requirement 2 — Session state (vars, history, logs) is visible in the UI

**User Story:** As a DevOps engineer, I want to see captured variables and each step's last run result in the browser, so that I can tell what has already run and what values are available to later steps, including after a page reload.

#### Acceptance Criteria

1. WHEN the document view renders (initial load, after `POST /api/open`, or after any execution completes) THEN the frontend SHALL display the current session's captured variables (`session.vars`) as a visible list, not merely fetch and discard them.
2. WHEN a Step has at least one prior execution in `session.history` THEN the frontend SHALL render that Step's most recent run result (exit code, duration, and either a timestamp or relative time) alongside its Run control.
3. WHEN a Step's most recent execution wrote a log file (`log_path` in the history entry) THEN the frontend SHALL offer a way to view that log's contents from the UI, without requiring the user to leave the browser and open a terminal.
4. WHEN the page is reloaded or a different file is opened via `POST /api/open` and the frontend then re-renders THEN the vars display and per-step last-run info SHALL be rebuilt from the server's `GET /api/doc` / `GET /api/session` responses, not solely from in-memory state accumulated during the current page's lifetime.
5. WHEN `execHistoryView` (or an equivalent view type) is serialized for the frontend THEN the system SHALL include the execution's start time so the UI can display "when," not just "how long."

### Requirement 3 — Reopening or switching documents does not silently discard session state

**User Story:** As a DevOps engineer, I want a sidebar click that reopens the file I'm already on to be a no-op, and a click on a different file to warn me first if I have unsaved session state, so that a misclick can't wipe captured variables, cwd, and history without warning.

#### Acceptance Criteria

1. WHEN `POST /api/open` is called with a file that resolves to the currently active document's path THEN the system SHALL leave the existing session (vars, cwd, history) untouched and SHALL NOT construct a new `SessionStore`.
2. WHEN `POST /api/open` is called with a file that resolves to a different document than the currently active one, AND the active session has a non-empty `Vars` map or non-empty `History` THEN the system SHALL refuse the switch by default and report that confirmation is required, rather than discarding the session silently.
3. WHEN the frontend receives the "confirmation required" response from Acceptance Criteria 2 THEN it SHALL prompt the user to confirm before retrying the open request with an explicit override flag.
4. WHEN `POST /api/open` is called with the override flag set (from Acceptance Criteria 3), or the active session has no vars and no history THEN the system SHALL proceed to open the new file and replace the session as it does today.
5. WHEN a session is replaced per Acceptance Criteria 4 THEN previously written log files under `.synacklab/` for the discarded session SHALL remain untouched on disk (only in-memory state is discarded).

### Requirement 4 — Output panel scroll containment and step cancellation

**User Story:** As a DevOps engineer, I want a verbose step's output to scroll within a fixed-height panel instead of pushing the Run button off-screen, and a way to stop a step that's hung, so that one runaway or stuck command doesn't degrade the whole page or force me to kill the server process.

#### Acceptance Criteria

1. WHEN a Step's output panel accumulates more content than fits in a bounded height THEN the panel SHALL scroll internally (`overflow-y: auto` with a `max-height`) rather than growing the step card without bound.
2. WHEN new stdout/stderr lines are appended to an output panel that is already scrolled to its bottom THEN the frontend SHALL auto-scroll to keep the newest line visible ("tail pinned").
3. WHEN a Step's execution is in progress THEN the frontend SHALL show a control to stop it.
4. WHEN the user activates the stop control for a running execution THEN the system SHALL cancel that specific execution's process group server-side (not merely hide the UI state) and SHALL NOT wait for the step's configured timeout to elapse.
5. WHEN an execution is stopped via Acceptance Criteria 4 THEN the terminal event delivered to the frontend SHALL distinguish "canceled by user" from "timed out" and from a normal exit, and the execution log (Requirement 4 of the original spec) SHALL still be written with whatever partial output was produced.
6. WHEN an execution has already finished (naturally or via timeout) THEN a stop request for its execution id SHALL be a no-op that does not error the client, since there is nothing left to cancel.

### Requirement 5 — Masked input for sensitive values

**User Story:** As a DevOps engineer, I want to mark an `input=` field as sensitive, so that tokens, passwords, and connection strings I type into the browser aren't shown in plaintext or offered up by browser autofill.

#### Acceptance Criteria

1. WHEN a Step's fence attributes declare `sensitive=<names>` (comma-separated, matching the parsing convention used for `input=`/`capture=`) THEN the system SHALL parse it into the Step's model as the set of input variable names to render masked.
2. IF a name listed in `sensitive=` is not also listed in that Step's `input=` THEN the system SHALL fail parsing with an error identifying the step and the offending name.
3. WHEN the frontend renders an input field whose name is in the Step's sensitive set THEN it SHALL render that field with `type="password"` and `autocomplete="off"` instead of `type="text"`.
4. WHEN a sensitive input field is rendered THEN the frontend SHALL provide an explicit, opt-in control to reveal its value (toggle to plain text), defaulting to masked.
5. WHEN a captured variable's name matches a name ever declared sensitive by the step that captured it THEN the session-vars display added by Requirement 2 SHALL mask that variable's value by default, with the same reveal-toggle pattern as Acceptance Criteria 4.
6. WHEN `synacklab runbook fmt` canonicalizes a fence's attribute order THEN it SHALL include `sensitive` in the canonical key ordering, consistent with how the other fence attributes are already handled.

### Requirement 6 — Execution serialization

**User Story:** As a DevOps engineer, I want the server to enforce that only one step executes at a time, so that two steps can't race to capture the same variable name or change the working directory concurrently, consistent with the tool's own "one step at a time, in order" framing.

#### Acceptance Criteria

1. WHEN a Step execution is already in flight for the active session THEN a `POST /api/steps/{name}/run` request for a different step (or the same step) SHALL be rejected with a `409 Conflict` identifying the currently-running execution, rather than being queued or started concurrently.
2. WHEN the in-flight execution referenced in Acceptance Criteria 1 completes (normally, by timeout, or by cancellation per Requirement 4) THEN the server SHALL immediately accept a new run request for any step.
3. WHEN the frontend has an execution in progress THEN it SHALL disable every other Step's Run control (and the reset control) in addition to the currently-running Step's own control, so the UI reflects the server-enforced constraint before a rejected request round-trips.
4. WHEN switching the active document via `POST /api/open` while an execution is in flight THEN the system SHALL refuse the switch (in addition to Requirement 3's vars/history check) until that execution completes or is canceled, since the in-flight execution belongs to the session being replaced.

### Requirement 7 — Sidebar and topbar text meets contrast guidelines

**User Story:** As a DevOps engineer, I want the cwd label and sidebar file-tree text to be legible, so that the one label telling me where a command is about to execute isn't the hardest thing on the page to read.

#### Acceptance Criteria

1. WHEN the `.cwd` label is rendered against the topbar's background (`--bg-surface`) THEN its text color SHALL achieve at least a 4.5:1 contrast ratio against that background (WCAG 2.1 AA for normal-size text).
2. WHEN `.tree-dir > .tree-label` text is rendered against the sidebar background (`--sidebar-bg`) THEN its text color SHALL achieve at least a 4.5:1 contrast ratio against that background.
3. WHEN `.tree-empty` text is rendered against the sidebar background THEN its text color SHALL achieve at least a 4.5:1 contrast ratio against that background.
4. WHEN any new or changed color token is introduced to satisfy Acceptance Criteria 1–3 THEN its contrast ratio against the specific background it is used on SHALL be computed (not estimated) and recorded in the design documentation for this change.
5. WHEN the token changes from Acceptance Criteria 1–3 are applied THEN no other element's existing (already-passing) contrast ratio SHALL regress below 4.5:1 as a side effect of a shared token being reused elsewhere.
