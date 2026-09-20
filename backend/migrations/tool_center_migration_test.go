package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolCenterMigrationSeedsEnabledFlagIdempotently(t *testing.T) {
	content, err := FS.ReadFile("256_tool_center.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "INSERT INTO settings (key, value)")
	require.Contains(t, sql, "VALUES ('tool_center_enabled', 'true')")
	require.Contains(t, sql, "ON CONFLICT (key) DO NOTHING")
}
