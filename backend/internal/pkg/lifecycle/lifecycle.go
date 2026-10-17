// Package lifecycle coordinates request admission and graceful server draining.
package lifecycle

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/felixge/httpsnoop"
)

// Snapshot reports work that must finish before application resources can stop.
type Snapshot struct {
	Draining            bool
	ActiveHTTP          int
	ActiveWebSockets    int
	ActiveTurns         int
	ActiveObservability int // Probe and metrics handlers; excluded from business gauges.
}

// Manager belongs to one HTTP server. Wrap its outermost handler before serving.
type Manager struct {
	mu              sync.Mutex
	draining        bool
	admissionClosed bool
	forced          bool
	requests        map[*Request]struct{}
	changed         chan struct{}
}

// New returns a ready manager that accepts requests.
func New() *Manager {
	return &Manager{requests: make(map[*Request]struct{}), changed: make(chan struct{})}
}

type contextKey struct{}
type managerContextKey struct{}

// ErrForcedShutdown is the cancellation cause when the request drain expires.
var ErrForcedShutdown = errors.New("server drain deadline exceeded")

// Request identifies one admitted request and its application work.
type Request struct {
	manager *Manager
	cancel  context.CancelCauseFunc
	// All state below is guarded by manager.mu.
	turns           int
	onDrain         func()
	drainNotified   bool
	handlerFinished bool
	hijacked        bool
	observability   bool
	connections     map[*trackedConn]struct{}
}

// FromContext returns the request lifecycle, or nil for unwrapped handlers.
func FromContext(ctx context.Context) *Request {
	request, _ := ctx.Value(contextKey{}).(*Request)
	return request
}

// ManagerFromContext returns the server lifecycle, including for health and
// metrics requests which are deliberately excluded from business gauges.
func ManagerFromContext(ctx context.Context) *Manager {
	manager, _ := ctx.Value(managerContextKey{}).(*Manager)
	return manager
}

type wrappedHandler struct {
	manager *Manager
	next    http.Handler
}

// Wrap installs readiness and business request admission at the server boundary.
func (m *Manager) Wrap(next http.Handler) http.Handler {
	return &wrappedHandler{manager: m, next: next}
}

// FromHandler returns the manager installed by Wrap, or nil for other handlers.
func FromHandler(handler http.Handler) *Manager {
	if wrapped, ok := handler.(*wrappedHandler); ok {
		return wrapped.manager
	}
	return nil
}

