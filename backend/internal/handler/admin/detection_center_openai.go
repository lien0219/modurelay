package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func runOpenAIDetection(ctx context.Context, target *detectionTarget, model, mode string) []detectionProbeResult {
	basic, basicBody := probeOpenAIBasic(ctx, target, model)
	probes := []detectionProbeResult{
		basic,
		probeOpenAIStreaming(ctx, target, model),
		probeOpenAITools(ctx, target, model),
		probeOpenAIStructuredOutputs(ctx, target, model),
		probeOpenAIUsage(basicBody),
	}
	if mode == detectionModeDeep {
		probes = append(probes, probeOpenAICacheObservation(ctx, target, model), probeOpenAIMediaEndpoint(ctx, target, model))
	} else {
		probes = append(probes,
			notApplicableProbe("openai.cache_observation", "Prompt Cache Observation", "Caching", "标准检测不执行长前缀缓存重复请求；深度检测会观察 cached_tokens"),
			notApplicableProbe("openai.media_endpoint", "媒体端点预检", "Media", "标准检测不请求图片/视频端点；深度检测会执行不触发生成的端点契约预检"),
		)
	}
	probes = append(probes,
		notApplicableProbe("openai.context_management", "Context Management", "Context", "该项是 Anthropic 协议专有探针，不应对 OpenAI-compatible 模型判失败"),
		notApplicableProbe("openai.citations", "Structured Citations", "Citations", "OpenAI-compatible 接口没有统一的 Anthropic Citations 响应契约，不能用同一规则判定"),
	)
	return probes
}


func probeOpenAIMediaEndpoint(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	kind, endpoint := openAIMediaEndpointForModel(model)
	if endpoint == "" {
		return notApplicableProbe("openai.media_endpoint", "媒体端点预检", "Media", "当前模型未识别为图片或视频模型，不执行媒体端点预检")
	}

	result, err := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodGet, endpoint, nil, nil)
	ev := requestEvidence(kind+" 端点契约预检", "不创建媒体任务；2xx/400/405/422 表示端点存在并能被网关识别，404 表示端点缺失", result, target)
	if err != nil {
		return inconclusiveProbe("openai.media_endpoint", "媒体端点预检", "Media", "媒体端点网络预检失败："+classifyHTTPFailure(result, err, target), ev)
	}
	switch result.StatusCode {
	case http.StatusOK, http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
		ev.Actual = fmt.Sprintf("HTTP %d，%s 端点存在；本探针未提交生成任务，不产生媒体生成费用", result.StatusCode, kind)
		return detectionProbeResult{
			ID:         "openai.media_endpoint",
			Name:       "媒体端点预检",
			Category:   "Media",
			Status:     "success",
			Confidence: 0.9,
			Summary:    kind + " 端点已识别，可进入真实小额生成验收",
			Evidence:   []detectionEvidence{ev},
		}
	case http.StatusUnauthorized, http.StatusForbidden:
		ev.Actual = fmt.Sprintf("HTTP %d，端点可达但当前 Key 无法通过鉴权", result.StatusCode)
		return unavailableProbe("openai.media_endpoint", "媒体端点预检", "Media", kind+" 端点可达，但当前 Key 没有可验证的媒体访问权限", ev)
	case http.StatusNotFound:
		ev.Actual = "HTTP 404，未发现兼容媒体端点"
		return failedProbe("openai.media_endpoint", "媒体端点预检", "Media", "MEDIA_ENDPOINT_NOT_FOUND", kind+" 端点不存在", "该 Base URL 没有暴露 "+endpoint+"，或供应商使用了非 OpenAI-compatible 媒体接口", 0.95, ev)
	default:
		ev.Actual = fmt.Sprintf("HTTP %d，无法仅凭预检确认端点能力", result.StatusCode)
		return inconclusiveProbe("openai.media_endpoint", "媒体端点预检", "Media", kind+" 端点返回非标准预检状态，需真实小额生成确认", ev)
	}
}

