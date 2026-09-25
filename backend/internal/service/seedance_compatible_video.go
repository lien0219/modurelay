package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func prepareSeedanceCompatibleCreateBody(account *Account, body []byte, contentType, routingModel string) ([]byte, GrokMediaRequestInfo, string, error) {
	if account == nil {
		return nil, GrokMediaRequestInfo{}, "", fmt.Errorf("seedance account is required")
	}
	if !gjson.ValidBytes(body) {
		return nil, GrokMediaRequestInfo{}, "", fmt.Errorf("official Seedance video create requires application/json")
	}
	info := ParseGrokMediaRequest(contentType, body)
	canonical := strings.TrimSpace(CanonicalVideoModel(compatibleVideoFirstNonEmpty(routingModel, info.Model)))
	if canonical == "" {
		canonical = strings.TrimSpace(CanonicalVideoModel(info.Model))
	}
	upstreamModel := strings.TrimSpace(account.GetMappedModel(canonical))
	if upstreamModel == "" {
		upstreamModel = canonical
	}
	content := make([]map[string]any, 0, len(info.InputImageURLs)+1)
	if info.Prompt != "" {
		content = append(content, map[string]any{"type": "text", "text": info.Prompt})
	}
	for _, imageURL := range info.InputImageURLs {
		if imageURL = strings.TrimSpace(imageURL); imageURL != "" {
			item := map[string]any{}
			item["type"] = "image_url"
			item["image_url"] = map[string]any{"url": imageURL}
			item["role"] = "reference_image"
			content = append(content, item)
		}
	}
	if len(content) == 0 {
		return nil, info, upstreamModel, fmt.Errorf("official Seedance requires prompt or reference content")
	}
	payload := map[string]any{"model": upstreamModel, "content": content}
	if v := strings.TrimSpace(compatibleVideoFirstNonEmpty(
		gjson.GetBytes(body, "resolution").String(),
		gjson.GetBytes(body, "metadata.resolution").String(),
	)); v != "" {
		payload["resolution"] = v
	}
	if v := strings.TrimSpace(compatibleVideoFirstNonEmpty(
		gjson.GetBytes(body, "ratio").String(),
		gjson.GetBytes(body, "aspect_ratio").String(),
		gjson.GetBytes(body, "size").String(),
	)); v != "" {
		payload["ratio"] = v
	}
	if info.DurationSeconds > 0 {
		payload["duration"] = info.DurationSeconds
	}
	for _, field := range []string{"generate_audio", "watermark", "camera_fixed"} {
		if v := gjson.GetBytes(body, field); v.Exists() {
			payload[field] = v.Value()
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, info, upstreamModel, err
	}
	return encoded, info, upstreamModel, nil
}

func (s *OpenAIGatewayService) doSeedanceCompatibleRequest(ctx context.Context, c *gin.Context, account *Account, method, target string, body []byte) ([]byte, http.Header, error) {
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, nil, fmt.Errorf("seedance account missing api_key")
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	started := time.Now()
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		// Never fail over an ambiguous async creation transport failure.
		return nil, nil, fmt.Errorf("seedance upstream transport failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, resp.Header.Clone(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Clone(), fmt.Errorf("seedance upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, resp.Header.Clone(), nil
}

func seedanceCompatibleStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return "queued"
	case "running":
		return "in_progress"
	case "succeeded":
		return "completed"
	case "failed", "cancelled", "canceled", "expired":
		return "failed"
	default:
		return "queued"
	}
}