func (h *wrappedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := h.manager
	r = r.WithContext(context.WithValue(r.Context(), managerContextKey{}, m))
	if r.URL.Path == "/ready" {
		w.Header().Set("Content-Type", "application/json")
		m.mu.Lock()
		draining := m.draining
		m.mu.Unlock()
		if draining {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"draining"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
		return
	}
	observability := r.URL.Path == "/health" || r.URL.Path == "/metrics" || r.URL.Path == "/metrics/instance"
	m.mu.Lock()
	if m.forced || (m.admissionClosed && !observability) {
		m.mu.Unlock()
		w.Header().Set("Retry-After", "1")
		http.Error(w, "server is draining", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithCancelCause(r.Context())
	request := &Request{manager: m, cancel: cancel, observability: observability}
	m.requests[request] = struct{}{}
	m.mu.Unlock()
	defer func() {
		cancel(nil)
		m.mu.Lock()
		request.handlerFinished = true
		request.releaseLocked()
		m.mu.Unlock()
	}()
	writer := httpsnoop.Wrap(w, httpsnoop.Hooks{Hijack: func(next httpsnoop.HijackFunc) httpsnoop.HijackFunc {
		return func() (net.Conn, *bufio.ReadWriter, error) {
			conn, buffer, err := next()
			if err != nil {
				return conn, buffer, err
			}
			tracked := &trackedConn{Conn: conn, request: request}
			m.mu.Lock()
			request.hijacked = true
			if request.connections == nil {
				request.connections = make(map[*trackedConn]struct{})
			}
			request.connections[tracked] = struct{}{}
			forced := m.forced
			m.mu.Unlock()
			// ForceClose can race with a blocking Hijack. Do not leave a new
			// upgraded connection alive after the forced-close snapshot.
			if forced {
				_ = tracked.Close()
			}
			return tracked, buffer, nil
		}
	}})
	if !observability {
		ctx = context.WithValue(ctx, contextKey{}, request)
	}
	h.next.ServeHTTP(writer, r.WithContext(ctx))
}

// BeginDrain makes readiness fail. Call StopAdmission after the routing
// propagation window to stop accepting new business requests.
func (m *Manager) BeginDrain() {
	m.mu.Lock()
	m.draining = true
	var callbacks []func()
	for request := range m.requests {
		if callback := request.drainCallbackLocked(); callback != nil {
			callbacks = append(callbacks, callback)
		}
	}
	m.mu.Unlock()
	for _, callback := range callbacks {
		go callback()
	}
}

// StopAdmission rejects subsequent business requests, while probes stay reachable.
func (m *Manager) StopAdmission() {
	m.mu.Lock()
	m.admissionClosed = true
	m.mu.Unlock()
}

// Snapshot returns the current drain state and work counts.
func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := Snapshot{Draining: m.draining}
	for request := range m.requests {
		if request.observability {
			snapshot.ActiveObservability++
			continue
		}
		if request.hijacked {
			// Count a session until both its transport and handler accounting
			// finish, whichever completes last.
			snapshot.ActiveWebSockets++
		} else {
			snapshot.ActiveHTTP++
		}
		snapshot.ActiveTurns += request.turns
	}
	return snapshot
}

// Wait waits for all admitted work. Call StopAdmission first to prevent new
// requests from entering after the current set has completed.
func (m *Manager) Wait(ctx context.Context) error {
	for {
		m.mu.Lock()
		if len(m.requests) == 0 {
			m.mu.Unlock()
			return nil
		}
		changed := m.changed
		m.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ForceClose cancels remaining requests and closes upgraded connections. The
// caller can use a separate bounded Wait afterward to let handlers finish
// accounting before stopping resources.
func (m *Manager) ForceClose() {
	m.mu.Lock()
	m.draining = true
	m.admissionClosed = true
	m.forced = true
	cancels := make([]context.CancelCauseFunc, 0, len(m.requests))
	var connections []*trackedConn
	for request := range m.requests {
		cancels = append(cancels, request.cancel)
		for conn := range request.connections {
			connections = append(connections, conn)
		}
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel(ErrForcedShutdown)
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
}

func (m *Manager) notifyLocked() {
	close(m.changed)
	m.changed = make(chan struct{})
}

// BeginTurn admits one WebSocket turn only while the server is not draining.
// It is synchronized with BeginDrain, so an admitted turn always finishes
// before the session's drain callback runs. A nil Request supports handlers
// used independently of Wrap.
func (r *Request) BeginTurn() bool {
	if r == nil {
		return true
	}
	m := r.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.draining || (r.handlerFinished && len(r.connections) == 0) {
		return false
	}
	r.turns++
	return true
}

// EndTurn marks a turn complete after its response and accounting are finished.
func (r *Request) EndTurn() {
	if r == nil {
		return
	}
	m := r.manager
	m.mu.Lock()
	if r.turns > 0 {
		r.turns--
	}
	callback := r.drainCallbackLocked()
	m.mu.Unlock()
	if callback != nil {
		go callback()
	}
}

// OnDrain registers the session's close callback. The first registration runs
// once, asynchronously, when draining and no turn is active. The callback may
// perform a close handshake without blocking request admission or other drains.
func (r *Request) OnDrain(callback func()) {
	if r == nil || callback == nil {
		return
	}
	m := r.manager
	m.mu.Lock()
	if r.onDrain == nil && !r.drainNotified {
		r.onDrain = callback
	}
	callback = r.drainCallbackLocked()
	m.mu.Unlock()
	if callback != nil {
		go callback()
	}
}

func (r *Request) drainCallbackLocked() func() {
	if !r.manager.draining || r.turns != 0 || r.onDrain == nil || r.drainNotified {
		return nil
	}
	r.drainNotified = true
	callback := r.onDrain
	r.onDrain = nil
	return callback
}

func (r *Request) releaseLocked() {
	if r.handlerFinished && len(r.connections) == 0 {
		delete(r.manager.requests, r)
		r.manager.notifyLocked()
	}
}

type trackedConn struct {
	net.Conn
	request *Request
	once    sync.Once
	err     error
}

func (c *trackedConn) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		m := c.request.manager
		m.mu.Lock()
		delete(c.request.connections, c)
		c.request.releaseLocked()
		m.mu.Unlock()
	})
	return c.err
}
