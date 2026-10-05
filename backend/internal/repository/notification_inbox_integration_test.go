//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNotificationInboxScopesUnreadReadAllAndPages(t *testing.T) {
	ctx, _, owner, workspace, projectID := outboxFixture(t)
	other := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("other-inbox-%d@example.com", owner.ID)})
	repo := NewNotificationRepository(integrationDB)
	for i, category := range []string{"budget", "project", "budget", "billing", "budget"} {
		event := outboxTestEvent(t, workspace.ID, projectID, owner.ID)
		insertOutboxTestEvent(t, ctx, event, "")
		recipient := owner.ID
		if i == 4 {
			recipient = other.ID
		}
		require.NoError(t, repo.CreateForRecipients(ctx, []service.UserNotification{{EventID: event.ID, RecipientUserID: recipient, WorkspaceID: &workspace.ID, ProjectID: &projectID, Category: category, TitleKey: "title", BodyKey: "body", Data: map[string]any{"status": "active"}}}))
	}
	count, err := repo.UnreadCount(ctx, owner.ID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(4), count)
	items, total, err := repo.List(ctx, owner.ID, service.NotificationListFilter{Page: 1, PageSize: 2, WorkspaceID: &workspace.ID, ProjectID: &projectID})
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, items, 2)
	for _, item := range items {
		require.Equal(t, owner.ID, item.RecipientUserID)
		require.Equal(t, workspace.ID, *item.WorkspaceID)
		require.Equal(t, projectID, *item.ProjectID)
	}
	page2, total, err := repo.List(ctx, owner.ID, service.NotificationListFilter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, page2, 2)
	require.NotEqual(t, items[0].ID, page2[0].ID)
	unread := true
	budget, total, err := repo.List(ctx, owner.ID, service.NotificationListFilter{Category: "budget", Unread: &unread})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, budget, 2)
	require.NoError(t, repo.MarkRead(ctx, owner.ID, budget[0].ID))
	require.NoError(t, repo.MarkRead(ctx, owner.ID, budget[0].ID))
	count, err = repo.UnreadCount(ctx, owner.ID, &workspace.ID, &projectID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
	read := false
	readItems, total, err := repo.List(ctx, owner.ID, service.NotificationListFilter{Unread: &read})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, readItems, 1)
	require.NotNil(t, readItems[0].ReadAt)
	otherItems, _, err := repo.List(ctx, other.ID, service.NotificationListFilter{})
	require.NoError(t, err)
	require.Len(t, otherItems, 1)
	require.Error(t, repo.MarkRead(ctx, owner.ID, otherItems[0].ID), "recipient predicate must deny cross-user mark-read")
	count, err = repo.UnreadCount(ctx, other.ID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	changed, err := repo.MarkAllRead(ctx, owner.ID, &workspace.ID, &projectID)
	require.NoError(t, err)
	require.Equal(t, int64(3), changed)
	changed, err = repo.MarkAllRead(ctx, owner.ID, nil, nil)
	require.NoError(t, err)
	require.Zero(t, changed)
	count, err = repo.UnreadCount(ctx, owner.ID, nil, nil)
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = repo.UnreadCount(ctx, other.ID, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	// Inbox is a user control-plane surface, including suspended workspaces.
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspaces SET status='suspended' WHERE id=$1`, workspace.ID)
	require.NoError(t, err)
	items, total, err = repo.List(ctx, owner.ID, service.NotificationListFilter{WorkspaceID: &workspace.ID})
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, items, 4)
}

func TestNotificationInboxCancellationStopsDatabaseWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewNotificationRepository(integrationDB)
	_, _, err := repo.List(ctx, 1, service.NotificationListFilter{})
	require.ErrorIs(t, err, context.Canceled)
	_, err = repo.UnreadCount(ctx, 1, nil, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, repo.MarkRead(ctx, 1, 1), context.Canceled)
	_, err = repo.MarkAllRead(ctx, 1, nil, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestNotificationRecipientResolverRoleAndAffectedUserSemantics(t *testing.T) {
	ctx, workspaceService, owner, workspace, projectID := outboxFixture(t)
	admin := workspaceJoin(t, ctx, workspaceService, owner.ID, workspace.ID, "admin")
	billing := workspaceJoin(t, ctx, workspaceService, owner.ID, workspace.ID, "billing")
	developer := workspaceJoin(t, ctx, workspaceService, owner.ID, workspace.ID, "developer")
	viewer := workspaceJoin(t, ctx, workspaceService, owner.ID, workspace.ID, "viewer")
	disabled := workspaceJoin(t, ctx, workspaceService, owner.ID, workspace.ID, "billing")
	deleted := workspaceJoin(t, ctx, workspaceService, owner.ID, workspace.ID, "developer")
	_, err := integrationDB.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, disabled.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET deleted_at=now() WHERE id=$1`, deleted.ID)
	require.NoError(t, err)
	resolver := NewNotificationRecipientResolver(integrationDB)
	for _, tc := range []struct {
		typ  string
		want []int64
	}{
		{service.EventBudgetThreshold, []int64{owner.ID, admin.ID, billing.ID}},
		{service.EventBillingPending, []int64{owner.ID, admin.ID, billing.ID}},
		{service.EventProjectArchived, []int64{owner.ID, admin.ID, developer.ID}},
		{service.EventWorkspaceSuspended, []int64{owner.ID, admin.ID, billing.ID, developer.ID, viewer.ID}},
	} {
		event, err := service.NewDomainEvent(tc.typ, workspace.ID, projectID, owner.ID, "workspace", fmt.Sprint(workspace.ID), service.DomainEventData{})
		require.NoError(t, err)
		ids, err := resolver.Resolve(ctx, event)
		require.NoError(t, err)
		require.Equal(t, tc.want, ids, tc.typ)
	}
	for _, tc := range []struct{ typ, status string }{{service.EventMemberSuspended, "suspended"}, {service.EventMemberRemoved, "removed"}} {
		if tc.status == "suspended" {
			_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, viewer.ID)
		} else {
			_, err = integrationDB.ExecContext(ctx, `DELETE FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, viewer.ID)
		}
		require.NoError(t, err)
		event, err := service.NewDomainEvent(tc.typ, workspace.ID, 0, owner.ID, "user", fmt.Sprint(viewer.ID), service.DomainEventData{"user_id": viewer.ID, "status": tc.status})
		require.NoError(t, err)
		ids, err := resolver.Resolve(ctx, event)
		require.NoError(t, err)
		require.Equal(t, []int64{owner.ID, admin.ID, viewer.ID}, ids, tc.typ)
	}
}
