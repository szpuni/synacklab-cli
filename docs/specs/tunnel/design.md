# Design Document

## Overview

`tunnel` adds a new `pkg/tunnel` package plus two thin `internal/cmd` wrappers
(`tunnel.go` parent, `tunnel_db.go`). `pkg/tunnel` owns the whole pod lifecycle —
create, wait, forward, tear down — behind one function, `Open`. Subcommands only
resolve a `Target{Host, Port}` and hand it over; nothing in `pkg/tunnel` knows
about databases beyond a port-defaults table.

The package talks to Kubernetes through client-go in-process. This is a hard
requirement from `.kiro/steering/tech.md` ("single binary executable — no external
runtime dependencies"): shelling out to `kubectl` would break it. Nothing in this
feature calls a cloud provider API, so it works against any cluster.

## Architecture

```
 laptop                                   cluster (node in "app" subnet)
┌──────────────────────────┐   SPDY over  ┌──────────────────────────────┐
│ psql → 127.0.0.1:5432    │   kube API   │ pod synacklab-tunnel-<u>-xyz │
│          │               │ ───────────> │  socat TCP-LISTEN:8080,fork  │──> db-host:5432
│ synacklab tunnel db      │              │        TCP:db-host:5432      │    ("db" subnet,
│  (client-go portforward) │ <─── logs ── │                              │     allows "app")
└──────────────────────────┘              └──────────────────────────────┘
```

Lifecycle inside `Open`:

```
check local port free ─┐ (fail before any cluster side effect)
create pod             │
wait ready (≤2m) ──────┼── fail fast on ImagePull/Unschedulable/Terminated
stream pod logs → stderr (goroutine)
forward loop:  forward(ctx) returns
                 ├─ ctx done          → teardown, return nil
                 ├─ pod still Running → notice, re-forward
                 └─ pod gone/failed   → teardown, return error with phase/reason
teardown: delete pod with fresh 10s context (always, via defer)
```

## Package Structure

```
pkg/tunnel/
  tunnel.go       Target, Options, Open — lifecycle orchestration
  pod.go          buildPod(Target, Options) *corev1.Pod — pure
  wait.go         waitReady(ctx, client, ns, name) — readiness + fail-fast reasons
  forward.go      PortForwarder(restConfig, client, errOut) Forwarder — real client-go portforward
  kube.go         NewClient(kubeContext) (*corev1client.CoreV1Client, *rest.Config, error)
  engine.go       engines table (default port + client hint); DefaultPort(engine string) (int, error)
  validate.go     ValidateHost, ValidatePort, SanitizeUser
  *_test.go
pkg/config/
  config.go       + TunnelConfig, TunnelDatabase, TunnelConfig.ValidateDatabases(), TunnelConfig.Lifetime()
internal/cmd/
  tunnel.go       `tunnel` parent: --context, --namespace persistent flags
  tunnel_db.go    `tunnel db [name]`: --host, --port, --engine, --local-port
docs/
  tunnel.md       operator guide (RBAC, NetworkPolicy, networking)
```

Every file is expected to stay well under 500 lines; `tunnel.go` is the one to
watch.

## Components and Interfaces

### Target and Options

```go
// Target is the remote TCP endpoint the tunnel pod connects to.
type Target struct {
	Kind string // "postgres", "mysql" — used for labels and the client hint only
	Host string
	Port int
}

type Options struct {
	Namespace    string
	Image        string
	MaxDuration  time.Duration
	NodeSelector map[string]string
	LocalPort    int
	User         string    // sanitized; used in pod name and label
	Out, Err     io.Writer // status lines and streamed pod logs
}
```

### Open

```go
// Forwarder forwards 127.0.0.1:localPort to podPort on the named pod until ctx
// is done or the stream breaks. It closes ready once listening.
type Forwarder func(ctx context.Context, namespace, pod string,
	localPort, podPort int, ready chan<- struct{}) error

func Open(ctx context.Context, client corev1client.CoreV1Interface, forward Forwarder,
	target Target, opts Options) error
```

- `corev1client.CoreV1Interface` is client-go's own interface; tests pass
  `fake.NewClientset().CoreV1()` from `k8s.io/client-go/kubernetes/fake`. No new interface is introduced.
- `Forwarder` is a function type rather than an interface: one method, one real
  implementation (`PortForwarder`), one test stub. It's the only seam the fake
  clientset can't cover, because port-forward goes over SPDY, not the typed API.
- `Open` blocks until the tunnel ends. It returns `nil` on user-initiated
  shutdown (ctx cancelled) and an error otherwise.

### buildPod

Pure function, table-tested. Produces:

```yaml
metadata:
  name: synacklab-tunnel-<user>-<5 random chars>   # set client-side
  namespace: <ns>
  labels:
    app.kubernetes.io/managed-by: synacklab
    synacklab.io/user: <user>
    synacklab.io/target: <kind>
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  activeDeadlineSeconds: <max_duration seconds>
  nodeSelector: <node_selector>
  securityContext: {runAsNonRoot: true, runAsUser: 65534, seccompProfile: {type: RuntimeDefault}}
  containers:
  - name: socat
    image: <image>
    args: ["TCP-LISTEN:8080,fork,reuseaddr", "TCP:<host>:<port>,connect-timeout=10"]
    ports: [{containerPort: 8080}]
    resources: {requests: {cpu: 10m, memory: 16Mi}, limits: {cpu: 200m, memory: 64Mi}}
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities: {drop: [ALL]}
```

- No probes (Req 3.6): a TCP probe makes socat dial the target every period.
  Against MySQL, which speaks first, that produced a `Broken pipe` log line
  every 2s and aborted connections on the server. socat listens within
  milliseconds of starting, so a Running container is treated as ready.
- `connect-timeout=10` turns silently dropped traffic (a firewall or security
  group blocking the pod) into a socat error on the user's stderr within
  seconds instead of the OS's ~2-minute TCP timeout.
- The pod always listens on 8080, regardless of target port, so it never needs
  `NET_BIND_SERVICE` (relevant later for OpenSearch on 443).
- The image's entrypoint is `socat`; `args` are socat's argv. No shell is
  involved, and `Host` has already been validated (no leading `-`, RFC 1123 or
  IP), so no injection path exists.
- `Host` is passed through as-is: DNS resolution happens in the pod, so private
  DNS names (`orders.internal`, RDS endpoints) work.

### waitReady

Polls the pod with `Get` every second (`wait.PollUntilContextTimeout`, first
check immediate) for up to 2 minutes. Polling is enough at this rate and avoids
watch-reconnect handling:

| Observation | Result |
|---|---|
| phase Running and container Ready | return nil |
| container waiting reason in {`ErrImagePull`, `ImagePullBackOff`, `InvalidImageName`, `CreateContainerConfigError`} | error with reason + message |
| `PodScheduled=False` with reason `Unschedulable` | error with message (typically a bad `node_selector`) |
| phase Failed/Succeeded, or container terminated | error with reason + exit code |
| 2 minutes elapsed | timeout error with last phase/reason and container waiting reason |

### Forward loop and log streaming

- After ready, a goroutine follows `Pods(ns).GetLogs(name, {Follow: true})` and
  copies to `opts.Err`. It ends with ctx or when the pod goes away.
- The forward loop calls `forward(...)` with a per-attempt context. Beside it,
  `stopWhenPodStops` polls the pod every 5s and cancels that context once the
  pod leaves Running: client-go's forwarder only listens locally, so an idle
  tunnel never notices the pod dying by itself (seen against kind with
  `max_duration: 1m` — the CLI hung on a `DeadlineExceeded` pod). NotFound
  (pod deleted from outside) also stops it; other lookup errors are retried.
- A forward that fails with Forbidden, or a first forward that never became
  ready, ends the tunnel: retrying cannot fix either (a user without
  `pods/portforward` otherwise saw "reconnecting" forever).
- When `forward` returns and ctx is not done, the loop looks the pod up via
  `getPodRetrying` (retries API errors every 1s for up to 1 minute — a
  restarting API server answers Forbidden for a few seconds while its RBAC
  cache warms up): Running → print "Connection to tunnel pod lost; reconnecting"
  and call `forward` again after 1s; anything else → return an error with
  phase/reason; NotFound → "tunnel pod was deleted". No reconnect cap: a Running pod is the only condition for
  retrying, and `activeDeadlineSeconds` bounds the pod's life.
- Teardown (`deletePod`) retries Delete every 1s within its 10s budget and
  treats NotFound as success, for the same API-restart reason.

### PortForwarder

Wraps `k8s.io/client-go/tools/portforward`, building the
`pods/<name>/portforward` URL from the clientset's REST client. Like kubectl it
dials SPDY-over-WebSocket first and falls back to plain SPDY
(`NewFallbackDialerForStreaming`), using the `k8s.io/streaming` httpstream types
(the apimachinery ones are deprecated). It listens via
`NewOnAddressesForStreamingWithContext(ctx, dialer, []string{"127.0.0.1"}, ...)`
so it never binds a non-loopback address. RBAC checks the WebSocket upgrade as
`get` and SPDY as `create` on `pods/portforward`, so a denied WebSocket upgrade
still falls back; client-go reports SPDY failures as plain text, so the
fallback predicate keeps the WebSocket Forbidden status and returns it when
SPDY fails too (before the forward became ready). Not unit-tested (needs a real API
server); covered by the manual verification against kind.

### NewClient

`clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{CurrentContext: ctx})`
→ `rest.Config` → `corev1client.NewForConfig` (core/v1 only: the full
`kubernetes.Clientset` added ~9 MB of unused API groups to the stripped binary). Default loading rules honour
`$KUBECONFIG` and exec credential plugins (e.g. the `aws eks get-token` entries
that `eks-config` writes).

### Command layer

`tunnel_db.go` only:

1. Loads config.
2. Resolves the target: positional name → `tunnel.databases` lookup; `--host` +
   `--engine` → ad hoc; neither → `fuzzy.New("Select database").AddOption(...)`
   then `Select()`. Conflicting inputs fail before any cluster call.
   `tunnel.databases` is validated (`ValidateDatabases`) only on the name/picker
   paths, so a broken entry never blocks `--host` (Req 1.4).
3. Builds `Options` (namespace precedence: flag → config → default via
   `cmp.Or`; `max_duration` validated by `Lifetime`; user from `os/user`,
   sanitized).
4. `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` — same pattern as
   `runbook_run.go`.
5. `NewClient`, then `tunnel.Open(ctx, client, tunnel.PortForwarder(restCfg, client, os.Stderr), target, opts)`.

The default port and client hint per engine live in one `engines` table in
`engine.go`, keyed by `Target.Kind` (`psql -h 127.0.0.1 -p <L> -U <user> <db>`,
`mysql -h 127.0.0.1 -P <L> -u <user> -p`). `Target.Kind` is the generic name
(a future redis target has no "engine"); for databases it holds the engine.

Adding `tunnel redis` later = a new `tunnel_redis.go` that resolves its own
`Target` and calls the same `Open`, plus a `DefaultPort` entry and client hint.
No provider interface until then.

## Data Models

```go
// pkg/config
type Config struct {
	AWS    AWSConfig    `yaml:"aws"`
	GitHub GitHubConfig `yaml:"github"`
	Tunnel TunnelConfig `yaml:"tunnel,omitempty"`
	LogLevel string     `yaml:"log_level,omitempty"`
}

type TunnelConfig struct {
	Namespace    string            `yaml:"namespace,omitempty"`
	Image        string            `yaml:"image,omitempty"`
	MaxDuration  string            `yaml:"max_duration,omitempty"`
	NodeSelector map[string]string `yaml:"node_selector,omitempty"`
	Databases    []TunnelDatabase  `yaml:"databases,omitempty"`
}

type TunnelDatabase struct {
	Name   string `yaml:"name"`
	Engine string `yaml:"engine"`
	Host   string `yaml:"host"`
	Port   int    `yaml:"port,omitempty"`
}
```

`ValidateDatabases()` and `Lifetime()` are called by the tunnel commands only,
so a bad `tunnel` section never breaks `auth` or `github` commands. `Lifetime`
requires whole seconds ≥ 1s, since it becomes `activeDeadlineSeconds`. Host/port/engine checks reuse `pkg/tunnel/validate.go`
and `engine.go` — one set of rules for config and flags.

Example:

```yaml
tunnel:
  namespace: synacklab-tunnels
  max_duration: 8h
  node_selector: {subnet: app}
  databases:
    - {name: prod-orders, engine: postgres, host: orders.internal, port: 5432}
    - {name: billing,     engine: mysql,    host: 10.20.3.15}
```

## Error Handling

- Errors are wrapped with `fmt.Errorf("context: %w", err)`, preserving client-go's
  `*StatusError`.
- `Open` adds the RBAC hint itself (`withRBACHint`) to any Forbidden error it
  returns, and to the log-stream and delete warnings, so the command layer
  does not inspect Kubernetes errors. No custom `*Error` type: no caller
  branches on the failure kind.
- Teardown runs from a `defer` in `Open` once the pod exists, so every exit path
  after creation deletes the pod (Req 4.4, 6.1). Deletion failure is printed,
  not returned over the original error (Req 6.3).
- The local-port check runs before pod creation so a busy port leaves no
  cluster side effects (Req 5.2).

## Security

- Least privilege for users: the documented Role grants pods
  create/get/list/watch/delete, `pods/log` get, `pods/portforward` create in one
  namespace only.
- Pod: non-root, read-only root fs, no capabilities, no service-account token,
  `RuntimeDefault` seccomp, resource limits, hard lifetime.
- Forward binds loopback only.
- Database credentials never touch the pod or synacklab; TLS between the client
  and the database is end-to-end (socat relays bytes).
- Image pinned by digest by default; overridable for corporate registry mirrors.

## Testing Strategy

### Unit testing (testify, table-driven)

Tests sit at public seams only; the pod shape and readiness handling are
observed through `Open`, not by calling `buildPod`/`waitReady` directly.

- `pkg/tunnel/validate_test.go`: hosts (names, IPv4/IPv6, leading `-`, socat
  syntax, zone IDs), ports, engine defaults, user sanitization.
- `pkg/tunnel/tunnel_test.go`: `Open` against the fake clientset, with a create
  reactor standing in for the scheduler/kubelet and stub `Forwarder`s:
  - the created pod (name, labels, argv, security contexts, deadline,
    nodeSelector, resources, no probes); IPv6 bracketing
  - fail-fast reasons (ImagePullBackOff, Unschedulable, socat exit) and timeout,
    each deleting the pod and never forwarding
  - invalid target / busy local port → no cluster calls
  - banner + client hint; pod logs to stderr
  - reconnect while Running, including through API errors; pod stopped (after a
    drop, and while idle) → error with reason
  - pod deleted while idle → "was deleted"; name collision → new name
  - Ctrl-C while waiting → nil + pod deleted;
    delete retried through API errors; persistent delete failure → warning
  - knobs (`ReadyTimeout`, `RetryInterval`, `DeleteTimeout`,
    `PodWatchInterval`) are exposed to tests via `export_test.go`
- `pkg/tunnel/tunnel_rbac_test.go`: Forbidden on create, port-forward (no
  reconnect loop), logs and delete each carry the RBAC hint; a first forward
  that never connects fails instead of retrying.
- `pkg/config/tunnel_test.go`: loading the section, `ValidateDatabases`,
  `Lifetime` (including sub-second and fractional values).
- `internal/cmd/tunnel_db_test.go`: `resolveDBTarget` — name, flags, picker
  (stubbed), conflicting/missing flags, unknown name, empty config.

### Integration testing (optional, `integration` build tag)

`test/integration/tunnel_test.go` against a kind cluster with an in-cluster
postgres: open a tunnel via the built binary, connect over TCP, Ctrl-C, assert
the pod is gone. Not required for v1 sign-off; manual verification against a dev
cluster covers the SPDY path.

## Toolchain baseline (Requirement 9)

Landed first, as its own commit, before any `pkg/tunnel` code:

| File | Change |
|---|---|
| `go.mod` | `go 1.23.0` → `go 1.26.0`; drop `toolchain go1.24.5` |
| `.github/workflows/ci.yml`, `release.yml` | `GO_VERSION: '1.24'` → `'1.27'` |
| `.github/workflows/ci.yml` | `golangci-lint-action@v3` → a version supporting golangci-lint v2, with a pinned v2 release built with Go ≥ 1.26 |
| `docs/installation.md` | "Go 1.21 or later" → "Go 1.26 or later"; prebuilt macOS binaries need macOS 12+ |

Why `go 1.26.0` and not `1.27.0`: 1.26 is the floor client-go v0.37 needs and is
still supported, so `go install` users on 1.26 aren't forced up; CI and releases
still build with 1.27 for the newest stdlib fixes.

Impact on existing behavior: GoReleaser builds stay static (`CGO_ENABLED=0`).
Raising the `go` directive enables newer version-gated GODEBUG defaults (mostly
stricter crypto/TLS); the trial bump passed `go vet` and the full test suite
unchanged. The macOS 12 floor comes from Go ≥ 1.25.

Noticed while scoping, not changed by this spec: the CI lint step passes
`--no-config`, so it ignores `.golangci.yml`; and golangci-lint 2.12 deprecates
`gomodguard` in favour of `gomodguard_v2`. Both are worth a follow-up.

## Dependencies to add

- `k8s.io/client-go` v0.37, with matching `k8s.io/api`, `k8s.io/apimachinery`,
  and `k8s.io/streaming` (port-forward dialer types).

Risks:
- Binary size grows noticeably (client-go pulls in the typed API). Measured
  stripped (`-s -w`, as GoReleaser builds): 18.2 MB on `main` → 45.4 MB with
  the core/v1-only client (54.4 MB with the full clientset). Accepted as the
  cost of the steering doc's no-external-binary rule.
- `.golangci.yml`'s gomodguard blocks `github.com/evanphx/json-patch`; confirm
  `golangci-lint run` stays clean after `go mod tidy` (client-go uses
  `gopkg.in/evanphx/json-patch.v4`, which is not blocked).
