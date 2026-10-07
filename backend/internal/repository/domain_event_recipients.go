package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type notificationRecipientResolver struct{ db *sql.DB }

func NewNotificationRecipientResolver(db *sql.DB) service.NotificationRecipientResolver {
	return &notificationRecipientResolver{db: db}
}

func (r *notificationRecipientResolver) Resolve(ctx context.Context, event *service.DomainEvent) ([]int64, error) {
	if r == nil || r.db == nil || event == nil || event.WorkspaceID == nil || *event.WorkspaceID <= 0 {
		return nil, service.ErrWorkspaceNotFound
	}
	// Routine identity updates and login reconciliation are durable audit/webhook
	// events, without an inbox notification on every login or failed DNS check.
	if event.Data["category"] == "scim" {
		// Provisioning changes remain in audit/webhooks without filling inboxes
		// during a directory sync. Operational failures use separate events.
		switch event.Type {
		case service.EventSCIMConnectorDisabled, service.EventSCIMSyncFailed, service.EventSCIMTokenExpiring, service.EventSCIMSecurityConflict:
		default:
			return nil, nil
		}
	}
	switch event.Type {
	case service.EventSCIMConnectorCreated, service.EventSCIMTokenCreated, service.EventSCIMTokenRevoked:
		return nil, nil
	case service.EventWorkspaceDomainCreated, service.EventWorkspaceDomainRegenerated, service.EventWorkspaceDomainRevoked,
		service.EventIdentityProviderCreated, service.EventIdentityProviderUpdated, service.EventOIDCMappingsUpdated,
		service.EventSAMLMetadataUpdated,
		service.EventOIDCJITProvisioned, service.EventOIDCIdentityLinked, service.EventOIDCRoleReconciled, service.EventOIDCTeamsReconciled:
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT m.user_id,m.role FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL`, *event.WorkspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	affected := eventDataInt64(event.Data, "user_id")
	if affected == 0 && event.Subject.Type == "user" {
		affected, _ = strconv.ParseInt(event.Subject.ID, 10, 64)
	}
	allowed := func(role string) bool { return true }
	switch event.Type {
	case service.EventWorkspaceDomainVerified, service.EventIdentityProviderDisabled, service.EventSAMLCertificateRotated,
		service.EventSSOEnforcementEnabled, service.EventSSOEnforcementDisabled, service.EventSSOBreakGlassUsed,
		service.EventSCIMConnectorDisabled, service.EventSCIMSyncFailed, service.EventSCIMTokenExpiring, service.EventSCIMSecurityConflict:
		allowed = func(role string) bool { return role == "owner" || role == "admin" }
	case service.EventBudgetThreshold, service.EventBudgetSoftLimit, service.EventBudgetHardLimit,
		service.EventBudgetUpdated, service.EventBillingPending, service.EventBillingRecovered:
		allowed = func(role string) bool { return role == "owner" || role == "admin" || role == "billing" }
	case service.EventProjectCreated, service.EventProjectUpdated, service.EventProjectArchived, service.EventProjectRestored:
		allowed = func(role string) bool { return role == "owner" || role == "admin" || role == "developer" }
	case service.EventServiceAccountCreated, service.EventServiceAccountUpdated, service.EventServiceAccountDisabled, service.EventServiceAccountEnabled, service.EventServiceAccountCredentialCreated, service.EventServiceAccountCredentialUpdated, service.EventServiceAccountCredentialRevoked, service.EventServiceAccountCredentialRotated, service.EventServiceAccountCredentialExpiring, service.EventServiceAccountCredentialExpired:
		allowed = func(role string) bool { return role == "owner" || role == "admin" || role == "developer" }
	case service.EventAPIKeyCreated, service.EventAPIKeyUpdated, service.EventAPIKeyRevoked:
		allowed = func(role string) bool { return role == "owner" || role == "admin" }
	case service.EventQuotaThreshold, service.EventQuotaExhausted:
		allowed = func(role string) bool { return role == "owner" || role == "admin" || role == "developer" }
	case service.EventMemberInvited, service.EventMemberJoined, service.EventMemberRoleChanged, service.EventMemberSuspended, service.EventMemberRemoved:
		allowed = func(role string) bool { return role == "owner" || role == "admin" }
	case service.EventWorkspaceTeamCreated, service.EventWorkspaceTeamUpdated, service.EventWorkspaceTeamArchived, service.EventWorkspaceTeamMemberAdded, service.EventWorkspaceTeamMemberRemoved,
		service.EventWorkspaceProjectAccessCreated, service.EventWorkspaceProjectAccessUpdated, service.EventWorkspaceProjectAccessDeleted, service.EventWorkspaceProjectAccessMode:
		allowed = func(role string) bool { return role == "owner" || role == "admin" }
	case service.EventWebhookTest:
		if event.ActorUserID != nil && *event.ActorUserID > 0 {
			return []int64{*event.ActorUserID}, nil
		}
		return nil, nil
	}

	ids := make(map[int64]struct{})
	for rows.Next() {
		var userID int64
		var role string
		if err := rows.Scan(&userID, &role); err != nil {
			return nil, err
		}
		if allowed(role) {
			ids[userID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if (event.Type == service.EventMemberInvited || event.Type == service.EventMemberJoined || event.Type == service.EventMemberRoleChanged || event.Type == service.EventMemberSuspended || event.Type == service.EventMemberRemoved) && affected > 0 {
		var exists bool
		// The immutable event identifies the affected user even after removal
		// or suspension has changed membership. Global identity must remain live.
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL)`, affected).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			ids[affected] = struct{}{}
		}
	}
	if event.ActorUserID != nil && *event.ActorUserID > 0 && (event.Type == service.EventAPIKeyCreated || event.Type == service.EventAPIKeyUpdated || event.Type == service.EventAPIKeyRevoked) {
		ids[*event.ActorUserID] = struct{}{}
	}
	out := make([]int64, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func eventDataInt64(data service.DomainEventData, key string) int64 {
	value, ok := data[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		id, _ := strconv.ParseInt(v, 10, 64)
		return id
	case json.Number:
		id, _ := v.Int64()
		return id
	default:
		return 0
	}
}
