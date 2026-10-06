//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceGovernanceTenantScopedAccessAndAtomicEvents(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, workspaces, owner, workspace := workspaceFixture(t)
	developer := workspaceJoin(t, ctx, workspaces, owner.ID, workspace.ID, "developer")
	otherOwner := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("governance-other-%d@example.com", owner.ID)})
	otherWorkspace, err := workspaces.CreateOrganization(ctx, otherOwner.ID, "Other", fmt.Sprintf("governance-other-%d", otherOwner.ID))
	require.NoError(t, err)
	otherMember := workspaceJoin(t, ctx, workspaces, otherOwner.ID, otherWorkspace.ID, "developer")

	project, err := workspaces.CreateProject(ctx, owner.ID, workspace.ID, service.ProjectInput{Name: "Production", Slug: "production"})
	require.NoError(t, err)
	otherProject, err := workspaces.CreateProject(ctx, otherOwner.ID, otherWorkspace.ID, service.ProjectInput{Name: "Other", Slug: "other"})
	require.NoError(t, err)
	team, err := workspaces.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Backend", Slug: "backend"})
	require.NoError(t, err)
	var developerMemberID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, developer.ID).Scan(&developerMemberID))
	require.NoError(t, workspaces.AddTeamMember(ctx, owner.ID, workspace.ID, team.ID, developerMemberID))

	beforeAudit, beforeEvents := governanceCounts(t, ctx, workspace.ID)
	_, err = workspaces.SetProjectAccessMode(ctx, owner.ID, workspace.ID, service.ProjectAccessModeAssigned)
	require.NoError(t, err)
	afterAudit, afterEvents := governanceCounts(t, ctx, workspace.ID)
	require.Greater(t, afterAudit, beforeAudit)
	require.Greater(t, afterEvents, beforeEvents)
	var outbox int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND e.event_type=$2`, workspace.ID, service.EventWorkspaceProjectAccessMode).Scan(&outbox))
	require.GreaterOrEqual(t, outbox, 1)

	projects, total, err := workspaces.ListProjects(ctx, developer.ID, workspace.ID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, projects)
	projects, total, err = workspaces.ListProjects(ctx, owner.ID, workspace.ID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, projects, 2)

	teamGrant, err := workspaces.CreateProjectAccessGrant(ctx, owner.ID, workspace.ID, project.ID, service.ProjectAccessGrantInput{SubjectType: service.ProjectAccessSubjectTeam, SubjectID: team.ID, Role: service.ProjectAccessRoleDeveloper})
	require.NoError(t, err)
	projects, total, err = workspaces.ListProjects(ctx, developer.ID, workspace.ID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, projects, 1)
	require.Equal(t, project.ID, projects[0].ID)
	_, err = service.NewWorkspaceAccessService(NewWorkspaceRepository(integrationDB)).RequireProject(ctx, developer.ID, workspace.ID, project.ID, "key.create")
	require.NoError(t, err)

	directViewer, err := workspaces.CreateProjectAccessGrant(ctx, owner.ID, workspace.ID, project.ID, service.ProjectAccessGrantInput{SubjectType: service.ProjectAccessSubjectMember, SubjectID: developerMemberID, Role: service.ProjectAccessRoleViewer})
	require.NoError(t, err)
	_, err = service.NewWorkspaceAccessService(NewWorkspaceRepository(integrationDB)).RequireProject(ctx, developer.ID, workspace.ID, project.ID, "key.create")
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	require.NoError(t, workspaces.DeleteProjectAccessGrant(ctx, owner.ID, workspace.ID, project.ID, directViewer.ID))
	_, err = service.NewWorkspaceAccessService(NewWorkspaceRepository(integrationDB)).RequireProject(ctx, developer.ID, workspace.ID, project.ID, "key.create")
	require.NoError(t, err)

	_, err = workspaces.CreateProjectAccessGrant(ctx, owner.ID, workspace.ID, project.ID, service.ProjectAccessGrantInput{SubjectType: service.ProjectAccessSubjectTeam, SubjectID: team.ID, Role: service.ProjectAccessRoleDeveloper})
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	require.NoError(t, workspaces.RemoveTeamMember(ctx, owner.ID, workspace.ID, team.ID, developerMemberID))
	projects, total, err = workspaces.ListProjects(ctx, developer.ID, workspace.ID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, projects)
	require.NoError(t, workspaces.DeleteProjectAccessGrant(ctx, owner.ID, workspace.ID, project.ID, teamGrant.ID))

	_, err = workspaces.CreateProjectAccessGrant(ctx, owner.ID, workspace.ID, project.ID, service.ProjectAccessGrantInput{SubjectType: service.ProjectAccessSubjectMember, SubjectID: otherMemberWorkspaceMemberID(t, ctx, otherWorkspace.ID, otherMember.ID), Role: service.ProjectAccessRoleViewer})
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	_, err = workspaces.CreateProjectAccessGrant(ctx, owner.ID, workspace.ID, otherProject.ID, service.ProjectAccessGrantInput{SubjectType: service.ProjectAccessSubjectTeam, SubjectID: team.ID, Role: service.ProjectAccessRoleViewer})
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)

	otherMemberID := otherMemberWorkspaceMemberID(t, ctx, otherWorkspace.ID, otherMember.ID)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO project_access_grants(workspace_id,project_id,subject_type,subject_id,role,created_by_user_id) VALUES($1,$2,'member',$3,'viewer',$4)`, workspace.ID, project.ID, otherMemberID, owner.ID)
	require.Error(t, err, "database trigger must reject cross-tenant member subjects")

	var eventType string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT event_type FROM domain_events WHERE workspace_id=$1 AND event_type=$2 ORDER BY created_at DESC LIMIT 1`, workspace.ID, service.EventWorkspaceProjectAccessDeleted).Scan(&eventType))
	require.Equal(t, service.EventWorkspaceProjectAccessDeleted, eventType)
}

func governanceCounts(t *testing.T, ctx context.Context, workspaceID int64) (audit, events int) {
	t.Helper()
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1`, workspaceID).Scan(&audit))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM domain_events WHERE workspace_id=$1`, workspaceID).Scan(&events))
	return audit, events
}

func otherMemberWorkspaceMemberID(t *testing.T, ctx context.Context, workspaceID, userID int64) int64 {
	t.Helper()
	var memberID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspaceID, userID).Scan(&memberID))
	return memberID
}
