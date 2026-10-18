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
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<title>Runbook</title>")
}

func TestRoutes_ServesEmbeddedStaticAssets(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	for _, path := range []string{"/app.js", "/session.js", "/app.css", "/ui/components.js", "/ui/tokens.css", "/ui/components.css"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "GET %s", path)
		assert.NotEmpty(t, rec.Body.String(), "GET %s should not be empty", path)
	}
}

func TestRoutes_FrontendReferencesRealEndpoints(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	js := rec.Body.String()
	for _, endpoint := range []string{"/api/doc", "/api/session", "/api/session/reset", "/api/steps/", "/ws/executions/"} {
		assert.True(t, strings.Contains(js, endpoint), "app.js should reference %s", endpoint)
	}
}

func TestRoutes_PageLoadsTheSharedKit(t *testing.T) {
	srv := newTestServer(t, "```bash {name=hello}\necho hi\n```\n")

	get := func(path string) string {
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Body.String()
	}
	html := get("/")
	assert.Contains(t, html, `href="/ui/tokens.css"`)
	assert.Contains(t, html, `href="/ui/components.css"`)
	assert.Contains(t, html, `<script type="module" src="app.js">`)
	assert.Contains(t, get("/app.js"), `from "/ui/components.js"`)
	assert.NotContains(t, get("/app.css"), ":root", "tokens live in /ui/tokens.css")
}
