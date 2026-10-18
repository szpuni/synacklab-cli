# Design Document

## Overview

```
internal/cmd/tunnel_serve.go   thin: load config, build Server with injected
                               resolve/open funcs, ValidateBind, webui.Serve
pkg/tunnel/server.go           HTTP API + single-tunnel state
pkg/tunnel/web/                index.html, app.js, app.css (tunnel-specific only;
                               components from /ui/)
pkg/tunnel/tunnel.go           + Options.Ready callback
pkg/tunnel/kube.go             + Contexts()
```

`pkg/tunnel` cannot import `pkg/config` (config imports tunnel), and target
resolution must be the CLI's own code (Requirement 2.4). So the command injects
it:

```go
// pkg/tunnel/server.go
type Request struct {
    Name      string `json:"name"`      // configured database, or
    Host      string `json:"host"`      // ad hoc target
    Engine    string `json:"engine"`
    Port      int    `json:"port"`
    LocalPort int    `json:"localPort"` // 0 = database port
    Context   string `json:"context"`
    Namespace string `json:"namespace"`
}

type ServerConfig struct {
    Databases   []Database // name, engine, host, port for the list
    DatabaseErr string     // validation error shown instead of the list
    Contexts    []string
    Defaults    Request    // context, namespace preselected
    // Start resolves req like `tunnel db` and runs tunnel.Open until ctx ends.
    // out receives the CLI's stdout+stderr text; ready is Options.Ready.
    Start func(ctx context.Context, req Request, out io.Writer, ready func(Status)) error
}
```

`internal/cmd` implements `Start` from `resolveDBTarget` (name or
host/engine/port branch; the picker func is never reached because the UI always
sends a name or a host), `tunnelOptions` (refactored to take namespace instead
of reading the package-level flag), `tunnel.NewClient(req.Context)`, and
`tunnel.Open`. One code path, so CLI and UI cannot drift.

## API

All JSON. Routes mount the kit at `/ui/` and app assets at `/`.

```
GET    /api/config           {databases, databaseError, contexts, engines, defaults}
GET    /api/tunnels          [{id, state, request, pod, localAddr, hint, error}]  (0 or 1 item in v1)
POST   /api/tunnels          Request → 201 {id} | 400 {error} | 409 {error}
DELETE /api/tunnels/{id}     cancel; 202, state → closing → closed
GET    /ws/tunnels/{id}      replays output so far, then streams
                             {"type":"output","text":…} and {"type":"state",…}
```

`400` is returned for resolution/validation errors found synchronously before
the pod is created (unknown name, bad host, bad engine, conflicting fields), so
the form can show them inline. Errors after that (cluster, RBAC, port in use)
arrive as `state: failed` with the error text.

States: `starting → connected → closing → closed`, or `starting|connected → failed`.

### Single tunnel, list-shaped

The server holds `tunnels map[string]*run` guarded by a mutex and a
`const maxActive = 1`. POST returns 409 when the active count is at the limit.
Finished tunnels stay in the map (for the page to show the last result) until
the next POST replaces them. Allowing several tunnels later means raising
`maxActive`, keeping finished entries, and letting the UI show the form while
one runs, with no API change.

## `pkg/tunnel` changes

- `Options.Ready func(pod string)`: called where `printConnected` runs today.
  nil-safe; the CLI leaves it nil. This is the only change to `Open`.
- `Contexts() (names []string, current string, err error)` in `kube.go`, from
  the same kubeconfig loading rules as `NewClient`.

Output capture: `Options.Out` and `Options.Err` both point at the run's
`outputBuffer` (a mutex-guarded byte slice plus subscriber notify), which the
WebSocket handler replays and then tails. `PortForwarder`'s stderr writer points
at it too.

## Lifecycle

```
POST → resolve (sync, 400 on error) → run goroutine:
        ctx = WithCancel(server base ctx)
        Start(ctx, req, buf, ready) ── ready → state connected
        returns nil  → closed      (cancelled by DELETE or shutdown)
        returns err  → failed(err)
DELETE → cancel(ctx)               (Open deletes the pod)
SIGINT → base ctx cancelled → server waits on all run goroutines (WaitGroup)
         before webui.Serve returns
```

Requirement 4.3 needs the wait: `http.Server.Shutdown` waits for handlers, not
for background goroutines, so `Server.Wait()` is called by the command after
`webui.Serve` returns.

## UI

One page using kit factories: a `card` with the form (target `select`
including an "Ad hoc host…" option that reveals host/engine/port `field`s,
local port, context `select`, namespace), a `card` per tunnel with a state
`badge`, details, client hint with a copy `button`, a `logView`, and a Close
`button`. The form is disabled while a tunnel is active.

## Testing

- `pkg/tunnel/server_test.go` with a fake `Start`: POST happy path → starting →
  connected (fake calls ready); 400 on resolve error; 409 when active; DELETE
  cancels and reaches closed; failed carries the error; WS replays output then
  streams; shutdown waits for the run to finish.
- `pkg/tunnel` `Ready` called once when the forward is ready (existing fake
  forwarder tests).
- `internal/cmd`: `Start` built from `resolveDBTarget` returns CLI-identical
  errors for name+host, unknown name, bad engine.
- End to end on kind (as for `tunnel db`): open from the page, connect with
  curl/psql through the local port, Close deletes the pod, Ctrl-C on the server
  with a tunnel open leaves no pod. Puppeteer drives the page.
