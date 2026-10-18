# Implementation Plan

- [x] 1. Upgrade Go toolchain baseline (own commit, before any tunnel code)
  - Install a golangci-lint v2 built with Go ≥ 1.27 locally (the current
    2.12.2 build, made with Go 1.26.2, fails typechecking the Go 1.27 stdlib
    even on unchanged `main`)
  - `go mod edit -go=1.26.0 -toolchain=none`
  - Set `GO_VERSION: '1.27'` in `.github/workflows/ci.yml` and `release.yml`
  - Move `golangci-lint-action@v3` to a version that supports golangci-lint v2
    and pin a v2 release
  - Update `docs/installation.md`: Go 1.26+ to build from source; prebuilt
    macOS binaries need macOS 12+
  - Verify: `go build ./...`, `go vet ./...`, `go test ./...`,
    `golangci-lint run` all pass locally; CI green on the branch
  - Raising the `go` directive enabled `usetesting`'s `t.Chdir` check: four
    runbook tests switched from manual `os.Chdir`+cleanup to `t.Chdir`
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.6_

- [x] 2. Tunnel config section and validation rules (TDD)
  - Write table-driven tests first in `pkg/tunnel/validate_test.go` and
    `engine_test.go`: host (hostname, IPv4, IPv6, leading `-`, spaces, empty),
    port bounds, `SanitizeUser` (uppercase, dots, empty → `user`, length cap),
    `DefaultPort` (mysql, postgres, unknown)
  - Implement `pkg/tunnel/validate.go` and `engine.go`
  - Write `ValidateTunnel` tests in `pkg/config/config_test.go`: defaults
    applied, duplicate/empty names, bad engine/host/port, bad `max_duration`
  - Add `TunnelConfig`/`TunnelDatabase` and `ValidateTunnel()` to
    `pkg/config/config.go`
  - Verify: `go test ./pkg/tunnel ./pkg/config`
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 1.5_

- [x] 3. Add client-go and build the tunnel pod (TDD)
  - `go get k8s.io/client-go k8s.io/api k8s.io/apimachinery` v0.37 (same minor for
    all three)
  - Pin the default socat image by digest (`alpine/socat:1.8.1.1@sha256:7f9a…`;
    entrypoint is `socat`, image user is root, hence `runAsUser: 65534`)
  - Write tests first, through `Open` in `pkg/tunnel/tunnel_test.go` (stub
    `Forwarder` reads the created pod from the fake clientset): name prefix +
    random suffix (set client-side; the fake clientset ignores `generateName`), labels, argv,
    restartPolicy, automount off, deadline from `MaxDuration`, nodeSelector,
    pod/container security contexts, probe, resources
  - Implement `pkg/tunnel/pod.go`
  - Verify: `go test ./pkg/tunnel` and `golangci-lint run` (gomodguard clean)
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 9.5_

- [x] 4. Wait for readiness with fail-fast reasons (TDD)
  - Write tests through `Open` (fake clientset + create reactor that sets pod
    status): Ready → forwards;
    ImagePullBackOff, Unschedulable, terminated container → immediate error
    with reason; never ready → timeout error (inject a short timeout)
  - Implement `pkg/tunnel/wait.go`
  - Verify: `go test ./pkg/tunnel`
  - _Requirements: 4.1, 4.2, 4.3_

- [x] 5. `Open` lifecycle: create, wait, forward loop, teardown (TDD)
  - Write `pkg/tunnel/tunnel_test.go` with fake clientset + stub `Forwarder`:
    busy local port → error, no pod created; ctx cancel → pod deleted, nil;
    not ready → pod deleted, error; forwarder drops with pod Running → called
    again; forwarder drops with pod Failed → error with phase/reason; Forbidden
    on create → `apierrors.IsForbidden(err)`; deletion failure → message names
    pod and mentions `max_duration`; status output contains local address and
    client hint
  - Implement `pkg/tunnel/tunnel.go` (incl. log streaming to `opts.Err`)
  - Verify: `go test -race ./pkg/tunnel`
  - _Requirements: 4.4, 5.1, 5.2, 5.4, 5.5, 5.6, 5.7, 6.1, 6.2, 6.3_

