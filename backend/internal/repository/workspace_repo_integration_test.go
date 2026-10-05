//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceFoundation(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	owner := mustCreateUser(t, client, &service.User{})
	outsider := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tenant-invite-%d@example.com", time.Now().UnixNano())})
	repo := NewWorkspaceRepository(integrationDB)
	svc := service.NewWorkspaceService(repo)
	access := service.NewWorkspaceAccessService(repo)
	p, err := svc.EnsurePersonalWorkspace(ctx, owner.ID)
	require.NoError(t, err)
	again, err := svc.EnsurePersonalWorkspace(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, p.ID, again.ID)
	renamed, err := svc.UpdateWorkspace(ctx, owner.ID, p.ID, "My Personal", p.Slug)
	require.NoError(t, err)
	require.Equal(t, "My Personal", renamed.Name)
	var n int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM projects WHERE workspace_id=$1 AND is_default`, p.ID).Scan(&n))
	require.Equal(t, 1, n)
	group := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("workspace-group-%d", owner.ID)})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: owner.ID, GroupID: &group.ID, Key: fmt.Sprintf("workspace-legacy-%d", owner.ID), Quota: 123, IPWhitelist: []string{"127.0.0.1"}})
	var before, after []byte
	require.NoError(t, integrationDB.QueryRow(`SELECT to_jsonb(k)-'project_id' FROM api_keys k WHERE id=$1`, key.ID).Scan(&before))
	require.NoError(t, svc.Bootstrap(ctx, 100))
	require.NoError(t, integrationDB.QueryRow(`SELECT to_jsonb(k)-'project_id' FROM api_keys k WHERE id=$1`, key.ID).Scan(&after))
	require.JSONEq(t, string(before), string(after))
	var projectID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT project_id FROM api_keys WHERE id=$1`, key.ID).Scan(&projectID))
	require.Positive(t, projectID)
	w, err := svc.CreateOrganization(ctx, owner.ID, "Acme", fmt.Sprintf("acme-%d", owner.ID))
	require.NoError(t, err)
	_, err = svc.CreateOrganization(ctx, owner.ID, "Duplicate", w.Slug)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	_, err = access.RequireWorkspace(ctx, outsider.ID, w.ID, "workspace.read")
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	proj, err := svc.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Production", Slug: "production", AllowedModels: []string{}})
	require.NoError(t, err)
	require.NotNil(t, proj.AllowedModels)
	_, err = svc.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Duplicate", Slug: "production"})
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	_, err = svc.CreateProject(ctx, owner.ID, p.ID, service.ProjectInput{Name: "Another tenant", Slug: "production"})
	require.NoError(t, err)
	_, err = access.RequireProject(ctx, owner.ID, p.ID, proj.ID, "project.read")
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	invitation, token, err := svc.CreateInvitation(ctx, owner.ID, w.ID, strings.ToUpper(outsider.Email), "viewer", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	encoded, _ := json.Marshal(invitation)
	require.NotContains(t, string(encoded), token)
	_, _, err = svc.CreateInvitation(ctx, owner.ID, w.ID, outsider.Email, "viewer", time.Hour)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	_, err = svc.AcceptInvitation(ctx, owner.ID, token)
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := svc.AcceptInvitation(ctx, outsider.ID, token); results <- e }()
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
	_, err = access.RequireWorkspace(ctx, outsider.ID, w.ID, "workspace.read")
	require.NoError(t, err)
	_, err = svc.CreateProject(ctx, outsider.ID, w.ID, service.ProjectInput{Name: "No", Slug: "no"})
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	err = svc.UpdateMember(ctx, owner.ID, w.ID, owner.ID, "viewer", "active")
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	err = svc.RemoveMember(ctx, owner.ID, w.ID, owner.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	err = svc.ChangeBillingOwner(ctx, owner.ID, w.ID, outsider.ID)
	require.NoError(t, err)
	err = svc.RemoveMember(ctx, owner.ID, w.ID, outsider.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	userRepo := NewUserRepository(client, integrationDB)
	require.ErrorIs(t, userRepo.Delete(ctx, outsider.ID), service.ErrWorkspaceConflict)
	outsider.Status = "disabled"
	require.ErrorIs(t, userRepo.Update(ctx, outsider, service.UserUpdateFields{Status: true}), service.ErrWorkspaceConflict)
	require.NoError(t, svc.ChangeBillingOwner(ctx, owner.ID, w.ID, owner.ID))
	require.NoError(t, svc.UpdateMember(ctx, owner.ID, w.ID, outsider.ID, "admin", "active"))
	_, _, err = svc.CreateInvitation(ctx, outsider.ID, w.ID, "grant-owner@example.com", "owner", time.Hour)
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	require.ErrorIs(t, svc.UpdateMember(ctx, outsider.ID, w.ID, owner.ID, "viewer", "active"), service.ErrWorkspaceForbidden)
	require.NoError(t, svc.ArchiveProject(ctx, owner.ID, w.ID, proj.ID))
	events, _, err := svc.ListAudit(ctx, owner.ID, w.ID, pagination.DefaultPagination())
	require.NoError(t, err)
	require.NotEmpty(t, events)
	b, _ := json.Marshal(events)
	require.NotContains(t, string(b), token)
	require.NoError(t, svc.ArchiveWorkspace(ctx, owner.ID, w.ID))
	require.NoError(t, userRepo.Delete(ctx, owner.ID))
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspaces WHERE id=$1`, p.ID).Scan(&state))
	require.Equal(t, "archived", state)
}
