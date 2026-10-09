package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func (r *workspaceRepository) CreateLifecycleExport(ctx context.Context, a, w int64) (*service.LifecycleExportJob, error) {
	tx, _, err := r.lifecycleTx(ctx, a, w, "export.create")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var recent int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM workspace_export_jobs WHERE workspace_id=$1 AND created_at>now()-interval '1 hour' LIMIT 6) b`, w).Scan(&recent); err != nil {
		return nil, err
	}
	if recent >= 5 {
		return nil, service.ErrLifecycleRateLimit
	}
	job, err := scanLifecycleExport(tx.QueryRowContext(ctx, `INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id) VALUES($1,$2,$3) RETURNING `+lifecycleExportColumns, uuid.NewString(), w, a))
	if err != nil {
		return nil, err
	}
	if err = lifecycleEvent(ctx, tx, w, a, service.EventWorkspaceExportRequested, job.ID, "pending"); err != nil {
		return nil, err
	}
	return job, workspaceError(tx.Commit())
}

func (r *workspaceRepository) ListLifecycleExports(ctx context.Context, a, w int64, page, size int) ([]service.LifecycleExportJob, int64, error) {
	if page < 1 || page > 10000 || size < 1 || size > 100 {
		return nil, 0, service.ErrWorkspaceInvalid
	}
	tx, _, err := r.lifecycleTx(ctx, a, w, "export.read")
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var total int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_export_jobs WHERE workspace_id=$1`, w).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+lifecycleExportColumns+` FROM workspace_export_jobs WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, w, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []service.LifecycleExportJob{}
	for rows.Next() {
		item, e := scanLifecycleExport(rows)
		if e != nil {
			return nil, 0, e
		}
		items = append(items, *item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	if err = rows.Close(); err != nil {
		return nil, 0, err
	}
	return items, total, tx.Commit()
}

func (r *workspaceRepository) GetLifecycleExport(ctx context.Context, a, w int64, id string) (*service.LifecycleExportJob, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, service.ErrWorkspaceNotFound
	}
	tx, _, err := r.lifecycleTx(ctx, a, w, "export.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanLifecycleExport(tx.QueryRowContext(ctx, `SELECT `+lifecycleExportColumns+` FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2`, w, id))
	if err != nil {
		return nil, err
	}
	return job, tx.Commit()
}

func (r *workspaceRepository) CancelLifecycleExport(ctx context.Context, a, w int64, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return service.ErrWorkspaceNotFound
	}
	tx, _, err := r.lifecycleTx(ctx, a, w, "export.cancel")
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanLifecycleExport(tx.QueryRowContext(ctx, `SELECT `+lifecycleExportColumns+` FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, w, id))
	if err != nil {
		return err
	}
	if job.State == "cancelled" {
		return tx.Commit()
	}
	if job.State != "pending" && job.State != "running" {
		return service.ErrWorkspaceConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_export_jobs SET state='cancelled',lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2`, w, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_export_objects SET cleanup_after=GREATEST(cleanup_after,now()+interval '10 minutes') WHERE workspace_id=$1 AND export_id=$2`, w, id); err != nil {
		return err
	}
	if err = lifecycleEvent(ctx, tx, w, a, service.EventWorkspaceExportCancelled, id, "cancelled"); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *workspaceRepository) AuthorizeLifecycleDownload(ctx context.Context, a, w int64, id string) (*service.LifecycleDownloadGrant, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, service.ErrWorkspaceNotFound
	}
	tx, _, err := r.lifecycleTx(ctx, a, w, "export.download")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var recent int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM workspace_export_download_grants WHERE workspace_id=$1 AND actor_user_id=$2 AND created_at>now()-interval '1 minute' LIMIT 21) bounded`, w, a).Scan(&recent); err != nil {
		return nil, err
	}
	if recent >= 20 {
		return nil, service.ErrLifecycleRateLimit
	}
	var valid bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2 AND state='completed' AND (expires_at IS NULL OR expires_at>now()))`, w, id).Scan(&valid); err != nil {
		return nil, err
	}
	if !valid {
		return nil, service.ErrWorkspaceNotFound
	}
	token, hash, err := lifecycleToken()
	if err != nil {
		return nil, err
	}
	grant := &service.LifecycleDownloadGrant{Token: token}
	if err = tx.QueryRowContext(ctx, `INSERT INTO workspace_export_download_grants(token_hash,workspace_id,export_id,actor_user_id,expires_at) VALUES($1,$2,$3,$4,now()+interval '60 seconds') RETURNING expires_at`, hash, w, id, a).Scan(&grant.ExpiresAt); err != nil {
		return nil, err
	}
	return grant, tx.Commit()
}

