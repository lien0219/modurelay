//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceBoundedBootstrapAndCorporateCreatorDeletion(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, w := workspaceFixture(t)
	client := testEntClient(t)
	creator := workspaceJoin(t, ctx, s, owner.ID, w.ID, "developer")
	p, e := s.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Keys", Slug: "keys"})
	require.NoError(t, e)
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: creator.ID, Key: fmt.Sprintf("corporate-%d", creator.ID), ProjectID: &p.ID})
	_, e = integrationDB.Exec(`INSERT INTO api_keys(user_id,key,name,status) SELECT $1::bigint,'legacy-batch-'||($1::bigint)::text||'-'||i,'Legacy','active' FROM generate_series(1,105) i`, creator.ID)
	require.NoError(t, e)
	_, e = s.EnsurePersonalWorkspace(ctx, creator.ID)
	require.NoError(t, e)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM api_keys WHERE user_id=$1 AND project_id IS NULL`, creator.ID).Scan(&count))
	require.Equal(t, 5, count)
	_, e = s.EnsurePersonalWorkspace(ctx, creator.ID)
	require.NoError(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM api_keys WHERE user_id=$1 AND project_id IS NULL`, creator.ID).Scan(&count))
	require.Zero(t, count)
	var assigned int64
	require.NoError(t, integrationDB.QueryRow(`SELECT project_id FROM api_keys WHERE id=$1`, key.ID).Scan(&assigned))
	require.Equal(t, p.ID, assigned)
	users := NewUserRepository(client, integrationDB)
	guard := users.(service.WorkspaceAdminLifecycleGuard)
	require.ErrorIs(t, guard.GuardWorkspaceUserDeletion(ctx, creator.ID), service.ErrWorkspaceConflict)
	// A developer who has left still cannot have organization keys erased by the
	// legacy creator-wide deletion path. Earlier key changes roll back as well.
	require.NoError(t, s.RemoveMember(ctx, owner.ID, w.ID, creator.ID))
	tx, e := client.Tx(ctx)
	require.NoError(t, e)
	defer tx.Rollback()
	txctx := dbent.NewTxContext(ctx, tx)
	require.NoError(t, NewAPIKeyRepository(client, integrationDB).DeleteWithAudit(txctx, key.ID))
	require.ErrorIs(t, users.Delete(txctx, creator.ID), service.ErrWorkspaceConflict)
	require.NoError(t, tx.Rollback())
	var live bool
	require.NoError(t, integrationDB.QueryRow(`SELECT deleted_at IS NULL FROM api_keys WHERE id=$1`, key.ID).Scan(&live))
	require.True(t, live)
}

