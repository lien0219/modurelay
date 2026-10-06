//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Workspace tests need committed rows to exercise concurrent transactions. The
// integration package runs these tests sequentially; retain the existing fixture
// IDs and remove only rows created by this test, in foreign-key dependency order.
func isolateWorkspaceTestFixtures(t *testing.T) {
	t.Helper()
	var userID, workspaceID, groupID, accountID int64
	err := integrationDB.QueryRow(`SELECT
	 COALESCE((SELECT MAX(id) FROM users),0),COALESCE((SELECT MAX(id) FROM workspaces),0),
	 COALESCE((SELECT MAX(id) FROM groups),0),COALESCE((SELECT MAX(id) FROM accounts),0)`).
		Scan(&userID, &workspaceID, &groupID, &accountID)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, statement := range []struct {
			query string
			args  []any
		}{
			{`DELETE FROM domain_events WHERE workspace_id>$1 OR actor_user_id>$2`, []any{workspaceID, userID}},
			{`DELETE FROM budget_alert_transitions WHERE (scope_type='workspace' AND scope_id>$1) OR (scope_type='project' AND scope_id IN (SELECT id FROM projects WHERE workspace_id>$1))`, []any{workspaceID}},
			{`DELETE FROM usage_logs WHERE user_id>$1 OR workspace_id>$2`, []any{userID, workspaceID}},
			{`DELETE FROM batch_image_jobs WHERE user_id>$1 OR workspace_id>$2`, []any{userID, workspaceID}},
			{`DELETE FROM usage_tenant_hourly_rollups WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM usage_service_account_hourly_rollups WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM budget_reservations WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM budget_counters WHERE workspace_scope_id>$1 OR project_scope_id IN (SELECT id FROM projects WHERE workspace_id>$1)`, []any{workspaceID}},
			{`DELETE FROM api_keys WHERE user_id>$1 OR project_id IN (SELECT id FROM projects WHERE workspace_id>$2)`, []any{userID, workspaceID}},
			{`DELETE FROM service_accounts WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM project_access_grants WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM workspace_team_members WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM workspace_teams WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM workspace_audit_logs WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM workspace_invitations WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM workspace_members WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM projects WHERE workspace_id>$1`, []any{workspaceID}},
			{`DELETE FROM workspaces WHERE id>$1`, []any{workspaceID}},
			{`DELETE FROM user_subscriptions WHERE user_id>$1 OR group_id>$2`, []any{userID, groupID}},
			{`DELETE FROM user_allowed_groups WHERE user_id>$1 OR group_id>$2`, []any{userID, groupID}},
			{`DELETE FROM account_groups WHERE account_id>$1 OR group_id>$2`, []any{accountID, groupID}},
			{`DELETE FROM users WHERE id>$1`, []any{userID}},
			{`DELETE FROM accounts WHERE id>$1`, []any{accountID}},
			{`DELETE FROM groups WHERE id>$1`, []any{groupID}},
		} {
			_, err := integrationDB.ExecContext(context.Background(), statement.query, statement.args...)
			require.NoError(t, err, "clean up committed workspace test fixtures")
		}
	})
}

// Registration now creates a personal workspace in the user transaction. Legacy
// fixtures must remove that owned graph before hard-deleting their test user.
func deletePersonalWorkspaceFixture(t *testing.T, userID int64) {
	t.Helper()
	deletePersonalWorkspaceFixtures(t, `type='personal' AND owner_user_id=$1`, userID)
}

func deleteAllPersonalWorkspaceFixtures(t *testing.T) {
	t.Helper()
	deletePersonalWorkspaceFixtures(t, `type='personal'`)
}

func deletePersonalWorkspaceFixtures(t *testing.T, scope string, args ...any) {
	t.Helper()
	workspaces := `SELECT id FROM workspaces WHERE ` + scope
	for _, query := range []string{
		`DELETE FROM domain_events WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM api_keys WHERE project_id IN (SELECT id FROM projects WHERE workspace_id IN (` + workspaces + `))`,
		`DELETE FROM service_accounts WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM project_access_grants WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM workspace_team_members WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM workspace_teams WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM workspace_audit_logs WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM workspace_invitations WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM workspace_members WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM projects WHERE workspace_id IN (` + workspaces + `)`,
		`DELETE FROM workspaces WHERE ` + scope,
	} {
		_, err := integrationDB.Exec(query, args...)
		require.NoError(t, err, "clean up personal workspace fixture dependencies")
	}
}
