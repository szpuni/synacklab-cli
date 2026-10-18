# Requirements Document

## Introduction

`synacklab tunnel` gives engineers on-demand TCP access to services that are only
reachable from inside a private network segment. The motivating case: a database
lives in a `db` subnet whose firewall/security group only accepts traffic from the
`app` subnet, where the Kubernetes nodes run. The command starts a short-lived
socat pod on those nodes, port-forwards the engineer's laptop to it through the
Kubernetes API, and deletes the pod when the engineer is done.

The design is deliberately cloud-agnostic: it depends only on a working kube
context (any provider, any cluster), never on a cloud provider's API. The first
subcommand is `tunnel db` for MySQL and PostgreSQL; `tunnel redis` /
`tunnel opensearch` are expected later and reuse the same core, but are out of
scope for this spec.

Out of scope (v1): `tunnel list` / `tunnel cleanup` commands, cloud-specific
target discovery (e.g. AWS RDS), non-database targets, UDP, sharing one tunnel pod
between users.

## Requirements

### Requirement 1 — Choosing a database target

**User Story:** As a DevOps engineer, I want to pick which database to tunnel to,
either from a list or directly on the command line, so that I can connect
interactively day to day and also script it.

#### Acceptance Criteria

1. WHEN the user runs `synacklab tunnel db` with no name argument and no `--host` THEN the system SHALL present the databases configured under `tunnel.databases` in an interactive fuzzy picker (`pkg/fuzzy`) showing each entry's name, engine, and `host:port`.
2. WHEN the user runs `synacklab tunnel db <name>` THEN the system SHALL use the configured database with that name without prompting.
3. IF `<name>` does not match any configured database THEN the system SHALL fail with an error listing the configured names.
4. WHEN the user runs `synacklab tunnel db --host <host> --engine <mysql|postgres>` THEN the system SHALL use that ad hoc target without reading `tunnel.databases`.
5. WHEN `--port` is omitted for an ad hoc target THEN the system SHALL use the engine's default port (mysql 3306, postgres 5432).
6. IF both `<name>` and `--host` are given, `--host` is given without `--engine`, or `--engine`/`--port` is given without `--host` THEN the system SHALL fail with a usage error before contacting the cluster.
7. IF no name and no `--host` are given and `tunnel.databases` is empty THEN the system SHALL fail with an error explaining how to configure databases or use `--host`.

### Requirement 2 — Configuration and input validation

**User Story:** As a DevOps engineer, I want tunnel settings and database targets
kept in my synacklab config and checked up front, so that mistakes fail fast with
a clear message instead of as a broken pod.

#### Acceptance Criteria

1. WHEN `~/.synacklab/config.yaml` contains a `tunnel` section THEN the system SHALL read `namespace`, `image`, `max_duration`, `node_selector`, and `databases` (each with `name`, `engine`, `host`, optional `port`) from it.
2. IF `namespace`, `image`, or `max_duration` is omitted THEN the system SHALL default to `synacklab-tunnels`, the pinned default socat image, and `8h` respectively.
3. WHEN a tunnel command uses `tunnel.databases` (a name argument or the picker) THEN the system SHALL validate every configured database and fail, naming the offending entry, if `engine` is not `mysql` or `postgres`, `host` is not a valid RFC 1123 hostname or IP address, `port` is outside 1–65535, or `name` is empty or duplicated.
4. WHEN `--host`, `--port`, `--engine`, or `--local-port` are given THEN the system SHALL apply the same validation as AC 3 before contacting the cluster.
5. IF `max_duration` does not parse as a Go duration of a whole number of seconds, at least 1s (it becomes `activeDeadlineSeconds`), THEN the system SHALL fail with an error naming the field.
6. WHEN a host value starts with `-` THEN the system SHALL reject it (it would be interpreted as a socat option).

### Requirement 3 — Tunnel pod

**User Story:** As a platform owner, I want the tunnel pod to be minimal, locked
down, and traceable to a user, so that granting tunnel access doesn't widen the
cluster's attack surface.

#### Acceptance Criteria

