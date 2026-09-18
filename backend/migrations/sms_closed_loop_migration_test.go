package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSMSClosedLoopMigrationsAreEmbeddedAndAppendOnly(t *testing.T) {
	for _, name := range []string{
		"243_sms_quotes_and_message_lifecycle.sql",
		"244_sms_quote_consumption_and_settlement.sql",
		"245_sms_reconciliation_actions.sql",
		"246_sms_pricing_and_catalog.sql",
		"247_sms_provider_mapping_seeds.sql",
	} {
		content, err := FS.ReadFile(name)
		require.NoError(t, err, name)
		sql := strings.Join(strings.Fields(string(content)), " ")
		require.NotEmpty(t, sql)
	}

	content, err := FS.ReadFile("245_sms_reconciliation_actions.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reconciliation_action")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reconciliation_attempts")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reconcile_after")
	require.Contains(t, sql, "idx_sms_orders_reconcile_after")

	pricing, err := FS.ReadFile("246_sms_pricing_and_catalog.sql")
	require.NoError(t, err)
	pricingSQL := strings.Join(strings.Fields(string(pricing)), " ")
	require.Contains(t, pricingSQL, "sms_pricing_settings")
	require.Contains(t, pricingSQL, "service_catalog JSONB")
	require.Contains(t, pricingSQL, "provider_refund_status")

	seed, err := FS.ReadFile("247_sms_provider_mapping_seeds.sql")
	require.NoError(t, err)
	seedSQL := strings.Join(strings.Fields(string(seed)), " ")
	require.Contains(t, seedSQL, "provider_country_id")
	require.Contains(t, seedSQL, "'usa'")
}
