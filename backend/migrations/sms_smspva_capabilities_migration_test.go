package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSMSPVACapabilitiesMigrationIsAppendOnlyAndFailClosed(t *testing.T) {
	content, err := FS.ReadFile("257_sms_smspva_capabilities.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ON CONFLICT (code) DO UPDATE")
	require.Contains(t, sql, `"supports_rental":true`)
	require.Contains(t, sql, `"supports_refund":false`)
	require.Contains(t, sql, `"supports_refund_status":false`)
	require.Contains(t, sql, `"supports_webhook":false`)
	require.Contains(t, sql, "ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS sender")
	require.NotContains(t, strings.ToLower(sql), "drop table")
	require.NotContains(t, strings.ToLower(sql), "delete from sms_orders")
}

func TestSMSPVAAdvancedRentalMigrationsAreEmbeddedAndAppendOnly(t *testing.T) {
	for _, name := range []string{
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
		sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))
		require.NotEmpty(t, sql, name)
		require.NotContains(t, sql, "drop table", name)
		require.NotContains(t, sql, "truncate ", name)
		require.NotContains(t, sql, "delete from sms_orders", name)
	}
}

func TestSMSPVARentalAdvancedBackfillIsBoundedAndPreservesTerminalStatus(t *testing.T) {
	content, err := FS.ReadFile("258_sms_smspva_rental_advanced.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS sms_order_services")
	require.Contains(t, sql, "DISTINCT ON (consumed_order_id)")
	require.Contains(t, sql, "LEFT JOIN latest_consumed_quotes")
	require.Contains(t, sql, "idx_sms_quotes_consumed_order_latest")
	require.Contains(t, sql, "o.provider_cost_snapshot, o.sale_price_snapshot, o.status")
	require.NotContains(t, strings.ToLower(sql), "join lateral")
	require.NotContains(t, sql, "THEN 'active'")
	require.Contains(t, sql, "ON CONFLICT (order_id, service_id) DO NOTHING")
}

func TestSMSPVARentalFinancialLedgersAreConstrainedAndRetained(t *testing.T) {
	chargesContent, err := FS.ReadFile("259_sms_rental_service_charges.sql")
	require.NoError(t, err)
	charges := strings.Join(strings.Fields(string(chargesContent)), " ")
	require.Contains(t, charges, "sms_rental_service_charges_amounts_nonnegative")
	require.Contains(t, charges, "captured_amount + released_amount <= reserved_amount")
	require.Contains(t, charges, "sms_rental_service_charges_settlement_status_check")
	require.Contains(t, charges, "ADD COLUMN IF NOT EXISTS currency_snapshot")
	require.Contains(t, charges, "DO $$")
	require.Contains(t, charges, "sms_rental_service_charges_user_fk_restrict")
	require.Contains(t, charges, "sms_rental_service_charges_order_fk_restrict")
	require.Contains(t, charges, "users(id) ON DELETE RESTRICT")
	require.Contains(t, charges, "sms_orders(id) ON DELETE RESTRICT")
	require.NotContains(t, charges, "ON DELETE CASCADE")

	recoveryContent, err := FS.ReadFile("261_sms_rental_service_recovery.sql")
	require.NoError(t, err)
	recovery := strings.Join(strings.Fields(string(recoveryContent)), " ")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS sms_rental_recovery_operations",
		"recovery_baseline JSONB",
		"service_codes_snapshot JSONB",
		"provider_history_order_id",
		"provider_parent_order_id",
		"provider_result_order_id",
		"restore_baseline_ids JSONB",
		"service_code TEXT",
		"country_code TEXT",
		"phone_number TEXT",
		"started_at TIMESTAMPTZ",
		"status TEXT",
		"settlement_status TEXT",
		"captured_amount + released_amount <= reserved_amount",
		"idx_sms_rental_recovery_restore_once",
	} {
		require.Contains(t, recovery, fragment)
	}
	require.NotContains(t, recovery, "ON DELETE CASCADE")
}

