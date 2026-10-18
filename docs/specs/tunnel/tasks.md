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

- [ ] 5. `Open` lifecycle: create, wait, forward loop, teardown (TDD)
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

- [ ] 6. `tunnel db` command with real kube client and SPDY forwarder (TDD for resolution)
  - Write `internal/cmd/tunnel_db_test.go` first: named target, ad hoc
    `--host/--engine` (+ default port), name and `--host` together → error,
    `--host` without `--engine` → error, unknown name lists names, empty config
    with no flags → guidance error
  - Implement `pkg/tunnel/kube.go` (`NewClient`), `pkg/tunnel/forward.go`
    (`SPDYForwarder`, loopback only), `internal/cmd/tunnel.go`,
    `internal/cmd/tunnel_db.go` (fuzzy picker, namespace precedence,
    `signal.NotifyContext`, Forbidden hint); register in `root.go`
  - Verify: `go test ./...`; manual against a dev cluster — open a tunnel to a
    postgres and a mysql target, connect with `psql`/`mysql`, kill the API
    connection (or sleep the laptop) and see reconnect, Ctrl-C and confirm the
    pod is gone (`kubectl get pods -n synacklab-tunnels`), set
    `max_duration: 1m` and confirm Kubernetes kills the pod
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.6, 1.7, 2.4, 4.5, 5.3, 6.4, 7.1, 7.2, 7.3, 7.4_

- [ ] 7. Documentation
  - Write `docs/tunnel.md`: Namespace + Role + RoleBinding manifests, optional
    NetworkPolicy, networking/source-IP explanation with `node_selector` and
    provider notes (EKS Security Groups for Pods)
  - Add `tunnel db` to `docs/commands.md` and the `tunnel` section to
    `docs/config-reference.md`; add `pkg/tunnel` to the CLAUDE.md architecture
    tree
  - Verify: apply the RBAC manifest to the dev cluster and repeat task 6's
    manual check as a non-admin user bound to the Role
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [ ] 8. Final gates
  - Verify: `go test ./...`, `golangci-lint run`, `make build`; all files under
    500 lines
  - _Requirements: all_
