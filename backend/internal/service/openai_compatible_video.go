package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// IsOpenAICompatibleVideoPlatform reports whether a platform should use the
// generic OpenAI-compatible Videos adapter. Grok is deliberately excluded:
// it keeps the dedicated xAI media implementation and billing semantics.
func IsOpenAICompatibleVideoPlatform(platform string) bool {
	switch strings.TrimSpace(platform) {
	case PlatformOpenAI, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo:
		return true
	default:
		return false
	}
}

// MediaVideoRequestAccountPlatform restores the concrete provider selected for
// an async video task. This is required for Composite GET status/content calls,
// because those requests no longer contain a model to resolve.
func (s *OpenAIGatewayService) MediaVideoRequestAccountPlatform(ctx context.Context, accountID int64) (string, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return "", fmt.Errorf("video request account is unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return "", err
	}
	if account == nil {
		return "", fmt.Errorf("video request account not found")
	}
	return strings.TrimSpace(account.Platform), nil
}

// ForwardCompatibleVideo transparently relays OpenAI-compatible video APIs for
// non-Grok providers. Provider-specific JSON/multipart fields are preserved;
// only the model is rewritten through the existing account mapping layer.
func (s *OpenAIGatewayService) ForwardCompatibleVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	requestID string,
	body []byte,
	contentType string,
	routingModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("compatible video account is required")
	}
	if !IsOpenAICompatibleVideoPlatform(account.Platform) {
		return nil, fmt.Errorf("account platform %s is not supported by the compatible video adapter", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("compatible video requires an API-key account")
	}

	token, _, err := s.getRequestCredential(ctx, c, account)
	if err != nil {
		return nil, err
	}

	requestInfo := ParseGrokMediaRequest(contentType, body)
	requestModel := compatibleVideoFirstNonEmpty(routingModel, requestInfo.Model)
	forwardBody, forwardContentType, upstreamModel, err := prepareCompatibleVideoBody(account, body, contentType, requestModel)
	if err != nil {
		return nil, err
	}

	paths := compatibleVideoEndpointPaths(endpoint, requestID)
	if len(paths) == 0 {
		return nil, fmt.Errorf("unsupported compatible video endpoint %s", endpoint)
	}

	var resp *http.Response
	for index, endpointPath := range paths {
		targetURL, buildErr := s.compatibleVideoURL(account, endpointPath)
		if buildErr != nil {
			return nil, buildErr
		}

		var bodyReader io.Reader
		if endpoint.RequiresRequestBody() {
			bodyReader = bytes.NewReader(forwardBody)
		}
		upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
		req, requestErr := http.NewRequestWithContext(upstreamCtx, endpoint.httpMethod(), targetURL, bodyReader)
		if requestErr != nil {
			releaseUpstreamCtx()
			return nil, requestErr
		}
		req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
		req.Header.Set("Authorization", "Bearer "+token)
		if endpoint == GrokMediaEndpointVideoContent {
			req.Header.Set("Accept", "*/*")
			if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" {
				req.Header.Set("Range", rangeHeader)
			}
		} else {
			req.Header.Set("Accept", "application/json")
		}
		if endpoint.RequiresRequestBody() {
			if strings.TrimSpace(forwardContentType) == "" {
				forwardContentType = "application/json"
			}
			req.Header.Set("Content-Type", forwardContentType)
		}
		account.ApplyHeaderOverrides(req.Header)

		proxyURL := ""
		if account.ProxyID != nil && account.Proxy != nil {
			proxyURL = account.Proxy.URL()
		}
		upstreamStart := time.Now()
		resp, err = s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
		releaseUpstreamCtx()
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			if isGrokVideoCreateEndpoint(endpoint) {
				// Once an async CREATE has been written to the upstream, a transport
				// error is ambiguous: the provider may already have accepted and
				// charged the task. Never replay it on another account/provider.
				return nil, fmt.Errorf("compatible video create transport failed: %w", err)
			}
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
		}

		// Providers commonly expose either /videos or /videos/generations.
		// Retry only route-level 404/405 so a submitted generation is never
		// duplicated after a business or provider error.
		if (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed) && index+1 < len(paths) {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			_ = resp.Body.Close()
			resp = nil
			continue
		}
		break
	}
	if resp == nil {
		return nil, fmt.Errorf("compatible video upstream returned no response")
	}
	defer func() { _ = resp.Body.Close() }()

	requestIDHeader := compatibleVideoFirstNonEmpty(
		resp.Header.Get("x-request-id"),
		resp.Header.Get("request-id"),
		resp.Header.Get("x-trace-id"),
	)
	if resp.StatusCode >= http.StatusBadRequest {
		result, handleErr := s.handleCompatErrorResponse(resp, c, account, writeGrokMediaErrorResponse, upstreamModel)
		if isGrokVideoCreateEndpoint(endpoint) && handleErr != nil {
			var failoverErr *UpstreamFailoverError
			if errors.As(handleErr, &failoverErr) {
				// Async CREATE is intentionally at-most-once across accounts.
				return result, fmt.Errorf("compatible video create upstream rejected request: status=%d", failoverErr.StatusCode)
			}
		}
		return result, handleErr
	}

	if endpoint == GrokMediaEndpointVideoContent {
		if err := writeGrokMediaContentResponse(c, resp); err != nil {
			return nil, err
		}
		return &OpenAIForwardResult{
			RequestID:       requestIDHeader,
			ResponseID:      strings.TrimSpace(requestID),
			ResponseHeaders: resp.Header.Clone(),
			Duration:        time.Since(startTime),
			VideoCount:      1,
		}, nil
	}

	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)

	result := compatibleVideoForwardResult(endpoint, requestID, requestInfo, requestModel, upstreamModel, responseBody)
	result.RequestID = requestIDHeader
	result.ResponseHeaders = resp.Header.Clone()
	result.Duration = time.Since(startTime)
	return result, nil
}

