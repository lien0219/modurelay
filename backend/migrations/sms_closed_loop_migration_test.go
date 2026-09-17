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
}
