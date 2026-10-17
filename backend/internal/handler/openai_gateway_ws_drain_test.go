package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIWSDrainHarness struct {
	client  *coderws.Conn
	manager *lifecycle.Manager
	usage   <-chan *service.UsageLog
	done    <-chan struct{}
}

func newOpenAIWSDrainHarness(t *testing.T, upstreamURL, mode string) *openAIWSDrainHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 10
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 10
	account := service.Account{
		ID: 9957, Name: "ws-drain-test", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": upstreamURL},
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    mode,
		},
	}
	usage := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(
		&openAIWSUsageHandlerAccountRepoStub{account: account}, usage, nil, nil, nil, nil,
		nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billing,
		&compositeWSHTTPUpstream{}, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
	)
	t.Cleanup(gateway.CloseOpenAIWSPool)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := &OpenAIGatewayHandler{
		cfg: cfg, gatewayService: gateway, billingCacheService: billing, apiKeyService: &service.APIKeyService{},
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
	groupID := int64(4201)
	apiKey := &service.APIKey{ID: 1801, GroupID: &groupID, User: &service.User{ID: 1701, Status: service.StatusActive}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	done := make(chan struct{})
	router.GET("/openai/v1/responses", func(c *gin.Context) {
		defer close(done)
		h.ResponsesWebSocket(c)
	})
	manager := lifecycle.New()
	server := httptest.NewServer(manager.Wrap(router))
	t.Cleanup(server.Close)
	t.Cleanup(manager.ForceClose)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
	require.NoError(t, err)
	client.SetReadLimit(4 << 20)
	t.Cleanup(func() { _ = client.CloseNow() })
	return &openAIWSDrainHarness{client: client, manager: manager, usage: usage.created, done: done}
}

func readOpenAIWSDrainFrame(t *testing.T, conn *coderws.Conn) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, payload, err := conn.Read(ctx)
	return payload, err
}

func writeOpenAIWSDrainTurn(t *testing.T, conn *coderws.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.4","input":"hello"}`)))
}

func requireOpenAIWSDrainClosed(t *testing.T, h *openAIWSDrainHarness) {
	t.Helper()
	_, err := readOpenAIWSDrainFrame(t, h.client)
	require.Equal(t, coderws.StatusServiceRestart, coderws.CloseStatus(err), "drain must send a reconnectable close frame: %v", err)
	select {
	case <-h.done:
	case <-time.After(4 * time.Second):
		t.Fatal("draining WebSocket handler did not finish")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, h.manager.Wait(ctx))
}

func TestOpenAIResponsesWebSocketDrainFinishesAcceptedTurn(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModeHTTPBridge} {
		t.Run(mode, func(t *testing.T) {
			for _, drainTurn := range []int{1, 2} {
				t.Run(fmt.Sprintf("turn_%d", drainTurn), func(t *testing.T) {
					started := make(chan struct{})
					finish := make(chan struct{})
					var finishOnce sync.Once
					allowFinish := func() { finishOnce.Do(func() { close(finish) }) }
					var bridgeTurns atomic.Int64
					completed := `{"type":"response.completed","response":{"id":"resp_drain","model":"gpt-5.4","usage":{"input_tokens":2,"output_tokens":1},"output_text":"` + strings.Repeat("x", 1<<20) + `"}}`
					upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if mode == service.OpenAIWSIngressModeHTTPBridge {
							// Each bridge turn uses its own HTTP request.
							turn := int(bridgeTurns.Add(1))
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
							if err := http.NewResponseController(w).Flush(); err != nil {
								t.Errorf("flush upstream delta: %v", err)
								return
							}
							if turn == drainTurn {
								close(started)
								<-finish
							}
							_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", completed)
							return
						}
						conn, err := coderws.Accept(w, r, nil)
						if err != nil {
							return
						}
						defer func() { _ = conn.CloseNow() }()
						for turn := 1; turn <= drainTurn; turn++ {
							if _, _, err = conn.Read(r.Context()); err != nil {
								return
							}
							if err = conn.Write(r.Context(), coderws.MessageText, []byte(`{"type":"response.output_text.delta","delta":"hello"}`)); err != nil {
								return
							}
							if turn == drainTurn {
								close(started)
								<-finish
							}
							if err = conn.Write(r.Context(), coderws.MessageText, []byte(completed)); err != nil {
								return
							}
						}
						_, _, _ = conn.Read(r.Context())
					}))
					t.Cleanup(upstream.Close)
					t.Cleanup(allowFinish)
					h := newOpenAIWSDrainHarness(t, upstream.URL, mode)
					for turn := 1; turn <= drainTurn; turn++ {
						writeOpenAIWSDrainTurn(t, h.client)
						payload, err := readOpenAIWSDrainFrame(t, h.client)
						require.NoError(t, err)
						require.Equal(t, "response.output_text.delta", gjson.GetBytes(payload, "type").String())
						if turn == drainTurn {
							<-started
							require.Equal(t, 1, h.manager.Snapshot().ActiveTurns)
							h.manager.BeginDrain()
							select {
							case <-h.done:
								t.Fatal("shutdown interrupted an accepted turn before its terminal response")
							case <-time.After(20 * time.Millisecond):
							}
							allowFinish()
						}
						payload, err = readOpenAIWSDrainFrame(t, h.client)
						require.NoError(t, err, "the full terminal event must arrive before drain closes the connection")
						require.Equal(t, "response.completed", gjson.GetBytes(payload, "type").String())
						require.Len(t, gjson.GetBytes(payload, "response.output_text").String(), 1<<20)
						select {
						case usage := <-h.usage:
							require.Equal(t, 2, usage.InputTokens)
						case <-time.After(time.Second):
							t.Fatal("completed turn usage was not submitted")
						}
					}
					requireOpenAIWSDrainClosed(t, h)
				})
			}
		})
	}
}

