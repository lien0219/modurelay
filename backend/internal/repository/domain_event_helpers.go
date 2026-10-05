package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func insertWorkspaceMutationEvent(ctx context.Context, q workspaceSQL, workspaceID, projectID, actorID int64, action, target string, targetID int64, data service.DomainEventData) error {
	eventType := ""
	if action == "member_role_changed" {
		if status, ok := data["status"].(string); ok && status == "suspended" {
			eventType = service.EventMemberSuspended
		}
	}
	switch action {
	case "workspace_created":
		eventType = service.EventWorkspaceCreated
	case "workspace_updated":
		eventType = service.EventWorkspaceUpdated
	case "workspace_archived":
		eventType = service.EventWorkspaceArchived
	case "workspace_suspended":
		eventType = service.EventWorkspaceSuspended
	case "workspace_active", "workspace_resumed":
		eventType = service.EventWorkspaceResumed
	case "billing_owner_changed":
		eventType = service.EventWorkspaceUpdated
	case "project_created":
		eventType = service.EventProjectCreated
	case "project_updated":
		eventType = service.EventProjectUpdated
	case "project_archived":
		eventType = service.EventProjectArchived
	case "member_invited":
		eventType = service.EventMemberInvited
	case "member_joined":
		eventType = service.EventMemberJoined
	case "member_role_changed":
		if eventType == "" {
			eventType = service.EventMemberRoleChanged
		}
	case "member_removed":
		eventType = service.EventMemberRemoved
	case "budget_updated":
		eventType = service.EventBudgetUpdated
	default:
		return nil
	}
	if targetID <= 0 {
		targetID = workspaceID
	}
	if target == "user" && (eventType == service.EventMemberRoleChanged || eventType == service.EventMemberSuspended || eventType == service.EventMemberRemoved) {
		data["user_id"] = targetID
	}
	event, err := service.NewDomainEvent(eventType, workspaceID, projectID, actorID, target, fmt.Sprintf("%d", targetID), data)
	if err != nil {
		return err
	}
	return insertDomainEventTx(ctx, q, event, "")
}
