package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRedactUsageLogForUserOmitsDisabledBillingFields(t *testing.T) {
	log := &UsageLog{
		ID:                1,
		InputCost:         1.25,
		OutputCost:        2.5,
		CacheCreationCost: 0.5,
		CacheReadCost:     0.25,
		ImageInputCost:    0.75,
		ImageOutputCost:   1.75,
		TotalCost:         7,
		ActualCost:        0.42,
		RateMultiplier:    0.06,
	}

	payload := RedactUsageLogForUser(log, service.UsageDetailVisibility{})
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	encoded := string(raw)

	for _, key := range append(append([]string{}, userUsageUnitPriceFields...), "rate_multiplier", "total_cost") {
		require.NotContains(t, encoded, `"`+key+`"`)
	}
	require.Contains(t, encoded, `"actual_cost":0.42`)
}

func TestRedactUsageLogForUserKeepsEnabledBillingFields(t *testing.T) {
	log := &UsageLog{
		ID:             1,
		InputCost:      1.25,
		OutputCost:     2.5,
		TotalCost:      3.75,
		ActualCost:     0.225,
		RateMultiplier: 0.06,
	}

	payload := RedactUsageLogForUser(log, service.UsageDetailVisibility{
		ShowUnitPrices:     true,
		ShowRateMultiplier: true,
		ShowOriginalCost:   true,
	})
	require.Equal(t, 1.25, payload["input_cost"])
	require.Equal(t, 2.5, payload["output_cost"])
	require.Equal(t, 3.75, payload["total_cost"])
	require.Equal(t, 0.06, payload["rate_multiplier"])
}
