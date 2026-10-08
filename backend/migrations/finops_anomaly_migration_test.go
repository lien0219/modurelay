package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinOpsAnomalyMigrationBoundary(t *testing.T) {
	raw, err := FS.ReadFile("298_finops_anomalies.sql")
	require.NoError(t, err)
	body := strings.ToLower(string(raw))
	for _, required := range []string{
		"create table if not exists finops_anomaly_detection_leases",
		"lease_token",
		"create table if not exists finops_anomaly_snapshots",
		"create table if not exists finops_anomaly_findings",
		"create table if not exists finops_anomaly_detector_status",
		"finops_anomaly_tenant_rollups_bucket_workspace",
		"finops_anomaly_service_rollups_bucket_workspace",
		"finops_anomaly_snapshot_immutable_guard",
		"finops_anomaly_finding_transition_guard",
		"'infinity'::numeric",
		"unique",
		"domain_event_payload_is_safe",
		"anomaly_id",
	} {
		require.Contains(t, body, required)
	}
	for _, forbidden := range []string{"update usage_logs", "delete from usage_logs", "insert into usage_logs", "cost_center", "llm"} {
		require.NotContains(t, body, forbidden)
	}
}
