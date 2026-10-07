package migrations

import (
	"strings"
	"testing"
)

func TestEnterpriseIdentityMigrationDefinesScopedOIDCState(t *testing.T) {
	b, err := FS.ReadFile("294_enterprise_identity_oidc.sql")
	if err != nil {
		t.Fatalf("read enterprise identity migration: %v", err)
	}
	s := strings.ToLower(strings.Join(strings.Fields(string(b)), " "))
	for _, want := range []string{
		"create table if not exists workspace_domains",
		"normalized_domain",
		"verification_token_hash",
		"workspace_domains_one_claim",
		"create table if not exists workspace_identity_providers",
		"encrypted_client_secret",
		"claim_mapping jsonb",
		"jit_config jsonb",
		"create table if not exists workspace_user_identities",
		"unique (workspace_id, provider_id, subject)",
		"foreign key (workspace_id, provider_id)",
		"create table if not exists workspace_identity_team_mappings",
		"create table if not exists workspace_identity_role_mappings",
		"role in ('viewer', 'developer', 'admin', 'billing')",
		"create table if not exists workspace_identity_auth_states",
		"state_hash bytea",
		"nonce_hash bytea",
		"pkce_verifier_ciphertext",
		"nonce_ciphertext",
		"consumed_at",
		"create table if not exists workspace_identity_link_requests",
		"create table if not exists workspace_security_policies",
		"require_sso boolean not null default false",
		"membership_source",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("migration missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"update usage_logs set",
		"delete from usage_logs",
		"update api_keys set",
		"update billing",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("enterprise identity migration must not contain %q", forbidden)
		}
	}
}