func openAIMediaEndpointForModel(model string) (kind, endpoint string) {
	value := strings.ToLower(strings.TrimSpace(model))
	for _, marker := range []string{"seedance", "kling", "hailuo", "minimax-video", "grok-imagine-video", "sora", "veo", "wan-video", "wan2", "video"} {
		if strings.Contains(value, marker) {
			return "视频", "/v1/videos"
		}
	}
	for _, marker := range []string{"gpt-image", "seedream", "flux", "recraft", "ideogram", "dall-e", "imagen", "qwen-image", "stable-diffusion", "sdxl", "kolors", "cogview", "nano-banana", "image-"} {
		if strings.Contains(value, marker) {
			return "图片", "/v1/images/generations"
		}
	}
	return "", ""
}

func openAIChatBody(model, prompt string) map[string]any {
	return map[string]any{
		"model":      model,
		"messages":   []any{map[string]any{"role": "user", "content": prompt}},
		"max_tokens": 64,
	}
}

func probeOpenAIBasic(ctx context.Context, target *detectionTarget, model string) (detectionProbeResult, []byte) {
	marker := "BASIC_OK_" + detectionID("o")
	body := openAIChatBody(model, "Reply with this exact token and nothing else: "+marker)
	result, err := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", nil, body)
	ev := requestEvidence("OpenAI Chat Completions 基础请求", "HTTP 2xx 且 choices[0].message.content 可解析", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		// Some modern compatible gateways expose only Responses API. Probe it before declaring failure.
		fallback, fallbackErr := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/responses", nil, map[string]any{
			"model": model, "input": "Reply with this exact token and nothing else: " + marker, "max_output_tokens": 64,
		})
		fallbackEv := requestEvidence("OpenAI Responses 回退探针", "HTTP 2xx 且可解析 output_text/output[]", fallback, target)
		if fallbackErr == nil && fallback.StatusCode >= 200 && fallback.StatusCode < 300 && extractOpenAIText(fallback.Body) != "" {
			fallbackEv.Actual = "Chat Completions 不可用，但 Responses API 可正常推理"
			return detectionProbeResult{ID: "openai.basic", Name: "基础推理", Category: "Protocol", Status: "partial", Confidence: 0.98, Summary: "检测到 OpenAI Responses 兼容，但 Chat Completions 不可用", ReasonCode: "PARTIAL_OPENAI_COMPAT", Evidence: []detectionEvidence{ev, fallbackEv}}, fallback.Body
		}
		return failedProbe("openai.basic", "基础推理", "Protocol", "BASE_REQUEST_FAILED", "OpenAI-compatible 基础推理未通过", classifyHTTPFailure(result, err, target), 0.99, ev, fallbackEv), nil
	}
	text := extractOpenAIText(result.Body)
	if text == "" {
		return failedProbe("openai.basic", "基础推理", "Protocol", "RESPONSE_SHAPE_MISMATCH", "HTTP 2xx，但 choices/message 响应结构无法解析", "没有识别到模型文本输出", 0.99, ev), result.Body
	}
	ev.Actual = "Chat Completions 响应结构可解析"
	return detectionProbeResult{ID: "openai.basic", Name: "基础推理", Category: "Protocol", Status: "success", Confidence: 1, Summary: "OpenAI Chat Completions 基础协议已验证", Evidence: []detectionEvidence{ev}}, result.Body
}

func probeOpenAIStreaming(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	body := openAIChatBody(model, "Reply exactly: STREAM_OK")
	body["stream"] = true
	result, err := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", http.Header{"Accept": []string{"text/event-stream"}}, body)
	ev := requestEvidence("Chat Completions SSE", "HTTP 2xx、data: 事件流、至少一个 choices delta、终止事件", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return unavailableProbe("openai.streaming", "流式响应", "Streaming", "Chat Completions 流式接口不可用："+classifyHTTPFailure(result, err, target), ev)
	}
	raw := string(result.Body)
	hasData := strings.Contains(raw, "data:")
	hasChoices := strings.Contains(raw, "\"choices\"")
	hasDone := strings.Contains(raw, "[DONE]") || strings.Contains(raw, "finish_reason")
	if !hasData || !hasChoices || !hasDone {
		ev.Actual = fmt.Sprintf("data=%t, choices=%t, done=%t", hasData, hasChoices, hasDone)
		return failedProbe("openai.streaming", "流式响应", "Streaming", "STREAM_EVENT_MISMATCH", "HTTP 2xx，但 SSE 事件结构不完整", ev.Actual, 0.98, ev)
	}
	return detectionProbeResult{ID: "openai.streaming", Name: "流式响应", Category: "Streaming", Status: "success", Confidence: 0.99, Summary: "OpenAI-compatible SSE 流式生命周期已验证", Evidence: []detectionEvidence{ev}}
}

