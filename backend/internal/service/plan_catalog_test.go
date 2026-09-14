package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanCatalogInputUnmarshalAcceptsNumericAndStringPrices(t *testing.T) {
	for _, raw := range []string{
		`{"name":"Starter","price":10,"original_price":11}`,
		`{"name":"Starter","price":"10","original_price":"11"}`,
	} {
		var input PlanCatalogInput
		require.NoError(t, json.Unmarshal([]byte(raw), &input))
		require.Equal(t, "10", input.Price)
		require.NotNil(t, input.OriginalPrice)
		require.Equal(t, "11", *input.OriginalPrice)
	}
}
