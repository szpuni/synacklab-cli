package cmd

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rblog "synacklab/pkg/log"
)

func TestServeHTTP_ShutsDownGracefullyWhenContextCanceled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- serveHTTP(ctx, ln, http.NewServeMux(), rblog.Discard())
	}()

	// Give the server a moment to actually start accepting.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err)
		assert.Less(t, time.Since(start), 2*time.Second)
	case <-time.After(3 * time.Second):
		t.Fatal("serveHTTP did not return after context cancellation")
	}
}

func TestServeHTTP_ReturnsNilWhenNeverCanceled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = serveHTTP(ctx, ln, http.NewServeMux(), rblog.Discard())
		close(done)
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + "/nonexistent")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	select {
	case <-done:
		t.Fatal("serveHTTP returned before its context was canceled")
	default:
	}
}
