package migrations

import (
	"strings"
	"testing"
)

func TestWorkspaceWebhookMigrationContainsTenantAndDeliveryGuards(t *testing.T) {
	sqlBytes, err := FS.ReadFile("281_workspace_webhooks.sql")
	if err != nil {
		t.Fatalf("read webhook migration: %v", err)
	}
	sql := strings.ToLower(string(sqlBytes))
	for _, required := range []string{
		"create table if not exists workspace_webhooks",
		"create table if not exists workspace_webhook_subscriptions",
		"create table if not exists workspace_webhook_deliveries",
		"unique(webhook_id,event_id)",
		"for update",
		"workspace_webhook_limit_update_guard",
		"workspace_webhook_delivery_immutable_guard",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration missing %q", required)
		}
	}
}
