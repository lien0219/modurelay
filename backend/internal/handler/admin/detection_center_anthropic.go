package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func runAnthropicDetection(ctx context.Context, target *detectionTarget, model, mode string) []detectionProbeResult {
	probes := []detectionProbeResult{
		probeAnthropicBasic(ctx, target, model),
		probeAnthropicStreaming(ctx, target, model),
		probeAnthropicTools(ctx, target, model),
		probeAnthropicStructured(ctx, target, model),
		probeAnthropicThinking(ctx, target, model),
		probeAnthropicCitations(ctx, target, model),
	}
	if mode == detectionModeDeep {
		probes = append(probes,
			probeAnthropicPromptCache(ctx, target, model),
			probeAnthropicUndeclaredCache(ctx, target, model),
			probeAnthropicContextManagement(ctx, target, model),
		)
	} else {
		probes = append(probes,
			notApplicableProbe("anthropic.prompt_cache", "Prompt Caching", "Caching", "标准检测不执行高 Token 重复缓存请求；深度检测会验证写入与读取"),
			notApplicableProbe("anthropic.undeclared_cache", "自动缓存探测", "Caching", "标准检测不执行无 cache_control 的 A/B 对照；深度检测会执行"),
			notApplicableProbe("anthropic.context_management", "Context Management", "Context", "标准检测不执行多轮 tool_result 清理探针；深度检测会执行"),
		)
	}
	return probes
}

func anthropicMessageBody(model string, prompt any, maxTokens int) map[string]any {
	return map[string]any{
		"model": model,
		"max_tokens": maxTokens,
		"messages": []any{map[string]any{"role": "user", "content": prompt}},
	}
}

func probeAnthropicBasic(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	body := anthropicMessageBody(model, "Reply exactly: BASIC_OK", 48)
	result, err := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, body)
	ev := requestEvidence("Anthropic Messages 基础请求", "HTTP 2xx、content[] 可解析、usage 存在", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return failedProbe("anthropic.basic", "基础 Messages", "Protocol", "BASE_REQUEST_FAILED", "基础 Messages 请求未通过", classifyHTTPFailure(result, err, target), .99, ev)
	}
	if extractAnthropicText(result.Body) == "" {
		return failedProbe("anthropic.basic", "基础 Messages", "Protocol", "RESPONSE_SHAPE_MISMATCH", "HTTP 2xx 但没有可解析文本块", "content[] 未包含有效 type=text 输出", .99, ev)
	}
	usage := anthropicUsage(result.Body)
	if usage == nil {
		return detectionProbeResult{ID:"anthropic.basic", Name:"基础 Messages", Category:"Protocol", Status:"partial", Confidence:.95, Summary:"基础调用成功，但 usage 缺失", ReasonCode:"USAGE_MISSING", Evidence:[]detectionEvidence{ev}}
	}
	ev.Actual = fmt.Sprintf("input_tokens=%d, output_tokens=%d", numberValue(usage["input_tokens"]), numberValue(usage["output_tokens"]))
	return detectionProbeResult{ID:"anthropic.basic", Name:"基础 Messages", Category:"Protocol", Status:"success", Confidence:1, Summary:"Messages 基础协议和 usage 均已验证", Evidence:[]detectionEvidence{ev}}
}

func probeAnthropicStreaming(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	body := anthropicMessageBody(model, "Reply exactly: STREAM_OK", 48)
	body["stream"] = true
	result, err := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", http.Header{"Accept":[]string{"text/event-stream"}}, body)
	ev := requestEvidence("SSE 流式探针", "包含 message_start、content_block_delta、message_stop", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return unavailableProbe("anthropic.streaming", "流式响应", "Streaming", "流式请求不可用："+classifyHTTPFailure(result, err, target), ev)
	}
	raw := string(result.Body)
	if !strings.Contains(raw, "message_start") || !strings.Contains(raw, "content_block_delta") || !strings.Contains(raw, "message_stop") {
		return failedProbe("anthropic.streaming", "流式响应", "Streaming", "STREAM_EVENT_MISMATCH", "HTTP 2xx 但 Anthropic SSE 生命周期不完整", "缺少一个或多个关键 SSE 事件", .99, ev)
	}
	return detectionProbeResult{ID:"anthropic.streaming", Name:"流式响应", Category:"Streaming", Status:"success", Confidence:1, Summary:"Anthropic SSE 生命周期已验证", Evidence:[]detectionEvidence{ev}}
}

