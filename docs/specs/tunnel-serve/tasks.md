# Implementation Plan

Depends on `docs/specs/web-ui-kit/` being merged.

- [ ] 1. `pkg/tunnel` hooks (TDD)
  - Test: `Options.Ready` is called once with the pod name when the forward is
    ready, and not at all when it never becomes ready
  - Test: `Contexts()` lists contexts and the current one from a temp
    `KUBECONFIG`
  - Implement `Options.Ready` and `Contexts()`
  - Verify: `go test ./pkg/tunnel`
  - _Requirements: 3.2, 2.3_

- [ ] 2. Tunnel server API with a fake `Start` (TDD)
  - Tests first in `pkg/tunnel/server_test.go`: config endpoint; POST →
    starting → connected; 400 on resolve error; 409 while active; DELETE →
    closed; failed with error text; WS replay + stream; `Wait()` blocks until
    the run returns after base ctx cancel
  - Implement `pkg/tunnel/server.go` (single tunnel, `maxActive = 1`)
  - Verify: `go test -race ./pkg/tunnel`
  - _Requirements: 3.1, 3.3, 3.4, 3.5, 3.6, 4.1, 4.3, 5.2_

- [ ] 3. `tunnel serve` command
  - Refactor `tunnelOptions` to take namespace; build `Start` from
    `resolveDBTarget` + `tunnelOptions` + `NewClient` + `Open`
  - Tests: CLI-identical errors through `Start`; invalid `tunnel.databases`
    still starts with `databaseError`; `--bind 0.0.0.0` fails
  - Verify: `go test ./internal/cmd`; `synacklab tunnel serve` prints URL;
    `curl /api/config`
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.4, 5.1_

- [ ] 4. Tunnel page
  - `pkg/tunnel/web/` using kit factories: form (configured/ad hoc, local
    port, context, namespace), tunnel card (state badge, details, copy hint,
    log view, Close)
  - Test: `TestRoutes_FrontendReferencesRealEndpoints` equivalent
  - Verify: Puppeteer on kind: open configured db, open ad hoc host, see
    connected + hint, curl through the port, reload page keeps state, Close
    deletes pod, second open while active is blocked
  - _Requirements: 2.1, 2.2, 2.3, 3.2, 3.5, 3.6, 4.2_

- [ ] 5. End-to-end, docs, gates
  - kind: Ctrl-C on server with a tunnel open → no pod left; RBAC-denied user
    sees failed + hint
  - `docs/tunnel.md` section for `tunnel serve`; `CLAUDE.md` architecture
  - Verify: `go vet ./...`, `go test -race ./...`, `golangci-lint run`,
    `make build`
  - _Requirements: 3.4, 4.3_