func (r *workspaceRepository) RedeemLifecycleDownload(ctx context.Context, a, w int64, id, token string) (*service.LifecycleExportJob, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, service.ErrWorkspaceNotFound
	}
	hash, err := lifecycleTokenHash(token)
	if err != nil {
		return nil, service.ErrWorkspaceNotFound
	}
	tx, _, err := r.lifecycleTx(ctx, a, w, "export.download")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE workspace_export_download_grants SET consumed_at=now() WHERE token_hash=$1 AND workspace_id=$2 AND export_id=$3 AND actor_user_id=$4 AND consumed_at IS NULL AND expires_at>now()`, hash, w, id, a)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, service.ErrWorkspaceNotFound
	}
	job, err := scanLifecycleExport(tx.QueryRowContext(ctx, `SELECT `+lifecycleExportColumns+` FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2 AND state='completed' AND (expires_at IS NULL OR expires_at>now()) FOR SHARE`, w, id))
	if err != nil {
		return nil, err
	}
	if !service.ValidLifecycleObjectKey(job.ObjectKey, w, id) {
		return nil, service.ErrWorkspaceConflict
	}
	if err = appendWorkspaceAudit(ctx, tx, w, a, nil, "workspace.export.downloaded", "workspace", w, map[string]any{"resource_id": id}); err != nil {
		return nil, err
	}
	return job, tx.Commit()
}

func (r *workspaceRepository) ClaimLifecycleExport(ctx context.Context) (*service.LifecycleExportJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := r.claimLifecycleExportTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	return job, tx.Commit()
}

func (r *workspaceRepository) claimLifecycleExportTx(ctx context.Context, tx *sql.Tx) (*service.LifecycleExportJob, error) {
	var err error
	if err = lifecycleReapExhausted(ctx, tx, "export"); err != nil {
		return nil, err
	}
	job, err := scanLifecycleExport(tx.QueryRowContext(ctx, `SELECT `+lifecycleExportColumns+` FROM workspace_export_jobs WHERE state IN ('pending','running') AND available_at<=now() AND attempts<5 AND (lease_expires_at IS NULL OR lease_expires_at<=now()) ORDER BY available_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`))
	if errors.Is(err, service.ErrWorkspaceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	token := uuid.NewString()
	key, err := service.LifecycleObjectKey(job.WorkspaceID, job.ID, token)
	if err != nil {
		return nil, err
	}
	job, err = scanLifecycleExport(tx.QueryRowContext(ctx, `UPDATE workspace_export_jobs SET state='running',attempts=attempts+1,progress=0,object_key=$2,lease_owner=$3,lease_token=$4,lease_expires_at=clock_timestamp()+interval '5 minutes',updated_at=now() WHERE id=$1 RETURNING `+lifecycleExportColumns, job.ID, key, lifecycleWorkerOwner(), token))
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_export_objects(object_key,workspace_id,export_id,lease_token,cleanup_after) VALUES($1,$2,$3,$4,now()+interval '1 day')`, key, job.WorkspaceID, job.ID, token); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *workspaceRepository) FinishLifecycleExport(ctx context.Context, claim *service.LifecycleExportJob, result *service.LifecycleExportResult, failure error) error {
	if claim == nil {
		return service.ErrWorkspaceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Match actor mutations' Workspace -> job order before inserting evidence
	// whose foreign keys reference Workspace. Cancellation cannot deadlock with
	// a completion that already holds the job row.
	if err = lockWorkspace(ctx, tx, claim.WorkspaceID, false); err != nil {
		return err
	}
	job, err := scanLifecycleExport(tx.QueryRowContext(ctx, `SELECT `+lifecycleExportColumns+` FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2 AND state='running' AND lease_token=$3 AND lease_expires_at>clock_timestamp() FOR UPDATE`, claim.WorkspaceID, claim.ID, claim.LeaseToken))
	if err != nil {
		return service.ErrLifecycleLeaseLost.WithCause(err)
	}
	if failure != nil {
		code, state := "EXPORT_BUILD_FAILED", "pending"
		if errors.Is(failure, service.ErrLifecycleExportLimit) {
			code = "EXPORT_LIMIT_EXCEEDED"
			state = "failed"
		}
		if job.Attempts >= 5 {
			state = "failed"
		}
		updated, updateErr := tx.ExecContext(ctx, `UPDATE workspace_export_jobs SET state=$4,failure_code=$5,lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,available_at=now()+attempts*interval '30 seconds',updated_at=now() WHERE workspace_id=$1 AND id=$2 AND lease_token=$3 AND state='running' AND lease_expires_at>clock_timestamp()`, job.WorkspaceID, job.ID, job.LeaseToken, state, code)
		if err = requireLifecycleAffected(updated, updateErr); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE workspace_export_objects SET cleanup_after=now() WHERE object_key=$1`, job.ObjectKey); err != nil {
			return err
		}
		if state == "failed" {
			if err = lifecycleEvent(ctx, tx, job.WorkspaceID, job.RequestedByUserID, service.EventWorkspaceExportFailed, job.ID, "failed"); err != nil {
				return err
			}
		}
	} else {
		if result == nil || result.DataCutoff.IsZero() || len(result.SHA256) != 64 || result.SizeBytes < 1 || result.SizeBytes > service.LifecycleMaxArtifactBytes {
			return service.ErrWorkspaceInvalid
		}
		updated, updateErr := tx.ExecContext(ctx, `UPDATE workspace_export_jobs SET state='completed',progress=$4,data_cutoff=$5,artifact_sha256=$6,size_bytes=$7,completed_at=now(),expires_at=now()+NULLIF(effective_lifecycle_retention_days(workspace_id,'temporary'),0)*interval '1 day',lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,failure_code='',updated_at=now() WHERE workspace_id=$1 AND id=$2 AND lease_token=$3 AND state='running' AND lease_expires_at>clock_timestamp()`, job.WorkspaceID, job.ID, job.LeaseToken, result.Records, result.DataCutoff, result.SHA256, result.SizeBytes)
		if err = requireLifecycleAffected(updated, updateErr); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE workspace_export_objects SET cleanup_after=(SELECT expires_at FROM workspace_export_jobs WHERE id=$2) WHERE object_key=$1`, job.ObjectKey, job.ID); err != nil {
			return err
		}
		if err = lifecycleEvent(ctx, tx, job.WorkspaceID, job.RequestedByUserID, service.EventWorkspaceExportCompleted, job.ID, "completed"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *workspaceRepository) CleanupLifecycleExports(ctx context.Context, remove func(context.Context, string) error) error {
	if remove == nil {
		return service.ErrLifecycleStorage
	}
	// One locked ledger row per transaction bounds object-store backpressure.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Fence platform/tenant policy and operator hold changes during object IO.
	// No Workspace lock is acquired here: tenant writers already take Workspace
	// before these tables. Job -> ledger matches cancellation and worker claims.
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='20s'; SET LOCAL lock_timeout='1s'; LOCK TABLE platform_retention_policies,workspace_retention_policies,workspace_lifecycle_holds IN SHARE MODE`); err != nil {
		return err
	}
	if err = lifecycleCleanupArtifactTx(ctx, tx, remove); err != nil {
		return err
	}
	for _, query := range []string{`WITH old AS (SELECT token_hash FROM workspace_export_download_grants WHERE expires_at<=now() ORDER BY expires_at,token_hash LIMIT 200 FOR UPDATE SKIP LOCKED) DELETE FROM workspace_export_download_grants g USING old WHERE g.token_hash=old.token_hash`, `WITH old AS (SELECT token_hash FROM workspace_deletion_challenges WHERE expires_at<=now() ORDER BY expires_at,token_hash LIMIT 200 FOR UPDATE SKIP LOCKED) DELETE FROM workspace_deletion_challenges g USING old WHERE g.token_hash=old.token_hash`} {
		if _, err = tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func lifecycleCleanupArtifactTx(ctx context.Context, tx *sql.Tx, remove func(context.Context, string) error) error {
	var key, id string
	var w int64
	// Inspect only the oldest indexed candidate. Retained candidates move their
	// next check forward, so indefinite scopes cannot starve finite ones.
	err := tx.QueryRowContext(ctx, `SELECT object_key,workspace_id,export_id FROM workspace_export_objects WHERE cleanup_after<=now() ORDER BY cleanup_after,object_key LIMIT 1`).Scan(&key, &w, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var jobID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2 FOR UPDATE SKIP LOCKED`, w, id).Scan(&jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT object_key FROM workspace_export_objects WHERE object_key=$1 AND cleanup_after<=now() FOR UPDATE SKIP LOCKED`, key).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !service.ValidLifecycleObjectKey(key, w, id) {
		return service.ErrWorkspaceConflict
	}
	var held, live, retained bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_lifecycle_holds WHERE workspace_id=$1 AND active),
	 COALESCE(state='running' AND lease_expires_at>clock_timestamp(),false),
	 COALESCE(state='completed' AND object_key=$3 AND (effective_lifecycle_retention_days(workspace_id,'temporary')=0
	 OR completed_at+effective_lifecycle_retention_days(workspace_id,'temporary')*interval '1 day'>clock_timestamp()),false)
	 FROM workspace_export_jobs WHERE workspace_id=$1 AND id=$2`, w, id, key).Scan(&held, &live, &retained)
	if err != nil {
		return err
	}
	if held || live || retained {
		_, err = tx.ExecContext(ctx, `UPDATE workspace_export_objects SET cleanup_after=now()+interval '5 minutes' WHERE object_key=$1`, key)
		return err
	}
	removeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = remove(removeCtx, key)
	cancel()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workspace_export_objects WHERE object_key=$1`, key); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE workspace_export_jobs SET state=CASE WHEN state='completed' THEN 'expired' ELSE state END,object_key=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND object_key=$3`, w, id, key)
	return err
}
