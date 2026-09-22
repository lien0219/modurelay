package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailPublicPrivateChannelMigration(t *testing.T) {
	content, err := FS.ReadFile("266_email_public_private_channels.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, fragment := range []string{
		"'temp_tf'",
		"'https://temp.tf/api'",
		"'sonjj'",
		"'https://app.sonjj.com'",
		"'email_channel_1'",
		"'email_channel_2'",
		"'email_channel_3'",
		"\"force_free\":true",
		"'email_free_daily_limit'",
		"'email_free_active_limit'",
		"'email_free_generation_interval_seconds'",
		"sale_price=0",
		"visible=FALSE",
	} {
		require.Contains(t, sql, fragment)
	}
	require.NotContains(t, strings.ToLower(sql), "delete from email_orders")
	require.NotContains(t, strings.ToLower(sql), "drop table")
}
