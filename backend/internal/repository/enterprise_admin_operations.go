package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const enterpriseAdminOperationTimeout = 5 * time.Second

func (r *workspaceRepository) AdminOperateWorkspace(ctx context.Context, actorID, workspaceID int64, in service.AdminOperationInput) (*service.AdminOperationReceipt, error) {
	return r.adminOperation(ctx, actorID, workspaceID, 0, 0, in)
}

func (r *workspaceRepository) AdminRetryWebhook(ctx context.Context, actorID, workspaceID, webhookID, deliveryID int64, in service.AdminOperationInput) (*service.AdminOperationReceipt, error) {
	return r.adminOperation(ctx, actorID, workspaceID, webhookID, deliveryID, in)
}

func (r *workspaceRepository) adminOperation(ctx context.Context, actorID, workspaceID, webhookID, deliveryID int64, in service.AdminOperationInput) (*service.AdminOperationReceipt, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrWorkspaceConflict
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	action := in.CanonicalAction()
	if action == "retry_webhook" && (webhookID <= 0 || deliveryID <= 0) {
		return nil, service.ErrWorkspaceInvalid
	}
	if action != "retry_webhook" && (webhookID != 0 || deliveryID != 0) {
		return nil, service.ErrWorkspaceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = setAdminOperationTimeouts(ctx, tx); err != nil {
		return nil, err
	}
	// Global authority is independent of tenant membership and policy. Keep the
	// actor lock first in every operation, before any workspace or endpoint row.
	var role, status string
	var deletedAt sql.NullTime
	var mfaEnrolled bool
	err = tx.QueryRowContext(ctx, `SELECT role,status,deleted_at,COALESCE(totp_enabled,false) FROM users WHERE id=$1 FOR SHARE`, actorID).Scan(&role, &status, &deletedAt, &mfaEnrolled)
	if errors.Is(err, sql.ErrNoRows) || role != service.RoleAdmin || status != service.StatusActive || deletedAt.Valid {
		return nil, service.ErrWorkspaceForbidden
	}
	if err != nil {
		return nil, workspaceError(err)
	}
	trustedCtx, err := trustedAdminContext(ctx, mfaEnrolled)
	if err != nil {
		return nil, err
	}
	if err = service.RequireGlobalAdminOperationAuthentication(trustedCtx, time.Now().UTC()); err != nil {
		return nil, err
	}
	targetType := "workspace"
	targetID := workspaceID
	if action == "retry_webhook" {
		targetType = "webhook_delivery"
		targetID = deliveryID
	}
	fingerprint, err := in.Fingerprint(targetType, targetID, workspaceID)
	if action == "retry_webhook" {
		fingerprint, err = in.WebhookFingerprint(workspaceID, webhookID, deliveryID)
	}
	if err != nil {
		return nil, err
	}
	if receipt, existingFingerprint, found, lookupErr := readAdminReceipt(ctx, tx, actorID, in.IdempotencyKey); lookupErr != nil {
		return nil, lookupErr
	} else if found {
		return replayAdminReceipt(ctx, tx, receipt, existingFingerprint, fingerprint)
	}
	if err = lockWorkspace(ctx, tx, workspaceID, true); err != nil {
		return nil, err
	}
	// A concurrent operation with the same actor/key may have committed while
	// this transaction waited for the workspace. Replay before any new guard.
	if receipt, existingFingerprint, found, lookupErr := readAdminReceipt(ctx, tx, actorID, in.IdempotencyKey); lookupErr != nil {
		return nil, lookupErr
	} else if found {
		return replayAdminReceipt(ctx, tx, receipt, existingFingerprint, fingerprint)
	}
	if action == "retry_webhook" {
		return r.adminRetryWebhookTx(ctx, tx, actorID, workspaceID, webhookID, deliveryID, in, fingerprint)
	}
	return r.adminWorkspaceStatusTx(ctx, tx, actorID, workspaceID, action, in, fingerprint)
}

func setAdminOperationTimeouts(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `SELECT set_config('statement_timeout',$1,true)`, fmt.Sprintf("%dms", enterpriseAdminOperationTimeout.Milliseconds())); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `SELECT set_config('lock_timeout',$1,true)`, "1000ms")
	return err
}

func trustedAdminContext(ctx context.Context, enrolled bool) (context.Context, error) {
	auth, ok := service.SessionAuthenticationFromContext(ctx)
	if !ok || auth.AuthMethod == "" || auth.AuthenticatedAt.IsZero() {
		return nil, service.ErrRecentAuthenticationRequired
	}
	auth.MFAEnrolled = enrolled
	return service.WithSessionAuthentication(ctx, auth), nil
}

