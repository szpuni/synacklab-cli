package runbook

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandlePostStepRun_CancelingBaseContextKillsInFlightExecution is a
// regression test for orphaned step processes: previously handlePostStepRun
// always ran steps under context.Background(), so nothing about the
// Server's own lifecycle could ever stop an in-flight execution — if the
// process hosting the server died (e.g. via a signal the CLI now handles
// for graceful shutdown), any running step's subprocess was left as an
// orphan, since it runs in its own process group (Requirement 9.2's
// timeout-kill mechanism deliberately isolates it from the parent's group).
func TestHandlePostStepRun_CancelingBaseContextKillsInFlightExecution(t *testing.T) {
	srv := newTestServer(t, "```bash {name=slow}\nsleep 6\n```\n")
	ctx, cancel := context.WithCancel(context.Background())
	srv.SetBaseContext(ctx)
	ts := serve(t, srv)

	conn := wsDial(t, ts, startRun(t, ts, "slow"))
	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	cancel()

	done := lastEventOf(t, conn)
	assert.Equal(t, true, done["canceled"])
	assert.Less(t, time.Since(start), 4*time.Second, "canceling the server's base context should kill the running step promptly, not wait out its own sleep")

	out, err := exec.Command("ps", "aux").Output()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "sleep 6", "no orphaned process should remain after the base context is canceled")
}
