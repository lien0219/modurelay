//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func seedanceTestAccount() *Account {
	return &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "ark-secret", "base_url": "https://ark.cn-beijing.volces.com/api/v3",
		"openai_capabilities": []string{"seedance"},
		"model_mapping":       map[string]any{"video": "ep-seedance"},
	}}
}

func seedanceFirstClassTestAccount() *Account {
	account := seedanceTestAccount()
	account.Platform = PlatformSeedance
	delete(account.Credentials, "openai_capabilities")
	return account
}

func TestSeedanceNativeForwarding(t *testing.T) {
	body := []byte(`{"model":"video","content":[{"type":"text","text":"waves"},{"type":"image_url","image_url":{"url":"https://example.com/first.png"},"role":"first_frame"},{"type":"audio_url","audio_url":{"url":"https://example.com/audio.mp3"}}],"duration":5,"resolution":"720p","generate_audio":true,"future_field":{"keep":true}}`)
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodPost, "/api/v3/contents/generations/tasks", nil)
	result, err := svc.ForwardSeedance(context.Background(), c, seedanceFirstClassTestAccount(), SeedanceEndpointCreate, "", body)
	require.NoError(t, err)
	require.Empty(t, w.Body.String(), "accepted task response is deferred until ownership and billing state are saved")
	require.NotNil(t, result.DeferredMediaResponse)
	require.JSONEq(t, `{"id":"task-1"}`, string(result.DeferredMediaResponse.Body))
	require.Equal(t, "seedance:task-1", result.ResponseID)
	require.Zero(t, result.Usage.OutputTokens)
	require.Equal(t, "video", result.BillingModel)
	require.Equal(t, "720p", result.VideoResolution)
	require.Equal(t, 5, result.VideoDurationSeconds)
	require.Equal(t, "ep-seedance", result.UpstreamModel)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks", upstream.request.URL.String())
	require.Equal(t, "Bearer ark-secret", upstream.request.Header.Get("Authorization"))
	forwarded, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.Equal(t, "ep-seedance", gjson.GetBytes(forwarded, "model").String())
	for _, field := range []string{"content", "duration", "generate_audio", "future_field"} {
		require.Equal(t, gjson.GetBytes(body, field).Raw, gjson.GetBytes(forwarded, field).Raw)
	}
}

func TestSeedanceCompatibleVideoCreateAndStatus(t *testing.T) {
	createBody := []byte(`{"model":"video","prompt":"a quiet lake","seconds":6,"resolution":"720p","aspect_ratio":"16:9","watermark":false,"generate_audio":true}`)
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-2"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	result, err := svc.ForwardSeedanceCompatibleVideo(
		context.Background(), c, seedanceFirstClassTestAccount(), GrokMediaEndpointVideosGenerations,
		"", createBody, "application/json", "video",
	)
	require.NoError(t, err)
	require.Empty(t, w.Body.String(), "accepted response waits for task binding and pending billing storage")
	require.Equal(t, "seedance:task-2", result.ResponseID)
	require.Equal(t, "ep-seedance", result.UpstreamModel)
	require.Equal(t, "720p", result.VideoResolution)
	require.Equal(t, 6, result.VideoDurationSeconds)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks", upstream.request.URL.String())
	forwarded, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.Equal(t, "ep-seedance", gjson.GetBytes(forwarded, "model").String())
	require.Equal(t, "a quiet lake", gjson.GetBytes(forwarded, "content.0.text").String())
	require.Equal(t, "720p", gjson.GetBytes(forwarded, "resolution").String())
	require.Equal(t, "16:9", gjson.GetBytes(forwarded, "ratio").String())
	require.Equal(t, 6, int(gjson.GetBytes(forwarded, "duration").Int()))
	require.False(t, gjson.GetBytes(forwarded, "watermark").Bool())
	require.True(t, gjson.GetBytes(forwarded, "generate_audio").Bool())

	upstream = &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(
		`{"id":"task-2","status":"succeeded","model":"ep-seedance","resolution":"720p","duration":6,"content":{"video_url":"https://cdn.example.com/video.mp4"}}`,
	)}
	svc = &OpenAIGatewayService{httpUpstream: upstream}
	c, w = grokMediaContentTestContext(http.MethodGet, "/v1/videos/seedance:task-2", nil)
	result, err = svc.ForwardSeedanceCompatibleVideo(
		context.Background(), c, seedanceFirstClassTestAccount(), GrokMediaEndpointVideoStatus,
		"seedance:task-2", nil, "", "video",
	)
	require.NoError(t, err)
	require.Equal(t, "completed", gjson.GetBytes(w.Body.Bytes(), "status").String())
	require.Equal(t, "https://cdn.example.com/video.mp4", gjson.GetBytes(w.Body.Bytes(), "metadata.result_url").String())
	require.Equal(t, 1, result.VideoCount)
}

