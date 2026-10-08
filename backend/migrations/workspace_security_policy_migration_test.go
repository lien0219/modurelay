package migrations

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestWorkspaceSecurityPolicyMigrationBoundary(t *testing.T) {
	raw, err := FS.ReadFile("297_workspace_security_policy.sql")
	require.NoError(t, err)
	body := strings.ToLower(string(raw))
	for _, required := range []string{"alter table workspace_security_policies", "require_mfa boolean not null default false", "allow_external_members boolean not null default true", "workspace_jit_enabled boolean not null default true", "between 900 and 2592000", "foreign key (workspace_id,provider_id)", "references workspace_identity_providers(workspace_id,id)", "domain_event_payload_is_safe", "'require_mfa'", "'changed_fields'"} {
		require.Contains(t, body, required)
	}
	for _, forbidden := range []string{"update usage_logs", "update api_keys", "update service_accounts", "update budget_reservations", "update workspace_members", "drop table", "workspace_mfa_settings", "workspace_invite_policy"} {
		require.NotContains(t, body, forbidden)
	}
}
