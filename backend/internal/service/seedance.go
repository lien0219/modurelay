package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	SeedanceEndpointCreate           GrokMediaEndpoint        = "seedance_create"
	SeedanceEndpointStatus           GrokMediaEndpoint        = "seedance_status"
	SeedanceEndpointDelete           GrokMediaEndpoint        = "seedance_delete"
	OpenAIEndpointCapabilitySeedance OpenAIEndpointCapability = "seedance"
)

func (e GrokMediaEndpoint) IsSeedance() bool {
	return e == SeedanceEndpointCreate || e == SeedanceEndpointStatus || e == SeedanceEndpointDelete
}

// SeedanceTaskKey isolates ownership and billing keys from other video providers.
func SeedanceTaskKey(id string) string { return "seedance:" + strings.TrimSpace(id) }

func seedanceVideoURLFromResponse(body []byte) string {
	return strings.TrimSpace(compatibleVideoFirstNonEmpty(
		gjson.GetBytes(body, "content.kz_video_url").String(),
		gjson.GetBytes(body, "content.video_url").String(),
		gjson.GetBytes(body, "kz_video_url").String(),
		gjson.GetBytes(body, "video_url").String(),
	))
}

func ParseSeedanceRequest(body []byte) (GrokMediaRequestInfo, error) {
	var info GrokMediaRequestInfo
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return info, fmt.Errorf("request body must be a JSON object")
	}
	info = ParseGrokMediaRequest("application/json", body)
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || strings.TrimSpace(model.String()) == "" {
		return info, fmt.Errorf("model is required")
	}
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() || len(content.Array()) == 0 {
		return info, fmt.Errorf("content must be a non-empty array")
	}
	info.Model = strings.TrimSpace(model.String())
	var texts []string
	for _, item := range content.Array() {
		switch item.Get("type").String() {
		case "text":
			texts = append(texts, item.Get("text").String())
		case "image_url":
			info.InputImageURLs = append(info.InputImageURLs, item.Get("image_url.url").String())
		}
	}
	info.Prompt = strings.Join(texts, "\n")
	return info, nil
}

func buildSeedanceURL(base string, endpoint GrokMediaEndpoint, taskID string) (string, error) {
	base = buildSeedanceAPIBase(base)
	base += "/contents/generations/tasks"
	if endpoint != SeedanceEndpointCreate {
		if err := validateUpstreamPathSegment("Seedance task ID", taskID); err != nil || strings.TrimSpace(taskID) == "" {
			return "", fmt.Errorf("invalid Seedance task ID")
		}
		base += "/" + taskID
	}
	return base, nil
}

const kuaiziSeedanceHost = "aiopenapi.kuaizi.cn"

func isKuaiziSeedanceBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && strings.EqualFold(parsed.Hostname(), kuaiziSeedanceHost)
}

func buildSeedanceAPIBase(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if isKuaiziSeedanceBaseURL(base) {
		parsed, err := url.Parse(base)
		if err == nil {
			path := strings.TrimRight(parsed.Path, "/")
			switch {
			case strings.HasSuffix(strings.ToLower(path), "/ai-open-platform-api/api/v3"):
			case strings.HasSuffix(strings.ToLower(path), "/ai-open-platform-api"):
				path += "/api/v3"
			case strings.HasSuffix(strings.ToLower(path), "/api/v3"):
				path = path[:len(path)-len("/api/v3")] + "/ai-open-platform-api/api/v3"
			case strings.HasSuffix(strings.ToLower(path), "/v3"):
				path = path[:len(path)-len("/v3")] + "/ai-open-platform-api/api/v3"
			default:
				path += "/ai-open-platform-api/api/v3"
			}
			parsed.Path = path
			parsed.RawPath = ""
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return strings.TrimRight(parsed.String(), "/")
		}
		return base + "/ai-open-platform-api/api/v3"
	}
	// Accept an origin, a proxy prefix, or the full Ark API base.
	if !strings.HasSuffix(base, "/api/v3") && !strings.HasSuffix(base, "/v3") {
		base += "/api/v3"
	}
	return base
}

// ForwardSeedance preserves the Ark protocol, including multimodal content and
// future fields. Only model is rewritten using the account's configured mapping.
func (s *OpenAIGatewayService) ForwardSeedance(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, taskID string, body []byte) (*OpenAIForwardResult, error) {
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance) || !endpoint.IsSeedance() {
		return nil, fmt.Errorf("seedance requires a Seedance API key account or a legacy OpenAI API key account with Seedance capability")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	target, err := buildSeedanceURL(base, endpoint, strings.TrimPrefix(taskID, "seedance:"))
	if err != nil {
		return nil, err
	}
	model, upstreamModel := "", ""
	var info GrokMediaRequestInfo
	method := http.MethodGet
	switch endpoint {
	case SeedanceEndpointCreate:
		var parseErr error
		info, parseErr = ParseSeedanceRequest(body)
		if parseErr != nil {
			return nil, parseErr
		}
		model = info.Model
		upstreamModel = account.GetMappedModel(model)
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, err
		}
		method = http.MethodPost
	case SeedanceEndpointDelete:
		method = http.MethodDelete
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("seedance account missing api_key")
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	markVideoProviderStarted(ctx, endpoint)
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	markVideoProviderRejected(ctx, endpoint, resp.StatusCode)
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	// Do not retry ambiguous asynchronous creates: the upstream may already have
	// accepted a billable job. Preserve native error codes and response bodies.
	if resp.StatusCode >= 300 {
		writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("seedance upstream status %d", resp.StatusCode)
	}
	result := &OpenAIForwardResult{
		Model: model, BillingModel: model, UpstreamModel: upstreamModel,
		Duration: time.Since(started), ResponseHeaders: resp.Header.Clone(),
	}
	if endpoint == SeedanceEndpointCreate {
		result.VideoResolution = info.Resolution
		result.VideoDurationSeconds = info.DurationSeconds
		id := strings.TrimSpace(gjson.GetBytes(responseBody, "id").String())
		if id == "" {
			return nil, fmt.Errorf("seedance create response missing task ID")
		}
		result.ResponseID = SeedanceTaskKey(id)
		result.DeferredMediaResponse = &DeferredMediaResponse{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       append([]byte(nil), responseBody...),
		}
	}
	if endpoint == SeedanceEndpointStatus {
		result.ResponseID = taskID
		result.UpstreamModel = gjson.GetBytes(responseBody, "model").String()
		videoURL := seedanceVideoURLFromResponse(responseBody)
		if videoURL != "" {
			if _, err := validateSeedanceVideoResultURL(videoURL); err != nil {
				return nil, err
			}
		}
		if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(responseBody, "status").String()), "succeeded") && videoURL != "" {
			result.VideoCount = 1
			result.VideoResolution = strings.TrimSpace(gjson.GetBytes(responseBody, "resolution").String())
			result.VideoDurationSeconds = int(gjson.GetBytes(responseBody, "duration").Int())
			result.Usage.OutputTokens = max(0, int(gjson.GetBytes(responseBody, "usage.completion_tokens").Int()))
		}
	}
	if endpoint != SeedanceEndpointCreate {
		writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
	}
	return result, nil
}