func readAdminReceipt(ctx context.Context, tx *sql.Tx, actorID int64, key string) (*service.AdminOperationReceipt, string, bool, error) {
	receipt := &service.AdminOperationReceipt{}
	var fingerprint string
	err := tx.QueryRowContext(ctx, `SELECT id::text,action,target_type,target_id,workspace_id,previous_status,result_status,result_updated_at,created_at,request_fingerprint FROM enterprise_admin_operations WHERE actor_user_id=$1 AND idempotency_key=$2`, actorID, key).Scan(&receipt.ID, &receipt.Action, &receipt.TargetType, &receipt.TargetID, &receipt.WorkspaceID, &receipt.PreviousStatus, &receipt.ResultStatus, &receipt.ResultUpdatedAt, &receipt.CreatedAt, &fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, workspaceError(err)
	}
	return receipt, fingerprint, true, nil
}

func replayAdminReceipt(ctx context.Context, tx *sql.Tx, receipt *service.AdminOperationReceipt, existingFingerprint, fingerprint string) (*service.AdminOperationReceipt, error) {
	if existingFingerprint != fingerprint {
		return nil, service.ErrAdminOperationIdempotencyConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, workspaceError(err)
	}
	return receipt, nil
}

func (r *workspaceRepository) adminWorkspaceStatusTx(ctx context.Context, tx *sql.Tx, actorID, workspaceID int64, action string, in service.AdminOperationInput, fingerprint string) (*service.AdminOperationReceipt, error) {
	var w service.Workspace
	err := tx.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1 FOR UPDATE`, workspaceID).Scan(&w.ID, &w.Name, &w.Slug, &w.Type, &w.Status, &w.ProjectAccessMode, &w.OwnerUserID, &w.BillingOwnerUserID, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, workspaceError(err)
	}
	if in.Confirmation != fmt.Sprintf("%s:%d", action, workspaceID) {
		return nil, service.ErrWorkspaceInvalid
	}
	if (action == "suspend" && w.Status != service.StatusActive) || (action == "resume" && w.Status != "suspended") {
		return nil, service.ErrWorkspaceConflict
	}
	if w.Status == "archived" || w.Status == "pending_deletion" || w.Status == "purging" || w.Status == "deleted" {
		return nil, service.ErrWorkspaceConflict
	}
	if in.ExpectedUpdatedAt == "" {
		return nil, service.ErrWorkspaceInvalid
	}
	expected, err := time.Parse(time.RFC3339Nano, in.ExpectedUpdatedAt)
	if err != nil || !w.UpdatedAt.Equal(expected) {
		return nil, service.ErrWorkspaceConflict
	}
	if action == "resume" {
		var valid bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL) AND EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.role='owner' AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL)`, workspaceID, w.BillingOwnerUserID).Scan(&valid); err != nil {
			return nil, workspaceError(err)
		}
		if !valid {
			return nil, service.ErrWorkspaceConflict
		}
	}
	resultStatus := "suspended"
	eventType := service.EventWorkspaceSuspended
	if action == "resume" {
		resultStatus, eventType = "active", service.EventWorkspaceResumed
	}
	var updatedAt time.Time
	// clock_timestamp() reflects lock-wait completion rather than transaction
	// start. GREATEST also guarantees a distinct microsecond token if two
	// successful transitions happen within one database clock tick.
	err = tx.QueryRowContext(ctx, `UPDATE workspaces SET status=$2,updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1 RETURNING updated_at`, workspaceID, resultStatus).Scan(&updatedAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "workspace_"+resultStatus, "workspace", workspaceID, map[string]any{"action": action, "previous_status": w.Status, "status": resultStatus, "reason": in.Reason}); err != nil {
		return nil, err
	}
	event, err := service.NewDomainEvent(eventType, workspaceID, 0, actorID, "workspace", fmt.Sprint(workspaceID), service.DomainEventData{"previous_status": w.Status, "status": resultStatus})
	if err != nil {
		return nil, err
	}
	if err = insertDomainEventTx(ctx, tx, event, ""); err != nil {
		return nil, err
	}
	return insertAdminReceipt(ctx, tx, actorID, workspaceID, action, "workspace", workspaceID, in.IdempotencyKey, fingerprint, in.Reason, w.Status, resultStatus, updatedAt)
}

