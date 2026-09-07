# synacklab CLI

Go CLI (module `synacklab`, ~29.5k LOC) for DevOps engineers: AWS SSO auth, EKS/kube
context management, and declarative GitHub repo management. Cobra-based.

## Architecture

```
cmd/synacklab/main.go        entrypoint -> internal/cmd.Execute()

internal/cmd/                thin Cobra command wrappers
  root.go, init.go, auth.go
  aws_config.go, aws_login.go, aws_sync.go
  eks_config.go, eks_ctx.go
  github_apply.go github_validate.go, github.go

internal/auth/                AWS SSO device-authorization flow
  manager.go   DefaultManager implements Manager (Authenticate, IsAuthenticated,
               GetStoredCredentials, ClearCredentials, ValidateSession).
               Polls ssooidc.CreateToken, exp. backoff 5s->30s cap, 10min timeout.
               Caches creds at ~/.synacklab/aws_credentials.json (0600, dir 0700).
  browser.go   BrowserOpener interface; shells out to open/xdg-open/cmd per OS.
  errors.go    ClassifyError -> typed *Error (network/AWS/filesystem/config) with
               troubleshooting steps and IsRetryable().

pkg/config/                   Config{AWS, GitHub}, YAML at ~/.synacklab/config.yaml
  config.go    LoadConfig/SaveConfig, written 0600. NOTE: GitHubConfig.Token is still a
               plaintext-storable PAT field (see Known Issues — storage design, not fixed).

pkg/fuzzy/                    interactive selection, three tiers with fallback
  fzf.go         FzfFinder, wraps junegunn/fzf in-process (temp file + stdio redirect,
                 no subprocess/shell exec)
  interactive.go InteractiveFinderImpl, raw-terminal arrow-key UI
  fuzzy.go       Finder, numbered-list fallback
  Select() cascades fzf -> interactive -> simple on failure.

pkg/github/                   core engine: Terraform-like plan/apply for repo config
  client.go            Client implements APIClient (interfaces.go), thin google/go-github
                       wrapper, every method wrapped in WithRetry. buildProtectionRequest
                       resolves EnforceAdmins via BranchProtectionRule.EnforceAdminsEnabled()
                       (defaults true if unset) and always sends Restrictions (even empty)
                       so clearing RestrictPushes in config actually clears it on GitHub.
  auth.go              Manager.GetToken: GITHUB_TOKEN env first, then cfg.GitHub.Token;
                       ValidateToken checks `repo` scope via X-OAuth-Scopes header
  reconciler.go        Plan() diffs desired RepositoryConfig vs live state ->
                       ReconciliationPlan; Apply() executes, collects failures into
                       PartialFailureError (repo-change failure is the only fast-fail)
  multi_reconciler.go  same pattern fanned out across repos (978 lines)
  config.go            RepositoryConfig/BranchProtectionRule + hand-rolled Validate() tree
  multi_config.go      MultiRepositoryConfig, ConfigDetector, streaming load for large files
  merger.go / merger_fields.go   DefaultConfigMerger (Override/Append/DeepMerge), split from
                       multi_config.go: merger.go = MergeDefaults/deep-copy core,
                       merger_fields.go = per-field merge/copy helpers (topics, branch
                       rules, collaborators, teams, webhooks) + isZeroValue
  validator.go         live GitHub API checks (collaborator/team existence, admin perm),
                       separate from static Validate() — confirm whether `apply` should
                       run this before mutating state (currently only github_validate.go
                       clearly does)
  errors.go            ErrorType consts, Error, NewGitHubError/WrapGitHubError/
                       parseGitHubAPIError, ValidationError(s), PartialFailureError
  retry.go             RetryConfig, DefaultRetryConfig, WithRetry (split from errors.go)
  multi_errors.go      MultiRepoError + ActionableGuidance + generateActionableGuidance +
                       IsAuthenticationError/ShouldFastFail (split from errors.go)
```

## Conventions

- Error wrapping: `fmt.Errorf("context: %w", err)`, plus structured `*Error` types with
  `Unwrap()` in `internal/auth` and `pkg/github`.
- Interfaces live next to their implementation (e.g. `APIClient` in
  `pkg/github/interfaces.go` alongside `Client`), not consumer-side.
