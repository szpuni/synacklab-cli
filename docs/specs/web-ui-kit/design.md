# Design Document

## Overview

A new package `pkg/webui` holds what every serve UI shares: embedded front-end
assets (tokens, component CSS, component factories) and the Go serve plumbing.
Apps keep their own `web/` folder for app-specific markup, CSS, and JS, and mount
the kit under `/ui/`.

```
pkg/webui/
  webui.go       Assets() http.Handler, Serve, ValidateBind
  guard.go       localOnly middleware (Host + Origin checks) and request logging
  assets/
    tokens.css       :root design tokens (moved from runbook app.css)
    components.css   btn, badge, field, select, card, log, empty-state, shell
    components.js    ES module of factories

pkg/runbook/web/     runbook-only: file tree, steps, vars panel
  index.html         <link href="/ui/tokens.css">, <link href="/ui/components.css">,
                     <script type="module" src="app.js">
  app.js, session.js ES modules importing from "/ui/components.js"
  app.css            runbook-specific rules only (steps.css folded in)
```

## Front end

### Factories

Plain functions, no classes. Each returns an element, or `{el, …}` when it has
behavior. Text goes in through `textContent` only.

```js
// components.js
export function h(tag, props = {}, ...children) { … }   // props: className, attrs, on: {click: fn}
export function button({ label, variant = "ghost", size = "sm", onClick }) { … }
export function confirmButton({ label, armLabel, variant, onConfirm }) { … } // → {el, reset}
export function field({ label, control, secret = false }) { … }             // reveal toggle when secret
export function textInput(props) { … }
export function revealToggle(onToggle) { … }
export function select({ name, options, value }) { … }                      // options: [{value, label}]
export function badge(text, tone = "neutral") { … }                         // neutral|success|warning|danger
export function card({ title, badges = [], body = [], actions = [] }) { … }
export function logView() { … }                                             // → {el, append(text), clear()}
export function emptyState({ icon, text }) { … }
```

`confirmButton` replaces runbook's `createConfirmGate`. `h` replaces
`escapeHTML` + `innerHTML` building. Variants and tones map to existing class
names (`btn-primary`, `badge-success`, …) so `components.css` stays a flat,
greppable stylesheet.

### Why not Custom Elements

Factories need no registration, have no shadow-DOM styling boundary (tokens
apply naturally), and are trivially testable by calling them. Custom Elements
can be added later behind the same names if encapsulation is ever needed.

### Script loading

Runbook's `session.js` and `app.js` currently share globals through classic
`<script>` tags. Both become ES modules: `session.js` exports what `app.js`
uses. No bundler: browsers load `/ui/components.js` directly.

## Back end

```go
// Assets serves the embedded kit; apps mount it at /ui/.
func Assets() http.Handler
//   mux.Handle("GET /ui/", http.StripPrefix("/ui", webui.Assets()))

// ValidateBind fails unless bind is loopback (127.0.0.0/8, ::1, localhost).
func ValidateBind(bind string) error

// Serve wraps h with the local-only guard (allowed hosts derived from
// ln.Addr()) and request logging, serves until ctx is done, then shuts down
// within 5s. Moved from internal/cmd/runbook_serve.go serveHTTP.
func Serve(ctx context.Context, ln net.Listener, h http.Handler, logger *rblog.Logger) error
```

Moves:

| From | To |
|---|---|
| `internal/cmd/runbook_serve.go` `serveHTTP` | `webui.Serve` |
| `internal/cmd/runbook_serve.go` `isLoopbackBind` + warning | `webui.ValidateBind` (now an error) |
| `pkg/runbook/server.go` `withRequestLogging`, `statusRecorder` | `pkg/webui/guard.go` |

`runbook.Server.Routes()` no longer wraps logging itself; `webui.Serve` does,
so every serve command logs the same way. The runbook WebSocket upgrader's
permissive `CheckOrigin` stays: the guard has already rejected foreign origins
before the upgrade handler runs.

### Guard rules

Allowed hosts for a listener on port P: `127.0.0.1:P`, `localhost:P`, `[::1]:P`
(and the actual listener IP if it is another 127.x address).

```
Host not allowed                       → 403  (DNS rebinding)
Origin present and != "http://"+allowed → 403  (cross-site fetch/WebSocket)
otherwise                               → next handler
```

A missing `Origin` is allowed: browsers send it on every cross-origin request
and every WebSocket handshake, so its absence means same-origin or non-browser.

## Testing

- `pkg/webui`: `ValidateBind` table; guard table via `httptest` (good host,
  rebinding host, foreign origin, foreign origin on WS upgrade, no origin);
  `Serve` shuts down on ctx cancel; assets served at `/ui/components.js` with a
  JS content type; request logging (moved test).
- `internal/cmd`: `runbook serve --bind 0.0.0.0` fails before listening.
- Front end: no JS test runner exists in the repo and none is added. Manual
  browser check of every runbook feature in Requirement 3.1 (golden path plus
  sensitive input, confirm gate, stop, reset), done with Puppeteer.
- Existing `TestRoutes_*` tests keep asserting the page references real
  endpoints and assets (extended to `/ui/` assets).

## Docs

- `docs/runbook.md`: `--bind` accepts loopback only.
- `docs/specs/runbook-server-bootstrap/requirements.md` 13.2: mark superseded
  by web-ui-kit 5.1.
- `CLAUDE.md` architecture: add `pkg/webui`.
