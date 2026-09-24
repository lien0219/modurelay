package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func runGeminiDetection(ctx context.Context, target *detectionTarget, model, mode string) []detectionProbeResult {
	basic, basicBody := probeGeminiBasic(ctx, target, model)
	probes := []detectionProbeResult{
		basic,
		probeGeminiStreaming(ctx, target, model),
		probeGeminiTools(ctx, target, model),
		probeGeminiStructuredOutputs(ctx, target, model),
		probeGeminiUsage(basicBody),
	}
	probes = append(probes,
		notApplicableProbe("gemini.context_management", "Context Management", "Context", "Anthropic Context Management 契约不适用于 Gemini Native API"),
		notApplicableProbe("gemini.anthropic_cache", "Anthropic Prompt Caching", "Caching", "Anthropic cache_control 契约不适用于 Gemini Native API"),
		notApplicableProbe("gemini.citations", "Anthropic Citations", "Citations", "Anthropic document.citations 契约不适用于 Gemini Native API"),
	)
	_ = mode
	return probes
}

func geminiGenerateEndpoint(model string) string {
	return "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(model, "models/")) + ":generateContent"
}

func geminiBody(prompt string) map[string]any {
	return map[string]any{
		"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": prompt}}}},
	}
}

func probeGeminiBasic(ctx context.Context, target *detectionTarget, model string) (detectionProbeResult, []byte) {
	marker := "BASIC_OK_" + detectionID("g")
	body := geminiBody("Reply with this exact token and nothing else: " + marker)
	result, err := target.doJSON(ctx, detectionProtocolGemini, http.MethodPost, geminiGenerateEndpoint(model), nil, body)
	ev := requestEvidence("Gemini generateContent 基础请求", "HTTP 2xx 且 candidates[].content.parts[].text 可解析", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return failedProbe("gemini.basic", "基础 generateContent", "Protocol", "BASE_REQUEST_FAILED", "Gemini Native 基础请求未通过", classifyHTTPFailure(result, err, target), 0.99, ev), nil
	}
	if extractGeminiText(result.Body) == "" {
		return failedProbe("gemini.basic", "基础 generateContent", "Protocol", "RESPONSE_SHAPE_MISMATCH", "HTTP 2xx，但 candidates/parts 文本结构无法解析", "没有识别到模型文本输出", 0.99, ev), result.Body
	}
	return detectionProbeResult{ID: "gemini.basic", Name: "基础 generateContent", Category: "Protocol", Status: "success", Confidence: 1, Summary: "Gemini Native generateContent 基础协议已验证", Evidence: []detectionEvidence{ev}}, result.Body
}

func probeGeminiStreaming(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	endpoint := "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(model, "models/")) + ":streamGenerateContent?alt=sse"
	result, err := target.doJSON(ctx, detectionProtocolGemini, http.MethodPost, endpoint, http.Header{"Accept": []string{"text/event-stream"}}, geminiBody("Reply exactly: STREAM_OK"))
	ev := requestEvidence("Gemini streamGenerateContent SSE", "HTTP 2xx、data: 事件且包含 candidates", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return unavailableProbe("gemini.streaming", "流式响应", "Streaming", "Gemini 流式接口不可用："+classifyHTTPFailure(result, err, target), ev)
	}
	raw := string(result.Body)
	if !strings.Contains(raw, "data:") || !strings.Contains(raw, "\"candidates\"") {
		ev.Actual = "HTTP 2xx，但缺少 Gemini SSE data/candidates 结构"
		return failedProbe("gemini.streaming", "流式响应", "Streaming", "STREAM_EVENT_MISMATCH", "流式 HTTP 成功但事件结构不符合预期", ev.Actual, 0.98, ev)
	}
	return detectionProbeResult{ID: "gemini.streaming", Name: "流式响应", Category: "Streaming", Status: "success", Confidence: 0.99, Summary: "Gemini streamGenerateContent SSE 已验证", Evidence: []detectionEvidence{ev}}
}

func probeGeminiTools(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	body := geminiBody("Call detection_probe with code TOOL_OK. Do not answer normally.")
	body["tools"] = []any{map[string]any{"functionDeclarations": []any{map[string]any{
		"name": "detection_probe", "description": "Protocol conformance probe",
		"parameters": map[string]any{"type": "OBJECT", "properties": map[string]any{"code": map[string]any{"type": "STRING"}}, "required": []string{"code"}},
	}}}}
	body["toolConfig"] = map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY", "allowedFunctionNames": []string{"detection_probe"}}}
	result, err := target.doJSON(ctx, detectionProtocolGemini, http.MethodPost, geminiGenerateEndpoint(model), nil, body)
	ev := requestEvidence("Gemini 强制 Function Calling", "candidate parts 中存在 functionCall.name=detection_probe", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return unavailableProbe("gemini.tools", "Function Calling", "Tools", "当前接口或模型没有完成 Gemini Function Calling："+classifyHTTPFailure(result, err, target), ev)
	}
	root := parseJSONMap(result.Body)
	for _, candidate := range sliceValue(root["candidates"]) {
		content := mapValue(mapValue(candidate)["content"])
		for _, part := range sliceValue(content["parts"]) {
			call := mapValue(mapValue(part)["functionCall"])
			if stringValue(call["name"]) == "detection_probe" {
				ev.Actual = "检测到 functionCall=detection_probe"
				return detectionProbeResult{ID: "gemini.tools", Name: "Function Calling", Category: "Tools", Status: "success", Confidence: 1, Summary: "Gemini Function Calling 结构已验证", Evidence: []detectionEvidence{ev}}
			}
		}
	}
	return failedProbe("gemini.tools", "Function Calling", "Tools", "TOOL_BLOCK_MISSING", "HTTP 2xx，但 Function Calling 没有真正发生", "没有检测到目标 functionCall", 0.99, ev)
}