func probeAnthropicTools(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	body := anthropicMessageBody(model, "Call detection_probe with code TOOL_OK. Do not answer in text.", 128)
	body["tools"] = []any{map[string]any{
		"name":"detection_probe",
		"description":"Protocol conformance probe",
		"input_schema":map[string]any{
			"type":"object",
			"properties":map[string]any{"code":map[string]any{"type":"string"}},
			"required":[]string{"code"},
			"additionalProperties":false,
		},
	}}
	body["tool_choice"] = map[string]any{"type":"tool","name":"detection_probe"}
	result, err := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, body)
	ev := requestEvidence("强制 Tool Use", "返回 tool_use=detection_probe 且 input.code=TOOL_OK", result, target)
	if err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		return unavailableProbe("anthropic.tools", "Tool Calling", "Tools", "模型或接口未接受 Tool Use："+classifyHTTPFailure(result, err, target), ev)
	}
	root := parseJSONMap(result.Body)
	for _, item := range sliceValue(root["content"]) {
		block := mapValue(item)
		if stringValue(block["type"]) == "tool_use" && stringValue(block["name"]) == "detection_probe" {
			if stringValue(mapValue(block["input"])["code"]) == "TOOL_OK" {
				return detectionProbeResult{ID:"anthropic.tools", Name:"Tool Calling", Category:"Tools", Status:"success", Confidence:1, Summary:"强制工具调用和参数结构已验证", Evidence:[]detectionEvidence{ev}}
			}
			return detectionProbeResult{ID:"anthropic.tools", Name:"Tool Calling", Category:"Tools", Status:"partial", Confidence:.96, Summary:"产生了 tool_use，但参数约束不一致", ReasonCode:"TOOL_ARGUMENT_MISMATCH", Evidence:[]detectionEvidence{ev}}
		}
	}
	return failedProbe("anthropic.tools", "Tool Calling", "Tools", "TOOL_BLOCK_MISSING", "HTTP 2xx 但没有真正产生 tool_use", "工具字段可能被忽略，或模型自由作答", .99, ev)
}

func probeAnthropicStructured(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	const expectedProbe = "SCHEMA_OK_91"
	const expectedCount = int64(7)
	schema := map[string]any{
		"type":"object",
		"properties":map[string]any{
			"probe":map[string]any{"type":"string","const":expectedProbe},
			"count":map[string]any{"type":"integer","const":expectedCount},
		},
		"required":[]string{"probe","count"},
		"additionalProperties":false,
	}
	validBody := anthropicMessageBody(model, "Return only the object required by the output schema.", 160)
	validBody["output_config"] = map[string]any{"format":map[string]any{"type":"json_schema","schema":schema}}
	valid, validErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, validBody)
	validEv := requestEvidence("合法 Structured Outputs", "HTTP 2xx 且输出严格满足 JSON Schema", valid, target)

	invalidBody := anthropicMessageBody(model, "Reply OK", 32)
	invalidBody["output_config"] = map[string]any{"format":map[string]any{"type":"__modurelay_invalid_output_format__","schema":schema}}
	invalid, invalidErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, invalidBody)
	invalidEv := requestEvidence("非法 output format 负向探针", "HTTP 4xx", invalid, target)

	if validErr != nil || valid.StatusCode < 200 || valid.StatusCode >= 300 {
		return unavailableProbe("anthropic.structured_outputs", "Structured Outputs", "Structured Output", "合法 JSON Schema 请求未被接受："+classifyHTTPFailure(valid, validErr, target), validEv, invalidEv)
	}
	ok, detail := validateDetectionSchemaPayload(extractAnthropicText(valid.Body), expectedProbe, expectedCount)
	rejected := invalidErr == nil && invalid.StatusCode >= 400 && invalid.StatusCode < 500
	if ok && rejected {
		validEv.Actual = detail
		return detectionProbeResult{ID:"anthropic.structured_outputs", Name:"Structured Outputs", Category:"Structured Output", Status:"success", Confidence:1, Summary:"Schema 约束和非法字段拒绝均已验证，能力真实生效", Evidence:[]detectionEvidence{validEv, invalidEv}}
	}
	if !ok && invalidErr == nil && invalid.StatusCode >= 200 && invalid.StatusCode < 300 {
		validEv.Actual = detail
		result := failedProbe("anthropic.structured_outputs", "Structured Outputs", "Structured Output", "PROTOCOL_FIELD_DROPPED", "HTTP 200 但 Structured Outputs 没有真正生效", "合法 schema 未约束输出，同时非法 output_config 枚举也返回 2xx；字段高度疑似在中转/转换层被过滤", .995, validEv, invalidEv)
		result.PossibleCauses = []string{"请求 DTO 未保留 output_config", "Provider Adapter 未转发 Structured Outputs", "协议转换仅保留基础字段"}
		result.Recommendations = []string{"抓取脱敏 outbound JSON 与客户端请求逐字段 diff", "对照直连上游的非法枚举响应", "检查字段白名单与 provider mapper"}
		return result
	}
	if !ok {
		validEv.Actual = detail
		return failedProbe("anthropic.structured_outputs", "Structured Outputs", "Structured Output", "SCHEMA_VALIDATION_FAILED", "返回内容未通过 JSON Schema 校验", detail, .98, validEv, invalidEv)
	}
	return detectionProbeResult{ID:"anthropic.structured_outputs", Name:"Structured Outputs", Category:"Structured Output", Status:"partial", Confidence:.9, Summary:"Schema 已生效，但非法字段错误语义异常", ReasonCode:"ERROR_SEMANTICS_MISMATCH", Evidence:[]detectionEvidence{validEv, invalidEv}}
}

