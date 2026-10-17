package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func startShutdownTestServer(t *testing.T, h http.Handler) (*http.Server, *lifecycle.Manager, string) {
	t.Helper()
	m := lifecycle.New()
	srv := &http.Server{Handler: m.Wrap(h), ReadHeaderTimeout: time.Second}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	served := make(chan struct{})
	go func() { defer close(served); _ = srv.Serve(ln) }()
	t.Cleanup(func() { m.ForceClose(); _ = srv.Close(); <-served })
	return srv, m, "http://" + ln.Addr().String()
}

func testShutdownOptions() shutdownOptions {
	return shutdownOptions{requestTimeout: time.Second, forceCloseTimeout: time.Second, cleanupTimeout: time.Second}
}

func TestShutdownDrainsSSEBeforeCleanup(t *testing.T) {
	release := make(chan struct{})
	srv, m, url := startShutdownTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: started\n\n")
		_ = http.NewResponseController(w).Flush()
		<-release
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	resp, err := http.Get(url + "/stream")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	opts := testShutdownOptions()
	opts.drainDelay = 100 * time.Millisecond
	cleaned := make(chan lifecycle.Snapshot, 1)
	done := make(chan error, 1)
	go func() {
		done <- shutdownServer(srv, func(context.Context) error { cleaned <- m.Snapshot(); return nil }, opts)
	}()
	require.Eventually(t, func() bool { return m.Snapshot().Draining }, time.Second, time.Millisecond)
	probe, err := http.Get(url + "/ready")
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, probe.StatusCode)
	_, _ = io.Copy(io.Discard, probe.Body)
	_ = probe.Body.Close()
	live, err := http.Get(url + "/health")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, live.StatusCode)
	_, _ = io.Copy(io.Discard, live.Body)
	_ = live.Body.Close()
	select {
	case <-cleaned:
		t.Fatal("cleanup raced the active stream")
	default:
	}
	close(release)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "[DONE]")
	require.NoError(t, <-done)
	require.Zero(t, (<-cleaned).ActiveHTTP)
}

func TestShutdownWaitsForWebSocketEvenAfterHTTPShutdownReturns(t *testing.T) {
	release := make(chan struct{})
	srv, m, url := startShutdownTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("started"))
		<-release
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("[DONE]"))
		_ = conn.Close(websocket.StatusNormalClosure, "complete")
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	_, _, err = conn.Read(ctx)
	require.NoError(t, err)
	cleaned := make(chan lifecycle.Snapshot, 1)
	done := make(chan error, 1)
	go func() {
		done <- shutdownServer(srv, func(context.Context) error { cleaned <- m.Snapshot(); return nil }, testShutdownOptions())
	}()
	require.Eventually(t, func() bool { return m.Snapshot().Draining }, time.Second, time.Millisecond)
	select {
	case <-cleaned:
		t.Fatal("cleanup ignored live WebSocket")
	default:
	}
	close(release)
	_, body, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, "[DONE]", string(body))
	_, _, err = conn.Read(ctx)
	require.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err))
	require.NoError(t, <-done)
	require.Zero(t, (<-cleaned).ActiveWebSockets)
}

func TestShutdownDeadlineCancelsBeforeCleanup(t *testing.T) {
	entered := make(chan struct{})
	canceled := make(chan error, 1)
	srv, m, url := startShutdownTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		canceled <- context.Cause(r.Context())
	}))
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered
	opts := testShutdownOptions()
	opts.requestTimeout = 20 * time.Millisecond
	var remaining int
	err := shutdownServer(srv, func(context.Context) error { remaining = m.Snapshot().ActiveHTTP; return nil }, opts)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorIs(t, <-canceled, lifecycle.ErrForcedShutdown)
	require.Zero(t, remaining)
	<-clientDone
}

func TestShutdownDoesNotCloseDependenciesOfUncooperativeHandler(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	srv, _, url := startShutdownTestServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(entered); <-release }))
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered
	opts := testShutdownOptions()
	opts.requestTimeout = 20 * time.Millisecond
	opts.forceCloseTimeout = 20 * time.Millisecond
	called := false
	err := shutdownServer(srv, func(context.Context) error { called = true; return nil }, opts)
	close(release)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, called)
	<-clientDone
}

func TestShutdownDoesNotCloseDependenciesOfCanceledMetricsHandler(t *testing.T) {
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	srv, m, url := startShutdownTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(canceled)
		<-release // Simulate a DB/Redis operation still unwinding after cancel.
	}))
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := http.Get(url + "/metrics")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered
	require.Zero(t, m.Snapshot().ActiveHTTP, "metrics must remain outside business traffic counts")
	require.Equal(t, 1, m.Snapshot().ActiveObservability)
	opts := testShutdownOptions()
	opts.requestTimeout = 20 * time.Millisecond
	opts.forceCloseTimeout = 20 * time.Millisecond
	cleaned := false
	err := shutdownServer(srv, func(context.Context) error { cleaned = true; return nil }, opts)
	close(release)
	<-canceled
	<-clientDone
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, cleaned, "cleanup closed dependencies before the metrics handler returned")
}

func TestShutdownBoundsLegacyCleanup(t *testing.T) {
	srv, _, _ := startShutdownTestServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	release := make(chan struct{})
	defer close(release)
	opts := testShutdownOptions()
	opts.cleanupTimeout = 20 * time.Millisecond
	started := time.Now()
	err := shutdownServer(srv, func(context.Context) error { <-release; return nil }, opts)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), time.Second)
}

func TestShutdownReportsCleanupFailure(t *testing.T) {
	srv, _, _ := startShutdownTestServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	failure := errors.New("usage flush failed")
	require.ErrorIs(t, shutdownServer(srv, func(context.Context) error { return failure }, testShutdownOptions()), failure)
}

func TestShutdownOptionsUseConfiguredBudgets(t *testing.T) {
	opts := shutdownOptionsFromConfig(config.ServerConfig{DrainDelaySeconds: 7, ShutdownTimeoutSeconds: 600, ForceCloseTimeoutSeconds: 11, CleanupTimeoutSeconds: 35})
	require.Equal(t, 7*time.Second, opts.drainDelay)
	require.Equal(t, 600*time.Second, opts.requestTimeout)
	require.Equal(t, 11*time.Second, opts.forceCloseTimeout)
	require.Equal(t, 35*time.Second, opts.cleanupTimeout)
}
