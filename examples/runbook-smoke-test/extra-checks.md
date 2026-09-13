# Extra Checks

A second file in the same directory, so you can also test switching
between files in the sidebar. Covers a few things `RUNBOOK.md` doesn't:
stderr, a non-zero exit code, and a step-level timeout.

## Stderr shows up separately from stdout

```bash {name=stderr_check}
echo "this is stdout"
echo "this is stderr" >&2
```

## Non-zero exit codes are reported, not treated as a crash

```bash {name=fails_on_purpose}
echo "about to fail"
exit 7
```

## Python failure looks the same way

```python {name=python_fails_on_purpose}
import sys

print("about to fail from python")
sys.exit(3)
```

## Timeout kills a step that runs too long

`timeout=2s` here is much shorter than the document default, so this
should be marked `timed_out` a couple of seconds after you click Run,
not after the full 5 seconds the script asks for.

```bash {name=slow_step, timeout=2s}
echo "sleeping too long..."
sleep 5
echo "you should never see this line"
```