func probeAnthropicThinking(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	validBody := anthropicMessageBody(model, "Solve carefully: find the smallest positive integer divisible by 7 and 11 whose decimal digits sum to 18. Briefly verify the final answer.", 900)
	validBody["thinking"] = map[string]any{"type":"adaptive","display":"summarized"}
	valid, validErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, validBody)
	validEv := requestEvidence("Adaptive Thinking", "请求参数被识别；若模型选择思考并允许显示，应出现 thinking 块", valid, target)

	invalidBody := anthropicMessageBody(model, "Reply OK", 32)
	invalidBody["thinking"] = map[string]any{"type":"__modurelay_invalid_thinking_type__"}
	invalid, invalidErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, invalidBody)
	invalidEv := requestEvidence("非法 thinking.type 负向探针", "HTTP 4xx", invalid, target)

	if validErr != nil || valid.StatusCode < 200 || valid.StatusCode >= 300 {
		return unavailableProbe("anthropic.adaptive_thinking", "Adaptive Thinking", "Reasoning", "adaptive thinking 请求未被接受："+classifyHTTPFailure(valid, validErr, target), validEv, invalidEv)
	}
	hasThinking := false
	for _, item := range sliceValue(parseJSONMap(valid.Body)["content"]) {
		block := mapValue(item)
		if stringValue(block["type"]) == "thinking" && stringValue(block["thinking"]) != "" {
			hasThinking = true
			break
		}
	}
	rejected := invalidErr == nil && invalid.StatusCode >= 400 && invalid.StatusCode < 500
	if hasThinking && rejected {
		return detectionProbeResult{ID:"anthropic.adaptive_thinking", Name:"Adaptive Thinking", Category:"Reasoning", Status:"success", Confidence:1, Summary:"可见 thinking 块与非法字段拒绝均已验证", Evidence:[]detectionEvidence{validEv, invalidEv}}
	}
	if !hasThinking && invalidErr == nil && invalid.StatusCode >= 200 && invalid.StatusCode < 300 {
		result := failedProbe("anthropic.adaptive_thinking", "Adaptive Thinking", "Reasoning", "PROTOCOL_FIELD_DROPPED", "没有 thinking 块，且非法 thinking.type 也返回 2xx", "缺少 thinking 块单独不足以判失败；但负向非法字段同时被接受，形成字段被忽略的高置信度证据", .96, validEv, invalidEv)
		result.Recommendations = []string{"检查 thinking 字段是否被 DTO/provider mapper 过滤", "对照直连上游非法 thinking.type 的错误响应"}
		return result
	}
	if hasThinking {
		return detectionProbeResult{ID:"anthropic.adaptive_thinking", Name:"Adaptive Thinking", Category:"Reasoning", Status:"partial", Confidence:.9, Summary:"观察到 thinking 块，但负向错误语义异常", ReasonCode:"ERROR_SEMANTICS_MISMATCH", Evidence:[]detectionEvidence{validEv, invalidEv}}
	}
	return inconclusiveProbe("anthropic.adaptive_thinking", "Adaptive Thinking", "Reasoning", "thinking 字段能够被上游校验，但 adaptive 模式本身可选择不生成可见思考；仅凭本次没有 thinking 块不能判失败", validEv, invalidEv)
}

