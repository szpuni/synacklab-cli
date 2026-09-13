# Runbook Server — Solution Specification

Status: Draft v1
Target branch: `runbook-server`
Component: `synaclab serve`

## 1. Overview

Add a feature to synacklab-cli that turns a Markdown file into an interactive,
human-in-the-loop runbook. A user runs `synaclab serve <file.md>`, which starts
a local Go web server. The server parses the Markdown, serves a small
single-page frontend, and lets the user execute individual `bash` or `python`
code blocks one at a time from the browser, in order, with the ability to feed
the output of an earlier block into a later one.

This is explicitly **not** an automation/agent runner. Nothing executes without
an explicit click. The server's job is to parse structure, execute on request,
and carry state between requests — not to decide what runs when.

## 2. Goals

- G1: Any Markdown file with fenced `bash`/`python` code blocks becomes
  executable via a local web UI, with zero required changes to the file for
  basic use.
- G2: Users can pass the output of one block into a later block, either by
  copy/paste (manual) or via an explicit, opt-in template reference
  (semi-automatic).
- G3: No process outlives a single block's execution. The server holds only
  data (session state), never a live shell.
- G4: Every execution is logged to disk (stdout, stderr, exit code, timing)
  regardless of whether its output was captured into a variable.
- G5: Destructive-looking commands require an explicit confirmation step in
  the UI, on by default.
- G6: AI-generated runbooks are a first-class input. A user should be able to
  paste an LLM-authored Markdown file and safely walk through it step by step.

## 3. Non-Goals (v1)

- No persistent shell / long-lived process per session.
- No multi-user auth, multi-tenant sessions, or remote/VPC execution.
- No sandboxing or containerization of executed commands (runs as the local
  user, on the local machine).
- No languages beyond `bash` and `python` (python3 via `python3` binary).
- No session persistence across a `serve` restart (in-memory only).
- No automatic `cd` tracking — working directory changes must be explicit
  (see 7.5).

## 4. Terminology

- **Document**: the Markdown file passed to `serve`.
- **Step**: a fenced code block with a runnable language (`bash` or
  `python`) and its parsed attributes.
- **Session**: server-side state for one open document: captured variables,
  current working directory, execution history. One session per `serve`
  process in v1.
- **Execution**: a single run of one step. Produces stdout, stderr, exit
  code, duration, and any captured variables.

## 5. Runbook Markdown Format

### 5.1 Fence attributes

Attributes go in curly braces on the fence's info string, space-separated,
`key=value`:

    ```bash {name=get_ip, capture=PUBLIC_IP, confirm=false, timeout=30s}
    export PUBLIC_IP=$(curl -s ifconfig.me)
    echo "$PUBLIC_IP"
    ```

    ```python {name=parse_ip, input=PUBLIC_IP}
    import os
    ip = os.environ["PUBLIC_IP"]
    print(ip.split("."))
    ```

| Attribute   | Values                      | Meaning                                                                                   |
|-------------|------------------------------|--------------------------------------------------------------------------------------------|
| `name`      | string, unique in doc        | Id for referencing this step's output. Auto-assigned (`step-1`, `step-2`, …) if omitted.  |
| `input`     | comma-separated var names    | UI renders a form field per name before enabling Run; values injected into the process env.|
| `capture`   | comma-separated var names    | After execution, server reads these out of the block's environment and stores them.       |
| `confirm`   | `true` / `false`             | Force an explicit confirm click before running, even if inputs are filled. Default `false`.|
| `cwd`       | path                         | Working directory for this step only. Relative paths resolve against session cwd.          |
| `set_cwd`   | `true` / `false`             | After execution, capture `$PWD`/`os.getcwd()` and update the session's cwd. Default `false`.|
| `timeout`   | duration, e.g. `30s`, `5m`   | Max execution time. Default from document frontmatter or server default (120s).            |

### 5.2 Document-level frontmatter (optional)

```yaml
---
default_timeout: 60s
danger_patterns:
  - "rm -rf"
  - "terraform destroy"
  - "kubectl delete"
---
```

### 5.3 Referencing a prior step's output (opt-in templating)

Inside a step's source text, `{{steps.<name>.stdout}}`, `{{steps.<name>.exit_code}}`,
and `{{vars.<name>}}` are substituted **literally as text** by the server before
the script is written to disk/spawned — this is plain string substitution, not
shell-aware escaping. See Security (13) for why this is opt-in and its risk.

The safer default path is `input=` + `capture=`, which passes values through
the process environment instead of text substitution.

## 6. Architecture

