package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const canvasProviderTargetURLMaxBytes = 16 * 1024

var (
	ErrCanvasProviderRequestInvalid = infraerrors.New(http.StatusBadRequest, "CANVAS_PROVIDER_REQUEST_INVALID", "canvas provider request is invalid")
	ErrCanvasProviderTargetRejected = infraerrors.New(http.StatusBadRequest, "CANVAS_PROVIDER_TARGET_REJECTED", "canvas provider target must be an allowed public HTTPS endpoint")
	ErrCanvasProviderProxyDisabled  = infraerrors.New(http.StatusServiceUnavailable, "CANVAS_PROVIDER_PROXY_UNAVAILABLE", "canvas provider proxy is unavailable")
	ErrCanvasProviderUpstream       = infraerrors.New(http.StatusBadGateway, "CANVAS_PROVIDER_UPSTREAM_FAILED", "canvas provider request failed")
	ErrCanvasProviderResponseLarge  = infraerrors.New(http.StatusBadGateway, "CANVAS_PROVIDER_RESPONSE_TOO_LARGE", "canvas provider response exceeds the configured limit")
)

type CanvasProviderProxyResult struct {
	Response *http.Response
	MaxBytes int64
}

func (s *CanvasService) ProxyProviderRequest(ctx context.Context, userID int64, incoming *http.Request, rawTargetURL, providerAuthorization string) (*CanvasProviderProxyResult, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	if s.httpUpstream == nil {
		return nil, ErrCanvasProviderProxyDisabled
	}
	if incoming == nil {
		return nil, ErrCanvasProviderRequestInvalid
	}

	method := strings.ToUpper(strings.TrimSpace(incoming.Method))
	if method != http.MethodGet && method != http.MethodPost {
		return nil, ErrCanvasProviderRequestInvalid.WithMetadata(map[string]string{"field": "method"})
	}

	targetURL, target, err := validateCanvasProviderTarget(rawTargetURL, method)
	if err != nil {
		return nil, ErrCanvasProviderTargetRejected.WithCause(err)
	}
	resolveHost := s.resolveModelHost
	if resolveHost == nil {
		resolveHost = urlvalidator.ResolveAndPinHost
	}
	requestCtx, err := resolveHost(ctx, target.Hostname())
	if err != nil {
		return nil, ErrCanvasProviderTargetRejected.WithCause(err)
	}
	requestCtx = WithHTTPUpstreamPublicHostsOnly(requestCtx)
	requestCtx = WithHTTPUpstreamResolvedIPPinning(requestCtx)
	if method == http.MethodPost {
		requestCtx = WithHTTPUpstreamRedirectsDisabled(requestCtx)
	}
	if canvasProviderUsesOpenAIProfile(target.Path) {
		requestCtx = WithHTTPUpstreamProfile(requestCtx, HTTPUpstreamProfileOpenAI)
	}

	upstreamRequest, err := http.NewRequestWithContext(requestCtx, method, targetURL, incoming.Body)
	if err != nil {
		return nil, ErrCanvasProviderRequestInvalid.WithCause(err)
	}
	upstreamRequest.ContentLength = incoming.ContentLength
	copyCanvasProviderRequestHeaders(upstreamRequest.Header, incoming.Header)
	if authorization := strings.TrimSpace(providerAuthorization); authorization != "" {
		upstreamRequest.Header.Set("Authorization", authorization)
	}

	response, err := s.httpUpstream.Do(upstreamRequest, "", userID, 1)
	if err != nil {
		return nil, ErrCanvasProviderUpstream.WithCause(err)
	}
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		_ = response.Body.Close()
		return nil, ErrCanvasProviderUpstream.WithCause(errors.New("canvas provider redirect was not accepted"))
	}

	maxBytes := s.providerResponseReadMaxBytes
	if maxBytes <= 0 {
		maxBytes = config.DefaultUpstreamResponseReadMaxBytes
	}
	if response.ContentLength > maxBytes {
		_ = response.Body.Close()
		return nil, ErrCanvasProviderResponseLarge
	}
	return &CanvasProviderProxyResult{Response: response, MaxBytes: maxBytes}, nil
}

func validateCanvasProviderTarget(rawTargetURL, method string) (string, *url.URL, error) {
	if method != http.MethodGet && method != http.MethodPost {
		return "", nil, errors.New("provider request method is not allowed")
	}
	rawTargetURL = strings.TrimSpace(rawTargetURL)
	if rawTargetURL == "" || len(rawTargetURL) > canvasProviderTargetURLMaxBytes {
		return "", nil, errors.New("provider target URL is missing or too long")
	}
	validated, err := urlvalidator.ValidateHTTPSURL(rawTargetURL, urlvalidator.ValidationOptions{})
	if err != nil {
		return "", nil, err
	}
	target, err := url.Parse(validated)
	if err != nil {
		return "", nil, err
	}
	if target.User != nil || target.Fragment != "" || target.Opaque != "" {
		return "", nil, errors.New("provider target URL contains unsupported components")
	}
	if method == http.MethodPost && !canvasProviderPOSTPathAllowed(target.Path) {
		return "", nil, errors.New("provider target path is not allowed")
	}
	return validated, target, nil
}

func canvasProviderPOSTPathAllowed(rawPath string) bool {
	providerPath := strings.ToLower(strings.TrimRight(rawPath, "/"))
	for _, suffix := range []string{
		"/images/generations",
		"/images/edits",
		"/responses",
		"/chat/completions",
		"/audio/speech",
		"/videos",
	} {
		if strings.HasSuffix(providerPath, suffix) {
			return true
		}
	}
	if !strings.Contains(providerPath, "/models/") {
		return false
	}
	return strings.HasSuffix(providerPath, ":generatecontent") ||
		strings.HasSuffix(providerPath, ":streamgeneratecontent") ||
		strings.HasSuffix(providerPath, ":predictlongrunning")
}

func canvasProviderUsesOpenAIProfile(rawPath string) bool {
	providerPath := strings.ToLower(rawPath)
	return strings.Contains(providerPath, "/v1/") || strings.HasSuffix(providerPath, "/v1")
}

func copyCanvasProviderRequestHeaders(destination, source http.Header) {
	for key, values := range source {
		if canvasProviderRequestHeaderBlocked(key) {
			continue
		}
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

func canvasProviderRequestHeaderBlocked(rawKey string) bool {
	key := strings.ToLower(strings.TrimSpace(rawKey))
	if strings.HasPrefix(key, "sec-") || strings.HasPrefix(key, "x-forwarded-") || strings.HasPrefix(key, "x-modurelay-") || strings.HasPrefix(key, "x-canvas-") {
		return true
	}
	switch key {
	case "authorization", "cookie", "host", "connection", "content-length", "accept-encoding", "origin", "referer", "forwarded", "proxy-authorization", "proxy-authenticate", "te", "trailer", "transfer-encoding", "upgrade", "x-user-ui-request":
		return true
	default:
		return false
	}
}
