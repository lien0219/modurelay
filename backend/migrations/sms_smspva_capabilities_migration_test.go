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
