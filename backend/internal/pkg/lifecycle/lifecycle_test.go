package lifecycle_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/coder/websocket"
)

func TestReadinessAndAdmissionDuringDrain(t *testing.T) {
	manager := lifecycle.New()
	called := 0
	handler := manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if lifecycle.ManagerFromContext(r.Context()) != manager {
			t.Error("request has no manager context")
		}
		if r.URL.Path == "/v1/responses" && lifecycle.FromContext(r.Context()) == nil {
			t.Error("business request has no lifecycle context")
		}
		if r.URL.Path != "/v1/responses" && (lifecycle.FromContext(r.Context()) != nil || manager.Snapshot().ActiveHTTP != 0) {
			t.Error("health or metrics request counted as business work")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	if lifecycle.FromHandler(handler) != manager {
		t.Fatal("wrapped handler does not expose its lifecycle manager")
	}
	request := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}
	if got := request("/ready").Code; got != http.StatusOK {
		t.Fatalf("readiness before drain = %d", got)
	}
	manager.BeginDrain()
	manager.BeginDrain()
	if got := request("/ready").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("readiness during drain = %d", got)
	}
	// Readiness propagation is separate from admission: already routed traffic
	// can still enter until the caller closes the admission gate.
	if got := request("/v1/responses").Code; got != http.StatusNoContent {
		t.Fatalf("request during propagation window = %d", got)
	}
	manager.StopAdmission()
	response := request("/v1/responses")
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") == "" {
		t.Fatalf("closed admission = %d, retry-after = %q", response.Code, response.Header().Get("Retry-After"))
	}
	for _, path := range []string{"/health", "/metrics", "/metrics/instance"} {
		if got := request(path).Code; got != http.StatusNoContent {
			t.Errorf("%s during drain = %d", path, got)
		}
	}
	if called != 4 {
		t.Fatalf("handler called %d times; readiness/rejected requests reached application", called)
	}
	if snapshot := manager.Snapshot(); !snapshot.Draining || snapshot.ActiveHTTP != 0 {
		t.Fatalf("snapshot after completed requests: %+v", snapshot)
	}
}

func TestDrainingLetsActiveSSEFinish(t *testing.T) {
	manager := lifecycle.New()
	finish := make(chan struct{})
	var release sync.Once
	unblock := func() { release.Do(func() { close(finish) }) }
	defer unblock()
	server := httptest.NewServer(manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("streaming response lost Flusher")
			return
		}
		flusher.Flush()
		select {
		case <-finish:
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		case <-r.Context().Done():
			t.Error("graceful drain canceled an active stream")
		}
	})))
	defer func() {
		unblock()
		server.Close()
	}()
	server.Client().Timeout = 2 * time.Second
	response, err := server.Client().Get(server.URL + "/v1/responses")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	reader := bufio.NewReader(response.Body)
	if first, err := reader.ReadString('\n'); err != nil || first != "data: first\n" {
		t.Fatalf("first SSE event = %q, %v", first, err)
	}
	manager.BeginDrain()
	manager.StopAdmission()
	if snapshot := manager.Snapshot(); snapshot.ActiveHTTP != 1 {
		t.Fatalf("active stream not counted: %+v", snapshot)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := manager.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait returned before stream finished: %v", err)
	}
	shutdown := make(chan error, 1)
	go func() { shutdown <- server.Config.Shutdown(context.Background()) }()
	unblock()
	rest, err := io.ReadAll(reader)
	if err != nil || !strings.Contains(string(rest), "data: [DONE]\n\n") {
		t.Fatalf("stream was truncated during drain: %q, %v", rest, err)
	}
	waitForDrain(t, manager)
	if err := <-shutdown; err != nil {
		t.Fatal(err)
	}
}

func TestForceCloseCancelsActiveHTTPAndPreservesClientCancellation(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "client cancellation", true: "force close"}[force], func(t *testing.T) {
			manager := lifecycle.New()
			entered := make(chan struct{})
			causes := make(chan error, 1)
			handler := manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-r.Context().Done()
				causes <- context.Cause(r.Context())
			}))
			parent, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(parent))
			<-entered
			clientCause := errors.New("client closed request")
			if force {
				manager.ForceClose()
			} else {
				cancel(clientCause)
			}
			select {
			case got := <-causes:
				if !force && !errors.Is(got, clientCause) {
					t.Fatalf("lost original cancellation cause: %v", got)
				}
				if force && got == nil {
					t.Fatal("forced request has no cancellation cause")
				}
			case <-time.After(time.Second):
				t.Fatal("request was not canceled")
			}
			waitForDrain(t, manager)
		})
	}
}

