package dto

import (
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var userUsageUnitPriceFields = []string{
	"input_cost",
	"output_cost",
	"cache_creation_cost",
	"cache_read_cost",
	"image_input_cost",
	"image_output_cost",
}

// UsageLogFromServiceWithVisibility converts a service usage record and removes
// disabled billing-calculation fields before the payload reaches JSON encoding.
// Admin endpoints continue using UsageLogFromServiceAdmin and remain unaffected.
func UsageLogFromServiceWithVisibility(log *service.UsageLog, visibility service.UsageDetailVisibility) map[string]any {
	return RedactUsageLogForUser(UsageLogFromService(log), visibility)
}

// RedactUsageLogForUser performs response-level redaction rather than UI-only
// hiding. The omitted keys therefore cannot be recovered through DevTools.
func RedactUsageLogForUser(log *UsageLog, visibility service.UsageDetailVisibility) map[string]any {
	if log == nil {
		return nil
	}
	raw, err := json.Marshal(log)
	if err != nil {
		return map[string]any{}
	}
	payload := make(map[string]any)
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}

	if !visibility.ShowUnitPrices {
		for _, field := range userUsageUnitPriceFields {
			delete(payload, field)
		}
	}
	if !visibility.ShowRateMultiplier {
		delete(payload, "rate_multiplier")
		redactNestedGroupRate(payload["group"])
		if apiKey, ok := payload["api_key"].(map[string]any); ok {
			redactNestedGroupRate(apiKey["group"])
		}
	}
	if !visibility.ShowOriginalCost {
		delete(payload, "total_cost")
	}
	return payload
}

func redactNestedGroupRate(value any) {
	if group, ok := value.(map[string]any); ok {
		delete(group, "rate_multiplier")
	}
}
