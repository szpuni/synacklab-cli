package webui

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	rblog "synacklab/pkg/log"
)

// localOnly rejects requests whose Host is not the listener's own loopback
// address (DNS rebinding) and requests whose Origin is another site
// (cross-site fetches and WebSocket handshakes). A missing Origin means a
// same-origin GET or a non-browser client, both fine.
func localOnly(addr net.Addr, logger *rblog.Logger, next http.Handler) http.Handler {
	_, port, _ := net.SplitHostPort(addr.String())
	allowed := map[string]bool{
		addr.String():                       true,
		net.JoinHostPort("localhost", port): true,
		net.JoinHostPort("127.0.0.1", port): true,
		net.JoinHostPort("::1", port):       true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Host] {
			logger.Warn("rejected %s %s: Host %q is not this local server", r.Method, r.URL.Path, r.Host)
			http.Error(w, "forbidden: serve UIs are local-only", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !allowedOrigin(allowed, origin) {
			logger.Warn("rejected %s %s: Origin %q is another site", r.Method, r.URL.Path, origin)
			http.Error(w, "forbidden: cross-site request", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedOrigin reports whether origin is plain http (the only scheme a local
// serve UI uses) to an allowed host.
func allowedOrigin(allowed map[string]bool, origin string) bool {
	host, ok := strings.CutPrefix(origin, "http://")
	return ok && allowed[host]
}

// logRequests logs every request at Info, so a running serve process's own
// console shows activity the browser causes.
func logRequests(logger *rblog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying ResponseWriter, since embedding only the
// http.ResponseWriter interface would otherwise hide it — required for the
// WebSocket upgrade, which hijacks the connection.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hj.Hijack()
}
