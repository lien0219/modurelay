//go:build integration

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLifecycleRetentionPostgresRepositoryAuthorizationAndFloors(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, workspace := workspaceFixture(t)
	repo := NewWorkspaceRepository(integrationDB).(service.WorkspaceLifecycleRetentionRepository)
	policies, err := s.LifecycleRetention(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	require.Len(t, policies, 5)
	want := map[string]int{"operational": 180, "financial": 0, "audit": 0, "security": 365, "temporary": 7}
	for _, policy := range policies {
		require.Equal(t, want[policy.Category], policy.RetentionDays, policy.Category)
		require.Equal(t, want[policy.Category], policy.MinimumDays, policy.Category)
		if policy.Category == "financial" {
			require.True(t, policy.Protected)
		}
	}
	admin := workspaceJoin(t, ctx, s, owner.ID, workspace.ID, "admin")
	_, err = s.LifecycleRetention(ctx, admin.ID, workspace.ID)
	require.NoError(t, err)
	_, err = repo.UpdateLifecycleRetention(ctx, admin.ID, workspace.ID, "operational", 365)
	require.Error(t, err, "read access cannot change retention")
	global := mustCreateUser(t, testEntClient(t), &service.User{Email: "global-retention@example.com", Role: service.RoleAdmin})
	_, err = repo.LifecycleRetention(ctx, global.ID, workspace.ID)
	require.Error(t, err, "Global Admin is not a tenant Owner")
	_, err = repo.UpdateLifecycleRetention(ctx, global.ID, workspace.ID, "operational", 365)
	require.Error(t, err)
	_, _, foreignOwner, foreign := workspaceFixture(t)
	_, err = repo.LifecycleRetention(ctx, owner.ID, foreign.ID)
	require.Error(t, err, "Workspace A's Owner cannot read B's policies")
	_, err = repo.UpdateLifecycleRetention(ctx, foreignOwner.ID, workspace.ID, "operational", 365)
	require.Error(t, err)
	for _, tc := range []struct {
		category string
		days     int
	}{
		{"operational", 179}, {"security", 364}, {"temporary", 6}, {"financial", 365}, {"audit", 365}, {"invalid", 365}, {"temporary", -1}, {"operational", 36501},
	} {
		_, err = s.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, tc.category, tc.days)
		require.ErrorIs(t, err, service.ErrLifecycleRetentionInvalid)
	}
	policies, err = s.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "operational", 365)
	require.NoError(t, err)
	for _, policy := range policies {
		if policy.Category == "operational" {
			require.Equal(t, 365, policy.RetentionDays)
		}
	}
	var audit, events, outbox int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='retention_updated'`, workspace.ID).Scan(&audit))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*),count(o.event_id) FROM domain_events e LEFT JOIN domain_event_outbox o ON o.event_id=e.id WHERE e.workspace_id=$1 AND e.event_type='workspace.retention.updated' AND e.payload->'data'->>'category'='operational' AND e.payload->'data'->>'retention_days'='365'`, workspace.ID).Scan(&events, &outbox))
	require.Equal(t, 1, audit)
	require.Equal(t, 1, events)
	require.Equal(t, 1, outbox)
	_, err = integrationDB.ExecContext(ctx, `UPDATE platform_retention_policies SET minimum_days=730,default_days=730 WHERE category='operational'`)
	require.NoError(t, err)
	policies, err = repo.LifecycleRetention(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	for _, policy := range policies {
		if policy.Category == "operational" {
			require.Equal(t, 730, policy.RetentionDays, "a raised floor constrains a previously stored policy")
		}
	}
	_, err = s.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "operational", 365)
	require.ErrorIs(t, err, service.ErrLifecycleRetentionInvalid)
	_, err = s.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "operational", 0)
	require.NoError(t, err, "indefinite tenant retention can always strengthen a finite floor")
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, owner.ID)
	require.NoError(t, err)
	_, err = repo.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "temporary", 30)
	require.Error(t, err, "repository rechecks current membership")
}

func TestLifecycleRetentionPostgresPolicyAuditAndOutboxRollBackTogether(t *testing.T) {
	for _, table := range []string{"workspace_audit_logs", "domain_events"} {
		t.Run(table, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, s, owner, workspace := workspaceFixture(t)
			_, err := s.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "operational", 365)
			require.NoError(t, err)
			// Failure injection belongs only to this disposable database. The
			// financial immutability triggers stay enabled throughout the test.
			_, err = integrationDB.ExecContext(ctx, `CREATE FUNCTION fail_retention_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected retention write failure'; END $$; CREATE TRIGGER fail_retention_write BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION fail_retention_write()`)
			require.NoError(t, err)
			_, err = s.UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "operational", 730)
			require.Error(t, err)
			var days, audit, events, outbox int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT retention_days FROM workspace_retention_policies WHERE workspace_id=$1 AND category='operational'`, workspace.ID).Scan(&days))
			require.Equal(t, 365, days, "failed evidence writes roll back the policy")
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='retention_updated'`, workspace.ID).Scan(&audit))
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*),count(o.event_id) FROM domain_events e LEFT JOIN domain_event_outbox o ON o.event_id=e.id WHERE e.workspace_id=$1 AND e.event_type='workspace.retention.updated'`, workspace.ID).Scan(&events, &outbox))
			require.Equal(t, 1, audit)
			require.Equal(t, 1, events)
			require.Equal(t, 1, outbox)
		})
	}
}

func TestLifecycleRetentionPostgresArchivedPoliciesReadableAndNotMutable(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, workspace := workspaceFixture(t)
	err := s.ArchiveWorkspace(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	_, err = s.LifecycleRetention(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	_, err = NewWorkspaceRepository(integrationDB).(service.WorkspaceLifecycleRetentionRepository).UpdateLifecycleRetention(ctx, owner.ID, workspace.ID, "temporary", 30)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
}