func (s *OpenAIGatewayService) ForwardSeedanceCompatibleVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	requestID string,
	body []byte,
	contentType string,
	routingModel string,
) (*OpenAIForwardResult, error) {
	if account == nil || !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance) {
		return nil, fmt.Errorf("official Seedance account is not eligible")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	started := time.Now()

	switch endpoint {
	case GrokMediaEndpointVideosGenerations:
		createBody, info, upstreamModel, err := prepareSeedanceCompatibleCreateBody(account, body, contentType, routingModel)
		if err != nil {
			return nil, err
		}
		target, err := buildSeedanceURL(base, SeedanceEndpointCreate, "")
		if err != nil {
			return nil, err
		}
		responseBody, headers, err := s.doSeedanceCompatibleRequest(ctx, c, account, http.MethodPost, target, createBody)
		if err != nil {
			return nil, err
		}
		rawID := strings.TrimSpace(gjson.GetBytes(responseBody, "id").String())
		if rawID == "" {
			return nil, fmt.Errorf("seedance create response missing task ID")
		}
		publicID := SeedanceTaskKey(rawID)
		response := gin.H{}
		response["id"] = publicID
		response["object"] = "video"
		response["model"] = info.Model
		response["status"] = "queued"
		response["progress"] = 0
		response["created_at"] = time.Now().Unix()
		c.JSON(http.StatusOK, response)
		return &OpenAIForwardResult{
			ResponseID: publicID, Model: info.Model, BillingModel: info.Model,
			UpstreamModel: upstreamModel, ResponseHeaders: headers,
			Duration: time.Since(started), VideoResolution: info.Resolution,
			VideoDurationSeconds: info.DurationSeconds,
		}, nil

	case GrokMediaEndpointVideoStatus, GrokMediaEndpointVideoContent:
		rawID := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(requestID), "seedance:"))
		target, err := buildSeedanceURL(base, SeedanceEndpointStatus, rawID)
		if err != nil {
			return nil, err
		}
		responseBody, headers, err := s.doSeedanceCompatibleRequest(ctx, c, account, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		status := seedanceCompatibleStatus(gjson.GetBytes(responseBody, "status").String())
		videoURL := strings.TrimSpace(gjson.GetBytes(responseBody, "content.video_url").String())
		if endpoint == GrokMediaEndpointVideoContent {
			if status != "completed" || videoURL == "" {
				return nil, fmt.Errorf("seedance video content is not available")
			}
			c.Header("Location", videoURL)
			c.Status(http.StatusFound)
		} else {
			metadata := gin.H{}
			if videoURL != "" {
				metadata["result_url"] = videoURL
			}
			payload := gin.H{}
			payload["id"] = SeedanceTaskKey(rawID)
			payload["object"] = "video"
			payload["model"] = gjson.GetBytes(responseBody, "model").String()
			payload["status"] = status
			payload["metadata"] = metadata
			payload["created_at"] = gjson.GetBytes(responseBody, "created_at").Int()
			if completedAt := gjson.GetBytes(responseBody, "updated_at").Int(); status == "completed" && completedAt > 0 {
				payload["completed_at"] = completedAt
				payload["progress"] = 100
			}
			if status == "failed" {
				taskError := gin.H{}
				taskError["code"] = gjson.GetBytes(responseBody, "error.code").String()
				taskError["message"] = gjson.GetBytes(responseBody, "error.message").String()
				payload["error"] = taskError
			}
			c.JSON(http.StatusOK, payload)
		}
		result := &OpenAIForwardResult{
			ResponseID: SeedanceTaskKey(rawID),
			Model: gjson.GetBytes(responseBody, "model").String(),
			UpstreamModel: gjson.GetBytes(responseBody, "model").String(),
			ResponseHeaders: headers, Duration: time.Since(started),
			VideoResolution: gjson.GetBytes(responseBody, "resolution").String(),
			VideoDurationSeconds: int(gjson.GetBytes(responseBody, "duration").Int()),
		}
		if status == "completed" && videoURL != "" {
			result.VideoCount = 1
			result.Usage.OutputTokens = max(0, int(gjson.GetBytes(responseBody, "usage.completion_tokens").Int()))
		}
		return result, nil
	default:
		return nil, fmt.Errorf("official Seedance does not support video endpoint %s", endpoint)
	}
}
