package runbook

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
	srv, _ := newTestServer(t, "```bash {name=slow}\nsleep 5\n```\n")
	ctx, cancel := context.WithCancel(context.Background())
	srv.SetBaseContext(ctx)

	req := httptest.NewRequest(http.MethodPost, "/api/steps/slow/run", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	time.Sleep(100 * time.Millisecond)
	start := time.Now()
	cancel()

	waitForNoRunningExecutions(t, srv)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 4*time.Second, "canceling the server's base context should kill the running step promptly, not wait out its own sleep")

	out, err := exec.Command("ps", "aux").Output()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "sleep 5", "no orphaned process should remain after the base context is canceled")
}

func TestServer_DefaultBaseContextIsBackground(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	assert.NoError(t, srv.baseCtx.Err())
}
