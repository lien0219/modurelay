package migrations

import (
	"strings"
	"testing"
)

func TestServiceAccountsMigrationsPreserveHistoricalUsage(t *testing.T) {
	for _, name := range []string{"284_service_accounts.sql", "285_service_account_usage_rollups.sql", "286_service_account_indexes_notx.sql", "287_service_account_domain_events.sql", "288_service_account_audit_attribution.sql", "289_service_account_audit_indexes_notx.sql"} {
		b, err := FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		s := strings.ToLower(string(b))
		if strings.Contains(s, "update usage_logs set") || strings.Contains(s, "update api_keys set") {
			t.Fatalf("%s must not backfill historical principal snapshots", name)
		}
	}
	b, _ := FS.ReadFile("284_service_accounts.sql")
	s := strings.ToLower(string(b))
	for _, want := range []string{"create table if not exists service_accounts", "on delete set null", "not valid", "service_account_id bigint", "alter column user_id drop not null", "alter column actor_user_id drop not null"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	b, _ = FS.ReadFile("286_service_account_indexes_notx.sql")
	if !strings.Contains(strings.ToLower(string(b)), "create index concurrently") {
		t.Error("hot indexes must be online")
	}
	b, _ = FS.ReadFile("287_service_account_domain_events.sql")
	events := strings.ToLower(string(b))
	for _, want := range []string{"create or replace function domain_event_payload_is_safe", "service_account_id", "credential_id", "expires_at"} {
		if !strings.Contains(events, want) {
			t.Errorf("domain-event migration missing %q", want)
		}
	}
}

func TestServiceAccountAuditMigrationsKeepHistoricalAttribution(t *testing.T) {
	b, err := FS.ReadFile("288_service_account_audit_attribution.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(b))
	for _, table := range []string{"content_moderation_logs", "prompt_audit_jobs", "prompt_audit_events", "ops_error_logs", "ops_system_logs"} {
		if !strings.Contains(s, "alter table "+table+" add column if not exists service_account_id bigint") {
			t.Errorf("missing nullable snapshot for %s", table)
		}
		if !strings.Contains(s, table+"_machine_actor") {
			t.Errorf("missing exclusive machine attribution for %s", table)
		}
	}
	if strings.Count(s, "references service_accounts(id) not valid") != 5 {
		t.Error("audit FKs must avoid historical scans")
	}
	b, err = FS.ReadFile("289_service_account_audit_indexes_notx.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.ToLower(string(b)), "create index concurrently") != 5 {
		t.Error("audit indexes must build online")
	}
}
