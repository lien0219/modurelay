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
		"253_sms_platform_30d_delivery_stats.sql",
		"254_sms_delivery_outcome.sql",
		"255_sms_batch_purchase_limit.sql",
		"257_sms_smspva_capabilities.sql",
		"258_sms_smspva_rental_advanced.sql",
		"259_sms_rental_service_charges.sql",
		"260_sms_rental_multi_service_quotes.sql",
		"261_sms_rental_service_recovery.sql",
		"262_sms_smspva_advanced_capabilities.sql",
		"263_sms_message_dedup_identity.sql",
		"264_sms_rental_legacy_constraints.sql",
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

	deliveryStats, err := FS.ReadFile("253_sms_platform_30d_delivery_stats.sql")
	require.NoError(t, err)
	deliveryStatsSQL := strings.Join(strings.Fields(string(deliveryStats)), " ")
	require.Contains(t, deliveryStatsSQL, "first_sms_received_at")
	require.Contains(t, deliveryStatsSQL, "idx_sms_orders_delivery_stats_30d")

	deliveryOutcome, err := FS.ReadFile("254_sms_delivery_outcome.sql")
	require.NoError(t, err)
	deliveryOutcomeSQL := strings.Join(strings.Fields(string(deliveryOutcome)), " ")
	require.Contains(t, deliveryOutcomeSQL, "delivery_outcome")
	require.Contains(t, deliveryOutcomeSQL, "delivery_finalized_at")
	require.Contains(t, deliveryOutcomeSQL, "idx_sms_orders_delivery_outcome_30d")

	batchLimit, err := FS.ReadFile("255_sms_batch_purchase_limit.sql")
	require.NoError(t, err)
	batchLimitSQL := strings.Join(strings.Fields(string(batchLimit)), " ")
	require.Contains(t, batchLimitSQL, "batch_purchase_limit")
	require.Contains(t, batchLimitSQL, "'5'::jsonb")
}
