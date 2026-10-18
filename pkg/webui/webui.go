// Package webui holds what every local `serve` UI shares: the embedded
// component kit served at /ui/ and the local-only HTTP plumbing.
package webui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	rblog "synacklab/pkg/log"
)

// ValidateBind fails unless bind is a loopback address: serve UIs are a
// convenience for the operator on their own machine, never a network service.
func ValidateBind(bind string) error {
	if bind == "localhost" {
		return nil
	}
	if ip := net.ParseIP(bind); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("--bind %s is not a loopback address: serve UIs are local-only (use 127.0.0.1, ::1 or localhost)", bind)
}

// Serve runs h on ln, accepting only requests addressed to ln's own loopback
// address and logging each one, until ctx is canceled; then it shuts down
// gracefully (bounded by a 5s timeout).
func Serve(ctx context.Context, ln net.Listener, h http.Handler, logger *rblog.Logger) error {
	httpServer := &http.Server{Handler: logRequests(logger, localOnly(ln.Addr(), logger, h))}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