func waitForDrain(t *testing.T, manager *lifecycle.Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Wait(ctx); err != nil {
		t.Fatalf("requests did not drain: %v, snapshot: %+v", err, manager.Snapshot())
	}
}

func TestDrainWaitsForCurrentTurnAndCallsCloseOnceOutsideLock(t *testing.T) {
	manager := lifecycle.New()
	requests := make(chan *lifecycle.Request, 1)
	finish := make(chan struct{})
	defer close(finish)
	go manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- lifecycle.FromContext(r.Context())
		<-finish
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	request := <-requests
	if !request.BeginTurn() {
		t.Fatal("first turn was rejected before drain")
	}
	closed := make(chan lifecycle.Snapshot, 2)
	callbackBlock := make(chan struct{})
	defer close(callbackBlock)
	request.OnDrain(func() {
		// Snapshot takes the manager lock: a callback under that lock deadlocks.
		closed <- manager.Snapshot()
		<-callbackBlock
	})
	manager.BeginDrain()
	if request.BeginTurn() {
		t.Fatal("a new WebSocket turn entered during drain")
	}
	select {
	case <-closed:
		t.Fatal("active turn was closed before finishing")
	default:
	}
	if snapshot := manager.Snapshot(); snapshot.ActiveTurns != 1 {
		t.Fatalf("active turn count: %+v", snapshot)
	}
	request.EndTurn()
	select {
	case snapshot := <-closed:
		if snapshot.ActiveTurns != 0 {
			t.Fatalf("close callback ran before active turn completed: %+v", snapshot)
		}
	case <-time.After(time.Second):
		t.Fatal("idle WebSocket was not closed after turn completed")
	}
	// A blocking close handshake must not block or duplicate later drain calls.
	manager.BeginDrain()
	request.EndTurn()
	request.OnDrain(func() { closed <- manager.Snapshot() })
	select {
	case <-closed:
		t.Fatal("drain callback ran more than once")
	default:
	}
}

func TestRegisterCloseCallbackAfterDrainClosesIdleSession(t *testing.T) {
	manager := lifecycle.New()
	closed := make(chan struct{})
	manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.BeginDrain()
		lifecycle.FromContext(r.Context()).OnDrain(func() { close(closed) })
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("late close callback missed drain")
	}
}

func TestConcurrentTurnAdmissionAndDrain(t *testing.T) {
	for i := 0; i < 200; i++ {
		manager := lifecycle.New()
		manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request := lifecycle.FromContext(r.Context())
			start := make(chan struct{})
			accepted := make(chan bool, 1)
			drained := make(chan struct{})
			closed := make(chan struct{}, 2)
			request.OnDrain(func() { closed <- struct{}{} })
			go func() { <-start; accepted <- request.BeginTurn() }()
			go func() { <-start; manager.BeginDrain(); close(drained) }()
			close(start)
			got := <-accepted
			<-drained
			if got {
				select {
				case <-closed:
					t.Fatal("drain closed a concurrently admitted active turn")
				default:
				}
				request.EndTurn()
			}
			if request.BeginTurn() {
				t.Fatal("new turn admitted after drain completed")
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("session close was lost during concurrent drain")
			}
		})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
		waitForDrain(t, manager)
	}
}

func TestUnwrappedRequestLifecycleIsOptional(t *testing.T) {
	request := lifecycle.FromContext(context.Background())
	if request != nil || !request.BeginTurn() {
		t.Fatal("unwrapped handler cannot admit a turn")
	}
	request.EndTurn()
	request.OnDrain(func() { t.Error("unwrapped request unexpectedly drained") })
	if lifecycle.FromHandler(http.NotFoundHandler()) != nil || lifecycle.ManagerFromContext(context.Background()) != nil {
		t.Fatal("unwrapped handler unexpectedly has a manager")
	}
}

