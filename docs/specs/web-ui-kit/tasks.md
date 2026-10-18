# Implementation Plan

- [x] 1. `pkg/webui` serve plumbing (TDD)
  - Tests first: `ValidateBind` table (127.0.0.1, 127.0.0.2, ::1, localhost,
    0.0.0.0, LAN IP, hostname); guard table (allowed host, rebinding host,
    foreign Origin, foreign Origin on WS upgrade, no Origin); `Serve` returns
    after ctx cancel; Warn log on rejection
  - Move `serveHTTP` → `webui.Serve`, request logging + `statusRecorder` →
    `guard.go`; `runbook.Server.Routes()` stops wrapping logging
  - `runbook serve` calls `webui.ValidateBind` before listening (error, not
    warning) and `webui.Serve`
  - Verify: `go test ./pkg/webui ./pkg/runbook ./internal/cmd`;
    `runbook serve --bind 0.0.0.0` fails; curl with `Host: evil.example` → 403
  - _Requirements: 4.1, 4.2, 4.3, 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 2. Embedded kit assets
  - Move `:root` tokens from runbook `app.css` to `pkg/webui/assets/tokens.css`;
    move generic component CSS (btn, badge, field, empty-state, shell, output)
    to `components.css`
  - Write `components.js` factories (Requirement 2.3)
  - `webui.Assets()`; runbook `Routes()` mounts it at `/ui/`
  - Test: `/ui/tokens.css`, `/ui/components.css`, `/ui/components.js` served
    with correct content types
  - Verify: `go test ./pkg/webui ./pkg/runbook`
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3_

- [ ] 3. Migrate runbook UI onto the kit
  - `index.html` links `/ui/` CSS and loads `app.js` as a module;
    `session.js` exports what `app.js` uses
  - Replace `createConfirmGate`, `escapeHTML`/`innerHTML` building, and
    hand-built buttons/fields/badges with factories; delete the duplicated CSS
  - Extend `TestRoutes_FrontendReferencesRealEndpoints` to the `/ui/` assets
  - Verify: `go test ./...`; Puppeteer pass over Requirement 3.1 features
    (open file, run step, stop, confirm gate, sensitive reveal, vars, last run
    + log, reset) with screenshots before/after showing no visual change
  - _Requirements: 3.1, 3.2, 3.3, 2.4_

- [ ] 4. Docs and gates
  - `docs/runbook.md` `--bind` loopback-only; supersede note on
    runbook-server-bootstrap 13.2; `CLAUDE.md` architecture entry for
    `pkg/webui`
  - Verify: `go vet ./...`, `go test -race ./...`, `golangci-lint run`,
    `make build`
  - _Requirements: 5.1_
