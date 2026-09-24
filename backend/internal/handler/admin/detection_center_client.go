package admin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const detectionMaxResponseBytes int64 = 4 << 20

type detectionTarget struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type detectionHTTPResult struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Duration   time.Duration
}

func newDetectionTarget(ctx context.Context, rawBaseURL, apiKey string) (*detectionTarget, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("api_key is required")
	}
	validated, err := urlvalidator.ValidateHTTPSURL(rawBaseURL, urlvalidator.ValidationOptions{AllowPrivate: false})
	if err != nil {
		return nil, fmt.Errorf("base_url must be a public HTTPS endpoint: %w", err)
	}
	u, err := url.Parse(validated)
	if err != nil {
		return nil, errors.New("invalid base_url")
	}
	if _, err := urlvalidator.ResolveAndPinHost(ctx, u.Hostname()); err != nil {
		return nil, fmt.Errorf("base_url host is not allowed: %w", err)
	}

	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:               nil,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     45 * time.Second,
		TLSHandshakeTimeout: 8 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
			return urlvalidator.DialContextWithPinnedIPs(dialCtx, network, address, dialer.DialContext)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   45 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &detectionTarget{baseURL: validated, apiKey: apiKey, client: client}, nil
}

func (t *detectionTarget) doJSON(ctx context.Context, protocol, method, endpoint string, headers http.Header, payload any) (detectionHTTPResult, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return detectionHTTPResult{}, err
		}
		body = bytes.NewReader(encoded)
	}
	return t.do(ctx, protocol, method, endpoint, headers, body, payload != nil)
}

func (t *detectionTarget) do(ctx context.Context, protocol, method, endpoint string, headers http.Header, body io.Reader, hasJSONBody bool) (detectionHTTPResult, error) {
	targetURL, err := joinDetectionEndpoint(t.baseURL, endpoint)
	if err != nil {
		return detectionHTTPResult{}, err
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return detectionHTTPResult{}, err
	}
	pinnedCtx, err := urlvalidator.ResolveAndPinHost(ctx, u.Hostname())
	if err != nil {
		return detectionHTTPResult{}, fmt.Errorf("target host rejected by SSRF policy: %w", err)
	}
	req, err := http.NewRequestWithContext(pinnedCtx, method, targetURL, body)
	if err != nil {
		return detectionHTTPResult{}, err
	}
	applyDetectionAuth(req.Header, protocol, t.apiKey)
	if hasJSONBody {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ModuRelay-DetectionCenter/1.0")
	for key, values := range headers {
		req.Header.Del(key)
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	started := time.Now()
	resp, err := t.client.Do(req)
	duration := time.Since(started)
	if err != nil {
		return detectionHTTPResult{Duration: duration}, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, detectionMaxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return detectionHTTPResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Duration: duration}, err
	}
	if int64(len(data)) > detectionMaxResponseBytes {
		return detectionHTTPResult{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Duration: duration}, errors.New("upstream response exceeds 4 MiB safety limit")
	}
	return detectionHTTPResult{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       data,
		Duration:   duration,
	}, nil
}

func applyDetectionAuth(headers http.Header, protocol, apiKey string) {
	switch protocol {
	case detectionProtocolAnthropic:
		headers.Set("X-Api-Key", apiKey)
		headers.Set("Anthropic-Version", "2023-06-01")
	case detectionProtocolGemini:
		headers.Set("X-Goog-Api-Key", apiKey)
	default:
		headers.Set("Authorization", "Bearer "+apiKey)
	}
}

func joinDetectionEndpoint(baseRaw, endpointRaw string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseRaw))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", errors.New("invalid base_url")
	}
	endpoint, err := url.Parse(strings.TrimSpace(endpointRaw))
	if err != nil {
		return "", errors.New("invalid endpoint")
	}
	basePath := strings.TrimRight(base.Path, "/")
	epPath := "/" + strings.TrimLeft(endpoint.Path, "/")
	if strings.HasSuffix(basePath, "/v1") && (epPath == "/v1" || strings.HasPrefix(epPath, "/v1/")) {
		epPath = strings.TrimPrefix(epPath, "/v1")
		if epPath == "" {
			epPath = "/"
		}
	}
	if strings.HasSuffix(basePath, "/v1beta") && (epPath == "/v1beta" || strings.HasPrefix(epPath, "/v1beta/")) {
		epPath = strings.TrimPrefix(epPath, "/v1beta")
		if epPath == "" {
			epPath = "/"
		}
	}
	joined := path.Clean(basePath + "/" + strings.TrimLeft(epPath, "/"))
	if joined == "." {
		joined = "/"
	}
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	base.Path = joined
	base.RawPath = ""
	base.RawQuery = endpoint.RawQuery
	base.Fragment = ""
	return base.String(), nil
}

