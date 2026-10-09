//go:build integration

package repository

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var workspaceFixtureSequence uint64
var workspaceFixtureTests sync.Map
var disposableWorkspaceDBName = regexp.MustCompile(`^modurelay_fixture_[0-9]+$`)

// Workspace tests need committed rows to exercise concurrent transactions. Each
// sequential test gets a pristine disposable database. Teardown never deletes
// protected evidence or disables triggers. Nested helper calls are idempotent;
// nested subtests restore the parent's database before the parent resumes.
func isolateWorkspaceTestFixtures(t *testing.T) {
	t.Helper()
	if _, ok := workspaceFixtureTests.Load(t); ok {
		return
	}
	name := fmt.Sprintf("modurelay_fixture_%d", atomic.AddUint64(&workspaceFixtureSequence, 1))
	require.True(t, disposableWorkspaceDBName.MatchString(name), "bounded disposable database name")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := integrationAdminDB.ExecContext(ctx, "CREATE DATABASE "+pq.QuoteIdentifier(name)+" WITH TEMPLATE "+pq.QuoteIdentifier(workspaceFixtureTemplate))
	require.NoError(t, err)
	previousDB, previousClient := integrationDB, integrationEntClient
	workspaceFixtureTests.Store(t, name)
	var client *dbent.Client
	t.Cleanup(func() {
		if client != nil {
			_ = client.Close()
		}
		integrationDB, integrationEntClient = previousDB, previousClient
		workspaceFixtureTests.Delete(t)
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		require.True(t, disposableWorkspaceDBName.MatchString(name), "DROP targets only this disposable database")
		_, err := integrationAdminDB.ExecContext(dropCtx, "DROP DATABASE "+pq.QuoteIdentifier(name)+" WITH (FORCE)")
		require.NoError(t, err, "drop disposable fixture database")
	})
	fixtureURL, err := url.Parse(integrationDSN)
	require.NoError(t, err)
	fixtureURL.Path = "/" + name
	db, err := openSQLWithRetry(ctx, fixtureURL.String(), 15*time.Second)
	require.NoError(t, err)
	client = dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	integrationDB, integrationEntClient = db, client
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
