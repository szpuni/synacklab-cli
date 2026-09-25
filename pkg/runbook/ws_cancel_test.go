package runbook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCancelExecution_RunningExecutionReturns202AndCancelsIt(t *testing.T) {
	srv := newTestServer(t, "```bash {name=slow}\nsleep 30\n```\n")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	execID := startRun(t, ts, "slow")

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/executions/" + execID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/executions/"+execID+"/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	events := readAllWSEvents(t, conn)
	require.NotEmpty(t, events)
	last := events[len(events)-1]
	assert.Equal(t, "done", last["type"])
	assert.Equal(t, true, last["canceled"])
	assert.NotEqual(t, true, last["timed_out"])
}

func TestHandleCancelExecution_AlreadyFinishedExecutionIsNoOp(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	execID := startRun(t, ts, "hello")
	lastEventOf(t, wsDial(t, ts, execID)) // wait until the execution has finished

	req := httptest.NewRequest(http.MethodPost, "/api/executions/"+execID+"/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleCancelExecution_UnknownIDReturns404(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodPost, "/api/executions/does-not-exist/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
