# Requirements Document

## Introduction

`synacklab tunnel serve` opens the same database tunnels as `synacklab tunnel db`,
driven from a local browser page instead of flags. It is a UX convenience for
the operator on their own machine: the page offers the same choices as the CLI
(configured database or ad hoc host/engine/port, local port, kube context,
namespace), shows the tunnel's progress and pod logs, and closes it with a
button.

It reuses `pkg/tunnel.Open` unchanged in behavior, the CLI's target resolution,
and the shared kit from `docs/specs/web-ui-kit/` (components and local-only
guard). It depends on that spec landing first.

v1 runs one tunnel at a time. The API and UI are shaped as a list of tunnels so
that allowing several later is a limit change, not a redesign.

Out of scope: several concurrent tunnels, editing `tunnel.databases` from the
UI, non-database targets, remote access to the page.

## Requirements

### Requirement 1 — Starting the server

**User Story:** As an operator, I want to start the tunnel UI with one command,
so that I can open tunnels without remembering flags.

#### Acceptance Criteria

1. WHEN the user runs `synacklab tunnel serve` THEN the system SHALL serve the tunnel UI on `127.0.0.1:4748` and print its URL.
2. WHEN `--port` or `--bind` is given THEN the system SHALL use them, subject to web-ui-kit Requirement 5 (loopback only, Host/Origin guard).
3. WHEN `--context` or `--namespace` is given THEN the system SHALL use them as the form's preselected values.
4. IF `~/.synacklab/config.yaml` or the kubeconfig fails to load, or `tunnel.max_duration` is invalid, THEN the system SHALL fail before listening, as `tunnel db` does before contacting the cluster.
5. IF `tunnel.databases` is invalid THEN the system SHALL still start, show the validation error in place of the database list, and allow ad hoc targets (tunnel Requirement 2.3: databases are validated only when used).

### Requirement 2 — Choosing a target (parity with `tunnel db`)

**User Story:** As an operator, I want the same target choices as the CLI, so
that the UI never offers something the CLI would reject or vice versa.

#### Acceptance Criteria

1. WHEN the page loads THEN it SHALL list the configured databases by name with engine and `host:port`, replacing the CLI's fuzzy picker.
2. WHEN the user chooses "Ad hoc host" THEN the page SHALL ask for host, engine (`mysql` or `postgres`), and optional port (default: the engine's port).
3. WHEN the page loads THEN it SHALL offer local port (default: the database port), kube context (list from kubeconfig, default: `--context`, else the current context), and namespace (default: `--namespace`, else `tunnel.namespace`, else `synacklab-tunnels`).
4. WHEN a tunnel is requested THEN the server SHALL resolve and validate the target with the same code as `tunnel db` (`resolveDBTarget` and `tunnelOptions`), returning the same error messages; the browser's own checks are a convenience, not the validation.

### Requirement 3 — Running the tunnel

**User Story:** As an operator, I want to see the tunnel come up and know how to
connect, so that I don't need the terminal.

#### Acceptance Criteria

1. WHEN a valid tunnel request is accepted THEN the system SHALL open it with `tunnel.Open` and show it as `starting`.
2. WHEN the port-forward becomes ready THEN the tunnel SHALL show as `connected` with pod name, namespace, `127.0.0.1:<local port>`, and the engine's client hint with a copy button.
3. WHILE the tunnel runs THEN the page SHALL stream the tunnel's output and warnings (the CLI's stdout/stderr text, including socat pod logs and reconnect notices) live.
4. WHEN `tunnel.Open` returns an error THEN the tunnel SHALL show as `failed` with that error text, including the RBAC hint for Forbidden errors.
5. IF a tunnel is already starting or connected THEN the system SHALL reject another request with HTTP 409 and the page SHALL disable the open form while one is active.
7. IF the local port is outside 1–65535 THEN the system SHALL reject the request with HTTP 400 and the CLI's message, before creating a pod.
6. WHEN the page is reloaded or reopened THEN it SHALL show the current tunnel's status and output so far.

### Requirement 4 — Closing and cleanup

**User Story:** As an operator, I want closing to be as clean as Ctrl-C in the
CLI, so that no pod is left behind.

#### Acceptance Criteria

1. WHEN the user clicks Close THEN the system SHALL cancel the tunnel, which deletes its pod (tunnel Requirement 5), and show it as `closed`.
2. WHEN the browser tab is closed THEN the tunnel SHALL keep running until Close is clicked or the server stops.
3. WHEN the server receives SIGINT/SIGTERM THEN it SHALL cancel any running tunnel and wait for its pod deletion to finish (bounded by the existing delete timeout) before exiting.

### Requirement 5 — Scope of exposure

**User Story:** As a platform owner, I want the tunnel UI to grant nothing the
CLI doesn't, so that it adds no attack surface.

#### Acceptance Criteria

1. The server SHALL only open tunnels on `127.0.0.1` local ports, exactly as the CLI.
2. The API SHALL NOT expose kubeconfig contents, credentials, or cluster objects other than the context names and the tunnel's own pod name and logs.