func TestOpenAIResponsesWebSocketDrainClosesBeforeFirstTurn(t *testing.T) {
	h := newOpenAIWSDrainHarness(t, "http://127.0.0.1:1", service.OpenAIWSIngressModePassthrough)
	h.manager.BeginDrain()
	requireOpenAIWSDrainClosed(t, h)
	require.Empty(t, h.usage)
}

func TestOpenAIResponsesWebSocketDrainClosesIdleBetweenTurns(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModeHTTPBridge} {
		t.Run(mode, func(t *testing.T) {
			var upstreamTurns atomic.Int64
			completed := []byte(`{"type":"response.completed","response":{"id":"resp_idle_drain","model":"gpt-5.4","usage":{"input_tokens":2,"output_tokens":1}}}`)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == service.OpenAIWSIngressModeHTTPBridge {
					upstreamTurns.Add(1)
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", completed)
					return
				}
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer func() { _ = conn.CloseNow() }()
				for {
					if _, _, err = conn.Read(r.Context()); err != nil {
						return
					}
					upstreamTurns.Add(1)
					if err = conn.Write(r.Context(), coderws.MessageText, completed); err != nil {
						return
					}
				}
			}))
			t.Cleanup(upstream.Close)
			h := newOpenAIWSDrainHarness(t, upstream.URL, mode)
			writeOpenAIWSDrainTurn(t, h.client)
			payload, err := readOpenAIWSDrainFrame(t, h.client)
			require.NoError(t, err)
			require.Equal(t, "response.completed", gjson.GetBytes(payload, "type").String())
			require.Eventually(t, func() bool { return h.manager.Snapshot().ActiveTurns == 0 }, time.Second, time.Millisecond)
			h.manager.BeginDrain()
			// A follow-up sent concurrently with the close handshake must never
			// become another upstream request after the drain admission boundary.
			writeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = h.client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.4"}`))
			cancel()
			requireOpenAIWSDrainClosed(t, h)
			require.EqualValues(t, 1, upstreamTurns.Load())
		})
	}
}

func TestOpenAIResponsesWebSocketDrainDeadlineForcesActiveTurn(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModeHTTPBridge} {
		t.Run(mode, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				delta := []byte(`{"type":"response.output_text.delta","delta":"hello"}`)
				if mode == service.OpenAIWSIngressModeHTTPBridge {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprintf(w, "data: %s\n\n", delta)
					if err := http.NewResponseController(w).Flush(); err != nil {
						t.Errorf("flush upstream delta: %v", err)
						return
					}
					<-r.Context().Done()
					return
				}
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer func() { _ = conn.CloseNow() }()
				if _, _, err = conn.Read(r.Context()); err != nil {
					return
				}
				if err = conn.Write(r.Context(), coderws.MessageText, delta); err != nil {
					return
				}
				_, _, _ = conn.Read(r.Context())
			}))
			t.Cleanup(upstream.Close)
			h := newOpenAIWSDrainHarness(t, upstream.URL, mode)
			writeOpenAIWSDrainTurn(t, h.client)
			_, err := readOpenAIWSDrainFrame(t, h.client)
			require.NoError(t, err)
			h.manager.BeginDrain()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			require.ErrorIs(t, h.manager.Wait(ctx), context.DeadlineExceeded)
			cancel()
			h.manager.ForceClose()
			_, err = readOpenAIWSDrainFrame(t, h.client)
			require.Error(t, err)
			ctx, cancel = context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			require.NoError(t, h.manager.Wait(ctx), "forced drain must release the handler and its turn")
			require.Zero(t, h.manager.Snapshot().ActiveTurns)
		})
	}
}

func TestOpenAIResponsesWebSocketDrainKeepsAcceptedTurnRetry(t *testing.T) {
	managerReady := make(chan *lifecycle.Manager, 1)
	var attempts atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		if _, _, err = conn.Read(r.Context()); err != nil {
			return
		}
		if attempts.Add(1) == 1 {
			// A transport failure before any downstream output is retryable.
			// Restart begins during this accepted turn, before the fresh dial.
			(<-managerReady).BeginDrain()
			return
		}
		_ = conn.Write(r.Context(), coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_drain_retry","model":"gpt-5.4","usage":{"input_tokens":2,"output_tokens":1}}}`))
		_, _, _ = conn.Read(r.Context())
	}))
	t.Cleanup(upstream.Close)
	h := newOpenAIWSDrainHarness(t, upstream.URL, service.OpenAIWSIngressModeDedicated)
	managerReady <- h.manager
	writeOpenAIWSDrainTurn(t, h.client)
	payload, err := readOpenAIWSDrainFrame(t, h.client)
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(payload, "type").String())
	require.EqualValues(t, 2, attempts.Load())
	requireOpenAIWSDrainClosed(t, h)
	select {
	case usage := <-h.usage:
		require.Equal(t, 2, usage.InputTokens)
	case <-time.After(time.Second):
		t.Fatal("retried turn usage was not submitted")
	}
}
