package migrations

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEnterpriseSAMLHasIndependentProtocolPersistence(t *testing.T) {
	b, err := FS.ReadFile("295_enterprise_identity_saml.sql")
	require.NoError(t, err)
	s := string(b)
	for _, required := range []string{"encrypted_saml_sp_keys", "saml_public_id", "saml_config", "workspace_identity_saml_replays", "request_id", "auth_method", "FOREIGN KEY (workspace_id,provider_id)"} {
		require.Contains(t, s, required)
	}
	for _, prohibited := range []string{"UPDATE usage_logs", "UPDATE api_keys", "UPDATE budget_reservations", "UPDATE users SET", "UPDATE workspace_members SET"} {
		require.NotContains(t, s, prohibited)
	}
}
