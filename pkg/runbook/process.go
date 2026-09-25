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

// process is one script to run in its own interpreter process.
type process struct {
	lang    string
	script  string
	dir     string
	env     []string
	timeout time.Duration
}

// startProcess spawns exactly one bash/python3 process for p (no process is
// kept alive between calls — Requirement 3.2) and streams every output line
// as an EventStdout/EventStderr, followed by one EventDone carrying
// ExitCode, Duration, TimedOut and Canceled. The returned CancelFunc kills
// the process group the same way a timeout does.
func startProcess(ctx context.Context, p process) (<-chan Event, context.CancelFunc, error) {
	binary, ok := interpreterFor(p.lang)
	if !ok {
		return nil, nil, &Error{Type: ErrorTypeExecution, Message: fmt.Sprintf("unsupported language %q", p.lang)}
	}

	runCtx, cancel := context.WithTimeout(ctx, p.timeout)

	cmd := exec.CommandContext(runCtx, binary, "-c", p.script)
	cmd.Dir = p.dir
	cmd.Env = p.env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, &Error{Type: ErrorTypeExecution, Message: "failed to open stdout pipe", Cause: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, nil, &Error{Type: ErrorTypeExecution, Message: "failed to open stderr pipe", Cause: err}
	}

	started := time.Now()
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, &Error{Type: ErrorTypeExecution, Message: "failed to start process", Cause: err}
	}

	events := make(chan Event, 32)
	go func() {
		defer cancel()
		defer close(events)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			streamLines(stdoutPipe, EventStdout, events)
		}()
		go func() {
			defer wg.Done()
			streamLines(stderrPipe, EventStderr, events)
		}()
		wg.Wait()

		waitErr := cmd.Wait()
		timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
		events <- Event{
			Type:     EventDone,
			ExitCode: exitCodeFrom(waitErr),
			Duration: time.Since(started),
			TimedOut: timedOut,
			Canceled: !timedOut && errors.Is(runCtx.Err(), context.Canceled),
		}
	}()

	return events, cancel, nil
}

func streamLines(r io.Reader, typ EventType, events chan<- Event) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		events <- Event{Type: typ, Data: scanner.Text()}
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
