# Runbook Server

Turn a Markdown file into an interactive, human-in-the-loop runbook. `serve`
starts a local web server that parses a document's fenced `bash`/`python`
code blocks into individually runnable steps and lets you execute them one
at a time from a browser, in order, carrying output from one step into a
later one.

This is not an automation/agent runner: nothing executes without an
explicit click, and no process outlives its own execution — the server
holds only session state (captured variables, working directory, execution
history) between requests, never a live shell.

## Console logging

Both `serve` and `run` log to the console: every HTTP request, each step's
start/finish (with duration), non-zero exit codes and timeouts, and
validation/open failures. Verbosity is controlled by `log_level` in
`~/.synacklab/config.yaml` — `error`, `warn`, or `info` (default `info`,
showing all three):

```yaml
log_level: warn
```

An unset or invalid value falls back to (or, for an invalid string, errors
out naming) `info`. There's no per-invocation flag — this is a global
setting alongside the rest of synacklab's config.

## Commands

`serve`, `run`, and `fmt` are subcommands of `synacklab runbook` (see
`synacklab runbook --help`). Each takes an optional `path`, which can be:

- a runbook file — used directly;
- a directory — resolved to `RUNBOOK.md` or `runbook.md` inside it, falling
  back to that directory's only `*.md` file if neither exists;
- omitted entirely — resolved the same way, against the current directory.

`run` and `fmt` need one specific file up front (there's no interactive way
for them to ask), so a directory with no `RUNBOOK.md`/`runbook.md` and
either zero or more than one `*.md` file is a clear, actionable error rather
than a guess.

`serve` never hard-fails on that ambiguity — see below.

### `synacklab runbook serve [path] [--port 4747] [--bind 127.0.0.1] [--cwd path]`

Starts the interactive web server. `path` may be a file, a directory, or
omitted (defaults to the current directory) — either way, the browser shows
a file-browser sidebar listing every `*.md` file under that directory
(recursively; other file types don't appear and can't be opened). If `path`
is a file, or the directory unambiguously resolves to one via the
`RUNBOOK.md`/`runbook.md`/single-file convention, that document opens
automatically; otherwise pick one from the sidebar. You can switch to a
different file in the sidebar at any time — that resets the session (vars,
cwd, history) to the newly opened document.

Binds to `127.0.0.1` by default; passing `--bind` with any non-loopback
address prints an explicit warning before starting, since v1 has no
authentication — anyone who can reach the port can execute arbitrary
commands as the local user. `--cwd` overrides the session's initial working
directory (default: each opened file's own directory).

### `synacklab runbook run [path] --non-interactive [--set VAR=value ...]`

Executes every step in the document top to bottom, using the same parser
and execution engine as `serve`, for use in CI. `--non-interactive` is
required — there is no interactive prompt fallback. `--set VAR=value`
(repeatable) supplies a value for each step's declared `input=`. A step
that requires confirmation (`confirm=true` or a `danger_patterns` match)
fails the run closed, since there's no way to confirm non-interactively;
run it via `serve` instead. The first step to fail or time out stops the
remaining steps and exits non-zero.

### `synacklab runbook fmt [path]`

Rewrites each runnable fence's attribute string into a canonical key order
and spacing, without altering prose, code content, or attribute values. On
a parse error the file is left untouched.

## Runbook Markdown format

### Fence attributes

Attributes go in curly braces on the fence's info string, space-separated,
`key=value`:

    ```bash {name=get_ip, capture=PUBLIC_IP, confirm=false, timeout=30s}
    export PUBLIC_IP=$(curl -s ifconfig.me)
    echo "$PUBLIC_IP"
    ```

| Attribute | Values | Meaning |
|---|---|---|
| `name` | string, unique in doc | Id for referencing this step's output. Auto-assigned (`step-1`, `step-2`, …) if omitted. |
| `input` | comma-separated var names | UI renders a form field per name before enabling Run; values injected into the process env. |
| `capture` | comma-separated var names | After execution, the server reads these out of the block's environment and stores them. |
| `confirm` | `true` / `false` | Force an explicit confirm click before running, even if inputs are filled. Default `false`. |
| `cwd` | path | Working directory for this step only. Relative paths resolve against the session cwd. |
| `set_cwd` | `true` / `false` | After execution, capture the shell's cwd and update the session's cwd. Default `false`. |
| `timeout` | duration, e.g. `30s`, `5m` | Max execution time. Default from document frontmatter or the server default (120s). |

To make a value capturable, the step must explicitly assign it as an
environment variable (bash `export`, or write it to `os.environ` in
python) — the same convention either language uses.

### Document-level frontmatter (optional)

```yaml
---
default_timeout: 60s
danger_patterns:
  - "rm -rf"
  - "terraform destroy"
---
```

A `danger_patterns` match forces confirmation on any step whose source
matches, regardless of that step's own `confirm=` value.

### Referencing a prior step's output

Inside a step's source, `{{steps.<name>.stdout}}`, `{{steps.<name>.exit_code}}`,
and `{{vars.<name>}}` are substituted **literally as text** before the
script is written to disk/spawned.

**This is plain string substitution, not shell-aware escaping.** If a prior
step's output is attacker-influenced or otherwise unpredictable (for
example, content from an external API), substituting it directly into a
later bash command is a command-injection risk. Prefer `input=`/`capture=`
(environment-variable based, no text substitution) as the default —
reach for `{{...}}` templating only when you've deliberately decided the
value is safe to inline.

## Example

See [`examples/runbook-demo.md`](../examples/runbook-demo.md) for a
runnable example covering `input=`, `capture=`, `set_cwd=`, `confirm=`,
and `danger_patterns`.

```bash
synacklab runbook serve examples/runbook-demo.md
```

Its first three steps (`greet`, `get_ip`, `make_workdir`) also run
non-interactively:

```bash
synacklab runbook run examples/runbook-demo.md --non-interactive --set NAME=world
```

That command exits non-zero at the fourth step (`confirm_before_running`) —
by design: `confirm=true` and `danger_patterns` steps always fail closed in
non-interactive mode (Requirement 11.3), so the run stops there rather than
silently skipping the confirmation. The last two steps are meant to be run
interactively via `serve`.
