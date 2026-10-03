package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedancePlatformMigrationPreservesPlatformChecksAndLegacyRows(t *testing.T) {
	content, err := FS.ReadFile("271_seedance_platform.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS composite_model_routes_target_platform_check")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS composite_model_routes_endpoint_check")
	require.Contains(t, sql,
		"CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'seedance'))")
	require.Contains(t, sql,
		"CHECK (target_platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'seedance'))")
	require.Contains(t, sql,
		"CHECK (endpoint IN ('any', 'messages', 'count_tokens', 'responses', 'chat_completions', 'embeddings', 'images', 'videos', 'gemini'))")
	require.NotContains(t, strings.ToUpper(sql), "UPDATE ACCOUNTS")
	require.NotContains(t, strings.ToUpper(sql), "UPDATE COMPOSITE_MODEL_ROUTES")
	require.NotContains(t, sql, "channel_monitors_provider_check")
	require.NotContains(t, sql, "channel_monitor_request_templates_provider_check")
}