func (r *workspaceRepository) adminRetryWebhookTx(ctx context.Context, tx *sql.Tx, actorID, workspaceID, webhookID, deliveryID int64, in service.AdminOperationInput, fingerprint string) (*service.AdminOperationReceipt, error) {
	var workspaceStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM workspaces WHERE id=$1`, workspaceID).Scan(&workspaceStatus); err != nil {
		return nil, workspaceError(err)
	}
	if workspaceStatus != service.StatusActive {
		return nil, service.ErrWorkspaceConflict
	}
	var endpointEnabled bool
	if err := tx.QueryRowContext(ctx, `SELECT enabled FROM workspace_webhooks WHERE workspace_id=$1 AND id=$2 FOR SHARE`, workspaceID, webhookID).Scan(&endpointEnabled); err != nil {
		return nil, workspaceError(err)
	}
	if !endpointEnabled || in.Confirmation != fmt.Sprintf("retry_webhook:%d", deliveryID) {
		return nil, service.ErrWorkspaceConflict
	}
	var (
		status                string
		attempts              int
		lastAttempt, lockedAt sql.NullTime
		lockOwner             sql.NullString
	)
	err := tx.QueryRowContext(ctx, `SELECT status,attempts,last_attempt_at,locked_at,lock_owner FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND webhook_id=$2 AND id=$3 FOR UPDATE`, workspaceID, webhookID, deliveryID).Scan(&status, &attempts, &lastAttempt, &lockedAt, &lockOwner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWebhookDeliveryNotFound
	}
	if err != nil {
		return nil, workspaceError(err)
	}
	if status != "dead" || lockedAt.Valid || lockOwner.Valid || attempts != in.ExpectedAttempts || (in.ExpectedLastAttemptAt == "" && lastAttempt.Valid) {
		return nil, service.ErrWorkspaceConflict
	}
	if in.ExpectedLastAttemptAt != "" {
		expected, parseErr := time.Parse(time.RFC3339Nano, in.ExpectedLastAttemptAt)
		if parseErr != nil || !lastAttempt.Valid || !lastAttempt.Time.Equal(expected) {
			return nil, service.ErrWorkspaceConflict
		}
	}
	var successful int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM enterprise_admin_operations WHERE workspace_id=$1 AND target_type='webhook_delivery' AND target_id=$2 AND action='retry_webhook' LIMIT 4) capped`, workspaceID, deliveryID).Scan(&successful); err != nil {
		return nil, workspaceError(err)
	}
	if successful >= 3 {
		return nil, service.ErrWorkspaceConflict
	}
	var queuedAt time.Time
	err = tx.QueryRowContext(ctx, `UPDATE workspace_webhook_deliveries SET status='retrying',attempts=0,next_attempt_at=clock_timestamp(),last_error=NULL,error_code=NULL,lock_owner=NULL,locked_at=NULL,delivered_at=NULL,finished_at=NULL WHERE workspace_id=$1 AND webhook_id=$2 AND id=$3 RETURNING next_attempt_at`, workspaceID, webhookID, deliveryID).Scan(&queuedAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "administrator_webhook_retry", "webhook_delivery", deliveryID, map[string]any{"action": "retry_webhook", "previous_status": "dead", "status": "retrying", "reason": in.Reason}); err != nil {
		return nil, err
	}
	event, err := service.NewDomainEvent(service.EventWebhookAdministratorRetried, workspaceID, 0, actorID, "webhook_delivery", fmt.Sprint(deliveryID), service.DomainEventData{"previous_status": "dead", "status": "retrying", "delivery_id": int64(deliveryID)})
	if err != nil {
		return nil, err
	}
	if err = insertDomainEventTx(ctx, tx, event, ""); err != nil {
		return nil, err
	}
	return insertAdminReceipt(ctx, tx, actorID, workspaceID, "retry_webhook", "webhook_delivery", deliveryID, in.IdempotencyKey, fingerprint, in.Reason, "dead", "retrying", queuedAt)
}

func insertAdminReceipt(ctx context.Context, tx *sql.Tx, actorID, workspaceID int64, action, targetType string, targetID int64, idempotencyKey, fingerprint, reason, previousStatus, resultStatus string, resultUpdatedAt time.Time) (*service.AdminOperationReceipt, error) {
	id := uuid.New()
	var createdAt time.Time
	err := tx.QueryRowContext(ctx, `INSERT INTO enterprise_admin_operations(id,actor_user_id,workspace_id,idempotency_key,action,target_type,target_id,request_fingerprint,reason,previous_status,result_status,result_updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING created_at`, id, actorID, workspaceID, uuid.MustParse(strings.TrimSpace(idempotencyKey)), action, targetType, targetID, fingerprint, reason, previousStatus, resultStatus, resultUpdatedAt).Scan(&createdAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "enterprise_admin_operations_actor_user_id_idempotency_key_key" {
			return nil, service.ErrAdminOperationIdempotencyConflict
		}
		return nil, workspaceError(err)
	}
	if err = tx.Commit(); err != nil {
		return nil, workspaceError(err)
	}
	return &service.AdminOperationReceipt{ID: id.String(), Action: action, TargetType: targetType, TargetID: targetID, WorkspaceID: workspaceID, PreviousStatus: previousStatus, ResultStatus: resultStatus, ResultUpdatedAt: resultUpdatedAt, CreatedAt: createdAt}, nil
}

var _ service.EnterpriseAdminRepository = (*workspaceRepository)(nil)
