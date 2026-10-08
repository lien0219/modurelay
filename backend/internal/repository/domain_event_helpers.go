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
	case "allocation_center_created", "allocation_center_updated", "allocation_center_archived",
		"allocation_tag_created", "allocation_tag_updated", "allocation_tag_archived":
		eventType = service.EventWorkspaceUpdated
	case "allocation_project_updated", "allocation_key_updated", "allocation_service_account_updated":
		eventType = service.EventProjectUpdated
	case "policy_updated":
		eventType = service.EventPolicyUpdated
	case "workspace_project_access_mode_updated":
		eventType = service.EventWorkspaceProjectAccessMode
	case "team_created":
		eventType = service.EventWorkspaceTeamCreated
	case "team_updated":
		eventType = service.EventWorkspaceTeamUpdated
	case "team_archived":
		eventType = service.EventWorkspaceTeamArchived
	case "team_member_added":
		eventType = service.EventWorkspaceTeamMemberAdded
	case "team_member_removed":
		eventType = service.EventWorkspaceTeamMemberRemoved
	case "project_access_grant_created":
		eventType = service.EventWorkspaceProjectAccessCreated
	case "project_access_grant_updated":
		eventType = service.EventWorkspaceProjectAccessUpdated
	case "project_access_grant_deleted":
		eventType = service.EventWorkspaceProjectAccessDeleted
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
