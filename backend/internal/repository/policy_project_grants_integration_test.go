//go:build integration

package repository

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPolicyHTTPServiceAccountProjectGrantAndRoleMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, mode, role, grant string
		status                  int
	}{
		{"all_developer", "all_projects", "developer", "", 200},
		{"all_viewer", "all_projects", "viewer", "", 403},
		{"assigned_owner", "assigned_projects", "owner", "", 200},
		{"assigned_admin", "assigned_projects", "admin", "", 200},
		{"assigned_developer_without_grant", "assigned_projects", "developer", "", 403},
		{"assigned_developer_viewer", "assigned_projects", "developer", "viewer", 403},
		{"assigned_developer_developer", "assigned_projects", "developer", "developer", 200},
		{"assigned_developer_admin", "assigned_projects", "developer", "admin", 200},
		{"assigned_viewer_developer", "assigned_projects", "viewer", "developer", 403},
		{"assigned_viewer_admin", "assigned_projects", "viewer", "admin", 403},
		{"assigned_billing_admin", "assigned_projects", "billing", "admin", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			f := serviceAccountSchemaFixture(t)
			ctx := context.Background()
			svc, _, ws := serviceAccountTestService(t)
			actor := f.payer
			if tc.role != "owner" {
				actor = workspaceJoin(t, ctx, ws, f.payer, f.workspace, tc.role).ID
			}
			_, err := ws.SetProjectAccessMode(ctx, f.payer, f.workspace, tc.mode)
			require.NoError(t, err)
			if tc.grant != "" {
				_, err = ws.CreateProjectAccessGrant(ctx, f.payer, f.workspace, f.project, service.ProjectAccessGrantInput{SubjectType: "member", SubjectID: otherMemberWorkspaceMemberID(t, ctx, f.workspace, actor), Role: tc.grant})
				require.NoError(t, err)
			}
			repo := NewPolicyRepository(integrationDB)
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actor}) })
			handler.NewPolicyHandler(ws, svc, repo, nil).RegisterTenantRoutes(router.Group("/api/v1"))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/workspaces/%d/projects/%d/service-accounts/%d/policy", f.workspace, f.project, f.sa), strings.NewReader(`{"expected_revision":0,"allowed_models":[]}`)))
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			policy, err := repo.GetPolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: f.sa})
			require.NoError(t, err)
			audit, events, outbox := policyMutationCounts(t, f.workspace)
			if tc.status == 200 {
				require.NotNil(t, policy)
				require.EqualValues(t, 1, policy.Revision)
				require.NotNil(t, policy.AllowedModels)
				require.Empty(t, policy.AllowedModels)
				require.Equal(t, 1, audit)
				require.Equal(t, 1, events)
				require.Equal(t, 1, outbox)
			} else {
				require.Nil(t, policy)
				require.Zero(t, audit)
				require.Zero(t, events)
				require.Zero(t, outbox)
			}
		})
	}
}