- [x] 6. `tunnel db` command with real kube client and port forwarder (TDD for resolution)
  - Write `internal/cmd/tunnel_db_test.go` first: named target, ad hoc
    `--host/--engine` (+ default port), name and `--host` together → error,
    `--host` without `--engine` → error, unknown name lists names, empty config
    with no flags → guidance error
  - Implement `pkg/tunnel/kube.go` (`NewClient`), `pkg/tunnel/forward.go`
    (`PortForwarder`: WebSocket with SPDY fallback, loopback only), `internal/cmd/tunnel.go`,
    `internal/cmd/tunnel_db.go` (fuzzy picker, namespace precedence,
    `signal.NotifyContext`, Forbidden hint); register in `root.go`
  - Verify: `go test ./...`; manual against a dev cluster — open a tunnel to a
    postgres and a mysql target, connect with `psql`/`mysql`, kill the API
    connection (or sleep the laptop) and see reconnect, Ctrl-C and confirm the
    pod is gone (`kubectl get pods -n synacklab-tunnels`), set
    `max_duration: 1m` and confirm Kubernetes kills the pod
  - Verified against a throwaway kind cluster (own kubeconfig; postgres:16 and
    mysql:8.4 in-cluster): psql handshake and `mysql -e "select version()"`
    through the tunnel, loopback-only listener, Ctrl-C → exit 0 + pod gone,
    picker via stdin fallback, refused and dropped upstream connections
    reported from socat logs, API server restart mid-tunnel → reconnect,
    `max_duration: 1m` → exit 1 with `DeadlineExceeded` after 63s
  - Fixes from that run (each test-first): removed the readiness probe (it
    made socat dial MySQL every 2s → `Broken pipe` noise), added socat
    `connect-timeout=10`, retried pod lookup/delete through transient API
    errors, polled pod phase while forwarding (idle tunnels hung on a dead
    pod), `SilenceUsage` on `tunnel db`
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.6, 1.7, 2.4, 4.5, 5.3, 6.4, 7.1, 7.2, 7.3, 7.4_, 5.7, 5.8, 6.2

- [x] 7. Documentation
  - Write `docs/tunnel.md`: Namespace + Role + RoleBinding manifests, optional
    NetworkPolicy, networking/source-IP explanation with `node_selector` and
    provider notes (EKS Security Groups for Pods)
  - Add `tunnel db` to `docs/commands.md` and the `tunnel` section to
    `docs/config-reference.md`; add `pkg/tunnel` to the CLAUDE.md architecture
    tree
  - Verify: apply the RBAC manifest to the dev cluster and repeat task 6's
    manual check as a non-admin user bound to the Role
  - Verified on kind (Kubernetes 1.36): the documented Role, bound to a
    ServiceAccount group, runs a tunnel end to end as a non-admin user;
    `pods/portforward: create` alone is enough (no `get`); a namespace without
    the Role gives the Forbidden error plus the RBAC hint; the NetworkPolicy
    passes server-side validation (enforcement depends on the CNI, not tested)
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [x] 8. Final gates
  - Verify: `go test ./...`, `golangci-lint run`, `make build`; all files under
    500 lines
  - Binary size (stripped): 18.2 MB → 54.4 MB with the full clientset; switched
    to the core/v1 client → 45.4 MB. `tunnel_test.go` hit 498 lines and was
    split into `tunnel_test.go` / `tunnel_resilience_test.go`
  - _Requirements: all_

- [x] 9. Code-review fixes
  - Forbidden from any step carries the RBAC hint, added in `pkg/tunnel`
    (`withRBACHint`); a denied port-forward, or a first forward that never
    connects, ends the tunnel instead of looping on "reconnecting"
  - `PortForwarder` keeps the WebSocket Forbidden status when the SPDY fallback
    fails too (client-go flattens SPDY errors to text)
  - Pod deleted from outside → "tunnel pod … was deleted" within one pod check;
    `getPodRetrying` no longer retries NotFound
  - Timeout message includes the container waiting reason
  - `max_duration` must be whole seconds ≥ 1s
  - `tunnel.databases` validated only when used, so `--host` ignores a broken
    entry (`TunnelConfig.ValidateDatabases` replaces `Config.ValidateTunnel`)
  - Pod name collision retried with a new suffix (up to 3 creates)
  - One `engines` table (port + client hint); `cmp.Or` for flag/config
    precedence; `config.example.yaml` gains a `tunnel` section
  - Verify: unit tests above; on kind (Kubernetes 1.36), a user without
    `pods/portforward` fails in ~2s with both denials and the RBAC hint and
    the pod is deleted; with the documented Role (`create` only) the tunnel
    serves traffic over the SPDY fallback; `kubectl delete pod` on an idle
    tunnel exits 1 with "was deleted" after 4s
  - _Requirements: 1.4, 1.6, 2.3, 2.5, 3.1, 4.3, 4.5, 5.6, 5.7_

- [x] 10. Second review fixes
  - Port-forward denial comes from SPDY's own 403 (`deniedTransport` decodes the
    `metav1.Status`), not from the always-denied WebSocket attempt: a transient
    SPDY failure under the documented Role now reconnects, and a SPDY denial
    after a non-403 WebSocket failure is reported with the RBAC hint
  - Ctrl-C while the create request is in flight deletes the client-named pod
    and exits zero
  - `TunnelDatabase.Target()` is the one definition of a valid target (used by
    `ValidateDatabases`, `resolveDBTarget`, the picker); `Target.Validate()`
    guards `Open`
  - Pod helpers take a `PodInterface` instead of `(client, namespace, name)`;
    `Lifetime` → `ParseMaxDuration`; private `open` → `run`
  - `config.example.yaml` shows `tunnel.image`
  - Verify: `forward_test.go` runs the real WebSocket→SPDY dialers against an
    `httptest` API server; resilience test for cancelled create
  - _Requirements: 4.5, 5.6, 6.1, 2.3, 2.4_
