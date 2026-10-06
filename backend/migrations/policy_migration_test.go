package migrations

import (
	"strings"
	"testing"
)

func TestPolicyFoundationMigrationDefinesTenantScopedTriStatePolicies(t *testing.T) {
	b, err := FS.ReadFile("290_policy_foundation.sql")
	if err != nil {
		t.Fatalf("read policy migration: %v", err)
	}
	s := strings.ToLower(string(b))
	for _, table := range []string{"workspace_policies", "project_policies", "service_account_policies"} {
		if !strings.Contains(s, "create table if not exists "+table) {
			t.Errorf("missing policy table %s", table)
		}
	}
	for _, field := range []string{"allowed_models text[]", "allowed_platforms text[]", "rpm_limit bigint", "daily_request_limit bigint", "monthly_request_limit bigint", "daily_token_limit bigint", "monthly_token_limit bigint", "revision bigint"} {
		if !strings.Contains(s, field) {
			t.Errorf("missing policy field %s", field)
		}
	}
	for _, required := range []string{
		"workspace_id bigint primary key",
		"project_id bigint primary key",
		"service_account_id bigint primary key",
		"references workspaces(id)",
		"references projects(id)",
		"references service_accounts(id",
		"check (revision > 0)",
		"cardinality(allowed_models)",
		"cardinality(allowed_platforms)",
		"check (rpm_limit is null or rpm_limit > 0)",
	} {
		if !strings.Contains(s, required) {
			t.Errorf("migration missing %q", required)
		}
	}
}

func TestPolicyFoundationMigrationDoesNotRewriteHistoricalUsage(t *testing.T) {
	b, err := FS.ReadFile("290_policy_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(b))
	for _, forbidden := range []string{"update usage_logs set", "update api_keys set", "delete from usage_logs"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("policy migration must not rewrite history: %q", forbidden)
		}
	}
}

func TestPolicyQuotaAlertsMigrationDefinesDurableThresholdState(t *testing.T) {
	b, err := FS.ReadFile("292_policy_quota_alerts.sql")
	if err != nil {
		t.Fatalf("read policy quota alert migration: %v", err)
	}
	s := strings.ToLower(string(b))
	for _, required := range []string{
		"create table if not exists policy_quota_alerts",
		"policy_revision bigint not null",
		"quota_type text not null",
		"threshold integer not null check (threshold in (80, 100))",
		"primary key (scope_type, scope_id, policy_revision, period_type, period_start, quota_type, threshold)",
		"create or replace function domain_event_payload_is_safe",
		"'period_end'",
		"'quota_type'",
		"'used'",
		"'limit'",
		"'service_account_id'",
		"'credential_id'",
		"'credential_name'",
		"'old_credential_id'",
		"'new_credential_id'",
		"'expires_at'",
	} {
		if !strings.Contains(s, required) {
			t.Errorf("migration missing %q", required)
		}
	}
}