func probeOpenAITools(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	body := openAIChatBody(model, "Call detection_probe with code TOOL_OK. Do not answer in plain text.")
	body["tools"] = []any{map[string]any{"type": "function", "function": map[string]any{
		"name": "detection_probe", "description": "Protocol conformance probe",
		"parameters": map[string]any{"type": "object", "properties": map[string]any{"code": map[string]any{"type": "string"}}, "required": []string{"code"}, "additionalProperties": false},
	}}}
	body["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": "detection_probe"}}
	result, err := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", nil, body)
	ev := requestEvidence("强制 Function Calling", "choices[0].message.tool_calls 中存在 detection_probe", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return unavailableProbe("openai.tools", "Tool Calling", "Tools", "当前接口或模型未完成 Function Calling："+classifyHTTPFailure(result, err, target), ev)
	}
	root := parseJSONMap(result.Body)
	choices := sliceValue(root["choices"])
	if len(choices) == 0 {
		return failedProbe("openai.tools", "Tool Calling", "Tools", "TOOL_BLOCK_MISSING", "HTTP 2xx，但没有 choices/tool_calls", "响应结构未体现工具调用", 0.98, ev)
	}
	message := mapValue(mapValue(choices[0])["message"])
	for _, call := range sliceValue(message["tool_calls"]) {
		fn := mapValue(mapValue(call)["function"])
		if stringValue(fn["name"]) == "detection_probe" {
			ev.Actual = "检测到结构化 tool_calls.function=detection_probe"
			return detectionProbeResult{ID: "openai.tools", Name: "Tool Calling", Category: "Tools", Status: "success", Confidence: 1, Summary: "强制 Function Calling 结构已验证", Evidence: []detectionEvidence{ev}}
		}
	}
	return failedProbe("openai.tools", "Tool Calling", "Tools", "TOOL_BLOCK_MISSING", "HTTP 2xx，但工具调用没有真正发生", "没有检测到目标 function tool_call", 0.99, ev)
}

func probeOpenAIStructuredOutputs(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	const expectedProbe = "SCHEMA_OK_91"
	const expectedCount = int64(7)
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"probe": map[string]any{"type": "string", "const": expectedProbe}, "count": map[string]any{"type": "integer", "const": expectedCount}},
		"required":   []string{"probe", "count"}, "additionalProperties": false,
	}
	body := openAIChatBody(model, "Return the object required by the response schema.")
	body["response_format"] = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "detection_probe", "strict": true, "schema": schema}}
	valid, validErr := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", nil, body)
	validEv := requestEvidence("合法 response_format=json_schema", "HTTP 2xx 且输出严格通过 schema 校验", valid, target)

	negative := openAIChatBody(model, "Reply OK")
	negative["response_format"] = map[string]any{"type": "__modurelay_invalid_response_format__"}
	invalid, invalidErr := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", nil, negative)
	invalidEv := requestEvidence("非法 response_format.type 负向探针", "HTTP 4xx", invalid, target)

	if validErr != nil || valid.StatusCode < 200 || valid.StatusCode >= 300 {
		return unavailableProbe("openai.structured_outputs", "Structured Outputs", "Structured Output", "当前接口或模型没有接受 json_schema 请求："+classifyHTTPFailure(valid, validErr, target), validEv, invalidEv)
	}
	ok, detail := validateDetectionSchemaPayload(extractOpenAIText(valid.Body), expectedProbe, expectedCount)
	negativeRejected := invalidErr == nil && invalid.StatusCode >= 400 && invalid.StatusCode < 500
	if ok && negativeRejected {
		validEv.Actual = detail
		return detectionProbeResult{ID: "openai.structured_outputs", Name: "Structured Outputs", Category: "Structured Output", Status: "success", Confidence: 1, Summary: "JSON Schema 约束和非法参数拒绝均已验证", Evidence: []detectionEvidence{validEv, invalidEv}}
	}
	if !ok && invalidErr == nil && invalid.StatusCode >= 200 && invalid.StatusCode < 300 {
		validEv.Actual = detail
		result := failedProbe("openai.structured_outputs", "Structured Outputs", "Structured Output", "PROTOCOL_FIELD_DROPPED", "HTTP 200 但 Structured Outputs 没有真正生效", "合法 schema 未约束输出，同时非法 response_format 也被正常接受；兼容层高度疑似忽略该字段", 0.995, validEv, invalidEv)
		result.PossibleCauses = []string{"response_format 被请求 DTO 丢弃", "兼容层没有向真实上游映射 JSON Schema", "上游响应被转换为普通文本"}
		return result
	}
	if !ok {
		validEv.Actual = detail
		return failedProbe("openai.structured_outputs", "Structured Outputs", "Structured Output", "SCHEMA_VALIDATION_FAILED", "结构化输出未通过 Schema 校验", detail, 0.98, validEv, invalidEv)
	}
	return detectionProbeResult{ID: "openai.structured_outputs", Name: "Structured Outputs", Category: "Structured Output", Status: "partial", Confidence: 0.9, Summary: "Schema 行为正常，但负向错误语义异常", ReasonCode: "ERROR_SEMANTICS_MISMATCH", Evidence: []detectionEvidence{validEv, invalidEv}}
}