func (s *OpenAIGatewayService) compatibleVideoURL(account *Account, endpointPath string) (string, error) {
	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	if baseURL == "" {
		return "", fmt.Errorf("compatible video upstream base_url is required")
	}
	validated, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(validated)
	if err != nil {
		return "", fmt.Errorf("parse compatible video base_url: %w", err)
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(strings.ToLower(basePath), "/v1") {
		basePath += "/v1"
	}
	if basePath == "" {
		basePath = "/v1"
	}
	parsed.Path = basePath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/") + endpointPath, nil
}

func compatibleVideoEndpointPaths(endpoint GrokMediaEndpoint, requestID string) []string {
	escapedID := url.PathEscape(strings.TrimSpace(requestID))
	switch endpoint {
	case GrokMediaEndpointVideosGenerations:
		return []string{"/videos", "/videos/generations"}
	case GrokMediaEndpointVideosEdits:
		return []string{"/videos/edits"}
	case GrokMediaEndpointVideosExtensions:
		return []string{"/videos/extensions"}
	case GrokMediaEndpointVideoStatus:
		return []string{"/videos/" + escapedID, "/videos/generations/" + escapedID}
	case GrokMediaEndpointVideoContent:
		return []string{"/videos/" + escapedID + "/content", "/videos/generations/" + escapedID + "/content"}
	default:
		return nil
	}
}

func prepareCompatibleVideoBody(account *Account, body []byte, contentType, routingModel string) ([]byte, string, string, error) {
	if account == nil {
		return nil, "", "", fmt.Errorf("compatible video account is required")
	}
	routingModel = strings.TrimSpace(routingModel)
	if routingModel == "" {
		routingModel = strings.TrimSpace(ParseGrokMediaRequest(contentType, body).Model)
	}
	upstreamModel := strings.TrimSpace(account.GetMappedModel(routingModel))
	if upstreamModel == "" {
		upstreamModel = routingModel
	}
	if upstreamModel == "" || len(body) == 0 {
		return body, contentType, upstreamModel, nil
	}

	if gjson.ValidBytes(body) {
		rewritten, err := sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, "", "", fmt.Errorf("rewrite compatible video model: %w", err)
		}
		if IsAIStarsLabOpenAICompatibleAccount(account) {
			rewritten, err = normalizeAIStarsLabCompatibleVideoJSON(rewritten)
		} else {
			rewritten, err = normalizeCompatibleSeedanceVideoJSON(rewritten, upstreamModel)
		}
		if err != nil {
			return nil, "", "", err
		}
		return rewritten, "application/json", upstreamModel, nil
	}

	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return body, contentType, upstreamModel, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, "", "", fmt.Errorf("compatible video multipart boundary is missing")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	modelSeen := false
	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		if partErr != nil {
			return nil, "", "", fmt.Errorf("parse compatible video multipart body: %w", partErr)
		}
		dst, createErr := writer.CreatePart(part.Header)
		if createErr != nil {
			_ = part.Close()
			return nil, "", "", fmt.Errorf("rewrite compatible video multipart part: %w", createErr)
		}
		if part.FormName() == "model" {
			modelSeen = true
			if _, writeErr := io.WriteString(dst, upstreamModel); writeErr != nil {
				_ = part.Close()
				return nil, "", "", writeErr
			}
		} else if _, copyErr := io.Copy(dst, part); copyErr != nil {
			_ = part.Close()
			return nil, "", "", copyErr
		}
		_ = part.Close()
	}
	if !modelSeen {
		if err := writer.WriteField("model", upstreamModel); err != nil {
			return nil, "", "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", "", err
	}
	return out.Bytes(), writer.FormDataContentType(), upstreamModel, nil
}