func TestSMSPVAMultiServiceQuoteSnapshotIsPreparedButCapabilitiesStayClosed(t *testing.T) {
	multiContent, err := FS.ReadFile("260_sms_rental_multi_service_quotes.sql")
	require.NoError(t, err)
	multi := strings.Join(strings.Fields(string(multiContent)), " ")
	require.Contains(t, multi, "CREATE TABLE IF NOT EXISTS sms_rental_multi_service_quotes")
	require.Contains(t, multi, "service_codes_snapshot JSONB")
	require.Contains(t, multi, "jsonb_array_length(service_codes_snapshot) BETWEEN 2 AND 32")
	require.Contains(t, multi, "ADD COLUMN IF NOT EXISTS consumed_order_id")

	capabilityContent, err := FS.ReadFile("262_sms_smspva_advanced_capabilities.sql")
	require.NoError(t, err)
	capabilities := strings.Join(strings.Fields(string(capabilityContent)), " ")
	for _, fragment := range []string{
		`"supports_rental_constraints": true`,
		`"supports_rental_restore": false`,
		`"supports_rental_multi_service": false`,
		`"supports_rental_add_service": false`,
		`"supports_refund": false`,
		`"supports_refund_status": false`,
		`"supports_webhook": false`,
	} {
		require.Contains(t, capabilities, fragment)
	}
	require.Contains(t, capabilities, "COALESCE(capabilities, '{}'::jsonb) ||")
	require.NotContains(t, strings.ToLower(capabilities), "enabled = true")
	require.NotContains(t, strings.ToLower(capabilities), "update sms_channels")
}

func TestSMSMessageDedupIdentityMigrationIsProviderAware(t *testing.T) {
	content, err := FS.ReadFile("263_sms_message_dedup_identity.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS dedupe_hash")
	require.Contains(t, sql, "ROW_NUMBER() OVER")
	require.Contains(t, sql, "PARTITION BY order_id, message_text, sender, message_type, service_code, other_sms, provider_received_at")
	require.Contains(t, sql, "provider_received_at")
	require.Contains(t, sql, "legacy.sender")
	require.Contains(t, sql, "legacy.message_type")
	require.Contains(t, sql, "service_code")
	// The first legacy row must hash exactly like persistSMSMessage: concat_ws
	// skips the absent provider timestamp and duplicate suffix. An empty string
	// here would add an extra delimiter and let the first replay through.
	require.Contains(t, sql, "CASE WHEN legacy.duplicate_rank = 1 THEN NULL ELSE legacy.id::text END")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_messages_order_dedupe_hash")
	require.NotContains(t, strings.ToLower(sql), "drop table")
}

func TestSMSRentalLegacyConstraintRepairIsGuardedAndIdempotent(t *testing.T) {
	content, err := FS.ReadFile("264_sms_rental_legacy_constraints.sql")
	require.NoError(t, err)
	sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))
	require.Contains(t, sql, "information_schema.columns")
	require.Contains(t, sql, "to_regclass(format('public.%i', fk.local_table))")
	require.Contains(t, sql, "to_regclass(format('public.%i', fk.ref_table))")
	require.Contains(t, sql, "c.conrelid")
	require.Contains(t, sql, "c.confrelid")
	require.Contains(t, sql, "c.conkey")
	require.Contains(t, sql, "c.confkey")
	require.Contains(t, sql, "c.confdeltype")
	require.Contains(t, sql, "a.attname::text")
	require.Contains(t, sql, "not valid")
	require.Contains(t, sql, "pg_index")
	require.Contains(t, sql, "i.indisunique")
	require.Contains(t, sql, "pg_get_expr(i.indpred, i.indrelid)")
	require.Contains(t, sql, "create unique index")
	for _, table := range []string{
		"sms_order_services",
		"sms_rental_service_quotes",
		"sms_rental_restore_quotes",
		"sms_rental_service_charges",
		"sms_rental_multi_service_quotes",
		"sms_rental_recovery_operations",
	} {
		require.Contains(t, sql, table)
	}
	require.NotContains(t, sql, "drop table")
	require.NotContains(t, sql, "delete from")
}
