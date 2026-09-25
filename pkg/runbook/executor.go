package runbook

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DefaultTimeout is used when neither a step nor the document declares one.
const DefaultTimeout = 120 * time.Second

// EffectiveTimeout resolves a step's timeout per Requirement 9.1's
// precedence: step timeout= -> document default_timeout -> DefaultTimeout.
func EffectiveTimeout(step *Step, docDefault time.Duration) time.Duration {
	if step.Timeout > 0 {
		return step.Timeout
	}
	if docDefault > 0 {
		return docDefault
	}
	return DefaultTimeout
}

// Engine runs a single Step in its own process and streams its output. The
// returned context.CancelFunc lets a caller stop this specific execution
// (its process group is killed the same way a timeout kills it) without
// waiting for the step's configured timeout to elapse.
type Engine interface {
	Run(ctx context.Context, step *Step, inputs map[string]string, timeout time.Duration, store SessionStore) (*Execution, <-chan Event, context.CancelFunc, error)
}

// ProcessEngine spawns exactly one bash/python3 process per Run call. No
// process is kept alive between calls (Requirement 3.2).
type ProcessEngine struct {
	logWriter LogWriter
}

// NewEngine creates an Engine. logWriter may be nil to skip disk logging
// (used by tests that don't care about that side effect); real callers
// should always pass one so every execution is logged (Requirement 4).
func NewEngine(logWriter LogWriter) Engine {
	return &ProcessEngine{logWriter: logWriter}
}

func (e *ProcessEngine) Run(ctx context.Context, step *Step, inputs map[string]string, timeout time.Duration, store SessionStore) (*Execution, <-chan Event, context.CancelFunc, error) {
	sess := store.Get()

	src, err := Substitute(step.Source, sess)
	if err != nil {
		return nil, nil, nil, err
	}
	wrapped := src + buildTrailer(step.Lang, step.Capture, step.SetCwd)

	binary, ok := interpreterFor(step.Lang)
	if !ok {
		return nil, nil, nil, &Error{Type: ErrorTypeExecution, Message: fmt.Sprintf("unsupported language %q", step.Lang)}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)

	cmd := exec.CommandContext(runCtx, binary, "-c", wrapped)
	cmd.Dir = resolveDir(sess.Cwd, step.Cwd)
	cmd.Env = mergeEnv(sess.Vars, inputs)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, &Error{Type: ErrorTypeExecution, Message: "failed to open stdout pipe", Cause: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, &Error{Type: ErrorTypeExecution, Message: "failed to open stderr pipe", Cause: err}
	}

	execution := &Execution{StepName: step.Name, Started: time.Now()}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, nil, &Error{Type: ErrorTypeExecution, Message: "failed to start process", Cause: err}
	}

	events := make(chan Event, 32)
	go e.wait(runCtx, cancel, cmd, stdoutPipe, stderrPipe, step, store, execution, events)

	return execution, events, cancel, nil
}

func (e *ProcessEngine) wait(
	runCtx context.Context,
	cancel context.CancelFunc,
	cmd *exec.Cmd,
	stdoutPipe, stderrPipe io.Reader,
	step *Step,
	store SessionStore,
	execution *Execution,
	events chan<- Event,
) {
	defer cancel()
	defer close(events)

	captured := newCaptureResult()
	var rawStdout, rawStderr strings.Builder

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamStdout(stdoutPipe, events, captured, &rawStdout)
	}()
	go func() {
		defer wg.Done()
		streamStderr(stderrPipe, events, &rawStderr)
	}()
	wg.Wait()

	waitErr := cmd.Wait()

	execution.Duration = time.Since(execution.Started)
	execution.TimedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
	execution.Canceled = !execution.TimedOut && errors.Is(runCtx.Err(), context.Canceled)
	execution.Stdout = rawStdout.String()
	execution.Stderr = rawStderr.String()
	execution.Captured = captured.vars
	execution.ExitCode = exitCodeFrom(waitErr)

	for name, value := range captured.vars {
		store.SetVar(step.Name+"."+name, value)
		store.SetVar(name, value)
	}
	if step.SetCwd && captured.cwdFound {
		store.SetCwd(captured.cwd)
	}

	if e.logWriter != nil {
		sess := store.Get()
		index := len(sess.History) + 1
		if path, werr := e.logWriter.Write(sess.DocPath, sess.ID, index, *execution); werr == nil {
			execution.LogPath = path
		}
	}
	store.AppendHistory(*execution)

	events <- Event{
		Type:     "done",
		ExitCode: execution.ExitCode,
		Duration: execution.Duration,
		TimedOut: execution.TimedOut,
		Canceled: execution.Canceled,
		Captured: captured.vars,
	}
}

func streamStdout(r io.Reader, events chan<- Event, captured *captureResult, rawStdout *strings.Builder) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if captured.consume(line) {
			continue
		}
		events <- Event{Type: "stdout", Data: line}
		rawStdout.WriteString(line)
		rawStdout.WriteString("\n")
	}
}

func streamStderr(r io.Reader, events chan<- Event, rawStderr *strings.Builder) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		events <- Event{Type: "stderr", Data: line}
		rawStderr.WriteString(line)
		rawStderr.WriteString("\n")
	}
}

func exitCodeFrom(waitErr error) int {
	if waitErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func interpreterFor(lang string) (string, bool) {
	switch lang {
	case "bash":
		return "bash", true
	case "python":
		return "python3", true
	default:
		return "", false
	}
}

func resolveDir(sessionCwd, stepCwd string) string {
	if stepCwd == "" {
		return sessionCwd
	}
	if filepath.IsAbs(stepCwd) {
		return stepCwd
	}
	return filepath.Join(sessionCwd, stepCwd)
}

// mergeEnv layers the host environment, then session vars, then declared
// input= values (highest precedence — Requirement 5.3) into a Cmd.Env slice.
func mergeEnv(sessionVars, inputs map[string]string) []string {
	merged := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			merged[k] = v
		}
	}
	for k, v := range sessionVars {
		merged[k] = v
	}
	for k, v := range inputs {
		merged[k] = v
	}

	env := make([]string, 0, len(merged))
	for k, v := range merged {
		env = append(env, k+"="+v)
	}
	return env
}
