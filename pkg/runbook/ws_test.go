package runbook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startRun(t *testing.T, ts *httptest.Server, stepName string) string {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/steps/"+stepName+"/run", "application/json", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var got struct {
		ExecutionID string `json:"execution_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	return got.ExecutionID
}

func readAllWSEvents(t *testing.T, conn *websocket.Conn) []map[string]any {
	t.Helper()
	var events []map[string]any
	for {
		var ev map[string]any
		if err := conn.ReadJSON(&ev); err != nil {
			return events
		}
		events = append(events, ev)
		if ev["type"] == "done" {
			return events
		}
	}
}

func TestHandleWSExecution_StreamsEventsInOrder(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	execID := startRun(t, ts, "hello")

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/executions/" + execID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	events := readAllWSEvents(t, conn)
	require.NotEmpty(t, events)
	assert.Equal(t, "stdout", events[0]["type"])
	assert.Equal(t, "hi", events[0]["data"])
	last := events[len(events)-1]
	assert.Equal(t, "done", last["type"])
	assert.Equal(t, float64(0), last["exit_code"])
}

func TestHandleWSExecution_LateConnectReplaysBufferedEvents(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	execID := startRun(t, ts, "hello")
	lastEventOf(t, wsDial(t, ts, execID)) // wait until the execution has finished

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/executions/" + execID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	events := readAllWSEvents(t, conn)
	require.NotEmpty(t, events)
	assert.Equal(t, "stdout", events[0]["type"])
	assert.Equal(t, "done", events[len(events)-1]["type"])
}

func TestHandleWSExecution_UnknownIDReturns404(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/executions/does-not-exist"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