func normalizeAIStarsLabCompatibleVideoJSON(body []byte) ([]byte, error) {
	if !gjson.ValidBytes(body) {
		return body, nil
	}
	out := body
	var err error

	seconds := compatibleVideoFirstNonEmpty(gjson.GetBytes(out, "seconds").String(), gjson.GetBytes(out, "duration").String())
	if seconds != "" {
		out, err = sjson.SetBytes(out, "seconds", seconds)
		if err != nil {
			return nil, fmt.Errorf("normalize AIStarsLab seconds: %w", err)
		}
	}
	size := compatibleVideoFirstNonEmpty(
		gjson.GetBytes(out, "size").String(), gjson.GetBytes(out, "metadata.size").String(),
		gjson.GetBytes(out, "aspect_ratio").String(), gjson.GetBytes(out, "ratio").String(),
	)
	if size != "" {
		out, err = sjson.SetBytes(out, "size", size)
		if err != nil {
			return nil, fmt.Errorf("normalize AIStarsLab size: %w", err)
		}
	}
	resolution := compatibleVideoFirstNonEmpty(
		gjson.GetBytes(out, "metadata.resolution").String(), gjson.GetBytes(out, "resolution").String(),
		gjson.GetBytes(out, "resolution_name").String(),
	)
	if resolution != "" {
		out, err = sjson.SetBytes(out, "metadata.resolution", resolution)
		if err != nil {
			return nil, fmt.Errorf("normalize AIStarsLab resolution: %w", err)
		}
	}

	mediaRefs := parseVideoMediaReferences(out)
	images, videos, audios := splitVideoReferences(mediaRefs)
	if len(images) > 0 {
		existing := aiStarsLabJSONStrings(out, "metadata.images")
		out, err = sjson.SetBytes(out, "metadata.images", append(existing, images...))
		if err != nil {
			return nil, fmt.Errorf("normalize AIStarsLab metadata.images: %w", err)
		}
	}
	if len(videos) > 0 {
		existing := aiStarsLabJSONStrings(out, "metadata.videos")
		out, err = sjson.SetBytes(out, "metadata.videos", append(existing, videos...))
		if err != nil {
			return nil, fmt.Errorf("normalize AIStarsLab metadata.videos: %w", err)
		}
	}
	if len(audios) > 0 {
		existing := aiStarsLabJSONStrings(out, "metadata.audios")
		out, err = sjson.SetBytes(out, "metadata.audios", append(existing, audios...))
		if err != nil {
			return nil, fmt.Errorf("normalize AIStarsLab metadata.audios: %w", err)
		}
	}
	allRefs := parseUnifiedVideoReferences(out)
	mode := normalizeUnifiedVideoMode(compatibleVideoFirstNonEmpty(
		gjson.GetBytes(out, "metadata.mode_type").String(),
		gjson.GetBytes(out, "mode_type").String(),
		gjson.GetBytes(out, "mode").String(),
	), allRefs)
	out, err = sjson.SetBytes(out, "metadata.mode_type", mode)
	if err != nil {
		return nil, fmt.Errorf("normalize AIStarsLab mode_type: %w", err)
	}

	for _, path := range []string{
		"resolution", "resolution_name", "aspect_ratio", "ratio",
		"mode_type", "mode", "media", "audio", "generate_audio", "watermark",
	} {
		out, err = sjson.DeleteBytes(out, path)
		if err != nil {
			return nil, fmt.Errorf("remove unsupported AIStarsLab field %s: %w", path, err)
		}
	}
	return out, nil
}

