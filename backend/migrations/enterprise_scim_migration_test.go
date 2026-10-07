package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseSCIMMigrationHasTypedTenantSources(t *testing.T) {
	b, e := FS.ReadFile("296_enterprise_identity_scim.sql")
	require.NoError(t, e)
	for _, s := range []string{"workspace_scim_connectors", "workspace_scim_tokens", "workspace_scim_users", "workspace_scim_groups", "workspace_scim_group_members", "workspace_membership_sources", "workspace_team_membership_sources", "administratively_suspended", "provider_id,source_type"} {
		require.Contains(t, string(b), s)
	}
	for _, s := range []string{"UPDATE usage_logs", "UPDATE api_keys", "UPDATE users SET"} {
		require.NotContains(t, string(b), s)
	}
}