func probeAnthropicCitations(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	content := []any{
		map[string]any{
			"type":"document",
			"source":map[string]any{"type":"text","media_type":"text/plain","data":"Project CITE-7 began in 2037 and its coordinator is Aria Vale."},
			"title":"Detection Citation Document",
			"citations":map[string]any{"enabled":true},
		},
		map[string]any{"type":"text","text":"In what year did CITE-7 begin and who coordinated it? Answer from the document and cite it."},
	}
	validBody := anthropicMessageBody(model, content, 200)
	valid, validErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, validBody)
	validEv := requestEvidence("Citations 文档引用", "text block 内存在结构化 citations metadata", valid, target)

	if validErr != nil || valid.StatusCode < 200 || valid.StatusCode >= 300 {
		return unavailableProbe("anthropic.citations", "Citations", "Citations", "citations 请求未被当前模型/接口接受："+classifyHTTPFailure(valid, validErr, target), validEv)
	}
	hasCitation := false
	for _, item := range sliceValue(parseJSONMap(valid.Body)["content"]) {
		block := mapValue(item)
		if stringValue(block["type"]) == "text" && len(sliceValue(block["citations"])) > 0 {
			hasCitation = true
			break
		}
	}
	if hasCitation {
		return detectionProbeResult{ID:"anthropic.citations", Name:"Citations", Category:"Citations", Status:"success", Confidence:1, Summary:"已返回结构化 citations metadata，引用能力真实生效", Evidence:[]detectionEvidence{validEv}}
	}
	return failedProbe("anthropic.citations", "Citations", "Citations", "CITATION_METADATA_MISSING", "HTTP 200 且模型能回答文档事实，但没有任何结构化引用", "不能把模型自行生成的 [1]、URL 或自然语言来源说明当成 Citations 成功；必须验证结构化 citation metadata", .98, validEv)
}

func probeAnthropicPromptCache(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	nonce := detectionID("cache")
	prefix := strings.Repeat("Stable cache conformance prefix "+nonce+". Keep byte-identical. ", 650)
	body := anthropicMessageBody(model, "Reply exactly: CACHE_OK", 32)
	body["system"] = []any{map[string]any{"type":"text","text":prefix,"cache_control":map[string]any{"type":"ephemeral"}}}
	first, firstErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, body)
	second, secondErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, body)
	firstEv := requestEvidence("Prompt Caching 首次请求", "应建立可验证的缓存写入证据", first, target)
	secondEv := requestEvidence("Prompt Caching 重复请求", "相同长前缀在 TTL 内应出现 cache_read_input_tokens > 0", second, target)
	if firstErr != nil || secondErr != nil || first.StatusCode < 200 || first.StatusCode >= 300 || second.StatusCode < 200 || second.StatusCode >= 300 {
		return inconclusiveProbe("anthropic.prompt_cache", "Prompt Caching", "Caching", "两次缓存探针未全部成功，无法可靠判定", firstEv, secondEv)
	}
	create1, read1 := cacheUsage(first.Body)
	create2, read2 := cacheUsage(second.Body)
	firstEv.Actual = fmt.Sprintf("cache_creation=%d, cache_read=%d", create1, read1)
	secondEv.Actual = fmt.Sprintf("cache_creation=%d, cache_read=%d", create2, read2)
	if read2 > 0 && (create1 > 0 || create2 > 0) {
		return detectionProbeResult{ID:"anthropic.prompt_cache", Name:"Prompt Caching", Category:"Caching", Status:"success", Confidence:1, Summary:"已形成缓存写入 → 重复请求命中的完整证据链", Evidence:[]detectionEvidence{firstEv, secondEv}}
	}
	if create1 > 0 || read1 > 0 || create2 > 0 || read2 > 0 {
		return detectionProbeResult{ID:"anthropic.prompt_cache", Name:"Prompt Caching", Category:"Caching", Status:"partial", Confidence:.88, Summary:"观察到部分缓存 activity，但没有形成完整写入→命中链路", ReasonCode:"CACHE_EVIDENCE_PARTIAL", Evidence:[]detectionEvidence{firstEv, secondEv}}
	}
	result := failedProbe("anthropic.prompt_cache", "Prompt Caching", "Caching", "CACHE_NOT_OBSERVED", "两次请求均 HTTP 2xx，但没有产生缓存写入或读取", "已使用唯一长前缀和显式 cache_control，仍未观察到 cache_creation_input_tokens/cache_read_input_tokens", .97, firstEv, secondEv)
	result.PossibleCauses = []string{"cache_control 未透传", "目标兼容服务没有实现该缓存能力", "响应 usage 转换层丢失缓存 token 字段"}
	result.Recommendations = []string{"抓取实际 outbound request 确认 cache_control 存在", "对照直连上游相同请求的原始 usage", "费用判断必须结合供应商实际账单/原始 usage"}
	return result
}

