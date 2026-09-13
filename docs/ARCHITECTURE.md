# Architecture Overview

Generated from static analysis (graphify) on 2026-09-13: 1535 nodes, 3675 edges, 77 communities across 179 files.

## The Big Picture

**synacklab-cli** is a Go DevOps tool split into three main domains:

1. **AWS SSO Authentication** — device-auth flow, credential caching, browser + interactive terminal selection
2. **GitHub Repository Management** — Terraform-like plan/apply for single and multi-repo configurations
3. **Runbook Execution** — in-browser interactive shell with safety gates and session persistence

## Core Abstractions (God Nodes)

These are the highest-touch concepts — touch them and ripples spread everywhere:

1. **`RepositoryConfig`** (49 edges) — the desired state for a GitHub repo; referenced by every reconciler, validator, plan display, and merger
2. **`NewMultiReconciler()`** (35 edges) — orchestrates config detection → merge → plan → apply across repos
3. **`NewSessionStore()`** (31 edges) — runbook server's state for active documents and variables
4. **`Client`** (25 edges) — GitHub API wrapper; touches all mutations and reads
5. **`MultiReconciler`** (25 edges) — runtime engine for multi-repo operations

## Architectural Patterns (Hyperedges)

### AWS SSO Login Flow
**Components:** AuthManager, BrowserOpener, FzfFinder, SSOSession, device-auth polling (5s→30s, 10min timeout)

- Picks AWS profile via fzf (or fallback interactive terminal)
- Polls `ssooidc.CreateToken` with exponential backoff
- Caches credentials at `~/.synacklab/aws_credentials.json` (mode 0600)
- Entry points: `aws-login` and `aws-ctx` commands

### Multi-Repo Detect→Merge→Plan→Apply Pipeline
**Components:** ConfigDetector, ConfigMerger, MultiReconciler, MultiRepoRateLimiter

1. **Detect** — distinguish single vs. multi format (YAML magic number)
2. **Merge** — apply defaults, override, or deep-merge per strategy
3. **Plan** — diff desired vs. live; show human-readable changes
4. **Apply** — execute plan with rate limiting (default 5 req/sec); collect partial failures

**Design:** Reconciler interface appears in single-repo (GitHub Repository Reconciler spec), reused at multi-repo scale with batch processing.

### Runbook Step Execution Pipeline
**Components:** Parser → Frontmatter → Step → Template (env merge) → Capture trailer → Execute (spawn subprocess) → Log

- **Safety gates:** danger patterns (regex on output), confirm steps (human approval in interactive mode)
- **Non-interactive mode:** fails closed on any confirm step
- **Capture:** optional stdout/stderr recording with timeout detection
- **Session vars:** steps can export `${step_name.stdout}` and `${step_name.exit_code}` for later steps

### CI/CD → Release Pipeline
**Flow:** lint (golangci-lint) → test → build → integration tests → GoReleaser on v* tags

**Note:** Lint job runs `--no-config`, so `.golangci-lint.yml` is never applied in CI (flagged AMBIGUOUS).

## Key Tensions / Open Questions

These edges are flagged AMBIGUOUS (low confidence) and worth documenting:

### Credentials in Plaintext
- **Policy:** `.kiro/steering/product.md` and `README.md` claim "Security First: never store credentials in plaintext"
- **Reality:** `GitHubConfig.Token` and `Webhook.Secret` are plaintext-storable in config.yaml (intentionally deprioritized per CLAUDE.md)
- **Status:** Known issue, left open by design; re-raise only if asked

### Interfaces Location
- **`.kiro/steering/tech.md`:** define interfaces where they're used (consumer-side)
- **`CLAUDE.md`:** interfaces sit next to their implementation
- **Current pattern:** most interfaces are defined next to implementation (e.g., `APIClient` in `pkg/github/interfaces.go`)