```
                     ┌─────────────────────────┐
   browser  <──WS──> │   Go HTTP/WS server      │
   (SPA,             │  ┌────────────────────┐  │
    embedded         │  │ Markdown parser     │  │
    via go:embed)    │  │ (goldmark + custom  │  │
                      │  │  fence-attr hook)   │  │
                      │  └────────────────────┘  │
                      │  ┌────────────────────┐  │
                      │  │ Session store       │  │
                      │  │ (in-memory map,     │  │
                      │  │  mutex-guarded)     │  │
                      │  └────────────────────┘  │
                      │  ┌────────────────────┐  │
                      │  │ Execution engine    │  │
                      │  │ (spawns bash/python │  │
                      │  │  per request only)  │  │
                      │  └────────────────────┘  │
                      └─────────────────────────┘
                                  │
                         .synaclab/<session>/
                         step logs, session.json
```

### 6.1 Request flow for running a step

1. Frontend sends `POST /api/steps/{name}/run` with any `input=` values the
   user filled in, plus `confirmed: true` if the step required confirmation.
2. Server resolves the step's source: applies `{{...}}` template substitution
   if present, merges session vars + declared `input=` values into an env map.
3. Server wraps the script with a capture trailer (7.4) and spawns
   `exec.CommandContext(ctx, "bash", "-c", wrapped)` or
   `exec.CommandContext(ctx, "python3", "-c", wrapped)`, with `Dir` set from
   session cwd (or step's `cwd=` override) and `Env` set from the merged map.
4. Stdout/stderr are streamed to the frontend over the execution's WebSocket
   as they're produced.
5. On process exit, server parses and strips sentinel capture lines, updates
   session vars / cwd, writes the full log to disk, and sends a final
   `{exit_code, duration}` event.

## 7. Execution Engine

### 7.1 On-demand process model

One process per execution, full stop. No process is kept alive waiting for
the next step. This trades "implicit shell state" for "explicit, inspectable
state" (7.4), and avoids the memory/lifecycle problems of long-lived shells
(orphaned processes, session timeouts, multi-tab conflicts).

### 7.2 Bash executor

`bash -c "<user script>\n<capture trailer>"`. Single invocation, so the
trailer runs in the same process and sees whatever the user script exported.

### 7.3 Python executor

`python3 -c "<user script>\n<capture trailer>"`. Same principle: the trailer
is appended source, executed in the same interpreter process, and reads
`os.environ` (which the user script must have written to, for consistency
with bash's `export` convention — see 7.4).

### 7.4 Variable capture mechanism

For a step declaring `capture=A,B`, the server appends, for bash:

```bash
echo "__SYNACLAB_CAP__A=${A}"
echo "__SYNACLAB_CAP__B=${B}"
```

and for python:

```python
import os
print(f"__SYNACLAB_CAP__A={os.environ.get('A','')}")
print(f"__SYNACLAB_CAP__B={os.environ.get('B','')}")
```

The server scans stdout for `__SYNACLAB_CAP__` lines, removes them before
display/logging, and stores the values in the session's variable map keyed by
step name (e.g. `get_ip.PUBLIC_IP`) and also under a short alias if globally
unique.

**Convention**: to make a value capturable, the step must explicitly assign it
as an environment variable. This is identical in spirit to bash's `export`,
applied uniformly to python via `os.environ`.

### 7.5 Working directory handling

Session cwd defaults to the document's directory. A step's own `cd` commands
do **not** persist (fresh process each time) — this is a known, documented
limitation. To change the session's cwd going forward, a step must declare
`set_cwd=true`, which appends `echo "__SYNACLAB_CWD__$PWD"` (bash) or
`print(f"__SYNACLAB_CWD__{os.getcwd()}")` (python) to the trailer; the server
parses this the same way as variable capture and updates session cwd.

### 7.6 Timeouts

Every execution runs under `context.WithTimeout`. Default 120s, overridable
per-step (`timeout=`) or per-document (frontmatter `default_timeout`). On
timeout, the process group is killed and the step is marked `timed_out`.

## 8. Session & State Model

```go
type Session struct {
    ID       string
    DocPath  string
    Cwd      string
    Vars     map[string]string   // flat namespace, last-write-wins
    History  []Execution
        mu   sync.Mutex
}

type Execution struct {
    StepName string
    Started  time.Time
    Duration time.Duration
    ExitCode int
    LogPath  string
    Captured map[string]string
}
```

v1: one session per running `serve` process, in-memory only. Restarting
`serve` starts fresh. (Persisting `session.json` to resume later is listed
under Future Work, 16.)

## 9. API Specification

### 9.1 REST

- `GET /api/doc` — parsed document: ordered list of prose blocks and steps
  (name, language, attributes, source), plus current session var/cwd state.
- `POST /api/steps/{name}/run` — body `{inputs: {VAR: "value"}, confirmed: bool}`.
  Returns `{execution_id}` immediately; results stream over WebSocket.
- `GET /api/session` — current vars, cwd, execution history summary.
- `POST /api/session/reset` — clears vars and resets cwd to document default.

### 9.2 Streaming

`GET /ws/executions/{execution_id}` — WebSocket. Server pushes JSON events:
`{"type":"stdout","data":"..."}`, `{"type":"stderr","data":"..."}`,
`{"type":"done","exit_code":0,"duration_ms":842,"captured":{"PUBLIC_IP":"1.2.3.4"}}`.

## 10. CLI Commands

- `synaclab serve <file.md> [--port 4747] [--bind 127.0.0.1] [--cwd path]`
  — starts the interactive web server (this feature).
- `synaclab run <file.md> --non-interactive [--set VAR=value ...]`
  — headless execution of the same engine for CI use, reusing the parser and
  execution engine but driven top-to-bottom without a browser.
- `synaclab fmt <file.md>` — canonicalizes fence attribute formatting.

## 11. Storage Layout

```
.synaclab/
  <doc-slug>/
    <session-id>/
      session.json          # optional, if persistence is enabled
      steps/
        1-get_ip.log         # stdout+stderr+exit code+timing
        2-parse_ip.log
```

## 12. Security & Safety

- `serve` binds `127.0.0.1` by default. `--bind 0.0.0.0` requires the flag
  explicitly and prints a loud warning: anyone reaching the port can execute
  arbitrary commands as the local user. No auth in v1.
- `{{steps.x.stdout}}` template substitution is literal text substitution,
  **not shell-escaped**. If a prior command's output is attacker-influenced
  or unpredictable (e.g. content from an external API), substituting it
  directly into a later bash command is a command-injection risk. This is
  documented plainly in the templating docs; `input=`/`capture=` (env-var
  based) is the recommended default over inline templating.
- Document-level `danger_patterns` (frontmatter) force `confirm=true` on any
  step whose source matches, regardless of the step's own `confirm=` value.
- No sandboxing in v1 — commands run with the same privileges as the user
  running `synaclab serve`. This is a stated limitation, not a hidden one.

## 13. Functional Requirements

- FR-1: Parse fenced code blocks with language `bash` or `python`, extracting
  attributes per 5.1; non-bash/python fences render as static code, not runnable.
- FR-2: Serve a frontend that renders the document's prose and steps in
  document order, with a Run control on each runnable step.
- FR-3: Execute a step only on explicit user action (button click via API).
- FR-4: Stream stdout/stderr live during execution.
- FR-5: Persist a full log (stdout, stderr, exit code, duration) for every
  execution regardless of `capture=`.
- FR-6: Support `input=` — render form fields, inject values as env vars.
- FR-7: Support `capture=` — extract named env vars post-execution into
  session state, addressable by later steps.
- FR-8: Support `set_cwd=` — update session working directory post-execution.
- FR-9: Support `confirm=` and document-level `danger_patterns` overriding it.
- FR-10: Support per-step and per-document `timeout=`, killing the process
  group on expiry.
- FR-11: Support `{{steps.<name>.stdout|exit_code}}` and `{{vars.<name>}}`
  template substitution, applied before execution.
- FR-12: `synaclab run --non-interactive` executes the full document top to
  bottom using the same engine, for CI.
- FR-13: `synaclab fmt` rewrites a document's fence attributes into a
  canonical form without altering behavior.

## 14. Non-Functional Requirements

- NFR-1: Single self-contained Go binary; frontend assets embedded via
  `go:embed` — no separate frontend build/deploy step for end users.
- NFR-2: No process lives longer than its own execution; steady-state memory
  usage of `serve` with no execution in flight should be flat regardless of
  session length.
- NFR-3: Target platforms: macOS and Linux (bash and python3 present on
  PATH). Windows/WSL explicitly out of scope for v1.
- NFR-4: Execution start latency (click to first stdout byte) under ~200ms
  for a trivial command, excluding the command's own runtime.

## 15. V1 Scope Cut

In: single document, single session, bash + python, local-only,
in-memory session, manual confirm flow, disk logging.

Out: auth, multi-session/multi-user, containerized execution, session
persistence across restarts, remote/VPC execution, languages beyond
bash/python, automatic `cd` tracking.

## 16. Open Questions / Future Work

- Session persistence (`session.json`) to resume a runbook after restarting
  `serve`.
- Remote execution mode (server runs inside a VPC/bastion, browser stays
  local) — same pattern Runme's GitHub issue #616 explores.
- Auth story once `--bind 0.0.0.0` is a real use case (team-shared runbooks).
- Container/sandbox execution backend as an alternative to local-process
  execution, for untrusted or AI-generated runbooks.
- Additional languages beyond bash/python (this spec's env-var capture
  convention should generalize to any language that can read/write process
  environment variables).