func TestHijackedWebSocketSurvivesHTTPShutdownUntilClosed(t *testing.T) {
	manager := lifecycle.New()
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		connections <- conn
		// A hijacked session can outlive the HTTP handler.
	})))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.CloseNow() }()
	conn := <-connections
	defer func() { _ = conn.CloseNow() }()
	manager.BeginDrain()
	manager.StopAdmission()
	if err := server.Config.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot(); got.ActiveHTTP != 0 || got.ActiveWebSockets != 1 {
		t.Fatalf("hijacked session was lost after handler returned: %+v", got)
	}
	waitCtx, stopWait := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stopWait()
	if err := manager.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("drain failed to wait for hijacked connection: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte("final result")); err != nil {
		t.Fatal(err)
	}
	_, data, err := client.Read(ctx)
	if err != nil || string(data) != "final result" {
		t.Fatalf("websocket output truncated by HTTP shutdown: %q, %v", data, err)
	}
	manager.ForceClose()
	if _, _, err := client.Read(ctx); err == nil {
		t.Fatal("forced shutdown did not close hijacked transport")
	}
	waitForDrain(t, manager)
	if got := manager.Snapshot(); got.ActiveWebSockets != 0 {
		t.Fatalf("closed WebSocket retained in manager: %+v", got)
	}
}

func TestWebSocketCloseStillWaitsForHandlerAccounting(t *testing.T) {
	manager := lifecycle.New()
	accepted := make(chan struct{})
	closeConnection := make(chan struct{})
	connectionClosed := make(chan struct{})
	finishAccounting := make(chan struct{})
	var release sync.Once
	unblock := func() { release.Do(func() { close(finishAccounting) }) }
	defer unblock()
	server := httptest.NewServer(manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		close(accepted)
		<-closeConnection
		_ = conn.CloseNow()
		close(connectionClosed)
		<-finishAccounting
	})))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.CloseNow() }()
	<-accepted
	close(closeConnection)
	<-connectionClosed
	if got := manager.Snapshot(); got.ActiveWebSockets != 1 {
		t.Fatalf("session stopped counting before accounting completed: %+v", got)
	}
	waitCtx, stopWait := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stopWait()
	if err := manager.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait skipped post-connection accounting: %v", err)
	}
	unblock()
	waitForDrain(t, manager)
}

func TestForceCloseRacingSuccessfulHijackClosesTransport(t *testing.T) {
	manager := lifecycle.New()
	serverConn, peer := net.Pipe()
	defer func() { _ = peer.Close() }()
	entered := make(chan struct{})
	release := make(chan struct{})
	writer := &blockingHijacker{
		ResponseRecorder: httptest.NewRecorder(), conn: serverConn, entered: entered, release: release,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Errorf("hijack failed: %v", err)
			}
		})).ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	}()
	<-entered
	manager.ForceClose()
	close(release)
	<-done
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	var buf [1]byte
	if _, err := peer.Read(buf[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("transport created during ForceClose was not closed: %v", err)
	}
	waitForDrain(t, manager)
}

type blockingHijacker struct {
	*httptest.ResponseRecorder
	conn    net.Conn
	entered chan struct{}
	release chan struct{}
}

func (w *blockingHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	close(w.entered)
	<-w.release
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

func TestResponseWriterPreservesStreamingAndControllerCapabilities(t *testing.T) {
	for _, plain := range []bool{true, false} {
		t.Run(map[bool]string{true: "plain writer", false: "streaming writer"}[plain], func(t *testing.T) {
			manager := lifecycle.New()
			recorder := httptest.NewRecorder()
			features := &featureWriter{ResponseRecorder: recorder}
			var base http.ResponseWriter = features
			if plain {
				base = struct{ http.ResponseWriter }{recorder}
			}
			deadline := time.Now().Add(time.Minute)
			manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, flusher := w.(http.Flusher)
				_, hijacker := w.(http.Hijacker)
				reader, readerFrom := w.(io.ReaderFrom)
				push, pusher := w.(http.Pusher)
				_, closeNotifier := w.(http.CloseNotifier) //nolint:staticcheck // Verify the wrapper preserves legacy interfaces used by existing handlers.
				if flusher != !plain || hijacker != !plain || readerFrom != !plain || pusher != !plain || closeNotifier != !plain {
					t.Fatalf("writer capabilities changed: flush=%v hijack=%v readFrom=%v push=%v closeNotify=%v", flusher, hijacker, readerFrom, pusher, closeNotifier)
				}
				controller := http.NewResponseController(w)
				if plain {
					if !errors.Is(controller.Flush(), http.ErrNotSupported) {
						t.Error("plain writer unexpectedly supports flush")
					}
					return
				}
				if err := controller.SetWriteDeadline(deadline); err != nil {
					t.Fatal(err)
				}
				if err := controller.SetReadDeadline(deadline); err != nil {
					t.Fatal(err)
				}
				if err := controller.EnableFullDuplex(); err != nil {
					t.Fatal(err)
				}
				if _, err := reader.ReadFrom(strings.NewReader("stream")); err != nil {
					t.Fatal(err)
				}
				if err := push.Push("/asset", nil); err != nil {
					t.Fatal(err)
				}
				if err := controller.Flush(); err != nil {
					t.Fatal(err)
				}
			})).ServeHTTP(base, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
			if !plain && (!features.readDeadline.Equal(deadline) || !features.writeDeadline.Equal(deadline) || !features.fullDuplex || features.pushed != "/asset" || !recorder.Flushed || recorder.Body.String() != "stream") {
				t.Fatalf("response capabilities did not reach original writer: %+v, body=%q", features, recorder.Body.String())
			}
		})
	}
}

