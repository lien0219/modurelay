package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type canvasProviderHTTPUpstream struct {
	request   *http.Request
	response  *http.Response
	err       error
	accountID int64
}

func (u *canvasProviderHTTPUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.request = req
	u.accountID = accountID
	return u.response, u.err
}

func (u *canvasProviderHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func newCanvasProviderService(upstream HTTPUpstream) *CanvasService {
	service := NewCanvasService(nil, canvasModelsSettingRepo{}, nil, nil)
	service.httpUpstream = upstream
	service.providerResponseReadMaxBytes = 1024
	service.resolveModelHost = func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }
	return service
}

func TestCanvasProxyProviderRequestForwardsGuardedRequest(t *testing.T) {
	upstream := &canvasProviderHTTPUpstream{response: &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: 2,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(`{}`)),
	}}
	service := newCanvasProviderService(upstream)
	incoming, err := http.NewRequest(http.MethodPost, "/api/v1/canvas/upstream", strings.NewReader(`{"model":"image-test"}`))
	require.NoError(t, err)
	incoming.Header.Set("Authorization", "Bearer modurelay-session-token")
	incoming.Header.Set("X-Canvas-Upstream-URL", "https://provider.example/v1/images/generations")
	incoming.Header.Set("X-Canvas-Provider-Authorization", "Bearer provider-key")
	incoming.Header.Set("Cookie", "session=must-not-forward")
	incoming.Header.Set("Origin", "https://modurelay.example")
	incoming.Header.Set("X-Forwarded-For", "127.0.0.1")
	incoming.Header.Set("Content-Type", "application/json")
	incoming.Header.Set("OpenAI-Beta", "responses=v1")

	result, err := service.ProxyProviderRequest(t.Context(), 42, incoming, incoming.Header.Get("X-Canvas-Upstream-URL"), incoming.Header.Get("X-Canvas-Provider-Authorization"))

	require.NoError(t, err)
	require.Equal(t, upstream.response, result.Response)
	require.Equal(t, int64(1024), result.MaxBytes)
	require.Equal(t, int64(42), upstream.accountID)
	require.Equal(t, "https://provider.example/v1/images/generations", upstream.request.URL.String())
	require.Equal(t, "Bearer provider-key", upstream.request.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.request.Header.Get("Content-Type"))
	require.Equal(t, "responses=v1", upstream.request.Header.Get("OpenAI-Beta"))
	require.Empty(t, upstream.request.Header.Get("Cookie"))
	require.Empty(t, upstream.request.Header.Get("Origin"))
	require.Empty(t, upstream.request.Header.Get("X-Forwarded-For"))
	require.Empty(t, upstream.request.Header.Get("X-Canvas-Upstream-URL"))
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.request.Context()))
	require.True(t, HTTPUpstreamResolvedIPPinningRequired(upstream.request.Context()))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.request.Context()))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.request.Context()))
	body, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"image-test"}`, string(body))
}

func TestCanvasProxyProviderRequestAllowsGuardedMediaGet(t *testing.T) {
	upstream := &canvasProviderHTTPUpstream{response: &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: 5,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader("video")),
	}}
	service := newCanvasProviderService(upstream)
	incoming, err := http.NewRequest(http.MethodGet, "/api/v1/canvas/upstream", nil)
	require.NoError(t, err)

	_, err = service.ProxyProviderRequest(t.Context(), 9, incoming, "https://cdn.example/generated/item?id=1", "")

	require.NoError(t, err)
	require.Equal(t, "https://cdn.example/generated/item?id=1", upstream.request.URL.String())
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.request.Context()))
	require.True(t, HTTPUpstreamResolvedIPPinningRequired(upstream.request.Context()))
	require.False(t, HTTPUpstreamRedirectsDisabled(upstream.request.Context()))
}

func TestCanvasProxyProviderRequestPreservesMultipartBoundary(t *testing.T) {
	upstream := &canvasProviderHTTPUpstream{response: &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: 2,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(`{}`)),
	}}
	service := newCanvasProviderService(upstream)
	body := "--canvas-boundary\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\ntest\r\n--canvas-boundary--\r\n"
	incoming, err := http.NewRequest(http.MethodPost, "/api/v1/canvas/upstream", strings.NewReader(body))
	require.NoError(t, err)
	incoming.Header.Set("Content-Type", "multipart/form-data; boundary=canvas-boundary")

	_, err = service.ProxyProviderRequest(t.Context(), 42, incoming, "https://provider.example/v1/images/edits", "Bearer provider-key")

	require.NoError(t, err)
	require.Equal(t, "multipart/form-data; boundary=canvas-boundary", upstream.request.Header.Get("Content-Type"))
	forwarded, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(forwarded))
}

func TestValidateCanvasProviderTarget(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		target  string
		wantErr bool
	}{
		{name: "openai image", method: http.MethodPost, target: "https://provider.example/v1/images/generations"},
		{name: "azure image", method: http.MethodPost, target: "https://provider.example/openai/deployments/image/images/generations?api-version=2025-04-01-preview"},
		{name: "gemini image", method: http.MethodPost, target: "https://provider.example/v1beta/models/gemini-image:generateContent"},
		{name: "gemini video", method: http.MethodPost, target: "https://provider.example/v1beta/models/veo:predictLongRunning"},
		{name: "public media", method: http.MethodGet, target: "https://cdn.example/result/no-extension?signature=value"},
		{name: "reject arbitrary post", method: http.MethodPost, target: "https://provider.example/admin/reset", wantErr: true},
		{name: "reject http", method: http.MethodGet, target: "http://provider.example/media", wantErr: true},
		{name: "reject private", method: http.MethodGet, target: "https://127.0.0.1/media", wantErr: true},
		{name: "reject credentials", method: http.MethodGet, target: "https://user:pass@provider.example/media", wantErr: true},
		{name: "reject fragment", method: http.MethodGet, target: "https://provider.example/media#secret", wantErr: true},
		{name: "reject method", method: http.MethodDelete, target: "https://provider.example/v1/videos/id", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := validateCanvasProviderTarget(tt.target, tt.method)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCanvasProxyProviderRequestRejectsKnownOversizedResponse(t *testing.T) {
	upstream := &canvasProviderHTTPUpstream{response: &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: 2048,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader("oversized")),
	}}
	service := newCanvasProviderService(upstream)
	incoming, err := http.NewRequest(http.MethodGet, "/api/v1/canvas/upstream", nil)
	require.NoError(t, err)

	_, err = service.ProxyProviderRequest(t.Context(), 9, incoming, "https://cdn.example/result", "")

	require.ErrorIs(t, err, ErrCanvasProviderResponseLarge)
}
