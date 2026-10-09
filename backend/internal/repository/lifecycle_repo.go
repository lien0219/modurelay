package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// User locks precede Workspace locks, matching admission/global lifecycle.
// Repository authorization repeats the service boundary for every tenant ID.
func (r *workspaceRepository) lifecycleTx(ctx context.Context, a, w int64, permission string) (*sql.Tx, *service.WorkspaceAccess, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	fail := func(e error) (*sql.Tx, *service.WorkspaceAccess, error) { _ = tx.Rollback(); return nil, nil, e }
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='20s'; SET LOCAL lock_timeout='5s'`); err != nil {
		return fail(err)
	}
	var enrolled bool
	if err = tx.QueryRowContext(ctx, `SELECT totp_enabled FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR SHARE`, a).Scan(&enrolled); err != nil {
		return fail(workspaceError(err))
	}
	if err = lockWorkspace(ctx, tx, w, true); err != nil {
		return fail(err)
	}
	ac, err := workspaceAccess(ctx, tx, a, w, 0)
	if err != nil {
		return fail(err)
	}
	if err = service.CheckWorkspacePermission(ac, permission); err != nil {
		return fail(err)
	}
	if ac.Workspace.Type == "organization" {
		if err = evaluateWorkspaceSecurityTx(ctx, tx, w, a); err != nil {
			return fail(err)
		}
	}
	return tx, ac, nil
}

func lifecycleStrongAuthTx(ctx context.Context, tx *sql.Tx, a, w int64) error {
	var enrolled bool
	if err := tx.QueryRowContext(ctx, `SELECT totp_enabled FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, a).Scan(&enrolled); err != nil {
		return workspaceError(err)
	}
	auth, _ := service.SessionAuthenticationFromContext(ctx)
	auth.MFAEnrolled = enrolled
	ctx = service.WithSessionAuthentication(ctx, auth)
	if err := service.RequireLifecycleStrongAuthentication(ctx, time.Now().UTC()); err != nil {
		return err
	}
	return evaluateWorkspaceSecurityTx(ctx, tx, w, a)
}

func lifecycleToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func lifecycleTokenHash(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, service.ErrLifecycleChallenge
	}
	hash := sha256.Sum256([]byte(token))
	return hash[:], nil
}

func lifecycleEvent(ctx context.Context, q workspaceSQL, w, a int64, eventType, jobID, state string) error {
	event, err := service.NewDomainEvent(eventType, w, 0, a, "lifecycle_job", jobID, service.DomainEventData{"resource_id": jobID, "status": state, "category": "lifecycle"})
	if err != nil {
		return err
	}
	if err = appendWorkspaceAudit(ctx, q, w, a, nil, eventType, "workspace", w, map[string]any{"resource_id": jobID, "status": state}); err != nil {
		return err
	}
	return insertDomainEventTx(ctx, q, event, "lifecycle:"+jobID+":"+eventType+":"+state)
}

const lifecycleExportColumns = `id,workspace_id,requested_by_user_id,state,attempts,progress,COALESCE(object_key,''),COALESCE(lease_token::text,''),lease_expires_at,data_cutoff,artifact_sha256,size_bytes,failure_code,created_at,updated_at,completed_at,expires_at`

func scanLifecycleExport(row workspaceScanner) (*service.LifecycleExportJob, error) {
	item := &service.LifecycleExportJob{}
	err := row.Scan(&item.ID, &item.WorkspaceID, &item.RequestedByUserID, &item.State, &item.Attempts, &item.Progress, &item.ObjectKey, &item.LeaseToken, &item.LeaseExpiresAt, &item.DataCutoff, &item.ArtifactSHA256, &item.SizeBytes, &item.FailureCode, &item.CreatedAt, &item.UpdatedAt, &item.CompletedAt, &item.ExpiresAt)
	return item, workspaceError(err)
}

const lifecycleDeletionColumns = `id,workspace_id,requested_by_user_id,state,previous_status,phase,cursor,attempts,COALESCE(lease_token::text,''),lease_expires_at,progress,failure_code,blocking_reasons,protected_evidence_retained,business_closed,earliest_purge_at,created_at,updated_at,completed_at`

func scanLifecycleDeletion(row workspaceScanner) (*service.LifecycleDeletionJob, error) {
	item := &service.LifecycleDeletionJob{}
	var blockers []byte
	err := row.Scan(&item.ID, &item.WorkspaceID, &item.RequestedByUserID, &item.State, &item.PreviousStatus, &item.Phase, &item.Cursor, &item.Attempts, &item.LeaseToken, &item.LeaseExpiresAt, &item.Progress, &item.FailureCode, &blockers, &item.ProtectedEvidenceRetained, &item.BusinessClosed, &item.EarliestPurgeAt, &item.CreatedAt, &item.UpdatedAt, &item.CompletedAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = json.Unmarshal(blockers, &item.BlockingReasons); err != nil {
		return nil, err
	}
	return item, nil
}

func requireLifecycleAffected(result sql.Result, err error) error {
	if err != nil {
		return workspaceError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrLifecycleLeaseLost
	}
	return nil
}

func (r *workspaceRepository) GetLifecycleDeletion(ctx context.Context, a, w int64) (*service.LifecycleDeletionJob, error) {
	tx, _, err := r.lifecycleTx(ctx, a, w, "lifecycle.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := scanLifecycleDeletion(tx.QueryRowContext(ctx, `SELECT `+lifecycleDeletionColumns+` FROM workspace_deletion_jobs WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, w))
	if errors.Is(err, service.ErrWorkspaceNotFound) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	return item, tx.Commit()
}

func lifecycleWorkerOwner() string { return "modurelay-lifecycle" }