func probeAnthropicUndeclaredCache(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	nonce := detectionID("nocache")
	prefix := strings.Repeat("Undeclared cache observation prefix "+nonce+". No cache_control is sent. ", 650)
	body := anthropicMessageBody(model, "Reply exactly: NO_CACHE_CONTROL_OK", 32)
	body["system"] = prefix
	first, firstErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, body)
	second, secondErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", nil, body)
	firstEv := requestEvidence("无 cache_control 首次请求", "只记录缓存 usage，不预设来源", first, target)
	secondEv := requestEvidence("无 cache_control 重复请求", "若出现缓存字段，仅作为隐式/注入缓存线索", second, target)
	if firstErr != nil || secondErr != nil || first.StatusCode < 200 || first.StatusCode >= 300 || second.StatusCode < 200 || second.StatusCode >= 300 {
		return inconclusiveProbe("anthropic.undeclared_cache", "自动缓存探测", "Caching", "A/B 请求未全部成功，无法观察", firstEv, secondEv)
	}
	create1, read1 := cacheUsage(first.Body)
	create2, read2 := cacheUsage(second.Body)
	firstEv.Actual = fmt.Sprintf("cache_creation=%d, cache_read=%d", create1, read1)
	secondEv.Actual = fmt.Sprintf("cache_creation=%d, cache_read=%d", create2, read2)
	if create1 > 0 || read1 > 0 || create2 > 0 || read2 > 0 {
		return detectionProbeResult{
			ID:"anthropic.undeclared_cache", Name:"自动缓存探测", Category:"Caching", Status:"partial", Confidence:.98,
			Summary:"请求未携带 cache_control，但 usage 出现缓存 activity，需要核对网关注入或上游缓存语义",
			ReasonCode:"UNDECLARED_CACHE_ACTIVITY",
			PossibleCauses:[]string{"网关自动注入 cache_control", "兼容服务存在自有服务端缓存", "usage 字段经过重映射，语义与原生 Anthropic 不完全一致"},
			Recommendations:[]string{"抓取 outbound request 确认是否注入 cache_control", "核对上游原始 usage 和账单", "不能仅凭 read/write token 数字推断缓存来自哪段前缀或计算额外费用"},
			Evidence:[]detectionEvidence{firstEv, secondEv},
		}
	}
	return detectionProbeResult{ID:"anthropic.undeclared_cache", Name:"自动缓存探测", Category:"Caching", Status:"success", Confidence:.96, Summary:"本次无 cache_control 的唯一前缀 A/B 请求未观察到未声明缓存 activity", Evidence:[]detectionEvidence{firstEv, secondEv}}
}