1. WHEN a tunnel opens THEN the system SHALL create exactly one pod in the resolved namespace with name prefix `synacklab-tunnel-<user>-` (random suffix, regenerated if the name is already taken), where `<user>` is the local OS username sanitized to a DNS-1123 label fragment.
2. WHEN the pod is created THEN it SHALL carry labels `app.kubernetes.io/managed-by=synacklab`, `synacklab.io/user=<user>`, and `synacklab.io/target=<engine>`.
3. WHEN the pod is created THEN its single container SHALL run the configured image with socat arguments passed as an argv list (no shell): listen on TCP port 8080 with `fork,reuseaddr`, connect to `<host>:<port>` with a 10-second connect timeout (IPv6 hosts bracketed).
4. WHEN the pod is created THEN it SHALL set `restartPolicy: Never`, `automountServiceAccountToken: false`, `activeDeadlineSeconds` equal to `max_duration`, and the configured `node_selector`.
5. WHEN the pod is created THEN its container SHALL run as a non-root UID with `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, all capabilities dropped, seccomp profile `RuntimeDefault`, and CPU/memory requests and limits set.
6. WHEN the pod is created THEN it SHALL NOT define readiness, liveness, or startup probes, because each probe connection makes socat open a real connection to the target (observed against MySQL: a `Broken pipe` log line every probe period and aborted connections on the server). A Running container is treated as ready.

### Requirement 4 — Readiness and failure reporting

**User Story:** As a DevOps engineer, I want to know quickly and specifically why
a tunnel failed to start, so that I can fix it or tell the platform team what's
wrong.

#### Acceptance Criteria

1. WHEN the pod is created THEN the system SHALL wait up to 2 minutes for the pod to be Running with its container Ready before forwarding.
2. IF the pod reports an image pull failure, an unschedulable condition, or a terminated container while waiting THEN the system SHALL stop waiting immediately and report the Kubernetes reason and message.
3. IF the pod is not ready within 2 minutes THEN the system SHALL fail with a timeout error including the pod's last observed phase and reason, and its container's waiting reason (e.g. `ContainerCreating`).
4. WHEN the tunnel fails to become ready for any reason THEN the system SHALL delete the pod before exiting non-zero.
5. IF the Kubernetes API returns Forbidden for any tunnel operation (create, get, logs, port-forward, delete) THEN the system SHALL report the denied verb/resource/namespace and point to the RBAC section of `docs/tunnel.md`; a denied port-forward SHALL end the tunnel rather than be retried. A port-forward counts as denied only when the request that decides the outcome (the SPDY fallback, after a failed WebSocket upgrade) was denied.

### Requirement 5 — Port forwarding

**User Story:** As a DevOps engineer, I want the database to appear on a local port
on my machine, so that I can point `psql`, `mysql`, or a GUI client at it.

#### Acceptance Criteria

1. WHEN `--local-port` is omitted THEN the system SHALL use the target's port as the local port.
2. IF the local port is already in use THEN the system SHALL fail with an error suggesting `--local-port` before creating any pod.
3. WHEN the pod is ready THEN the system SHALL forward `127.0.0.1:<local-port>` to pod port 8080 through the Kubernetes API, and SHALL NOT listen on any non-loopback address.
4. WHEN forwarding starts THEN the system SHALL print the pod name, namespace, target engine and `host:port`, the local address, an example client command for the engine, and how to stop.
5. WHILE the tunnel is open THEN the system SHALL stream the pod's log output (socat errors such as refused or timed-out upstream connections) to stderr.
6. IF the forwarding stream drops while the pod is still Running THEN the system SHALL re-establish forwarding on the same local port and print a notice, retrying the pod lookup through transient API errors (e.g. an API server restarting) for up to 1 minute. IF the first forward fails before it ever listened THEN the system SHALL fail instead of retrying.
7. IF the pod is no longer Running (e.g. `max_duration` reached) or has been deleted THEN the system SHALL stop forwarding within one pod check (5 seconds plus the API call), even while no connection is open, report the pod's phase and reason (or that it was deleted), and exit non-zero.
8. IF a tunnel command fails at runtime THEN the system SHALL print the error without the command's usage text.

### Requirement 6 — Teardown

**User Story:** As a platform owner, I want tunnel pods to disappear when the
engineer is done or the CLI dies, so that idle open paths into the database
don't accumulate.

#### Acceptance Criteria

1. WHEN the user presses Ctrl-C or the process receives SIGTERM THEN the system SHALL stop forwarding, delete the pod (including one whose create request was in flight), and exit zero.
2. WHEN deleting the pod during teardown THEN the system SHALL use a fresh context with a 10-second timeout, independent of the cancelled command context, retrying through transient API errors within that timeout and treating NotFound as success.
3. IF pod deletion fails THEN the system SHALL print the pod name, namespace, and the error, and SHALL note that the pod will still be removed when `max_duration` expires.
4. IF the CLI exits without running teardown (crash, killed, laptop asleep) THEN Kubernetes SHALL terminate the pod no later than `max_duration` after creation via `activeDeadlineSeconds`.

### Requirement 7 — Kubernetes access

**User Story:** As a DevOps engineer on any cloud, I want the tunnel to use my
normal kube credentials and contexts, with nothing else to install.

#### Acceptance Criteria

1. WHEN connecting to the cluster THEN the system SHALL load kubeconfig using standard rules (`$KUBECONFIG`, else `~/.kube/config`), including exec credential plugins.
2. WHEN `--context` is given THEN the system SHALL use that kube context; otherwise the kubeconfig's current context.
3. WHEN resolving the namespace THEN the system SHALL use `--namespace`, else `tunnel.namespace`, else `synacklab-tunnels`.
4. WHEN talking to the Kubernetes API THEN the system SHALL do so in-process and SHALL NOT require `kubectl` or any other external binary beyond what the kubeconfig's own credential plugin needs.

### Requirement 8 — Operator documentation

**User Story:** As a platform owner, I want copy-pasteable setup instructions, so
that I can enable tunnels for a team safely.

#### Acceptance Criteria

1. WHEN the feature ships THEN `docs/tunnel.md` SHALL document a Namespace, Role (`pods`: create/get/list/watch/delete; `pods/log`: get; `pods/portforward`: create), and RoleBinding for an engineer group.
2. WHEN the feature ships THEN `docs/tunnel.md` SHALL include an optional NetworkPolicy restricting tunnel pod egress to database CIDRs/ports.
3. WHEN the feature ships THEN `docs/tunnel.md` SHALL explain that the pod's source IP must be accepted by the database's firewall/security group, and how `node_selector` (or provider features such as EKS Security Groups for Pods) achieves that.
4. WHEN the feature ships THEN `docs/commands.md` and `docs/config-reference.md` SHALL document the `tunnel db` command and the `tunnel` config section.

### Requirement 9 — Supported Go toolchain baseline

**User Story:** As the maintainer, I want synacklab built with a Go release that
still receives security fixes and that current client-go supports, so that
shipped binaries get stdlib TLS/HTTP patches and the tunnel can use a supported
Kubernetes client.

Context: `go.mod` declares `go 1.23.0` / `toolchain go1.24.5` and CI/release
build with Go 1.24, which is out of security support (supported: 1.26, 1.27).
client-go v0.37 requires Go ≥ 1.26; the newest client-go on Go 1.23 (v0.32)
targets an end-of-life Kubernetes release. A trial bump to `go 1.26.0` built,
vetted, and passed all tests with no code changes. This upgrade lands before
any tunnel code and on its own commit.

#### Acceptance Criteria

1. WHEN the module is built THEN `go.mod` SHALL declare `go 1.26.0` and SHALL NOT carry a separate `toolchain` line older than it.
2. WHEN CI or the release workflow builds, tests, or lints THEN it SHALL use Go 1.27 (`GO_VERSION` in `.github/workflows/ci.yml` and `release.yml`).
3. WHEN CI runs golangci-lint THEN it SHALL use a golangci-lint build compiled with Go ≥ 1.26 and a golangci-lint-action version that supports golangci-lint v2.
4. WHEN the toolchain is upgraded THEN `go build ./...`, `go vet ./...`, `go test ./...`, and `golangci-lint run` SHALL pass without behavior changes to existing commands.
5. WHEN the tunnel feature adds Kubernetes dependencies THEN it SHALL use client-go v0.37 (with matching `k8s.io/api` and `k8s.io/apimachinery`).
6. WHEN the toolchain is upgraded THEN `docs/installation.md` SHALL state Go 1.26 or later as the build-from-source requirement, and note that prebuilt macOS binaries require macOS 12 or later.
