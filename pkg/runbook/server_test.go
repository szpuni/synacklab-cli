package runbook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutes_ServesEmbeddedFrontend(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<title>Runbook</title>")
}

func TestRoutes_ServesEmbeddedStaticAssets(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	for _, path := range []string{"/app.js", "/app.css"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "GET %s", path)
		assert.NotEmpty(t, rec.Body.String(), "GET %s should not be empty", path)
	}
}

func TestRoutes_FrontendReferencesRealEndpoints(t *testing.T) {
	srv, _ := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	js := rec.Body.String()
	for _, endpoint := range []string{"/api/doc", "/api/session", "/api/session/reset", "/api/steps/", "/ws/executions/"} {
		assert.True(t, strings.Contains(js, endpoint), "app.js should reference %s", endpoint)
	}
}