func probeAnthropicContextManagement(ctx context.Context, target *detectionTarget, model string) detectionProbeResult {
	sentinel := "CTX_SENTINEL_" + detectionID("x")
	toolID := "toolu_" + detectionID("w")
	weather := "气压 1013hPa；湿度 40%；风速 8km/h；能见度 10km；秘密标记 " + sentinel
	messages := []any{
		map[string]any{"role":"user","content":"Call weather_probe."},
		map[string]any{"role":"assistant","content":[]any{map[string]any{"type":"tool_use","id":toolID,"name":"weather_probe","input":map[string]any{"city":"Oslo"}}}},
		map[string]any{"role":"user","content":[]any{
			map[string]any{"type":"tool_result","tool_use_id":toolID,"content":weather},
			map[string]any{"type":"text","text":"If the old tool result was removed, reply only REMOVED. If it is still visible, repeat all weather values and the secret marker."},
		}},
	}
	body := map[string]any{
		"model":model,
		"max_tokens":180,
		"messages":messages,
		"context_management":map[string]any{"edits":[]any{map[string]any{
			"type":"clear_tool_uses_20250919",
			"trigger":map[string]any{"type":"input_tokens","value":1},
			"keep":map[string]any{"type":"tool_uses","value":0},
			"clear_at_least":map[string]any{"type":"input_tokens","value":0},
		}}},
	}
	headers := http.Header{"Anthropic-Beta":[]string{"context-management-2025-06-27"}}
	valid, validErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", headers, body)
	validEv := requestEvidence("Context Management 哨兵清理探针", "应报告 applied_edits，且模型不能访问被清理 tool_result", valid, target)

	invalidBody := anthropicMessageBody(model, "Reply OK", 32)
	invalidBody["context_management"] = map[string]any{"edits":[]any{map[string]any{"type":"__modurelay_invalid_edit_type__"}}}
	invalid, invalidErr := target.doJSON(ctx, detectionProtocolAnthropic, http.MethodPost, "/v1/messages", headers, invalidBody)
	invalidEv := requestEvidence("非法 edit type 负向探针", "HTTP 4xx", invalid, target)

	if validErr != nil || valid.StatusCode < 200 || valid.StatusCode >= 300 {
		return unavailableProbe("anthropic.context_management", "Context Management", "Context", "合法 Context Management 请求未被接受："+classifyHTTPFailure(valid, validErr, target), validEv, invalidEv)
	}
	root := parseJSONMap(valid.Body)
	applied := sliceValue(mapValue(root["context_management"])["applied_edits"])
	text := extractAnthropicText(valid.Body)
	lower := strings.ToLower(text)
	leaked := strings.Contains(text, sentinel) || strings.Contains(lower, "1013hpa") || strings.Contains(lower, "40%") || strings.Contains(lower, "8km/h") || strings.Contains(lower, "10km")
	rejected := invalidErr == nil && invalid.StatusCode >= 400 && invalid.StatusCode < 500
	validEv.Actual = fmt.Sprintf("applied_edits=%d, sentinel_or_weather_visible=%t, output=%s", len(applied), leaked, sanitizeDetectionText(text, target.apiKey, 220))
	if len(applied) > 0 && !leaked && rejected {
		return detectionProbeResult{ID:"anthropic.context_management", Name:"Context Management", Category:"Context", Status:"success", Confidence:1, Summary:"applied_edits、哨兵删除和非法字段拒绝三层证据全部通过", Evidence:[]detectionEvidence{validEv, invalidEv}}
	}
	if leaked && invalidErr == nil && invalid.StatusCode >= 200 && invalid.StatusCode < 300 {
		result := failedProbe("anthropic.context_management", "Context Management", "Context", "PROTOCOL_FIELD_DROPPED", "HTTP 200 但 Context Management 没有真正生效", "本应删除的 tool_result 仍可被模型复述，同时非法 edit type 也返回 2xx；两项独立证据共同指向 context_management 被中转/兼容层丢弃", .999, validEv, invalidEv)
		result.PossibleCauses = []string{"请求结构体未声明 context_management", "Provider Mapper 未转发 context_management", "Anthropic beta header 或字段被过滤"}
		result.Recommendations = []string{"记录脱敏 outbound JSON 与客户端原始请求逐字段 diff", "确认 Anthropic-Beta 头被保留", "增加非法 enum 的网关集成测试"}
		return result
	}
	if leaked {
		return failedProbe("anthropic.context_management", "Context Management", "Context", "BEHAVIOR_MISMATCH", "待删除工具结果仍参与了模型推理", "模型复述了只存在于目标 tool_result 中的唯一哨兵或天气值", .98, validEv, invalidEv)
	}
	if len(applied) == 0 {
		return inconclusiveProbe("anthropic.context_management", "Context Management", "Context", "哨兵未泄漏，但没有观察到 applied_edits，证据不足以确认真正执行了清理", validEv, invalidEv)
	}
	return detectionProbeResult{ID:"anthropic.context_management", Name:"Context Management", Category:"Context", Status:"partial", Confidence:.9, Summary:"清理行为可见，但负向错误语义异常", ReasonCode:"ERROR_SEMANTICS_MISMATCH", Evidence:[]detectionEvidence{validEv, invalidEv}}
}