type featureWriter struct {
	*httptest.ResponseRecorder
	readDeadline  time.Time
	writeDeadline time.Time
	fullDuplex    bool
	pushed        string
}

func (w *featureWriter) SetReadDeadline(deadline time.Time) error {
	w.readDeadline = deadline
	return nil
}

func (w *featureWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadline = deadline
	return nil
}

func (w *featureWriter) EnableFullDuplex() error {
	w.fullDuplex = true
	return nil
}

func (w *featureWriter) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(w.ResponseRecorder, reader)
}

func (w *featureWriter) Push(target string, _ *http.PushOptions) error {
	w.pushed = target
	return nil
}

func (w *featureWriter) CloseNotify() <-chan bool { return make(chan bool) }

func (w *featureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, http.ErrNotSupported
}

func TestHTTP2CountsConcurrentRequestsOnOneConnection(t *testing.T) {
	manager := lifecycle.New()
	finish := make(chan struct{})
	var release sync.Once
	unblock := func() { release.Do(func() { close(finish) }) }
	defer unblock()
	server := httptest.NewUnstartedServer(manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "first\n")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("HTTP/2 response lost Flusher")
			return
		}
		flusher.Flush()
		<-finish
		_, _ = io.WriteString(w, "complete\n")
	})))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer func() {
		unblock()
		server.Close()
	}()
	client := server.Client()
	client.Timeout = 2 * time.Second
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("test server client is not using an HTTP transport")
	}
	transport.MaxConnsPerHost = 1
	first, err := client.Get(server.URL + "/first")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Body.Close() }()
	second, err := client.Get(server.URL + "/second")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Body.Close() }()
	if first.ProtoMajor != 2 || second.ProtoMajor != 2 {
		t.Fatal("test did not negotiate HTTP/2")
	}
	if got := manager.Snapshot(); got.ActiveHTTP != 2 {
		t.Fatalf("two HTTP/2 streams were not counted individually: %+v", got)
	}
	manager.BeginDrain()
	manager.StopAdmission()
	unblock()
	for _, response := range []*http.Response{first, second} {
		body, err := io.ReadAll(response.Body)
		if err != nil || string(body) != "first\ncomplete\n" {
			t.Fatalf("HTTP/2 request did not finish: %q, %v", body, err)
		}
	}
	waitForDrain(t, manager)
}

func TestClosingUpgradedTransportRacesHandlerCompletion(t *testing.T) {
	manager := lifecycle.New()
	for i := 0; i < 100; i++ {
		serverConn, peer := net.Pipe()
		entered := make(chan struct{})
		release := make(chan struct{})
		close(release)
		writer := &blockingHijacker{ResponseRecorder: httptest.NewRecorder(), conn: serverConn, entered: entered, release: release}
		start := make(chan struct{})
		done := make(chan struct{})
		go manager.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response lost Hijacker")
				close(done)
				return
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				close(done)
				return
			}
			go func() {
				<-start
				_ = conn.Close()
				// Repeated closes must not decrement twice.
				_ = conn.Close()
				close(done)
			}()
			close(start)
		})).ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
		<-done
		waitForDrain(t, manager)
		_ = peer.Close()
		if got := manager.Snapshot(); got.ActiveHTTP != 0 || got.ActiveWebSockets != 0 {
			t.Fatalf("completed upgraded request retained after close race: %+v", got)
		}
	}
}