func TestPolicyRepositoryDirectViewerOverridesTeamAndRevokedTeamIsLive(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	ctx := context.Background()
	svc, _, ws := serviceAccountTestService(t)
	actor := workspaceJoin(t, ctx, ws, f.payer, f.workspace, "developer")
	memberID := otherMemberWorkspaceMemberID(t, ctx, f.workspace, actor.ID)
	team, err := ws.CreateTeam(ctx, f.payer, f.workspace, service.WorkspaceTeamInput{Name: "Policy writers", Slug: "policy-writers"})
	require.NoError(t, err)
	require.NoError(t, ws.AddTeamMember(ctx, f.payer, f.workspace, team.ID, memberID))
	_, err = ws.SetProjectAccessMode(ctx, f.payer, f.workspace, service.ProjectAccessModeAssigned)
	require.NoError(t, err)
	_, err = ws.CreateProjectAccessGrant(ctx, f.payer, f.workspace, f.project, service.ProjectAccessGrantInput{SubjectType: "team", SubjectID: team.ID, Role: "admin"})
	require.NoError(t, err)
	viewer, err := ws.CreateProjectAccessGrant(ctx, f.payer, f.workspace, f.project, service.ProjectAccessGrantInput{SubjectType: "member", SubjectID: memberID, Role: "viewer"})
	require.NoError(t, err)
	ref := domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: f.sa}
	repo := NewPolicyRepository(integrationDB)
	_, err = repo.UpdatePolicy(service.WithPolicyActor(ctx, actor.ID), ref, 0, domain.Policy{})
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden, "direct viewer must override the stronger Team grant at the repository boundary")
	require.ErrorIs(t, svc.RequirePolicy(ctx, actor.ID, f.workspace, f.project, f.sa, "service_account_policy.update"), service.ErrWorkspaceForbidden)
	require.NoError(t, svc.RequirePolicy(ctx, actor.ID, f.workspace, f.project, f.sa, "policy.read"), "read-only access is preserved")
	require.NoError(t, ws.DeleteProjectAccessGrant(ctx, f.payer, f.workspace, f.project, viewer.ID))
	created, err := repo.UpdatePolicy(service.WithPolicyActor(ctx, actor.ID), ref, 0, domain.Policy{})
	require.NoError(t, err, "removing the direct override reveals the active Team admin grant")
	require.EqualValues(t, 1, created.Revision)
	require.NoError(t, ws.RemoveTeamMember(ctx, f.payer, f.workspace, team.ID, memberID))
	_, err = repo.UpdatePolicy(service.WithPolicyActor(ctx, actor.ID), ref, 1, domain.Policy{AllowedModels: []string{}})
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden, "removed Team membership must immediately deny policy writes")
	current, err := repo.GetPolicy(ctx, ref)
	require.NoError(t, err)
	require.EqualValues(t, 1, current.Revision)
	audit, events, outbox := policyMutationCounts(t, f.workspace)
	require.Equal(t, 1, audit)
	require.Equal(t, 1, events)
	require.Equal(t, 1, outbox)
}

func TestPolicyHTTPMixedTenantIDsAndArchivedProjectDenyMutation(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	gin.SetMode(gin.TestMode)
	f := serviceAccountSchemaFixture(t)
	other := serviceAccountSchemaFixture(t)
	ctx := context.Background()
	svc, _, ws := serviceAccountTestService(t)
	wrongProject, err := ws.CreateProject(ctx, f.payer, f.workspace, service.ProjectInput{Name: "Wrong", Slug: "wrong"})
	require.NoError(t, err)
	repo := NewPolicyRepository(integrationDB)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: f.payer})
	})
	handler.NewPolicyHandler(ws, svc, repo, nil).RegisterTenantRoutes(router.Group("/api/v1"))
	for _, ids := range [][3]int64{{f.workspace, wrongProject.ID, f.sa}, {f.workspace, other.project, f.sa}, {f.workspace, f.project, other.sa}, {other.workspace, other.project, other.sa}} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/workspaces/%d/projects/%d/service-accounts/%d/policy", ids[0], ids[1], ids[2]), strings.NewReader(`{"expected_revision":0,"allowed_models":[]}`)))
		require.Equal(t, 404, rec.Code, rec.Body.String())
	}
	require.NoError(t, ws.ArchiveProject(ctx, f.payer, f.workspace, f.project))
	for _, ref := range []domain.PolicyRef{{Scope: domain.PolicyScopeProject, ScopeID: f.project}, {Scope: domain.PolicyScopeServiceAccount, ScopeID: f.sa}} {
		_, err = repo.UpdatePolicy(service.WithPolicyActor(ctx, f.payer), ref, 0, domain.Policy{})
		require.ErrorIs(t, err, service.ErrWorkspaceConflict)
		policy, err := repo.GetPolicy(ctx, ref)
		require.NoError(t, err)
		require.Nil(t, policy)
	}
	for _, workspaceID := range []int64{f.workspace, other.workspace} {
		audit, events, outbox := policyMutationCounts(t, workspaceID)
		require.Zero(t, audit)
		require.Zero(t, events)
		require.Zero(t, outbox)
	}
}