func probeOpenAIUsage(body []byte) detectionProbeResult {
	if len(body) == 0 {
		return inconclusiveProbe("openai.usage", "Usage 透传", "Usage", "基础请求没有可用于检查的响应体")
	}
	root := parseJSONMap(body)
	usage := mapValue(root["usage"])
	if usage == nil {
		return failedProbe("openai.usage", "Usage 透传", "Usage", "USAGE_MISSING", "模型请求成功，但 usage 字段缺失", "无法核对输入/输出 token 或缓存/推理 token 明细", 0.97)
	}
	return detectionProbeResult{ID: "openai.usage", Name: "Usage 透传", Category: "Usage", Status: "success", Confidence: 0.99, Summary: fmt.Sprintf("usage 已返回，字段：%s", strings.Join(sortedKeys(usage), ", "))}
}

func probeOpenAICacheObservation(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	nonce := detectionID("openai-cache")
	prefix := strings.Repeat("Stable OpenAI cache observation prefix "+nonce+". Keep this prefix byte-identical. ", 450)
	body := openAIChatBody(model, prefix+"\nReply exactly: CACHE_OBSERVE_OK")
	first, firstErr := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", nil, body)
	second, secondErr := target.doJSON(ctx, detectionProtocolOpenAI, http.MethodPost, "/v1/chat/completions", nil, body)
	firstEv := requestEvidence("缓存观察首次请求", "记录 usage.prompt_tokens_details.cached_tokens", first, target)
	secondEv := requestEvidence("缓存观察重复请求", "若服务暴露自动缓存，重复长前缀可能出现 cached_tokens > 0", second, target)
	if firstErr != nil || secondErr != nil || first.StatusCode < 200 || first.StatusCode >= 300 || second.StatusCode < 200 || second.StatusCode >= 300 {
		return inconclusiveProbe("openai.cache_observation", "Prompt Cache Observation", "Caching", "两次长前缀请求未全部成功，无法观察缓存 metadata", firstEv, secondEv)
	}
	cached := func(body []byte) int64 {
		usage := mapValue(parseJSONMap(body)["usage"])
		details := mapValue(usage["prompt_tokens_details"])
		return numberValue(details["cached_tokens"])
	}
	firstCached, secondCached := cached(first.Body), cached(second.Body)
	firstEv.Actual = fmt.Sprintf("cached_tokens=%d", firstCached)
	secondEv.Actual = fmt.Sprintf("cached_tokens=%d", secondCached)
	if secondCached > 0 {
		return detectionProbeResult{ID: "openai.cache_observation", Name: "Prompt Cache Observation", Category: "Caching", Status: "success", Confidence: 0.95, Summary: "重复长前缀请求观察到 cached_tokens，存在可验证的缓存命中证据", Evidence: []detectionEvidence{firstEv, secondEv}}
	}
	return inconclusiveProbe("openai.cache_observation", "Prompt Cache Observation", "Caching", "未观察到 cached_tokens；OpenAI-compatible 服务可能不暴霚该字段或该次请求没有命中，不能据此直接判定缓存不支持", firstEv, secondEv)
}