func workspaceFixture(t *testing.T) (context.Context, *service.WorkspaceService, *service.User, *service.Workspace) {
	t.Helper()
	ctx := context.Background()
	u := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("ws-owner-%d@example.com", time.Now().UnixNano())})
	s := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	w, e := s.CreateOrganization(ctx, u.ID, "Safety", fmt.Sprintf("safety-%d", u.ID))
	require.NoError(t, e)
	return ctx, s, u, w
}
func workspaceJoin(t *testing.T, ctx context.Context, s *service.WorkspaceService, owner, w int64, role string) *service.User {
	t.Helper()
	u := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("ws-member-%d@example.com", time.Now().UnixNano())})
	_, token, e := s.CreateInvitation(ctx, owner, w, u.Email, role, time.Hour)
	require.NoError(t, e)
	_, e = s.AcceptInvitation(ctx, u.ID, token)
	require.NoError(t, e)
	return u
}
func TestWorkspaceInvitationSafety(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, w := workspaceFixture(t)
	u := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("ws-expiry-%d@example.com", time.Now().UnixNano())})
	inv, token, e := s.CreateInvitation(ctx, owner.ID, w.ID, u.Email, "developer", time.Hour)
	require.NoError(t, e)
	var stored []byte
	require.NoError(t, integrationDB.QueryRow(`SELECT token_hash FROM workspace_invitations WHERE workspace_id=$1 AND id=$2`, w.ID, inv.ID).Scan(&stored))
	hash := sha256.Sum256([]byte(token))
	require.Equal(t, hash[:], stored)
	require.NoError(t, s.RevokeInvitation(ctx, owner.ID, w.ID, inv.ID))
	_, e = s.AcceptInvitation(ctx, u.ID, token)
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	inv, token, e = s.CreateInvitation(ctx, owner.ID, w.ID, u.Email, "developer", time.Hour)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_invitations SET created_at=now()-interval '2 days',expires_at=now()-interval '1 day' WHERE workspace_id=$1 AND id=$2`, w.ID, inv.ID)
	require.NoError(t, e)
	_, e = s.AcceptInvitation(ctx, u.ID, token)
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	_, fresh, e := s.CreateInvitation(ctx, owner.ID, w.ID, u.Email, "viewer", time.Hour)
	require.NoError(t, e)
	_, e = s.AcceptInvitation(ctx, u.ID, fresh)
	require.NoError(t, e)
	_, _, e = s.CreateInvitation(ctx, owner.ID, w.ID, u.Email, "viewer", time.Hour)
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	_, e = s.AcceptInvitation(ctx, u.ID, "invalid-token")
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
}

func TestWorkspaceAllRolesAndAdmin(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, w := workspaceFixture(t)
	repo := NewWorkspaceRepository(integrationDB)
	access := service.NewWorkspaceAccessService(repo)
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		u := workspaceJoin(t, ctx, s, owner.ID, w.ID, role)
		for _, permission := range service.WorkspacePermissions("owner") {
			_, e := access.RequireWorkspace(ctx, u.ID, w.ID, permission)
			if service.HasWorkspacePermission(role, permission) {
				require.NoError(t, e, role+":"+permission)
			} else {
				require.ErrorIs(t, e, service.ErrWorkspaceForbidden, role+":"+permission)
			}
		}
		_, e := repo.Mutate(ctx, u.ID, w.ID, service.WorkspaceMutation{Action: "project.create", Project: service.ProjectInput{Name: "Role", Slug: role}})
		if role == "owner" || role == "admin" {
			require.NoError(t, e)
		} else {
			require.ErrorIs(t, e, service.ErrWorkspaceForbidden)
		}
	}
	outsider := mustCreateUser(t, testEntClient(t), &service.User{})
	_, _, e := s.ListProjects(ctx, outsider.ID, w.ID, pagination.DefaultPagination())
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	_, _, e = s.ListMembers(ctx, outsider.ID, w.ID, pagination.DefaultPagination())
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	_, _, e = s.ListInvitations(ctx, outsider.ID, w.ID, pagination.DefaultPagination())
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	_, _, e = s.ListAudit(ctx, outsider.ID, w.ID, pagination.DefaultPagination())
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	require.ErrorIs(t, s.AdminSetStatus(ctx, owner.ID, w.ID, "suspended"), service.ErrWorkspaceForbidden)
	admin := mustCreateUser(t, testEntClient(t), &service.User{Role: service.RoleAdmin})
	_, e = s.GetWorkspace(ctx, admin.ID, w.ID)
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	require.NoError(t, s.AdminSetStatus(ctx, admin.ID, w.ID, "suspended"))
	_, e = s.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Blocked", Slug: "blocked"})
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	_, e = s.GetWorkspace(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	require.NoError(t, s.AdminSetStatus(ctx, admin.ID, w.ID, "active"))
	require.NoError(t, s.AdminSetStatus(ctx, admin.ID, w.ID, "archived"))
	require.ErrorIs(t, s.AdminSetStatus(ctx, admin.ID, w.ID, "active"), service.ErrWorkspaceConflict)
}

func TestWorkspaceConcurrentOwnersAndLifecycle(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, w := workspaceFixture(t)
	second := workspaceJoin(t, ctx, s, owner.ID, w.ID, "owner")
	payer := workspaceJoin(t, ctx, s, owner.ID, w.ID, "billing")
	require.NoError(t, s.ChangeBillingOwner(ctx, owner.ID, w.ID, payer.ID))
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, u := range []*service.User{owner, second} {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); results <- s.UpdateMember(ctx, id, w.ID, id, "developer", "active") }(u.ID)
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, service.ErrWorkspaceConflict)
		}
	}
	require.Equal(t, 1, successes)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND role='owner' AND status='active'`, w.ID).Scan(&count))
	require.Equal(t, 1, count)
	var remaining int64
	require.NoError(t, integrationDB.QueryRow(`SELECT user_id FROM workspace_members WHERE workspace_id=$1 AND role='owner' AND status='active'`, w.ID).Scan(&remaining))
	require.ErrorIs(t, s.RemoveMember(ctx, remaining, w.ID, remaining), service.ErrWorkspaceConflict)
	require.ErrorIs(t, s.UpdateMember(ctx, remaining, w.ID, remaining, "owner", "suspended"), service.ErrWorkspaceConflict)
	require.ErrorIs(t, s.UpdateMember(ctx, remaining, w.ID, payer.ID, "billing", "suspended"), service.ErrWorkspaceConflict)
	// Direct SQL cannot circumvent repository lifecycle checks.
	_, e := integrationDB.Exec(`UPDATE users SET status='disabled' WHERE id=$1`, remaining)
	require.Error(t, e)
	_, e = integrationDB.Exec(`UPDATE users SET deleted_at=now() WHERE id=$1`, payer.ID)
	require.Error(t, e)
	// Suspension of a regular member removes all panel authority immediately.
	member := workspaceJoin(t, ctx, s, remaining, w.ID, "developer")
	_, e = integrationDB.Exec(`UPDATE users SET status='disabled' WHERE id=$1`, member.ID)
	require.NoError(t, e)
	_, e = s.GetWorkspace(ctx, member.ID, w.ID)
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
}