func discoverDetectionModels(ctx context.Context, target *detectionTarget, requestedProtocol string) detectionDiscoveryResponse {
	protocols := []string{detectionProtocolOpenAI, detectionProtocolAnthropic, detectionProtocolGemini}
	if requestedProtocol != detectionProtocolAuto {
		protocols = []string{requestedProtocol}
	}
	allModels := make([]detectedModel, 0, 64)
	attempts := make([]detectionAttempt, 0, len(protocols))
	working := make([]string, 0, len(protocols))
	for _, protocol := range protocols {
		models, attempt := discoverDetectionModelsForProtocol(ctx, target, protocol)
		attempts = append(attempts, attempt)
		if attempt.Status == "success" {
			working = append(working, protocol)
			allModels = append(allModels, models...)
		}
	}
	working = uniqueSortedStrings(working)
	suggested := detectionProtocolAuto
	if len(working) == 1 {
		suggested = working[0]
	}
	return detectionDiscoveryResponse{
		SuggestedProtocol: suggested,
		Protocols:         working,
		Models:            mergeDetectedModels(allModels),
		Attempts:          attempts,
	}
}

func discoverDetectionModelsForProtocol(ctx context.Context, target *detectionTarget, protocol string) ([]detectedModel, detectionAttempt) {
	endpoint := "/v1/models"
	if protocol == detectionProtocolAnthropic {
		endpoint = "/v1/models?limit=1000"
	} else if protocol == detectionProtocolGemini {
		endpoint = "/v1beta/models?pageSize=1000"
	}
	result, err := target.doJSON(ctx, protocol, http.MethodGet, endpoint, nil, nil)
	attempt := detectionAttempt{Protocol: protocol, LatencyMS: result.Duration.Milliseconds()}
	if err != nil {
		attempt.Status = "failed"
		attempt.Message = sanitizeDetectionText(err.Error(), target.apiKey, 320)
		return nil, attempt
	}
	attempt.HTTPStatus = result.StatusCode
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		attempt.Status = statusFromHTTP(result.StatusCode)
		attempt.Message = "模型列表请求未成功：" + detectionExcerpt(result.Body, target.apiKey, 260)
		return nil, attempt
	}
	models, err := parseDetectionModelList(protocol, result.Body)
	if err != nil || len(models) == 0 {
		attempt.Status = "failed"
		if err != nil {
			attempt.Message = "模型列表响应无法识别：" + sanitizeDetectionText(err.Error(), target.apiKey, 260)
		} else {
			attempt.Message = "模型列表响应成功，但没有识别到可选模型"
		}
		return nil, attempt
	}
	attempt.Status = "success"
	attempt.Message = fmt.Sprintf("识别到 %d 个模型", len(models))
	return models, attempt
}

func parseDetectionModelList(protocol string, body []byte) ([]detectedModel, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	models := make([]detectedModel, 0, 32)
	if protocol == detectionProtocolGemini {
		items, _ := root["models"].([]any)
		for _, item := range items {
			obj, _ := item.(map[string]any)
			id := strings.TrimPrefix(stringValue(obj["name"]), "models/")
			if id == "" {
				continue
			}
			capabilities := map[string]any{}
			if methods, ok := obj["supportedGenerationMethods"].([]any); ok {
				capabilities["supported_generation_methods"] = methods
			}
			models = append(models, detectedModel{
				ID:           id,
				Name:         detectionFirstNonEmpty(stringValue(obj["displayName"]), id),
				Provider:     "Google Gemini compatible",
				Protocols:    []string{protocol},
				Capabilities: capabilities,
			})
		}
		return models, nil
	}
	items, _ := root["data"].([]any)
	for _, item := range items {
		obj, _ := item.(map[string]any)
		id := stringValue(obj["id"])
		if id == "" {
			continue
		}
		name := id
		provider := stringValue(obj["owned_by"])
		capabilities := map[string]any{}
		if protocol == detectionProtocolAnthropic {
			name = detectionFirstNonEmpty(stringValue(obj["display_name"]), id)
			provider = "Anthropic compatible"
			if caps, ok := obj["capabilities"].(map[string]any); ok {
				capabilities = caps
			}
		}
		models = append(models, detectedModel{ID: id, Name: name, Provider: provider, Protocols: []string{protocol}, Capabilities: capabilities})
	}
	return models, nil
}

