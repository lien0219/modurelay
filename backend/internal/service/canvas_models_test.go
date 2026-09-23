package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type canvasModelsSettingRepo struct{}

func (canvasModelsSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (canvasModelsSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if key == SettingKeyCanvasEnabled {
		return "true", nil
	}
	return "", ErrSettingNotFound
}
func (canvasModelsSettingRepo) Set(context.Context, string, string) error { return nil }
func (canvasModelsSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (canvasModelsSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (canvasModelsSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (canvasModelsSettingRepo) Delete(context.Context, string) error { return nil }

type canvasModelsHTTPUpstream struct {
	request  *http.Request
	response *http.Response
	err      error
}

func (u *canvasModelsHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	return u.response, u.err
}

func (u *canvasModelsHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func newCanvasModelsService(upstream HTTPUpstream) *CanvasService {
	service := NewCanvasService(nil, canvasModelsSettingRepo{}, nil, nil)
	service.httpUpstream = upstream
	service.modelsListReadMaxBytes = 1024
	service.resolveModelHost = func(ctx context.Context, _ string) (context.Context, error) { return ctx, nil }
	return service
}

func TestBuildCanvasModelListURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiFormat string
		wantURL   string
		wantErr   bool
	}{
		{name: "openai base", baseURL: "https://provider.example", apiFormat: "openai", wantURL: "https://provider.example/v1/models"},
		{name: "openai v1", baseURL: "https://provider.example/v1/", wantURL: "https://provider.example/v1/models"},
		{name: "openai models", baseURL: "https://provider.example/v1/models", wantURL: "https://provider.example/v1/models"},
		{name: "gemini", baseURL: "https://generativelanguage.googleapis.com", apiFormat: "gemini", wantURL: "https://generativelanguage.googleapis.com/v1beta/models"},
		{name: "gemini v1", baseURL: "https://provider.example/v1", apiFormat: "gemini", wantURL: "https://provider.example/v1/models"},
		{name: "gemini models", baseURL: "https://provider.example/v1beta/models/", apiFormat: "gemini", wantURL: "https://provider.example/v1beta/models"},
		{name: "reject http", baseURL: "http://provider.example", wantErr: true},
		{name: "reject loopback", baseURL: "https://127.0.0.1", wantErr: true},
		{name: "reject credentials", baseURL: "https://user:pass@provider.example", wantErr: true},
		{name: "reject query", baseURL: "https://provider.example?target=internal", wantErr: true},
		{name: "reject unsupported format", baseURL: "https://provider.example", apiFormat: "custom", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, _, err := buildCanvasModelListURL(tt.baseURL, tt.apiFormat)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantURL, gotURL)
		})
	}
}

func TestCanvasFetchProviderModelsUsesGuardedOpenAIRequest(t *testing.T) {
	upstream := &canvasModelsHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"model-b"},{"id":"model-a"},{"id":"model-a"}]}`)),
	}}
	service := newCanvasModelsService(upstream)
	resolvedHost := ""
	service.resolveModelHost = func(ctx context.Context, host string) (context.Context, error) {
		resolvedHost = host
		return ctx, nil
	}

	result, err := service.FetchProviderModels(t.Context(), 42, CanvasModelListRequest{
		BaseURL:   "https://provider.example/v1",
		APIKey:    "test-provider-key",
		APIFormat: "openai",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"model-a", "model-b"}, result.Models)
	require.Equal(t, "provider.example", resolvedHost)
	require.Equal(t, "https://provider.example/v1/models", upstream.request.URL.String())
	require.Equal(t, "Bearer test-provider-key", upstream.request.Header.Get("Authorization"))
	require.Empty(t, upstream.request.Header.Get("x-goog-api-key"))
	require.True(t, HTTPUpstreamPublicHostsOnly(upstream.request.Context()))
	require.True(t, HTTPUpstreamResolvedIPPinningRequired(upstream.request.Context()))
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.request.Context()))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.request.Context()))
}

func TestCanvasFetchProviderModelsUsesGeminiHeader(t *testing.T) {
	upstream := &canvasModelsHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"models":[{"name":"models/gemini-test"}]}`)),
	}}
	service := newCanvasModelsService(upstream)

	result, err := service.FetchProviderModels(t.Context(), 42, CanvasModelListRequest{
		BaseURL:   "https://generativelanguage.googleapis.com/v1beta",
		APIKey:    "test-gemini-key",
		APIFormat: "gemini",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-test"}, result.Models)
	require.Equal(t, "test-gemini-key", upstream.request.Header.Get("x-goog-api-key"))
	require.Empty(t, upstream.request.Header.Get("Authorization"))
	require.Equal(t, HTTPUpstreamProfileDefault, HTTPUpstreamProfileFromContext(upstream.request.Context()))
}

func TestCanvasFetchProviderModelsMapsSafeFailures(t *testing.T) {
	tests := []struct {
		name     string
		response *http.Response
		err      error
		want     error
		limit    int64
	}{
		{name: "authentication", response: &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"error":"secret upstream detail"}`))}, want: ErrCanvasModelUpstreamAuth},
		{name: "rate limit", response: &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(`{}`))}, want: ErrCanvasModelUpstreamLimit},
		{name: "transport", err: errors.New("dial failed"), want: ErrCanvasModelUpstream},
		{name: "invalid response", response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`not-json`))}, want: ErrCanvasModelResponse},
		{name: "oversized response", response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, want: ErrCanvasModelResponse, limit: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &canvasModelsHTTPUpstream{response: tt.response, err: tt.err}
			service := newCanvasModelsService(upstream)
			if tt.limit > 0 {
				service.modelsListReadMaxBytes = tt.limit
			}
			_, err := service.FetchProviderModels(t.Context(), 42, CanvasModelListRequest{
				BaseURL: "https://provider.example",
				APIKey:  "test-provider-key",
			})
			require.ErrorIs(t, err, tt.want)
			require.NotContains(t, err.Error(), "test-provider-key")
			require.NotContains(t, err.Error(), "secret upstream detail")
		})
	}
}

func TestCanvasFetchProviderModelsRejectsResolutionFailure(t *testing.T) {
	upstream := &canvasModelsHTTPUpstream{}
	service := newCanvasModelsService(upstream)
	service.resolveModelHost = func(context.Context, string) (context.Context, error) {
		return nil, errors.New("resolved private address")
	}

	_, err := service.FetchProviderModels(t.Context(), 42, CanvasModelListRequest{
		BaseURL: "https://provider.example",
		APIKey:  "test-provider-key",
	})

	require.ErrorIs(t, err, ErrCanvasModelURLRejected)
	require.Nil(t, upstream.request)
}
