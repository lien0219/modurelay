package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnableUpstreamBillingProbeMigrationPreservesExplicitOptOut(t *testing.T) {
	content, err := FS.ReadFile("236_enable_upstream_billing_probe.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "jsonb_set(")
	require.Contains(t, sql, "'{upstream_billing_probe_enabled}'")
	require.Contains(t, sql, "'true'::jsonb")
	require.Contains(t, sql, "type = 'apikey'")
	require.Contains(t, sql, "platform IN ( 'openai', 'anthropic', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek' )")
	require.Contains(t, sql, "type = 'upstream' AND platform = 'antigravity'")
	require.Contains(t, sql, "NOT (COALESCE(extra, '{}'::jsonb) ? 'upstream_billing_probe_enabled')")
}
