package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"math"
	"strings"
	"testing"
	"time"
)

func TestEnterpriseAdminFilterRejectsBroadInput(t *testing.T) {
	for _, f := range []AdminWorkspaceFilter{{PageSize: 101}, {Page: 102, PageSize: 100}, {NamePrefix: strings.Repeat("界", 101)}, {WorkspaceID: -1}, {Status: "oops"}, {Sort: "name; DROP TABLE users"}, {CreatedFrom: "infinity"}, {CreatedFrom: "2024-01-01T00:00:00Z", CreatedTo: "2026-01-01T00:00:00Z"}, {UpdatedFrom: "2026-01-02T00:00:00Z", UpdatedTo: "2026-01-01T00:00:00Z"}} {
		require.Error(t, f.Validate())
	}
	f := AdminWorkspaceFilter{NamePrefix: `50%_\`, Page: 1, PageSize: 20}
	require.NoError(t, f.Validate())
	require.Equal(t, `50\%\_\\%`, f.EscapedNamePrefix())
}

func TestEnterpriseAdminUnknownEvidenceCannotCertifyHealthOrRotation(t *testing.T) {
	now := time.Now().UTC()
	require.Equal(t, "unknown", AdminEvidenceFreshness(nil, now, 5*time.Minute))
	old := now.Add(-time.Hour)
	future := now.Add(time.Minute)
	require.Equal(t, "stale", AdminEvidenceFreshness(&old, now, 5*time.Minute))
	require.Equal(t, "unknown", AdminEvidenceFreshness(&future, now, 5*time.Minute))
	s := NewWorkspaceService(nil)
	rotation := AdminExportRotation{}
	s.appendAdminRotationCapabilities(&rotation)
	require.Equal(t, "blocked", rotation.State)
	require.False(t, rotation.RotationCertified)
	require.False(t, rotation.Capabilities.KeyAvailable)
	_, err := s.AdminSearchWorkspaces(context.Background(), 1, AdminWorkspaceFilter{PageSize: 101})
	require.ErrorIs(t, err, ErrWorkspaceInvalid)
}

func TestEnterpriseAdminMetricsReplaceExactInventoryWithUnknownForCap(t *testing.T) {
	now := time.Now().UTC()
	exact, overflow := int64(7), int64(10001)
	v := &AdminJobsDiagnostics{Workers: []AdminWorkerDiagnostics{{Worker: "export", ObservedAt: now, Freshness: "fresh", Counts: map[string]AdminCount{"pending": {Value: &exact, Available: true, Freshness: "fresh"}}}}}
	observeAdminWorkers(v)
	v.Workers[0].Counts["pending"] = AdminCount{Value: &overflow, Available: true, Capped: true, Freshness: "fresh"}
	observeAdminWorkers(v)
	v.Workers = append(v.Workers, AdminWorkerDiagnostics{Worker: "private-dynamic-label", ObservedAt: now, Counts: map[string]AdminCount{"private-state": {Value: &exact, Available: true, Freshness: "fresh"}}})
	observeAdminWorkers(v)
	metrics, e := EnterpriseAdminMetricsGatherer().Gather()
	require.NoError(t, e)
	foundBacklog, foundAvailable, foundTime := false, false, false
	for _, family := range metrics {
		for _, m := range family.Metric {
			isExportPending := false
			workerExport, statePending := false, false
			for _, label := range m.Label {
				require.NotContains(t, label.GetValue(), "private")
				if label.GetName() == "worker" && label.GetValue() == "export" {
					workerExport = true
				}
				if label.GetName() == "state" && label.GetValue() == "pending" {
					statePending = true
				}
			}
			isExportPending = workerExport && statePending
			if family.GetName() == "modurelay_enterprise_admin_worker_backlog" && isExportPending {
				foundBacklog = true
				require.True(t, math.IsNaN(m.GetGauge().GetValue()))
			}
			if family.GetName() == "modurelay_enterprise_admin_worker_inventory_available" && isExportPending {
				foundAvailable = true
				require.Zero(t, m.GetGauge().GetValue())
			}
			if family.GetName() == "modurelay_enterprise_admin_worker_observed_timestamp_seconds" && workerExport {
				foundTime = true
				require.Equal(t, float64(now.Unix()), m.GetGauge().GetValue())
			}
		}
	}
	require.True(t, foundBacklog)
	require.True(t, foundAvailable)
	require.True(t, foundTime)
}

func TestEnterpriseAdminMetricsKeepBillingAlertAgeSeparateFromDueLag(t *testing.T) {
	now := time.Now().UTC()
	age := int64(3600)
	v := &AdminJobsDiagnostics{Workers: []AdminWorkerDiagnostics{{Worker: "billing_recovery", ObservedAt: now, Freshness: "fresh", PendingAlertAge: AdminTimeEvidence{AgeSeconds: &age, Available: true, Freshness: "fresh"}}}}
	observeAdminWorkers(v)
	families, e := EnterpriseAdminMetricsGatherer().Gather()
	require.NoError(t, e)
	found := false
	for _, family := range families {
		if family.GetName() == "modurelay_enterprise_admin_billing_pending_alert_age_seconds" {
			found = true
			require.Equal(t, float64(3600), family.Metric[0].GetGauge().GetValue())
		}
		if family.GetName() == "modurelay_enterprise_admin_worker_due_lag_seconds" {
			for _, m := range family.Metric {
				if m.Label[0].GetValue() == "billing_recovery" {
					require.True(t, math.IsNaN(m.GetGauge().GetValue()))
				}
			}
		}
	}
	require.True(t, found)
}
