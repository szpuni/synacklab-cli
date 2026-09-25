package runbook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_InputCaptureChainingAcrossTwoSteps drives a full document
// through the real HTTP+WS API (as a browser client would): run step one,
// read its captured output back over the WebSocket, feed that value in as
// the second step's declared input=, and confirm the session accumulates
// both executions in order (Requirements 3.1-3.5, 6.4).
func TestIntegration_InputCaptureChainingAcrossTwoSteps(t *testing.T) {
	src := "# Deploy\n\nFetch then greet.\n\n" +
		"```bash {name=get_ip, capture=PUBLIC_IP}\nexport PUBLIC_IP=203.0.113.42\n```\n\n" +
		"```bash {name=greet, input=PUBLIC_IP}\necho \"hello $PUBLIC_IP\"\n```\n"

	srv := newTestServer(t, src)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	firstID := startRun(t, ts, "get_ip")
	firstDone := lastEventOf(t, wsDial(t, ts, firstID))
	require.Equal(t, "done", firstDone["type"])
	captured := firstDone["captured"].(map[string]any)
	require.Equal(t, "203.0.113.42", captured["PUBLIC_IP"])

	body, _ := json.Marshal(map[string]any{"inputs": map[string]string{"PUBLIC_IP": captured["PUBLIC_IP"].(string)}})
	resp, err := http.Post(ts.URL+"/api/steps/greet/run", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var runResp struct {
		ExecutionID string `json:"execution_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&runResp))

	events := readAllWSEvents(t, wsDial(t, ts, runResp.ExecutionID))
	var stdout []string
	for _, ev := range events {
		if ev["type"] == "stdout" {
			stdout = append(stdout, ev["data"].(string))
		}
	}
	assert.Equal(t, []string{"hello 203.0.113.42"}, stdout)

	sessResp, err := http.Get(ts.URL + "/api/session")
	require.NoError(t, err)
	defer sessResp.Body.Close()
	var session struct {
		History []struct {
			StepName string `json:"step_name"`
		} `json:"history"`
	}
	require.NoError(t, json.NewDecoder(sessResp.Body).Decode(&session))
	require.Len(t, session.History, 2)
	assert.Equal(t, "get_ip", session.History[0].StepName)
	assert.Equal(t, "greet", session.History[1].StepName)
}

// TestIntegration_TimeoutViaAPILeavesNoOrphanProcess exercises the timeout
// path through the same async goroutine the real API handler uses (not a
// direct Session.Start call), confirming the process group is actually gone
// afterward (Requirements 9.1, 9.2).
func TestIntegration_TimeoutViaAPILeavesNoOrphanProcess(t *testing.T) {
	src := "```bash {name=slow, timeout=200ms}\nsleep 5\n```\n"
	srv := newTestServer(t, src)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	start := time.Now()
	execID := startRun(t, ts, "slow")
	done := lastEventOf(t, wsDial(t, ts, execID))
	elapsed := time.Since(start)

	assert.Equal(t, true, done["timed_out"])
	assert.Less(t, elapsed, 4*time.Second)

	out, err := exec.Command("ps", "aux").Output()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "sleep 5", "no orphaned sleep process should remain after a timeout")
}

func wsDial(t *testing.T, ts *httptest.Server, execID string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/executions/" + execID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	return conn
}

func lastEventOf(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	defer conn.Close()
	events := readAllWSEvents(t, conn)
	require.NotEmpty(t, events)
	return events[len(events)-1]
}
