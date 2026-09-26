package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const aiStarsLabTaskPrefix = "aistarslab:"

func AIStarsLabTaskKey(id string) string {
	id = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(id), aiStarsLabTaskPrefix))
	if id == "" {
		return ""
	}
	return aiStarsLabTaskPrefix + id
}

func stripAIStarsLabTaskKey(id string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(id), aiStarsLabTaskPrefix))
}

func aiStarsLabJSONStrings(body []byte, path string) []string {
	v := gjson.GetBytes(body, path)
	if !v.Exists() {
		return nil
	}
	if v.IsArray() {
		out := make([]string, 0, len(v.Array()))
		for _, item := range v.Array() {
			s := strings.TrimSpace(item.String())
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s := strings.TrimSpace(v.String()); s != "" {
		return []string{s}
	}
	return nil
}

func prepareAIStarsLabOpenAPIVideoCreate(account *Account, body []byte, contentType, routingModel string) ([]byte, string, GrokMediaRequestInfo, error) {
	if account == nil || !IsAIStarsLabOpenAPIAccount(account) {
		return nil, "", GrokMediaRequestInfo{}, fmt.Errorf("AIStarsLab OpenAPI account is required")
	}
	if !gjson.ValidBytes(body) {
		return nil, "", GrokMediaRequestInfo{}, fmt.Errorf("AIStarsLab OpenAPI video create requires application/json")
	}
	info := ParseGrokMediaRequest(contentType, body)
	rawModel := strings.TrimSpace(compatibleVideoFirstNonEmpty(routingModel, info.Model))
	mapped := strings.TrimSpace(account.GetMappedModel(rawModel))
	if mapped == "" {
		mapped = rawModel
	}
	ref := ParseVideoModelRef(mapped)
	if ref.ChannelCode == "" {
		ref = ParseVideoModelRef(rawModel)
	}
	if ref.ChannelCode == "" || strings.TrimSpace(ref.CanonicalModel) == "" {
		return nil, "", info, fmt.Errorf("AIStarsLab OpenAPI model must resolve to channel:model")
	}

	aspectRatio := compatibleVideoFirstNonEmpty(
		gjson.GetBytes(body, "aspectRatio").String(),
		gjson.GetBytes(body, "aspect_ratio").String(),
		gjson.GetBytes(body, "size").String(),
		gjson.GetBytes(body, "metadata.size").String(),
		info.AspectRatio,
		info.Size,
	)
	quality := compatibleVideoFirstNonEmpty(
		gjson.GetBytes(body, "quality").String(),
		gjson.GetBytes(body, "metadata.resolution").String(),
		gjson.GetBytes(body, "resolution").String(),
		gjson.GetBytes(body, "resolution_name").String(),
		info.Resolution,
	)
	duration := info.DurationSeconds
	for _, path := range []string{"duration", "seconds"} {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		if value.Type == gjson.Number {
			duration = int(value.Int())
		} else if parsed, err := strconv.Atoi(strings.TrimSpace(value.String())); err == nil {
			duration = parsed
		}
		break
	}

	refs := parseUnifiedVideoReferences(body)
	mode := normalizeUnifiedVideoMode(compatibleVideoFirstNonEmpty(
		gjson.GetBytes(body, "metadata.mode_type").String(),
		gjson.GetBytes(body, "mode_type").String(),
		gjson.GetBytes(body, "mode").String(),
	), refs)
	if err := validateAIStarsLabOpenAPIReferences(mode, refs); err != nil {
		return nil, "", info, err
	}
	images, videos, audios := splitVideoReferences(refs)
	prompt := strings.TrimSpace(info.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	}
	if prompt == "" {
		return nil, "", info, fmt.Errorf("AIStarsLab OpenAPI prompt is required")
	}
	if aspectRatio == "" || quality == "" || duration <= 0 {
		return nil, "", info, fmt.Errorf("AIStarsLab OpenAPI requires aspect ratio, quality, and duration")
	}

	payload := map[string]any{
		"channel":     ref.ChannelCode,
		"model":       ref.CanonicalModel,
		"prompt":      prompt,
		"aspectRatio": aspectRatio,
		"quality":     quality,
		"duration":    duration,
		"mode":        mode,
	}
	if len(images) > 0 {
		payload["inputImages"] = images
	}
	if len(videos) > 0 {
		payload["inputVideos"] = videos
	}
	if len(audios) > 0 {
		payload["inputAudios"] = audios
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, "", info, fmt.Errorf("encode AIStarsLab OpenAPI video request: %w", err)
	}
	return encoded, mapped, info, nil
}

type aiStarsLabEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type aiStarsLabCreateData struct {
	TaskID      string `json:"taskId"`
	Status      int    `json:"status"`
	CostCredits int    `json:"costCredits"`
}

type aiStarsLabStatusData struct {
	TaskID       string   `json:"taskId"`
	Type         int      `json:"type"`
	Status       int      `json:"status"`
	Progress     *int     `json:"progress,omitempty"`
	Outputs      []string `json:"outputs,omitempty"`
	ErrorCode    string   `json:"errorCode,omitempty"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
}

func aiStarsLabOpenAIStatus(status int) string {
	switch status {
	case 1:
		return "queued"
	case 2:
		return "in_progress"
	case 3:
		return "completed"
	case 4:
		return "failed"
	default:
		return "queued"
	}
}

func (s *OpenAIGatewayService) doAIStarsLabOpenAPI(ctx context.Context, c *gin.Context, account *Account, method, target string, body []byte) ([]byte, http.Header, error) {
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, nil, fmt.Errorf("AIStarsLab account missing api_key")
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
		// Async create transport errors are ambiguous: the supplier may already
		// have accepted and charged the task. Return a non-failover error so the
		// caller never creates a duplicate task on another provider.
		return nil, nil, fmt.Errorf("AIStarsLab upstream transport failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, resp.Header.Clone(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Clone(), fmt.Errorf("AIStarsLab upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var envelope aiStarsLabEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, resp.Header.Clone(), fmt.Errorf("decode AIStarsLab response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, resp.Header.Clone(), fmt.Errorf("AIStarsLab error %d: %s", envelope.Code, strings.TrimSpace(envelope.Msg))
	}
	return envelope.Data, resp.Header.Clone(), nil
}

func (s *OpenAIGatewayService) ForwardAIStarsLabOpenAPIVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	requestID string,
	body []byte,
	contentType string,
	routingModel string,
) (*OpenAIForwardResult, error) {
	if !IsAIStarsLabOpenAPIAccount(account) {
		return nil, fmt.Errorf("AIStarsLab OpenAPI account is required")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	base = strings.TrimRight(base, "/")
	started := time.Now()

	switch endpoint {
	case GrokMediaEndpointVideosGenerations:
		createBody, upstreamModel, info, err := prepareAIStarsLabOpenAPIVideoCreate(account, body, contentType, routingModel)
		if err != nil {
			return nil, err
		}
		data, headers, err := s.doAIStarsLabOpenAPI(ctx, c, account, http.MethodPost, base+"/generation/create/video", createBody)
		if err != nil {
			return nil, err
		}
		var created aiStarsLabCreateData
		if err := json.Unmarshal(data, &created); err != nil || strings.TrimSpace(created.TaskID) == "" {
			return nil, fmt.Errorf("AIStarsLab create response missing taskId")
		}
		publicID := AIStarsLabTaskKey(created.TaskID)
		response := gin.H{}
		response["id"] = publicID
		response["object"] = "video"
		response["model"] = info.Model
		response["status"] = aiStarsLabOpenAIStatus(created.Status)
		response["progress"] = 0
		response["created_at"] = time.Now().Unix()
		response["metadata"] = gin.H{"provider_cost_credits": created.CostCredits}
		c.JSON(http.StatusOK, response)
		return &OpenAIForwardResult{
			ResponseID: publicID, Model: info.Model, BillingModel: info.Model,
			UpstreamModel: upstreamModel, ResponseHeaders: headers,
			Duration: time.Since(started), VideoResolution: info.Resolution,
			VideoDurationSeconds: info.DurationSeconds,
		}, nil

	case GrokMediaEndpointVideoStatus, GrokMediaEndpointVideoContent:
		taskID := stripAIStarsLabTaskKey(requestID)
		if taskID == "" {
			return nil, fmt.Errorf("AIStarsLab task id is required")
		}
		target := base + "/generation/status?taskId=" + url.QueryEscape(taskID)
		data, headers, err := s.doAIStarsLabOpenAPI(ctx, c, account, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		var status aiStarsLabStatusData
		if err := json.Unmarshal(data, &status); err != nil {
			return nil, fmt.Errorf("decode AIStarsLab status: %w", err)
		}
		publicID := AIStarsLabTaskKey(compatibleVideoFirstNonEmpty(status.TaskID, taskID))
		resultURL := ""
		if len(status.Outputs) > 0 {
			resultURL = strings.TrimSpace(status.Outputs[0])
		}
		if endpoint == GrokMediaEndpointVideoContent {
			if aiStarsLabOpenAIStatus(status.Status) != "completed" || resultURL == "" {
				return nil, fmt.Errorf("AIStarsLab video content is not available")
			}
			c.Header("Location", resultURL)
			c.Status(http.StatusFound)
			result := &OpenAIForwardResult{}
			result.ResponseID = publicID
			result.ResponseHeaders = headers
			result.Duration = time.Since(started)
			result.VideoCount = 1
			return result, nil
		}
		metadata := gin.H{}
		if resultURL != "" {
			metadata["result_url"] = resultURL
		}
		response := gin.H{}
		response["id"] = publicID
		response["object"] = "video"
		response["status"] = aiStarsLabOpenAIStatus(status.Status)
		response["metadata"] = metadata
		if status.Progress != nil {
			response["progress"] = *status.Progress
		}
		if status.Status == 4 {
			taskError := gin.H{}
			taskError["code"] = status.ErrorCode
			taskError["message"] = status.ErrorMessage
			response["error"] = taskError
		}
		c.JSON(http.StatusOK, response)
		result := &OpenAIForwardResult{}
		result.ResponseID = publicID
		result.ResponseHeaders = headers
		result.Duration = time.Since(started)
		if status.Status == 3 && resultURL != "" {
			result.VideoCount = 1
		}
		return result, nil
	default:
		return nil, fmt.Errorf("AIStarsLab OpenAPI does not support video endpoint %s", endpoint)
	}
}
