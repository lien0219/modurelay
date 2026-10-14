package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func bedrockCCCompatGateway(enabled bool) *GatewayService {
	channels := &ChannelService{}
	channels.cache.Store(populateChannelCache([]Channel{{
		ID: 1, Status: StatusActive, GroupIDs: []int64{10},
		FeaturesConfig: map[string]any{featureKeyBedrockCCCompat: enabled},
	}}, map[int64]string{10: PlatformComposite}))
	return &GatewayService{channelService: channels}
}

func TestApplyBedrockCCCompatMixedChannel(t *testing.T) {
	groupID := int64(10)
	body := []byte(`{"model":"claude-sonnet-5-5","service_tier":"standard","max_tokens":4096,"thinking":{"type":"enabled","budget_tokens":2048},"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_bad.id","name":"lookup","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_bad.id","content":"ok"}]}]}`)
	for _, tc := range []struct {
		name    string
		account *Account
	}{
		{"bedrock", &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}},
		{"anthropic apikey", &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}},
		{"anthropic oauth", &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}},
		{"openai", &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}},
		{"antigravity", &Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}},
		{"nil account", nil},
	} {
		for _, enabled := range []bool{true, false} {
			t.Run(tc.name+"/"+map[bool]string{true: "on", false: "off"}[enabled], func(t *testing.T) {
				svc := bedrockCCCompatGateway(enabled)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				c.Request.Header.Set("Anthropic-Beta", "context-1m-2025-08-07,unsupported-feature")
				originalHeader := c.Request.Header.Clone()
				out := svc.ApplyBedrockCCCompat(c, body, "claude-sonnet-5-5", tc.account, &groupID)
				if enabled && tc.name == "bedrock" {
					require.False(t, gjson.GetBytes(out, "service_tier").Exists())
					require.Equal(t, "adaptive", gjson.GetBytes(out, "thinking.type").String())
					require.NotContains(t, gjson.GetBytes(out, "messages.0.content.0.id").String(), ".")
					require.Equal(t, gjson.GetBytes(out, "messages.0.content.0.id").String(), gjson.GetBytes(out, "messages.1.content.0.tool_use_id").String())
				} else {
					require.Equal(t, string(body), string(out))
				}
				// Keep original headers for beta policy and a later non-Bedrock attempt.
				require.Equal(t, originalHeader, c.Request.Header)
			})
		}
	}
}

func TestBedrockCCCompatForwardedRequest(t *testing.T) {
	groupID := int64(10)
	for _, enabled := range []bool{true, false} {
		for _, stream := range []bool{true, false} {
			svc := bedrockCCCompatGateway(enabled)
			svc.cfg = &config.Config{}
			upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK, Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{"type":"message","content":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
			}}
			if stream {
				upstream.resp.Body = io.NopCloser(strings.NewReader(""))
			}
			svc.httpUpstream = upstream
			account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeBedrock,
				Credentials: map[string]any{"auth_mode": "apikey", "api_key": "test-only", "aws_region": "us-east-1"}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Request.Header.Set("Anthropic-Beta", "context-1m-2025-08-07,unsupported-feature")
			body := []byte(`{"model":"claude-opus-4-6","max_tokens":4096,"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_bad.id","name":"lookup","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_bad.id","content":"ok"}]}]}`)
			converted := svc.ApplyBedrockCCCompat(c, body, "claude-opus-4-6", account, &groupID)
			_, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
				Model: "claude-opus-4-6", Body: NewRequestBodyRef(converted), Stream: stream,
			})
			require.NoError(t, err)
			require.Contains(t, upstream.lastReq.URL.Host, "bedrock-runtime.us-east-1.amazonaws.com")
			if stream {
				require.True(t, strings.HasSuffix(upstream.lastReq.URL.Path, "/invoke-with-response-stream"))
			} else {
				require.True(t, strings.HasSuffix(upstream.lastReq.URL.Path, "/invoke"))
			}
			wantID := "toolu_bad.id"
			if enabled {
				wantID = "toolu_bad_id"
			}
			require.Equal(t, wantID, gjson.GetBytes(upstream.lastBody, "messages.0.content.0.id").String())
			require.Equal(t, wantID, gjson.GetBytes(upstream.lastBody, "messages.1.content.0.tool_use_id").String())
			require.NotContains(t, string(upstream.lastBody), "unsupported-feature")
			require.Equal(t, "context-1m-2025-08-07", gjson.GetBytes(upstream.lastBody, "anthropic_beta.0").String())
			require.Empty(t, upstream.lastReq.Header.Get("Anthropic-Beta"))
			require.Equal(t, "context-1m-2025-08-07,unsupported-feature", c.GetHeader("Anthropic-Beta"))
			// A later attempt on a regular Anthropic account must get the original input.
			fallback := svc.ApplyBedrockCCCompat(c, body, "claude-opus-4-6", newAnthropicAPIKeyAccountForTest(), &groupID)
			require.Equal(t, string(body), string(fallback))
		}
	}
}