func TestSeedanceCompatibleVideoCancelUsesDelete(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodDelete, "/v1/videos/seedance:task-2", nil)
	result, err := svc.ForwardSeedanceCompatibleVideo(
		context.Background(), c, seedanceFirstClassTestAccount(), GrokMediaEndpointVideoCancel,
		"seedance:task-2", nil, "", "video",
	)
	require.NoError(t, err)
	require.Equal(t, "seedance:task-2", result.ResponseID)
	require.Equal(t, http.MethodDelete, upstream.request.Method)
	require.Equal(t, "/api/v3/contents/generations/tasks/task-2", upstream.request.URL.Path)
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestSeedanceCompatibleVideoCancelRejectsUnsupportedUpstream(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"route not found"}}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodDelete, "/v1/videos/seedance:task-2", nil)
	_, err := svc.ForwardSeedanceCompatibleVideo(
		context.Background(), c, seedanceFirstClassTestAccount(), GrokMediaEndpointVideoCancel,
		"seedance:task-2", nil, "", "video",
	)
	require.Error(t, err)
	require.Equal(t, http.StatusNotImplemented, w.Code)
	require.Contains(t, w.Body.String(), "does not support task cancellation")
}

func TestSeedanceStatusAndDelete(t *testing.T) {
	for _, status := range []string{"queued", "running", "failed", "cancelled", "expired", "deleted", "succeeded"} {
		t.Run(status, func(t *testing.T) {
			body := `{"id":"task-1","status":"` + status + `","model":"ep-seedance","content":{"video_url":"https://cdn.example.com/video.mp4"},"usage":{"completion_tokens":12345}}`
			upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(body)}
			svc := &OpenAIGatewayService{httpUpstream: upstream}
			c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/task-1", nil)
			result, err := svc.ForwardSeedance(context.Background(), c, seedanceFirstClassTestAccount(), SeedanceEndpointStatus, "seedance:task-1", nil)
			require.NoError(t, err)
			require.JSONEq(t, body, w.Body.String())
			require.Equal(t, "/api/v3/contents/generations/tasks/task-1", upstream.request.URL.Path)
			if status == "succeeded" {
				require.Equal(t, 12345, result.Usage.OutputTokens)
				require.Equal(t, 1, result.VideoCount, "only a successful task with a playable result is billable")
			} else {
				require.Zero(t, result.Usage.OutputTokens)
				require.Zero(t, result.VideoCount)
			}
		})
	}
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1","status":"succeeded"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, _ := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/task-1", nil)
	result, err := svc.ForwardSeedance(context.Background(), c, seedanceFirstClassTestAccount(), SeedanceEndpointStatus, "seedance:task-1", nil)
	require.NoError(t, err)
	require.Zero(t, result.VideoCount, "a successful state without playable content must not be billed")

	upstream = &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse("")}
	upstream.response.StatusCode = http.StatusNoContent
	svc = &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodDelete, "/api/v3/contents/generations/tasks/task-1", nil)
	_, err = svc.ForwardSeedance(context.Background(), c, seedanceFirstClassTestAccount(), SeedanceEndpointDelete, "seedance:task-1", nil)
	require.NoError(t, err)
	require.Equal(t, http.MethodDelete, upstream.request.Method)
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestSeedanceStatusUsesKuaiziPersistentVideoURL(t *testing.T) {
	body := `{"id":"kz-cgt-task-1","status":"succeeded","model":"doubao-seedance-2-0-fast-260128","content":{"video_url":"https://cdn.example.com/temporary.mp4","kz_video_url":"https://cdn.example.com/persistent.mp4"}}`
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(body)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/kz-cgt-task-1", nil)
	result, err := svc.ForwardSeedance(context.Background(), c, seedanceFirstClassTestAccount(), SeedanceEndpointStatus, "seedance:kz-cgt-task-1", nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, "https://cdn.example.com/persistent.mp4", gjson.GetBytes(w.Body.Bytes(), "content.kz_video_url").String())
}

