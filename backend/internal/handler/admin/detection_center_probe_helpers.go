package admin

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractAnthropicText(body []byte) string {
	root := parseJSONMap(body)
	if root == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	for _, item := range sliceValue(root["content"]) {
		block := mapValue(item)
		if stringValue(block["type"]) == "text" {
			if text := stringValue(block["text"]); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func extractOpenAIText(body []byte) string {
	root := parseJSONMap(body)
	if root == nil {
		return ""
	}
	if text := stringValue(root["output_text"]); text != "" {
		return text
	}
	choices := sliceValue(root["choices"])
	if len(choices) > 0 {
		message := mapValue(mapValue(choices[0])["message"])
		if content := message["content"]; content != nil {
			switch value := content.(type) {
			case string:
				return strings.TrimSpace(value)
			case []any:
				var parts []string
				for _, item := range value {
					obj := mapValue(item)
					if text := stringValue(obj["text"]); text != "" {
						parts = append(parts, text)
					}
				}
				return strings.Join(parts, "\n")
			}
		}
	}
	for _, item := range sliceValue(root["output"]) {
		obj := mapValue(item)
		for _, content := range sliceValue(obj["content"]) {
			block := mapValue(content)
			if text := stringValue(block["text"]); text != "" {
				return text
			}
		}
	}
	return ""
}

func extractGeminiText(body []byte) string {
	root := parseJSONMap(body)
	if root == nil {
		return ""
	}
	for _, candidate := range sliceValue(root["candidates"]) {
		content := mapValue(mapValue(candidate)["content"])
		for _, part := range sliceValue(content["parts"]) {
			if text := stringValue(mapValue(part)["text"]); text != "" {
				return text
			}
		}
	}
	return ""
}

func validateDetectionSchemaPayload(text, expectedProbe string, expectedCount int64) (bool, string) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &obj); err != nil {
		return false, "输出不是合法 JSON：" + err.Error()
	}
	if len(obj) != 2 {
		return false, fmt.Sprintf("JSON 字段数量为 %d，期望严格等于 2", len(obj))
	}
	if stringValue(obj["probe"]) != expectedProbe {
		return false, fmt.Sprintf("probe=%q，期望 %q", stringValue(obj["probe"]), expectedProbe)
	}
	if numberValue(obj["count"]) != expectedCount {
		return false, fmt.Sprintf("count=%d，期望 %d", numberValue(obj["count"]), expectedCount)
	}
	return true, "JSON Schema 约束满足"
}

func anthropicUsage(body []byte) map[string]any {
	root := parseJSONMap(body)
	if root == nil {
		return nil
	}
	return mapValue(root["usage"])
}

func cacheUsage(body []byte) (creation, read int64) {
	usage := anthropicUsage(body)
	if usage == nil {
		return 0, 0
	}
	return numberValue(usage["cache_creation_input_tokens"]), numberValue(usage["cache_read_input_tokens"])
}

func failedProbe(id, name, category, reasonCode, summary, failure string, confidence float64, evidence ...detectionEvidence) detectionProbeResult {
	return detectionProbeResult{
		ID:            id,
		Name:          name,
		Category:      category,
		Status:        "failed",
		Confidence:    confidence,
		Summary:       summary,
		FailureReason: failure,
		ReasonCode:    reasonCode,
		Evidence:      evidence,
	}
}

func unavailableProbe(id, name, category, summary string, evidence ...detectionEvidence) detectionProbeResult {
	return detectionProbeResult{ID: id, Name: name, Category: category, Status: "unavailable", Confidence: 0.9, Summary: summary, ReasonCode: "FEATURE_UNAVAILABLE", Evidence: evidence}
}

func inconclusiveProbe(id, name, category, summary string, evidence ...detectionEvidence) detectionProbeResult {
	return detectionProbeResult{ID: id, Name: name, Category: category, Status: "inconclusive", Confidence: 0.55, Summary: summary, ReasonCode: "INCONCLUSIVE", Evidence: evidence}
}

func notApplicableProbe(id, name, category, summary string) detectionProbeResult {
	return detectionProbeResult{ID: id, Name: name, Category: category, Status: "not_applicable", Confidence: 1, Summary: summary}
}

func requestEvidence(label, expected string, result detectionHTTPResult, target *detectionTarget) detectionEvidence {
	return detectionEvidence{
		Kind:            "http_probe",
		Label:           label,
		Expected:        expected,
		HTTPStatus:      result.StatusCode,
		DurationMS:      result.Duration.Milliseconds(),
		ResponseExcerpt: detectionExcerpt(result.Body, target.apiKey, 900),
	}
}

func classifyHTTPFailure(result detectionHTTPResult, err error, target *detectionTarget) string {
	if err != nil {
		return sanitizeDetectionText(err.Error(), target.apiKey, 360)
	}
	return fmt.Sprintf("HTTP %d: %s", result.StatusCode, detectionExcerpt(result.Body, target.apiKey, 360))
}
