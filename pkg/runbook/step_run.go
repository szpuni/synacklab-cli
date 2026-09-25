package runbook

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	rblog "synacklab/pkg/log"
)

// DefaultTimeout is used when neither a step nor the document declares one.
const DefaultTimeout = 120 * time.Second

// Start runs the named step in its own process and streams its output.
// It enforces every rule a run must pass before anything is spawned:
//   - the step exists (ErrorTypeNotFound);
//   - every input= name is present in inputs (ErrorTypeValidation) — only
//     declared inputs reach the step's environment;
//   - a step needing confirmation (confirm=true, or a danger_patterns
//     match) is only run with confirmed=true (ErrorTypeValidation);
//   - {{...}} template references resolve (ErrorTypeTemplate).
//
// The events channel carries the step's visible output lines, then one
// EventDone; by the time EventDone is delivered the execution has been
// committed to the session (vars, cwd, history, disk log). The CancelFunc
// kills the step's process group. ctx is the parent for the execution:
// canceling it kills the step too.
func (s *Session) Start(ctx context.Context, stepName string, inputs map[string]string, confirmed bool) (<-chan Event, context.CancelFunc, error) {
	step, ok := s.doc.Steps[stepName]
	if !ok {
		return nil, nil, &Error{Type: ErrorTypeNotFound, Message: fmt.Sprintf("unknown step %q", stepName)}
	}

	declared, err := declaredInputs(step, inputs)
	if err != nil {
		return nil, nil, err
	}
	if required, reason := requiresConfirmation(step, s.doc.Frontmatter.DangerPatterns); required && !confirmed {
		return nil, nil, &Error{Type: ErrorTypeValidation, Message: fmt.Sprintf("step %q requires confirmation: %s", step.Name, reason)}
	}

	state := s.State()
	src, err := Substitute(step.Source, state)
	if err != nil {
		return nil, nil, err
	}

	started := time.Now()
	raw, cancel, err := startProcess(ctx, process{
		lang:    step.Lang,
		script:  src + buildTrailer(step.Lang, step.Capture, step.SetCwd),
		dir:     resolveDir(state.Cwd, step.Cwd),
		env:     mergeEnv(state.Vars, declared),
		timeout: effectiveTimeout(step, s.doc.Frontmatter.DefaultTimeout),
	})
	if err != nil {
		return nil, nil, err
	}

	events := make(chan Event, 32)
	go s.relay(step, started, raw, events)
	return events, cancel, nil
}

// relay forwards a step's process events, keeping capture sentinel lines
// out of its visible output, and commits the execution before forwarding
// EventDone.
func (s *Session) relay(step *Step, started time.Time, raw <-chan Event, events chan<- Event) {
	defer close(events)

	captured := newCaptureResult()
	var stdout, stderr strings.Builder
	for ev := range raw {
		switch ev.Type {
		case EventStdout:
			if captured.consume(ev.Data) {
				continue
			}
			stdout.WriteString(ev.Data + "\n")
		case EventStderr:
			stderr.WriteString(ev.Data + "\n")
		case EventDone:
			ev.Captured = captured.vars
			s.record(step, Execution{
				StepName: step.Name,
				Started:  started,
				Duration: ev.Duration,
				ExitCode: ev.ExitCode,
				TimedOut: ev.TimedOut,
				Canceled: ev.Canceled,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Captured: captured.vars,
			}, captured)
		}
		events <- ev
	}
}

// RunAll runs every step in document order without confirmation, writing
// each output line to out and stopping at the first failure (Requirement
// 11.4). A step with an unmet input=, or one requiring confirmation, fails
// before any process is spawned for it (Requirements 11.2, 11.3).
func (s *Session) RunAll(ctx context.Context, inputs map[string]string, out io.Writer, logger *rblog.Logger) error {
	for _, block := range s.doc.Blocks {
		if block.Kind != BlockStep {
			continue
		}
		step := block.Step

		logger.Info("running step %q", step.Name)
		events, _, err := s.Start(ctx, step.Name, inputs, false)
		if err != nil {
			logger.Error("%s", err)
			return err
		}

		var done Event
		for ev := range events {
			if ev.Type == EventDone {
				done = ev
				continue
			}
			fmt.Fprintln(out, ev.Data)
		}

		if failure := done.failure(); failure != "" {
			logger.Warn("step %q %s", step.Name, failure)
			return &Error{Type: ErrorTypeExecution, Message: fmt.Sprintf("step %q %s", step.Name, failure)}
		}
		logger.Info("step %q finished (duration=%s)", step.Name, done.Duration)
	}
	return nil
}

// declaredInputs picks step's input= values out of supplied, failing if any
// is missing.
func declaredInputs(step *Step, supplied map[string]string) (map[string]string, error) {
	declared := make(map[string]string, len(step.Input))
	var missing []string
	for _, name := range step.Input {
		v, ok := supplied[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		declared[name] = v
	}
	if len(missing) > 0 {
		return nil, &Error{Type: ErrorTypeValidation, Message: fmt.Sprintf("step %q: missing required input(s): %s", step.Name, strings.Join(missing, ", "))}
	}
	return declared, nil
}

// requiresConfirmation reports whether running step needs an explicit
// confirmation, and why. A danger_patterns match forces confirmation
// regardless of the step's own confirm= value (Requirement 8.2).
func requiresConfirmation(step *Step, dangerPatterns []string) (required bool, reason string) {
	for _, p := range dangerPatterns {
		if strings.Contains(step.Source, p) {
			return true, fmt.Sprintf("matches danger pattern %q", p)
		}
	}
	if step.Confirm {
		return true, "step declares confirm=true"
	}
	return false, ""
}

// effectiveTimeout resolves a step's timeout per Requirement 9.1's
// precedence: step timeout= -> document default_timeout -> DefaultTimeout.
func effectiveTimeout(step *Step, docDefault time.Duration) time.Duration {
	if step.Timeout > 0 {
		return step.Timeout
	}
	if docDefault > 0 {
		return docDefault
	}
	return DefaultTimeout
}