func detectProtocolForModel(ctx context.Context, target *detectionTarget, model string) (string, detectionProbeResult) {
	order := []string{detectionProtocolOpenAI, detectionProtocolAnthropic, detectionProtocolGemini}
	lower := strings.ToLower(model)
	if strings.Contains(lower, "claude") {
		order = []string{detectionProtocolAnthropic, detectionProtocolOpenAI, detectionProtocolGemini}
	} else if strings.Contains(lower, "gemini") {
		order = []string{detectionProtocolGemini, detectionProtocolOpenAI, detectionProtocolAnthropic}
	}
	evidence := make([]detectionEvidence, 0, len(order))
	for _, protocol := range order {
		ok, ev := protocolSmoke(ctx, target, protocol, model)
		evidence = append(evidence, ev)
		if ok {
			return protocol, detectionProbeResult{
				ID:         "protocol.auto_detect",
				Name:       "协议自动识别",
				Category:   "Protocol",
				Status:     "success",
				Confidence: 0.98,
				Summary:    "已通过真实推理请求识别为 " + protocol + " 协议",
				Evidence:   evidence,
			}
		}
	}
	return "", detectionProbeResult{
		ID:              "protocol.auto_detect",
		Name:            "协议自动识别",
		Category:        "Protocol",
		Status:          "failed",
		Confidence:      0.95,
		Summary:         "已尝试 OpenAI、Anthropic、Gemini 协议，但没有得到可验证的模型响应",
		FailureReason:   "URL、Key、模型与协议组合无法完成最小推理请求",
		ReasonCode:      "PROTOCOL_NOT_DETECTED",
		Recommendations: []string{"确认 Base URL 是否包含正确的 API 前缀", "确认该 Key 对所选模型有推理权限", "可手动指定协议后重新检测"},
		Evidence:        evidence,
	}
}

func protocolSmoke(ctx context.Context, target *detectionTarget, protocol, model string) (bool, detectionEvidence) {
	var result detectionHTTPResult
	var err error
	switch protocol {
	case detectionProtocolAnthropic:
		result, err = target.doJSON(ctx, protocol, http.MethodPost, "/v1/messages", nil, map[string]any{
			"model": model, "max_tokens": 24,
			"messages": []any{map[string]any{"role": "user", "content": "Reply exactly: PROTOCOL_OK"}},
		})
	case detectionProtocolGemini:
		endpoint := "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(model, "models/")) + ":generateContent"
		result, err = target.doJSON(ctx, protocol, http.MethodPost, endpoint, nil, map[string]any{
			"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "Reply exactly: PROTOCOL_OK"}}}},
		})
	default:
		result, err = target.doJSON(ctx, protocol, http.MethodPost, "/v1/chat/completions", nil, map[string]any{
			"model": model, "messages": []any{map[string]any{"role": "user", "content": "Reply exactly: PROTOCOL_OK"}}, "max_tokens": 24,
		})
	}
	ev := detectionEvidence{Kind: "protocol_probe", Label: protocol + " 最小推理", Expected: "HTTP 2xx 且包含模型输出", HTTPStatus: result.StatusCode, DurationMS: result.Duration.Milliseconds()}
	if err != nil {
		ev.Actual = sanitizeDetectionText(err.Error(), target.apiKey, 240)
		return false, ev
	}
	ev.ResponseExcerpt = detectionExcerpt(result.Body, target.apiKey, 400)
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		ev.Actual = fmt.Sprintf("HTTP %d", result.StatusCode)
		return false, ev
	}
	text := extractProtocolText(protocol, result.Body)
	if strings.TrimSpace(text) == "" {
		ev.Actual = "HTTP 2xx，但未识别到文本输出"
		return false, ev
	}
	ev.Actual = "HTTP 2xx 且响应结构可解析"
	return true, ev
}

func runProtocolDetection(ctx context.Context, target *detectionTarget, protocol, model, mode string) []detectionProbeResult {
	switch protocol {
	case detectionProtocolAnthropic:
		return runAnthropicDetection(ctx, target, model, mode)
	case detectionProtocolGemini:
		return runGeminiDetection(ctx, target, model, mode)
	default:
		return runOpenAIDetection(ctx, target, model, mode)
	}
}

func detectionID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}

func detectionExcerpt(body []byte, apiKey string, max int) string {
	return sanitizeDetectionText(string(body), apiKey, max)
}

func sanitizeDetectionText(value, apiKey string, max int) string {
	value = strings.ReplaceAll(value, apiKey, "[REDACTED]")
	value = strings.ReplaceAll(value, "Bearer "+apiKey, "Bearer [REDACTED]")
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		value = value[:max] + "…"
	}
	return value
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func detectionFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractProtocolText(protocol string, body []byte) string {
	switch protocol {
	case detectionProtocolAnthropic:
		return extractAnthropicText(body)
	case detectionProtocolGemini:
		return extractGeminiText(body)
	default:
		return extractOpenAIText(body)
	}
}

func parseJSONMap(body []byte) map[string]any {
	var root map[string]any
	if json.Unmarshal(body, &root) != nil {
		return nil
	}
	return root
}

func numberValue(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func mapValue(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func sliceValue(value any) []any {
	s, _ := value.([]any)
	return s
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