### Fzf Execution Model
- **AWS Auth spec:** fzf runs as a separate subprocess
- **CLAUDE.md:** fzf runs in-process via junegunn/fzf (temp file + stdio redirect)

## Main Subsystems (77 Communities, 8 thin)

### Configuration & State
- **Config Load & Validation** (46 nodes) — load YAML, validate GitHub/AWS sections, persist to disk
- **Config Merger** (4 nodes) — Override/Append/DeepMerge strategies for multi-repo defaults
- **Config Format Detection** (11 nodes) — detect single vs. multi, stream large files

### GitHub Operations
- **Multi-Repo Reconciler** (36 nodes) — batch detect/merge/plan/apply with rate limiting
- **GitHub Client Mutations** (9 nodes) — branch protection, collaborators, webhooks, retry logic
- **Plan Apply & Display** (42 nodes) — render changes as human-readable diffs; execute mutations
- **Live API Validator** (4 nodes) — verify collaborator/team existence before apply

### Runbook Execution
- **Runbook HTTP API** (45 nodes) — WebSocket server, session management, live log streaming
- **Runbook Step Executor** (13 nodes) — spawn subprocess, capture output, parse exit code
- **Runbook Parser** (8 nodes) — parse frontmatter and code fences; name steps
- **Runbook Templating** (9 nodes) — substitute `${var}` and step references
- **Runbook Output Capture** (11 nodes) — parse stdout/stderr trailers for variable extraction

### AWS Auth
- **Auth Manager** (28 nodes test coverage) — DefaultManager implements device-auth flow, browser opener, credential storage
- **Auth Error Classification** (12 nodes) — ClassifyError → typed *Error with troubleshooting steps

### Finders (Selection UI)
- **Fzf & Simple Finder** (29 nodes) — in-process fzf wrapper; fallback to raw terminal UI
- **Interactive Terminal Finder** (28 nodes) — arrow-key navigation in raw terminal mode

### Testing & Fixtures
- **Command Tests** (47 nodes) — auth, github, runbook, eks commands
- **CLI Integration Tests** (38 nodes) — end-to-end browser, config, validate flows
- **GitHub Client Tests** (33 nodes) — mock HTTP server, verify request/response shapes
- **Runbook API Tests** (34 nodes) — mock WebSocket, test session state and live updates

### Documentation & Spec
- **User Docs & Runbook Spec** (48 nodes) — quick start, commands, config reference, runbook YAML format guide
- **AWS Auth Design Spec** (10 nodes) — device flow, interface contracts
- **Multi-Repo Design Spec** (12 nodes) — detect, merge, reconcile, error handling
- **Runbook Examples** (7 nodes) — smoke-test fixture, extra checks, danger patterns

## Code Organization Rules

From `CLAUDE.md`:

- **Files ≤ 500 lines:** split pattern used in `pkg/github/errors.go` → `{errors,retry,multi_errors}.go`
- **Thin wrappers:** Commands in `internal/cmd/` delegate to `pkg/` and `internal/auth` logic
- **Shared plan display:** Use `displayRepositoryPlanChanges()` for both single-repo and multi-repo output; don't duplicate
- **Interfaces next to implementation:** `APIClient` lives in `pkg/github/interfaces.go` alongside `Client`
- **Error wrapping:** Use `fmt.Errorf("context: %w", err)` and structured `*Error` types with `Unwrap()` in auth and github packages
- **Boolean pointers for defaults:** `BranchProtectionRule.EnforceAdmins` is `*bool` so YAML can distinguish "unset" (defaults enforced) from explicit `false`

## How to Navigate

1. **Start with a god node** (RepositoryConfig, MultiReconciler) and explore its 25-49 edges
2. **Follow a hyperedge** to understand end-to-end flows (e.g., detect→merge→plan→apply)
3. **Check ambiguous edges** for policy/implementation gaps worth resolving
4. **Find your community** in the 77 subsystems above to locate code

Detailed interactive graph in `graphify-out/graph.html` (open in browser).
