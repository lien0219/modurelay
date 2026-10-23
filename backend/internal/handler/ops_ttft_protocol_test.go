//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpmetrics"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type monitoringStreamUpstream struct{ body string }

func (u *monitoringStreamUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(u.body)), Request: req}, nil
}
func (u *monitoringStreamUpstream) DoWithTLS(req *http.Request, proxy string, id int64, limit int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, limit)
}

// Exercise actual protocol handlers and forwarding parsers, then inspect the
// histogram rendered by the real request middleware (no injected TTFT key).
func TestMonitoringTTFTActualProtocolHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, platform, route, path, body, upstream, metricRoute string
		noOutput                                                 bool
	}{
		{name: "anthropic", platform: service.PlatformAnthropic, route: "/v1/messages", path: "/v1/messages", body: `{"model":"claude-test","max_tokens":10,"messages":[{"role":"user","content":"hello"}],"stream":true}`, upstream: partialMessageStartSSE + "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", metricRoute: "messages"},
		{name: "gemini native", platform: service.PlatformGemini, route: "/v1beta/models/*modelAction", path: "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", body: `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`, upstream: "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"hello\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1}}\n\n", metricRoute: "gemini"},
		{name: "gemini no output", platform: service.PlatformGemini, route: "/v1beta/models/*modelAction", path: "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", body: `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`, upstream: "data: {\"usageMetadata\":{\"promptTokenCount\":1}}\n\n", metricRoute: "gemini", noOutput: true},
		{name: "anthropic metadata only", platform: service.PlatformAnthropic, route: "/v1/messages", path: "/v1/messages", body: `{"model":"claude-test","max_tokens":10,"messages":[{"role":"user","content":"hello"}],"stream":true}`, upstream: partialMessageStartSSE + "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", metricRoute: "messages", noOutput: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			httpmetrics.Reset()
			t.Cleanup(httpmetrics.Reset)
			groupID := int64(9730)
			group := &service.Group{ID: groupID, Hydrated: true, Platform: tc.platform, Status: service.StatusActive}
			account := &service.Account{ID: 9731, Platform: tc.platform, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-only"}, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, AccountGroups: []service.AccountGroup{{AccountID: 9731, GroupID: groupID}}}
			h, cleanup := newTestGatewayHandler(t, group, []*service.Account{account})
			t.Cleanup(cleanup)
			cfg := &config.Config{RunMode: config.RunModeSimple}
			upstream := &monitoringStreamUpstream{body: tc.upstream}
			snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
			h.gatewayService = service.NewGatewayService(nil, &fakeGroupRepo{group: group}, nil, nil, nil, nil, nil, nil, cfg, snapshot, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			h.geminiCompatService = service.NewGeminiMessagesCompatService(nil, nil, nil, nil, nil, nil, upstream, nil, cfg)
			key := &service.APIKey{ID: 9732, UserID: 9733, GroupID: &groupID, Group: group, Status: service.StatusActive, User: &service.User{ID: 9733, Concurrency: 10, Balance: 100}}
			router := gin.New()
			router.Use(middleware.Logger())
			router.POST(tc.route, func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: key.UserID, Concurrency: 10})
				if tc.platform == service.PlatformGemini {
					h.GeminiV1BetaModels(c)
				} else {
					h.Messages(c)
				}
			})
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), ctxkey.Group, group))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, 200, rec.Code, rec.Body.String())
			rendered := httpmetrics.Render()
			if tc.noOutput {
				require.NotContains(t, rendered, "modurelay_http_ttft_seconds_count{")
			} else {
				require.Contains(t, rec.Body.String(), "hello")
				require.Contains(t, rendered, `modurelay_http_ttft_seconds_count{method="POST",source="app",route="`+tc.metricRoute+`",protocol="sse"} 1`)
			}
		})
	}
}
