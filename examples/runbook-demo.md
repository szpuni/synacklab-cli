---
default_timeout: 30s
danger_patterns:
  - "rm -rf"
---

# Runbook Demo

This document demonstrates the runbook server's attributes: `input=`,
`capture=`, `set_cwd=`, `confirm=`, and a document-level `danger_patterns`
match. Run it with:

```
synacklab serve examples/runbook-demo.md
```

The first three steps also run non-interactively:

```
synacklab run examples/runbook-demo.md --non-interactive --set NAME=world
```

That command exits non-zero at `confirm_before_running` — non-interactive
mode always fails closed on a step requiring confirmation rather than
skip it, so the last two steps only run via `serve`.

## Step 1: greet

Takes a `NAME` input and prints a greeting.

```bash {name=greet, input=NAME}
echo "hello, $NAME"
```

## Step 2: get_ip

Captures a value into session state — in a real runbook this might be
`curl -s ifconfig.me`; here it's a static value so the demo doesn't depend
on network access.

```bash {name=get_ip, capture=PUBLIC_IP}
export PUBLIC_IP=203.0.113.42
echo "captured $PUBLIC_IP"
```

## Step 3: make_workdir

Creates and moves into a scratch directory, then updates the session's
working directory for every step that follows.

```bash {name=make_workdir, set_cwd=true}
mkdir -p runbook-demo-scratch
cd runbook-demo-scratch
```

## Step 4: confirm_before_running

`confirm=true` forces an explicit confirmation click even though nothing
here matches a danger pattern.

```bash {name=confirm_before_running, confirm=true}
echo "you had to confirm to run this"
```

## Step 5: dangerous_cleanup

This step's source matches the document's `danger_patterns` (`rm -rf`), so
it requires confirmation regardless of its own `confirm=` value — it's
deliberately left at the default (`false`) to demonstrate that the
document-level pattern still forces it.

```bash {name=dangerous_cleanup}
rm -rf runbook-demo-scratch
```