func TestWorkspaceBootstrapTransactionAndAuditAtomicity(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, w := workspaceFixture(t)
	// Raw SQL user insertion creates all personal resources in exactly this tx.
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	var id int64
	require.NoError(t, tx.QueryRow(`INSERT INTO users(email,password_hash,role,status) VALUES($1,'fixture','user','active') RETURNING id`, fmt.Sprintf("bootstrap-%d@example.com", time.Now().UnixNano())).Scan(&id))
	var personal, projects int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM workspaces WHERE type='personal' AND owner_user_id=$1`, id).Scan(&personal))
	require.Equal(t, 1, personal)
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM projects p JOIN workspaces w ON w.id=p.workspace_id WHERE w.owner_user_id=$1`, id).Scan(&projects))
	require.Equal(t, 1, projects)
	require.NoError(t, tx.Rollback())
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspaces WHERE owner_user_id=$1`, id).Scan(&personal))
	require.Zero(t, personal)
	// Simulate audit-storage failure for this workspace only; business mutation
	// must roll back, including its newly generated project ID.
	_, e = integrationDB.Exec(fmt.Sprintf(`ALTER TABLE workspace_audit_logs ADD CONSTRAINT workspace_test_audit_failure CHECK (NOT (workspace_id=%d AND action='project_created')) NOT VALID`, w.ID))
	require.NoError(t, e)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`ALTER TABLE workspace_audit_logs DROP CONSTRAINT IF EXISTS workspace_test_audit_failure`)
	})
	_, e = s.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Atomic", Slug: "audit-atomic"})
	require.Error(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM projects WHERE workspace_id=$1 AND slug='audit-atomic'`, w.ID).Scan(&projects))
	require.Zero(t, projects)
	_, e = integrationDB.Exec(`ALTER TABLE workspace_audit_logs DROP CONSTRAINT workspace_test_audit_failure`)
	require.NoError(t, e)
	require.NoError(t, ApplyMigrations(ctx, integrationDB))
}

func TestWorkspaceLegacyUserBootstrap(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	tx, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	defer tx.Rollback()
	// Represent a user that existed before migration 273. Trigger control remains
	// local to this isolated database transaction and is restored before commit.
	_, e = tx.Exec(`ALTER TABLE users DISABLE TRIGGER users_personal_workspace_bootstrap`)
	require.NoError(t, e)
	var id int64
	require.NoError(t, tx.QueryRow(`INSERT INTO users(email,password_hash,role,status) VALUES($1,'fixture','user','active') RETURNING id`, fmt.Sprintf("legacy-user-%d@example.com", time.Now().UnixNano())).Scan(&id))
	_, e = tx.Exec(`ALTER TABLE users ENABLE TRIGGER users_personal_workspace_bootstrap`)
	require.NoError(t, e)
	require.NoError(t, tx.Commit())
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspaces WHERE owner_user_id=$1`, id).Scan(&count))
	require.Zero(t, count)
	repo := NewWorkspaceRepository(integrationDB)
	require.NoError(t, repo.Bootstrap(ctx, 1))
	require.NoError(t, repo.Bootstrap(ctx, 1))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspaces WHERE owner_user_id=$1 AND type='personal'`, id).Scan(&count))
	require.Equal(t, 1, count)
}

func TestWorkspaceOwnerRemovalRace(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, s, owner, w := workspaceFixture(t)
	second := workspaceJoin(t, ctx, s, owner.ID, w.ID, "owner")
	payer := workspaceJoin(t, ctx, s, owner.ID, w.ID, "billing")
	require.NoError(t, s.ChangeBillingOwner(ctx, owner.ID, w.ID, payer.ID))
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, u := range []*service.User{owner, second} {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); results <- s.RemoveMember(ctx, id, w.ID, id) }(u.ID)
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, service.ErrWorkspaceConflict)
		}
	}
	require.Equal(t, 1, successes)
	var remaining int64
	require.NoError(t, integrationDB.QueryRow(`SELECT user_id FROM workspace_members WHERE workspace_id=$1 AND role='owner' AND status='active'`, w.ID).Scan(&remaining))
	member := workspaceJoin(t, ctx, s, remaining, w.ID, "admin")
	repo := NewWorkspaceRepository(integrationDB)
	_, e := service.NewWorkspaceAccessService(repo).RequireWorkspace(ctx, member.ID, w.ID, "project.create")
	require.NoError(t, e)
	require.NoError(t, s.RemoveMember(ctx, remaining, w.ID, member.ID))
	_, e = repo.Mutate(ctx, member.ID, w.ID, service.WorkspaceMutation{Action: "project.create", Project: service.ProjectInput{Name: "Stale", Slug: "stale"}})
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
}
