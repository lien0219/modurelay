package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

var (
	ErrCanvasModelRequestInvalid = infraerrors.New(http.StatusBadRequest, "CANVAS_MODEL_REQUEST_INVALID", "model provider configuration is invalid")
	ErrCanvasModelURLRejected    = infraerrors.New(http.StatusBadRequest, "CANVAS_MODEL_URL_REJECTED", "model provider URL must be a public HTTPS address")
	ErrCanvasModelProxyDisabled  = infraerrors.New(http.StatusServiceUnavailable, "CANVAS_MODEL_PROXY_UNAVAILABLE", "model list proxy is unavailable")
	ErrCanvasModelUpstreamAuth   = infraerrors.New(http.StatusBadGateway, "CANVAS_MODEL_UPSTREAM_AUTH_FAILED", "model provider rejected the API key")
	ErrCanvasModelUpstreamLimit  = infraerrors.New(http.StatusBadGateway, "CANVAS_MODEL_UPSTREAM_RATE_LIMITED", "model provider rate limited the request")
	ErrCanvasModelUpstream       = infraerrors.New(http.StatusBadGateway, "CANVAS_MODEL_UPSTREAM_FAILED", "model provider request failed")
	ErrCanvasModelResponse       = infraerrors.New(http.StatusBadGateway, "CANVAS_MODEL_RESPONSE_INVALID", "model provider returned an invalid model list")
)

type CanvasModelListRequest struct {
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	APIFormat string `json:"api_format"`
}

type CanvasModelListResult struct {
	Models []string `json:"models"`
}

func (s *CanvasService) FetchProviderModels(ctx context.Context, userID int64, input CanvasModelListRequest) (*CanvasModelListResult, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	if s.httpUpstream == nil {
		return nil, ErrCanvasModelProxyDisabled
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		return nil, ErrCanvasModelRequestInvalid.WithMetadata(map[string]string{"field": "api_key"})
	}
	targetURL, apiFormat, err := buildCanvasModelListURL(input.BaseURL, input.APIFormat)
	if err != nil {
		return nil, ErrCanvasModelURLRejected.WithCause(err)
	}
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, ErrCanvasModelURLRejected.WithCause(err)
	}

	resolveHost := s.resolveModelHost
	if resolveHost == nil {
		resolveHost = urlvalidator.ResolveAndPinHost
	}
	requestCtx, err := resolveHost(ctx, target.Hostname())
	if err != nil {
		return nil, ErrCanvasModelURLRejected.WithCause(err)
	}
	requestCtx = WithHTTPUpstreamPublicHostsOnly(requestCtx)
	requestCtx = WithHTTPUpstreamResolvedIPPinning(requestCtx)
	requestCtx = WithHTTPUpstreamRedirectsDisabled(requestCtx)
	if apiFormat == "openai" {
		requestCtx = WithHTTPUpstreamProfile(requestCtx, HTTPUpstreamProfileOpenAI)
	}

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, ErrCanvasModelRequestInvalid.WithCause(err)
	}
	req.Header.Set("Accept", "application/json")
	if apiFormat == "gemini" {
		req.Header.Set("x-goog-api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.httpUpstream.Do(req, "", 0, 1)
	if err != nil {
		return nil, ErrCanvasModelUpstream.WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyLimit := s.modelsListReadMaxBytes
	if bodyLimit <= 0 {
		bodyLimit = resolveModelsListReadLimit(nil)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	if err != nil {
		return nil, ErrCanvasModelUpstream.WithCause(err)
	}
	if int64(len(body)) > bodyLimit {
		return nil, ErrCanvasModelResponse.WithCause(fmt.Errorf("model list response exceeds configured limit"))
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, ErrCanvasModelUpstreamAuth
		case http.StatusTooManyRequests:
			return nil, ErrCanvasModelUpstreamLimit
		default:
			return nil, ErrCanvasModelUpstream.WithMetadata(map[string]string{"upstream_status": fmt.Sprintf("%d", resp.StatusCode)})
		}
	}

	models, err := extractUpstreamModelIDs(body)
	if err != nil {
		return nil, ErrCanvasModelResponse.WithCause(err)
	}
	return &CanvasModelListResult{Models: models}, nil
}

func buildCanvasModelListURL(rawBaseURL, rawAPIFormat string) (string, string, error) {
	apiFormat := strings.ToLower(strings.TrimSpace(rawAPIFormat))
	if apiFormat == "" {
		apiFormat = "openai"
	}
	if apiFormat != "openai" && apiFormat != "gemini" {
		return "", "", errors.New("unsupported API format")
	}

	baseURL, err := urlvalidator.ValidateHTTPSURL(rawBaseURL, urlvalidator.ValidationOptions{})
	if err != nil {
		return "", "", err
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", "", err
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", "", errors.New("provider base URL contains unsupported components")
	}

	if apiFormat == "gemini" {
		baseURL = buildCanvasGeminiModelsURL(baseURL)
	} else {
		baseURL = buildV1ModelsURL(baseURL)
	}
	validated, err := urlvalidator.ValidateHTTPSURL(baseURL, urlvalidator.ValidationOptions{})
	if err != nil {
		return "", "", err
	}
	return validated, apiFormat, nil
}

func buildCanvasGeminiModelsURL(base string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	lower := strings.ToLower(normalized)
	if strings.HasSuffix(lower, "/v1/models") || strings.HasSuffix(lower, "/v1beta/models") {
		return normalized
	}
	if strings.HasSuffix(lower, "/v1") || strings.HasSuffix(lower, "/v1beta") {
		return normalized + "/models"
	}
	return normalized + "/v1beta/models"
}