func TestSeedanceNativeStatusRejectsUnsafeVideoURL(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(
		`{"id":"task-1","status":"succeeded","video_url":"https://localhost./video.mp4"}`,
	)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodGet, "/api/v3/contents/generations/tasks/task-1", nil)
	_, err := svc.ForwardSeedance(context.Background(), c, seedanceFirstClassTestAccount(), SeedanceEndpointStatus, "seedance:task-1", nil)
	require.ErrorContains(t, err, "unsupported video content URL")
	require.Empty(t, w.Body.String())
}

func TestSeedanceVideoResultURLRejectsUnsafeTargets(t *testing.T) {
	for _, rawURL := range []string{
		"http://cdn.example.com/video.mp4",
		"https://127.0.0.1/video.mp4",
		"https://[::ffff:127.0.0.1]/video.mp4",
		"https://100.64.0.1/video.mp4",
		"https://localhost./video.mp4",
		"https://metadata.google.internal/latest/meta-data",
		"https://metadata/video.mp4",
		"https://user:password@cdn.example.com/video.mp4",
		"//cdn.example.com/video.mp4",
		"data:video/mp4;base64,AAAA",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := validateSeedanceVideoResultURL(rawURL)
			require.ErrorContains(t, err, "unsupported video content URL")
		})
	}

	got, err := validateSeedanceVideoResultURL("https://cdn.example.com/video.mp4?token=signed")
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/video.mp4?token=signed", got)
}

func TestSeedanceCompatibleContentRejectsUnsafeResultURL(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(
		`{"id":"task-2","status":"succeeded","content":{"video_url":"https://169.254.169.254/latest/meta-data"}}`,
	)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodGet, "/v1/videos/seedance:task-2/content", nil)
	_, err := svc.ForwardSeedanceCompatibleVideo(
		context.Background(), c, seedanceFirstClassTestAccount(), GrokMediaEndpointVideoContent,
		"seedance:task-2", nil, "", "video",
	)
	require.ErrorContains(t, err, "unsupported video content URL")
	require.Empty(t, w.Header().Get("Location"))
}

func TestSeedanceValidationAndCapability(t *testing.T) {
	for _, body := range []string{`{`, `[]`, `{}`, `{"model":12,"content":[{}]}`, `{"model":"x","content":[]}`} {
		_, err := ParseSeedanceRequest([]byte(body))
		require.Error(t, err)
	}
	info, err := ParseSeedanceRequest([]byte(`{"model":"x","content":[{"type":"text","text":"first"},{"type":"text","text":"second"},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}`))
	require.NoError(t, err)
	require.Contains(t, string(info.ModerationBody()), "first")
	require.Contains(t, string(info.ModerationBody()), "second")
	require.Contains(t, string(info.ModerationBody()), "https://example.com/a.png")
	for _, id := range []string{"", "..", "a/b", "a?b", "a#b", "%2e%2e"} {
		_, err := buildSeedanceURL("https://example.com", SeedanceEndpointStatus, id)
		require.Error(t, err, id)
	}
	for _, base := range []string{"https://example.com", "https://example.com/api/v3/", "https://example.com/v3"} {
		url, err := buildSeedanceURL(base, SeedanceEndpointCreate, "")
		require.NoError(t, err)
		require.NotContains(t, url, "/v3/api/v3")
	}
	a := seedanceTestAccount()
	require.True(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	firstClass := seedanceFirstClassTestAccount()
	require.True(t, firstClass.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	firstClass.Type = AccountTypeOAuth
	require.False(t, firstClass.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	a.Type = AccountTypeOAuth
	require.False(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	a.Type = AccountTypeAPIKey
	delete(a.Credentials, "openai_capabilities")
	require.False(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
}

func TestSeedancePreservesUpstreamErrorsWithoutRetry(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"error":{"code":"QuotaExceeded","message":"quota exhausted"}}`)}
	upstream.response.StatusCode = 429
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodPost, "/api/v3/contents/generations/tasks", nil)
	_, err := svc.ForwardSeedance(context.Background(), c, seedanceTestAccount(), SeedanceEndpointCreate, "", []byte(`{"model":"video","content":[{"type":"text","text":"waves"}]}`))
	require.Error(t, err)
	require.Equal(t, 429, w.Code)
	require.Contains(t, w.Body.String(), "QuotaExceeded")
	require.Len(t, upstream.requests, 1)
}
