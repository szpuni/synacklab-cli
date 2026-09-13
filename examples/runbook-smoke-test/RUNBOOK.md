# Runbook Smoke Test

A minimal runbook for manually checking that `synacklab runbook serve`
works end to end: bash execution, python execution, and passing output
from one step into another.

Start it with:

```
synacklab runbook serve examples/runbook-smoke-test
```

This directory has more than one `*.md` file, so the file browser sidebar
will show both — this one is named `RUNBOOK.md` so it opens automatically.

## 1. Bash works

Click Run. You should see `bash is working` stream into the output panel
and the step finish with exit code 0.

```bash {name=bash_hello}
echo "bash is working"
```

## 2. Python works

Same check, via `python3`.

```python {name=python_hello}
print("python is working")
```

## 3. Chaining: use one step's output in another

`generate_number` picks a random number and **captures** it into session
state (`capture=NUMBER`). `square_it` **declares** `input=NUMBER`, so the
UI renders a field for it before Run is enabled — fill it in with the
value `generate_number` printed (or whatever `GET /api/session` reports
after running it), and it gets injected as an environment variable for
this step, in a different language than the one that produced it.

```bash {name=generate_number, capture=NUMBER}
export NUMBER=$((RANDOM % 100))
echo "Generated: $NUMBER"
```

```python {name=square_it, input=NUMBER}
import os

n = int(os.environ["NUMBER"])
print(f"{n} squared is {n * n}")
```

## 4. Bonus: capture + set_cwd together

Exercises `set_cwd=true`: after this runs, the session's working directory
moves to a scratch folder it creates, and every step run afterwards uses
that as its `Dir` unless it declares its own `cwd=`.

```bash {name=make_scratch_dir, set_cwd=true}
mkdir -p smoke-test-scratch
cd smoke-test-scratch
echo "now working in $(pwd)"
```
