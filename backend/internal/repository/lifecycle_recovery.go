package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

// At most one exhausted lease is retired per poll. A dead fifth worker never
// leaves a permanently running job. Failure is durable, visible and retryable.
func lifecycleReapExhausted(ctx context.Context, tx *sql.Tx, kind string) error {
	table, eventType := "workspace_export_jobs", service.EventWorkspaceExportFailed
	if kind == "deletion" {
		table, eventType = "workspace_deletion_jobs", service.EventWorkspaceDeletionBlocked
	}
	var id string
	var w, a int64
	err := tx.QueryRowContext(ctx, `SELECT id,workspace_id,requested_by_user_id FROM `+table+` WHERE state='running' AND attempts>=5 AND lease_expires_at<=now() ORDER BY lease_expires_at,id LIMIT 1`).Scan(&id, &w, &a)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = lockWorkspace(ctx, tx, w, false); err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM `+table+` WHERE id=$1 AND state='running' AND attempts>=5 AND lease_expires_at<=now() FOR UPDATE SKIP LOCKED`, id).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE `+table+` SET state='failed',failure_code='LEASE_RETRIES_EXHAUSTED',lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	return lifecycleEvent(ctx, tx, w, a, eventType, id, "failed:LEASE_RETRIES_EXHAUSTED")
}
func (r *workspaceRepository) RetryLifecycleDeletion(ctx context.Context, a, w int64, id string) (*service.LifecycleDeletionJob, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, service.ErrWorkspaceNotFound
	}
	tx, ac, err := r.lifecycleTx(ctx, a, w, "deletion.retry")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lifecycleStrongAuthTx(ctx, tx, a, w); err != nil {
		return nil, err
	}
	job, err := scanLifecycleDeletion(tx.QueryRowContext(ctx, `SELECT `+lifecycleDeletionColumns+` FROM workspace_deletion_jobs WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, w, id))
	if err != nil {
		return nil, err
	}
	if job.State != "failed" && job.State != "blocked" {
		return nil, service.ErrWorkspaceConflict
	}
	preflight, err := lifecyclePreflightTx(ctx, tx, ac.Workspace)
	if err != nil {
		return nil, err
	}
	if !preflight.Eligible {
		return nil, service.ErrLifecycleBlocked
	}
	job, err = scanLifecycleDeletion(tx.QueryRowContext(ctx, `UPDATE workspace_deletion_jobs SET state='pending',phase=CASE WHEN state='failed' THEN 'credentials' ELSE phase END,cursor=CASE WHEN state='failed' THEN 0 ELSE cursor END,attempts=0,failure_code='',blocking_reasons='[]',available_at=GREATEST(now(),earliest_purge_at),updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+lifecycleDeletionColumns, w, id))
	if err != nil {
		return nil, err
	}
	if err = appendWorkspaceAudit(ctx, tx, w, a, nil, "workspace.deletion.retried", "workspace", w, map[string]any{"resource_id": id}); err != nil {
		return nil, err
	}
	return job, tx.Commit()
}