func probeGeminiStructuredOutputs(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	const expectedProbe = "SCHEMA_OK_91"
	const expectedCount = int64(7)
	schema := map[string]any{
		"type":       "OBJECT",
		"properties": map[string]any{"probe": map[string]any{"type": "STRING", "enum": []string{expectedProbe}}, "count": map[string]any{"type": "INTEGER"}},
		"required":   []string{"probe", "count"},
	}
	body := geminiBody("Return the exact object required by the response schema. count must be 7.")
	body["generationConfig"] = map[string]any{"responseMimeType": "application/json", "responseSchema": schema}
	valid, validErr := target.doJSON(ctx, detectionProtocolGemini, http.MethodPost, geminiGenerateEndpoint(model), nil, body)
	validEv := requestEvidence("Gemini JSON responseSchema", "HTTP 2xx 且 JSON 内容满足探针约束", valid, target)

	negative := geminiBody("Reply OK")
	negative["generationConfig"] = map[string]any{"responseMimeType": "application/x-modurelay-invalid"}
	invalid, invalidErr := target.doJSON(ctx, detectionProtocolGemini, http.MethodPost, geminiGenerateEndpoint(model), nil, negative)
	invalidEv := requestEvidence("非法 responseMimeType 负向探针", "HTTP 4xx", invalid, target)

	if validErr != nil || valid.StatusCode < 200 || valid.StatusCode >= 300 {
		return unavailableProbe("gemini.structured_outputs", "Structured JSON", "Structured Output", "当前模型或兼容层没有接受 responseSchema："+classifyHTTPFailure(valid, validErr, target), validEv, invalidEv)
	}
	ok, detail := validateDetectionSchemaPayload(extractGeminiText(valid.Body), expectedProbe, expectedCount)
	negativeRejected := invalidErr == nil && invalid.StatusCode >= 400 && invalid.StatusCode < 500
	if ok && negativeRejected {
		validEv.Actual = detail
		return detectionProbeResult{ID: "gemini.structured_outputs", Name: "Structured JSON", Category: "Structured Output", Status: "success", Confidence: 0.99, Summary: "Gemini responseSchema 行为和负向错误语义均已验证", Evidence: []detectionEvidence{validEv, invalidEv}}
	}
	if !ok && invalidErr == nil && invalid.StatusCode >= 200 && invalid.StatusCode < 300 {
		validEv.Actual = detail
		result := failedProbe("gemini.structured_outputs", "Structured JSON", "Structured Output", "PROTOCOL_FIELD_DROPPED", "HTTP 200 但 responseSchema 没有真正约束输出", "合法 schema 校验失败，同时非法 MIME type 也被正常接受；generationConfig 可能在兼容层被过滤", 0.99, validEv, invalidEv)
		result.Recommendations = []string{"检查 generationConfig.responseMimeType/responseSchema 的实际上游请求", "对照 Gemini Native 直连错误响应"}
		return result
	}
	if !ok {
		return failedProbe("gemini.structured_outputs", "Structured JSON", "Structured Output", "SCHEMA_VALIDATION_FAILED", "Structured JSON 输出未通过校验", detail, 0.97, validEv, invalidEv)
	}
	return detectionProbeResult{ID: "gemini.structured_outputs", Name: "Structured JSON", Category: "Structured Output", Status: "partial", Confidence: 0.85, Summary: "结构化 JSON 生效，但负向错误语义异常", ReasonCode: "ERROR_SEMANTICS_MISMATCH", Evidence: []detectionEvidence{validEv, invalidEv}}
}

func probeGeminiUsage(body []byte) detectionProbeResult {
	if len(body) == 0 {
		return inconclusiveProbe("gemini.usage", "Usage Metadata", "Usage", "基础请求没有可用于检查的响应体")
	}
	root := parseJSONMap(body)
	usage := mapValue(root["usageMetadata"])
	if usage == nil {
		return failedProbe("gemini.usage", "Usage Metadata", "Usage", "USAGE_MISSING", "模型请求成功，但 usageMetadata 缺失", "无法核对 prompt/candidate token 使用量", 0.96)
	}
	return detectionProbeResult{ID: "gemini.usage", Name: "Usage Metadata", Category: "Usage", Status: "success", Confidence: 0.99, Summary: fmt.Sprintf("usageMetadata 已返回，字段：%s", strings.Join(sortedKeys(usage), ", "))}
}