- Commands in `internal/cmd/` should be thin wrappers delegating to `pkg/`/`internal/`
  logic. `github_apply.go`'s single- and multi-repo plan display now share one
  implementation (`displayPlan` calls `displayRepositoryPlanChanges`) — don't reintroduce
  a second copy when adding new change types; add to `displayRepositoryPlanChanges` only.
- Files should stay under 500 lines (enforced during the 2026-09-07 review pass — see
  `pkg/github/errors.go`/`retry.go`/`multi_errors.go` and `merger.go`/`merger_fields.go`
  for the split pattern used when a file grows past it).
- `.kiro/steering/*.md` and `.kiro/specs/*/design.md` hold this project's own spec-driven
  design docs — check them before assuming intent on auth/config/GitHub features. Note:
  this project's maintainer has explicitly deprioritized aligning GitHub token/webhook
  secret storage with the steering doc's "no plaintext credentials" policy (see below) —
  don't re-raise that specific redesign unless asked.
- `BranchProtectionRule.EnforceAdmins` is `*bool` (not `bool`) so YAML can distinguish
  "unset" (defaults to enforced, via `EnforceAdminsEnabled()`) from explicit
  `enforce_admins: false`. Follow the same pattern for any other branch-protection flag
  where the old hardcoded default must stay the default.

## Known issues

Fixed 2026-09-07 (see [[synacklab-review-2026-09]] memory for the original findings):
- `pkg/config/config.go` — config.yaml now written `0600` (was `0644`, world-readable).
- `pkg/github/client.go` `buildProtectionRequest` — `EnforceAdmins` is now driven by
  `BranchProtectionRule.EnforceAdmins` (defaults true if unset) instead of hardcoded
  `true`; threaded through `types.go`, `reconciler.go` (diffing + apply), and the CLI
  plan display (`internal/cmd/github_apply.go`).
- `pkg/github/client.go` push restrictions — `Restrictions` is now always sent (even
  empty), so removing all entries from `restrict_pushes` in config actually clears it on
  GitHub instead of being silently ignored.
- `internal/auth/manager.go` `IsAuthenticated` — now also surfaces `ErrorTypeCredentialsAccess`
  (not just `ErrorTypePermissionDenied`), so real filesystem errors (disk I/O, etc.)
  reading the credentials file are no longer swallowed into "not authenticated".
- Modularity: `pkg/github/errors.go` (807 lines) split into `errors.go`/`retry.go`/
  `multi_errors.go`; `pkg/github/multi_config.go` (919 lines) split into `multi_config.go`/
  `merger.go`/`merger_fields.go`; `internal/cmd/github_apply.go`'s ~145-line duplicated
  plan-display block removed (single call into the multi-repo version).

Still open (deliberately not fixed — storage-design questions, not bugs):
- `pkg/github/types.go` — `GitHubConfig.Token` (`pkg/config/config.go`) and
  `Webhook.Secret` remain plaintext-storable fields (config file permission fixed above,
  but the values themselves aren't encrypted, and `Webhook.Secret` can end up committed
  in multi-repo YAML configs). Redesigning this (env-var interpolation, external secrets
  ref, or dropping file-based token storage) was explicitly out of scope for the
  2026-09-07 fix pass — raise it again only if asked.
- `internal/auth/browser.go` Windows branch (`cmd /c start <url>`) — low-risk, no
  scheme/host check before exec on an AWS-supplied URL.
- `pkg/github/client.go` push restrictions still only support user logins, not teams —
  no config field exists for team-based restrictions; would need a new
  `RestrictPushTeams` field threaded through config/client/reconciler if ever needed.

## Testing

Good parity: 30 `_test.go` files vs 31 source files across the reviewed packages
(`internal/auth`, `pkg/fuzzy`, `pkg/github`). `go.mod` uses testify. golangci-lint v2
config (`.golangci.yml`) enables depguard, dupl (threshold 400), gomodguard, govet,
ineffassign, misspell, nakedret, revive, staticcheck, thelper, unused, usestdlibvars,
usetesting — run `golangci-lint run` before considering a change done, not just `go test`.

```
make build   # -> ./bin/synacklab
go test ./...
golangci-lint run
```