func TestBedrockCCCompatFallbackForwardsOriginalAnthropicRequest(t *testing.T) {
	svc := bedrockCCCompatGateway(true)
	svc.cfg = &config.Config{}
	svc.rateLimitService = &RateLimitService{}
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"id":"msg_test","type":"message","usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}
	svc.httpUpstream = upstream
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Anthropic-Beta", "context-1m-2025-08-07,unsupported-feature")
	body := []byte(`{"model":"claude-opus-4-6","service_tier":"standard","max_tokens":4096,"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_bad.id","name":"lookup","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_bad.id","content":"ok"}]}]}`)
	groupID := int64(10)
	firstAttempt := svc.ApplyBedrockCCCompat(c, body, "claude-opus-4-6", &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}, &groupID)
	require.NotEqual(t, string(body), string(firstAttempt))
	account := newAnthropicAPIKeyAccountForTest()
	fallback := svc.ApplyBedrockCCCompat(c, body, "claude-opus-4-6", account, &groupID)
	_, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Model: "claude-opus-4-6", Body: NewRequestBodyRef(fallback),
	})
	require.NoError(t, err)
	require.Equal(t, "api.anthropic.com", upstream.lastReq.URL.Host)
	require.Equal(t, string(body), string(upstream.lastBody))
	// The passthrough builder may intentionally omit the client beta header
	// after body capability filtering; the important invariant is that Apply
	// did not mutate the request context used by the fallback attempt.
	require.Equal(t, "context-1m-2025-08-07,unsupported-feature", c.GetHeader("Anthropic-Beta"))
}

func TestBedrockCCCompatPreservesRawBetaBlockPolicy(t *testing.T) {
	raw, err := json.Marshal(&BetaPolicySettings{Rules: []BetaPolicyRule{{
		BetaToken: "advanced-tool-use-2025-11-20", Action: BetaPolicyActionBlock,
		Scope: BetaPolicyScopeAll, ErrorMessage: "advanced tool use is blocked",
	}}})
	require.NoError(t, err)
	svc := bedrockCCCompatGateway(true)
	svc.cfg = &config.Config{}
	svc.settingService = NewSettingService(&betaPolicySettingRepoStub{values: map[string]string{
		SettingKeyBetaPolicySettings: string(raw),
	}}, &config.Config{})
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"type":"message","content":[]}`)),
	}}
	svc.httpUpstream = upstream
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Anthropic-Beta", "advanced-tool-use-2025-11-20")
	body := []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hello"}]}`)
	groupID := int64(10)
	converted := svc.ApplyBedrockCCCompat(c, body, "claude-opus-4-6", &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}, &groupID)
	_, err = svc.Forward(context.Background(), c, &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeBedrock,
		Credentials: map[string]any{"auth_mode": "apikey", "api_key": "test-only", "aws_region": "us-east-1"}}, &ParsedRequest{
		Model: "claude-opus-4-6", Body: NewRequestBodyRef(converted),
	})
	var blocked *BetaBlockedError
	require.ErrorAs(t, err, &blocked)
	require.Equal(t, "advanced tool use is blocked", blocked.Message)
	require.Nil(t, upstream.lastReq, "blocked raw beta must not reach Bedrock")
	require.Equal(t, "advanced-tool-use-2025-11-20", c.GetHeader("Anthropic-Beta"))
}

func TestApplyBedrockCCCompatMissingOrDisabledChannel(t *testing.T) {
	svc := bedrockCCCompatGateway(true)
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}
	require.False(t, svc.isBedrockCCCompatEnabled(context.Background(), account, nil))
	unknown := int64(20)
	require.False(t, svc.isBedrockCCCompatEnabled(context.Background(), account, &unknown))
	svc.channelService.cache.Store(&channelCache{loadedAt: time.Now(), channelByGroupID: map[int64]*Channel{
		10: {Status: StatusDisabled, FeaturesConfig: map[string]any{featureKeyBedrockCCCompat: true}},
	}})
	groupID := int64(10)
	require.False(t, svc.isBedrockCCCompatEnabled(context.Background(), account, &groupID))
}
