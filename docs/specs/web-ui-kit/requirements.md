# Requirements Document

## Introduction

`synacklab runbook serve` is the first local web UI in the CLI, and more `serve`
commands are coming (`tunnel serve` is next). Today the runbook UI owns
everything itself: design tokens, generic components (buttons, badges, form
fields, confirm-to-run gate, output pane), and the HTTP plumbing (graceful
shutdown, request logging, bind checks). A second UI would have to copy all of it.

This spec extracts those shared parts into one embedded kit, `pkg/webui`, and
moves `runbook serve` onto it without visible change. It also makes every
`serve` command local-only: a serve UI is a UX convenience for the operator on
their own machine, never a network service.

Components are plain factory functions in an ES module (no Custom Element
classes, no framework, no npm/build step), styled by shared CSS. Everything stays
embedded in the single binary.

Out of scope: authentication, theming beyond the existing light tokens, a
JS bundler or TypeScript, a dev mode that serves assets from disk.

## Requirements

### Requirement 1 — Shared design tokens and component styles

**User Story:** As the maintainer, I want one place that defines how serve UIs
look, so that restyling a component changes it in every UI at once.

#### Acceptance Criteria

1. WHEN any serve UI loads THEN it SHALL take its colors, fonts, radii, shadows, and spacing from `pkg/webui`'s `tokens.css`, and no app stylesheet SHALL redefine those tokens.
2. WHEN a generic component (button, badge, field, select, card, log view, empty state, app shell: sidebar/topbar/main) is styled THEN its CSS SHALL live only in `pkg/webui`'s `components.css`; app stylesheets SHALL hold only app-specific rules.
3. WHEN the binary is built THEN the kit's assets SHALL be embedded (`go:embed`) and served at `/ui/` by every serve server, with no runtime file or network dependency other than the existing Google Fonts import.

### Requirement 2 — Component factories

**User Story:** As the maintainer, I want to build UI from small reusable
functions, so that new serve pages are assembled, not re-implemented.

#### Acceptance Criteria

1. WHEN app code needs a generic component THEN it SHALL import a factory from `/ui/components.js` (an ES module) that returns a DOM element, or `{el, …methods}` when the component has behavior.
2. WHEN a factory receives text (labels, values, log lines) THEN it SHALL set it as text content, never as HTML, so callers cannot inject markup by mistake.
3. The kit SHALL provide at least: `h` (element builder), `button`, `confirmButton` (arm-then-confirm, with `reset()`), `field` (label + input, optional reveal toggle for secrets), `select`, `badge`, `card`, `logView` (`append`, `clear`), and `emptyState`.
4. WHEN a factory's styling needs to change THEN the change SHALL be confined to `pkg/webui` (the factory and `components.css`).

### Requirement 3 — Runbook UI migrated with no behavior change

**User Story:** As a runbook user, I want the UI to keep working exactly as
before, so that the refactor costs me nothing.

#### Acceptance Criteria

1. WHEN `runbook serve` runs after the migration THEN every existing runbook feature (file browser, step run/stop, confirm gate, inputs and sensitive reveal, vars panel, last-run and log view, reset session) SHALL behave as before.
2. WHEN the runbook UI uses a generic component THEN it SHALL use the kit's factory or class instead of its own copy; `createConfirmGate`, `escapeHTML`, and duplicated generic CSS SHALL be removed from `pkg/runbook/web`.
3. WHEN the migration lands THEN the existing `pkg/runbook` and `internal/cmd` serve tests SHALL pass unchanged except for tests that move with code into `pkg/webui`.

### Requirement 4 — Shared serve plumbing

**User Story:** As the maintainer, I want every serve command to start, log, and
stop the same way, so that new serve commands are thin.

#### Acceptance Criteria

1. WHEN a serve command runs THEN it SHALL use `webui.Serve(ctx, listener, handler, logger)`, which serves until ctx is cancelled and then shuts down gracefully within 5 seconds.
2. WHEN a request is served THEN `webui.Serve` SHALL log its method, path, status, and duration at Info, including WebSocket upgrades.
3. WHEN a serve command starts THEN it SHALL print the URL it serves at.

### Requirement 5 — Local-only access

**User Story:** As an operator, I want serve UIs reachable only from my own
machine and only by pages they serve, so that a website I visit cannot drive my
runbooks or cluster credentials.

#### Acceptance Criteria

1. IF `--bind` is not a loopback address (`127.0.0.0/8`, `::1`, or `localhost`) THEN the serve command SHALL fail before listening, with an error saying serve UIs are local-only. This replaces runbook-server-bootstrap Requirement 13.2 (warn and continue).
2. IF a request's `Host` header is not the listener's own loopback address or `localhost` with the listener's port THEN the system SHALL reject it with 403 (blocks DNS rebinding).
3. IF a request carries an `Origin` header that is not `http://` plus an allowed host from AC 2 THEN the system SHALL reject it with 403, including WebSocket upgrades (blocks cross-site requests from other pages).
4. WHEN a request has no `Origin` header (same-origin GET, curl) and passes AC 2 THEN the system SHALL serve it.
5. WHEN AC 1–3 reject something THEN the rejection SHALL be logged at Warn with the offending value.