func normalizeCompatibleSeedanceVideoJSON(body []byte, model string) ([]byte, error) {
	if !strings.Contains(strings.ToLower(strings.TrimSpace(model)), "seedance") || !gjson.ValidBytes(body) {
		return body, nil
	}
	out := body
	var err error

	resolution := compatibleVideoFirstNonEmpty(
		gjson.GetBytes(out, "resolution").String(),
		gjson.GetBytes(out, "resolution_name").String(),
		gjson.GetBytes(out, "metadata.resolution").String(),
	)
	if normalized, ok := LookupVideoBillingResolution(resolution); ok {
		for _, path := range []string{"resolution", "resolution_name"} {
			if !gjson.GetBytes(out, path).Exists() {
				out, err = sjson.SetBytes(out, path, normalized)
				if err != nil {
					return nil, fmt.Errorf("normalize compatible Seedance %s: %w", path, err)
				}
			}
		}
		metadata := gjson.GetBytes(out, "metadata")
		if !metadata.Exists() || metadata.IsObject() {
			if !gjson.GetBytes(out, "metadata.resolution").Exists() {
				out, err = sjson.SetBytes(out, "metadata.resolution", normalized)
				if err != nil {
					return nil, fmt.Errorf("normalize compatible Seedance metadata resolution: %w", err)
				}
			}
		}
	}

	duration := gjson.GetBytes(out, "duration")
	if !duration.Exists() {
		duration = gjson.GetBytes(out, "seconds")
	}
	if duration.Exists() {
		if !gjson.GetBytes(out, "duration").Exists() {
			out, err = sjson.SetBytes(out, "duration", duration.Value())
			if err != nil {
				return nil, fmt.Errorf("normalize compatible Seedance duration: %w", err)
			}
		}
		if !gjson.GetBytes(out, "seconds").Exists() {
			out, err = sjson.SetBytes(out, "seconds", duration.Value())
			if err != nil {
				return nil, fmt.Errorf("normalize compatible Seedance seconds: %w", err)
			}
		}
	}

	ratio := compatibleVideoFirstNonEmpty(
		gjson.GetBytes(out, "aspect_ratio").String(),
		gjson.GetBytes(out, "ratio").String(),
	)
	if ratio != "" {
		if !gjson.GetBytes(out, "aspect_ratio").Exists() {
			out, err = sjson.SetBytes(out, "aspect_ratio", ratio)
			if err != nil {
				return nil, fmt.Errorf("normalize compatible Seedance aspect_ratio: %w", err)
			}
		}
		if !gjson.GetBytes(out, "ratio").Exists() {
			out, err = sjson.SetBytes(out, "ratio", ratio)
			if err != nil {
				return nil, fmt.Errorf("normalize compatible Seedance ratio: %w", err)
			}
		}
	}
	return out, nil
}

func compatibleVideoForwardResult(
	endpoint GrokMediaEndpoint,
	requestID string,
	requestInfo GrokMediaRequestInfo,
	requestModel string,
	upstreamModel string,
	body []byte,
) *OpenAIForwardResult {
	result := &OpenAIForwardResult{
		ResponseID:           compatibleVideoFirstNonEmpty(extractGrokMediaVideoRequestID(body), requestID),
		Model:                compatibleVideoFirstNonEmpty(compatibleVideoJSONField(body, "model", "data.model", "video.model", "result.model"), requestModel),
		BillingModel:         requestModel,
		UpstreamModel:        upstreamModel,
		VideoResolution:      compatibleVideoFirstNonEmpty(compatibleVideoJSONField(body, "resolution", "data.resolution", "video.resolution", "result.resolution"), requestInfo.Resolution),
		VideoDurationSeconds: compatibleVideoDuration(body),
	}
	if result.VideoDurationSeconds <= 0 {
		result.VideoDurationSeconds = requestInfo.DurationSeconds
	}
	if strings.TrimSpace(result.BillingModel) == "" {
		result.BillingModel = result.Model
	}

	switch endpoint {
	case GrokMediaEndpointVideosGenerations, GrokMediaEndpointVideosEdits, GrokMediaEndpointVideosExtensions:
		if strings.TrimSpace(extractGrokMediaVideoRequestID(body)) == "" && compatibleVideoHasResultURL(body) {
			result.VideoCount = 1
		}
	case GrokMediaEndpointVideoStatus:
		if compatibleVideoCompleted(body) {
			result.VideoCount = 1
		}
	}
	return result
}

func compatibleVideoCompleted(body []byte) bool {
	status := strings.ToLower(strings.TrimSpace(compatibleVideoJSONField(body, "status", "data.status", "result.status", "video.status")))
	switch status {
	case "failed", "error", "expired", "canceled", "cancelled", "rejected":
		return false
	case "completed", "done", "succeeded", "success", "finished":
		return true
	}
	return status == "" && compatibleVideoHasResultURL(body)
}

func compatibleVideoHasResultURL(body []byte) bool {
	return compatibleVideoJSONField(
		body,
		"video.url", "video_url", "result_url", "url", "content.video_url", "content.url",
		"data.video.url", "data.video_url", "data.result_url", "data.url", "data.content.video_url", "data.content.url",
		"result.video.url", "result.video_url", "result.url",
	) != ""
}

func compatibleVideoDuration(body []byte) int {
	for _, path := range []string{
		"video.duration", "duration", "seconds",
		"data.video.duration", "data.duration", "data.seconds",
		"result.video.duration", "result.duration",
	} {
		value := gjson.GetBytes(body, path)
		if value.Exists() {
			if duration := int(value.Int()); duration > 0 {
				return duration
			}
		}
	}
	return 0
}

func compatibleVideoJSONField(body []byte, paths ...string) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range paths {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			return value
		}
	}
	return ""
}

func compatibleVideoFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
