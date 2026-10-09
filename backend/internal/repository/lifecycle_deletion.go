package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func (r *workspaceRepository) LifecycleDeletionChallenge(ctx context.Context, a, w int64) (*service.LifecycleChallenge, error) {
	tx, ac, err := r.lifecycleTx(ctx, a, w, "deletion.request")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lifecycleStrongAuthTx(ctx, tx, a, w); err != nil {
		return nil, err
	}
	var recent int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT 1 FROM workspace_deletion_challenges WHERE workspace_id=$1 AND actor_user_id=$2 AND created_at>now()-interval '1 hour' LIMIT 6) b`, w, a).Scan(&recent); err != nil {
		return nil, err
	}
	if recent >= 5 {
		return nil, service.ErrLifecycleRateLimit
	}
	preflight, err := lifecyclePreflightTx(ctx, tx, ac.Workspace)
	if err != nil {
		return nil, err
	}
	token, hash, err := lifecycleToken()
	if err != nil {
		return nil, err
	}
	out := &service.LifecycleChallenge{Token: token, Preflight: preflight}
	if err = tx.QueryRowContext(ctx, `INSERT INTO workspace_deletion_challenges(token_hash,workspace_id,actor_user_id,expires_at) VALUES($1,$2,$3,now()+interval '10 minutes') RETURNING expires_at`, hash, w, a).Scan(&out.ExpiresAt); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

func (r *workspaceRepository) RequestLifecycleDeletion(ctx context.Context, a, w int64, name, token string) (*service.LifecycleDeletionJob, error) {
	hash, err := lifecycleTokenHash(token)
	if err != nil {
		return nil, err
	}
	tx, ac, err := r.lifecycleTx(ctx, a, w, "deletion.request")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if name != ac.Workspace.Name {
		return nil, service.ErrLifecycleChallenge
	}
	if err = lifecycleStrongAuthTx(ctx, tx, a, w); err != nil {
		return nil, err
	}
	// Consume and validate the challenge under the same lock/transaction as the
	// fresh preflight. A failed request rolls back its one-use authorization.
	result, err := tx.ExecContext(ctx, `UPDATE workspace_deletion_challenges SET consumed_at=now() WHERE token_hash=$1 AND workspace_id=$2 AND actor_user_id=$3 AND consumed_at IS NULL AND expires_at>now()`, hash, w, a)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrLifecycleChallenge
	}
	preflight, err := lifecyclePreflightTx(ctx, tx, ac.Workspace)
	if err != nil {
		return nil, err
	}
	if !preflight.Eligible {
		return nil, service.ErrLifecycleBlocked
	}
	id := uuid.NewString()
	job, err := scanLifecycleDeletion(tx.QueryRowContext(ctx, `INSERT INTO workspace_deletion_jobs(id,workspace_id,requested_by_user_id,previous_status,earliest_purge_at,available_at) SELECT $1,$2,$3,$4,now()+deletion_grace_days*interval '1 day',now()+deletion_grace_days*interval '1 day' FROM lifecycle_platform_settings WHERE id=1 RETURNING `+lifecycleDeletionColumns, id, w, a, ac.Workspace.Status))
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspaces SET status='pending_deletion',updated_at=now() WHERE id=$1`, w); err != nil {
		return nil, err
	}
	if err = lifecycleEvent(ctx, tx, w, a, service.EventWorkspaceDeletionRequested, id, "pending"); err != nil {
		return nil, err
	}
	return job, workspaceError(tx.Commit())
}

func (r *workspaceRepository) CancelLifecycleDeletion(ctx context.Context, a, w int64, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return service.ErrWorkspaceNotFound
	}
	tx, _, err := r.lifecycleTx(ctx, a, w, "deletion.cancel")
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lifecycleStrongAuthTx(ctx, tx, a, w); err != nil {
		return err
	}
	job, err := scanLifecycleDeletion(tx.QueryRowContext(ctx, `SELECT `+lifecycleDeletionColumns+` FROM workspace_deletion_jobs WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, w, id))
	if err != nil {
		return err
	}
	if job.State != "pending" && job.State != "blocked" && job.State != "failed" {
		return service.ErrWorkspaceConflict
	}
	if job.Progress != 0 || job.Phase != "credentials" || job.Cursor != 0 {
		return service.ErrWorkspaceConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_deletion_jobs SET state='cancelled',lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,updated_at=now(),completed_at=now() WHERE workspace_id=$1 AND id=$2`, w, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspaces SET status=$2,updated_at=now() WHERE id=$1`, w, job.PreviousStatus); err != nil {
		return err
	}
	if err = lifecycleEvent(ctx, tx, w, a, service.EventWorkspaceDeletionCancelled, id, "cancelled"); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *workspaceRepository) ClaimLifecycleDeletion(ctx context.Context) (*service.LifecycleDeletionJob, error) {
	token := uuid.NewString()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lifecycleReapExhausted(ctx, tx, "deletion"); err != nil {
		return nil, err
	}
	job, err := scanLifecycleDeletion(tx.QueryRowContext(ctx, `WITH due AS (
 SELECT id FROM workspace_deletion_jobs WHERE state IN ('pending','running','blocked') AND available_at<=now() AND earliest_purge_at<=now() AND attempts<5 AND (lease_expires_at IS NULL OR lease_expires_at<=now()) ORDER BY available_at,id LIMIT 1 FOR UPDATE SKIP LOCKED)
 UPDATE workspace_deletion_jobs j SET state='running',attempts=attempts+1,lease_owner=$2,lease_token=$1,lease_expires_at=clock_timestamp()+interval '60 seconds',updated_at=now() FROM due WHERE j.id=due.id RETURNING `+qualifiedLifecycleDeletionColumns("j"), token, lifecycleWorkerOwner()))
	if errors.Is(err, service.ErrWorkspaceNotFound) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	return job, tx.Commit()
}

func qualifiedLifecycleDeletionColumns(alias string) string {
	// RETURNING has a joined CTE; qualify the columns without accepting an SQL
	// identifier from an HTTP request. The sole caller supplies literal j.
	return alias + `.id,` + alias + `.workspace_id,` + alias + `.requested_by_user_id,` + alias + `.state,` + alias + `.previous_status,` + alias + `.phase,` + alias + `.cursor,` + alias + `.attempts,COALESCE(` + alias + `.lease_token::text,''),` + alias + `.lease_expires_at,` + alias + `.progress,` + alias + `.failure_code,` + alias + `.blocking_reasons,` + alias + `.protected_evidence_retained,` + alias + `.business_closed,` + alias + `.earliest_purge_at,` + alias + `.created_at,` + alias + `.updated_at,` + alias + `.completed_at`
}

type lifecyclePurgeStep struct{ phase, selection, mutation string }

// Each selection returns one monotonically increasing tenant-owned ID. Every
// mutation is an explicit allowlist; retained financial/audit parents are never
// deleted. Credential values are destroyed while their identifiers survive.
var lifecyclePurgeSteps = []lifecyclePurgeStep{
	{"credentials", `SELECT k.id FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1 AND k.id>$2 ORDER BY k.id LIMIT $3`,
		`UPDATE api_keys k SET name='Deleted key '||k.id,key='lifecycle-revoked-'||gen_random_uuid()::text,status='inactive',deleted_at=COALESCE(deleted_at,now()),updated_at=now() FROM batch b WHERE k.id=b.id AND k.project_id IN (SELECT id FROM projects WHERE workspace_id=$1)`},
	{"service_accounts", `SELECT id FROM service_accounts WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE service_accounts x SET name='Deleted service account '||x.id,status='disabled',description='',updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"invitations", `SELECT id FROM workspace_invitations WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_invitations x SET email='deleted-'||x.id||'@invalid.local',token_hash=sha256(gen_random_uuid()::text::bytea),revoked_at=COALESCE(revoked_at,now()) FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"access_grants", `SELECT id FROM project_access_grants WHERE workspace_id=$1 AND id>$2 AND effective_lifecycle_retention_days($1,'security')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'security')*interval '1 day' ORDER BY id LIMIT $3`,
		`DELETE FROM project_access_grants x USING batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"project_allocation_configuration", `SELECT project_id AS id FROM project_cost_allocations WHERE workspace_id=$1 AND project_id>$2 AND effective_lifecycle_retention_days($1,'operational')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'operational')*interval '1 day' ORDER BY project_id LIMIT $3`,
		`DELETE FROM project_cost_allocations x USING batch b WHERE x.workspace_id=$1 AND x.project_id=b.id`},
	{"key_allocation_configuration", `SELECT api_key_id AS id FROM api_key_allocation_overrides WHERE workspace_id=$1 AND api_key_id>$2 AND effective_lifecycle_retention_days($1,'operational')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'operational')*interval '1 day' ORDER BY api_key_id LIMIT $3`,
		`DELETE FROM api_key_allocation_overrides x USING batch b WHERE x.workspace_id=$1 AND x.api_key_id=b.id`},
	{"machine_allocation_configuration", `SELECT service_account_id AS id FROM service_account_allocation_overrides WHERE workspace_id=$1 AND service_account_id>$2 AND effective_lifecycle_retention_days($1,'operational')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'operational')*interval '1 day' ORDER BY service_account_id LIMIT $3`,
		`DELETE FROM service_account_allocation_overrides x USING batch b WHERE x.workspace_id=$1 AND x.service_account_id=b.id`},
	{"identity_credentials", `SELECT id FROM workspace_identity_providers WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_identity_providers x SET status='disabled',is_default=false,encrypted_client_secret=CASE WHEN encrypted_client_secret IS NULL THEN NULL ELSE 'destroyed' END,encrypted_saml_sp_keys=CASE WHEN encrypted_saml_sp_keys IS NULL THEN NULL ELSE 'destroyed' END,disabled_at=COALESCE(disabled_at,now()),revision=revision+1,updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"scim_credentials", `SELECT id FROM workspace_scim_tokens WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_scim_tokens x SET status='revoked',token_hash=sha256(gen_random_uuid()::text::bytea),revoked_at=COALESCE(revoked_at,now()) FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"scim_connectors", `SELECT id FROM workspace_scim_connectors WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_scim_connectors x SET status='disabled',disabled_at=COALESCE(disabled_at,now()),revision=revision+1,updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"webhook_credentials", `SELECT id FROM workspace_webhooks WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_webhooks x SET enabled=false,secret_current_encrypted='destroyed',secret_previous_encrypted=NULL,previous_secret_until=NULL,updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"member_access", `SELECT id FROM workspace_members WHERE workspace_id=$1 AND role<>'owner' AND user_id<>(SELECT billing_owner_user_id FROM workspaces WHERE id=$1) AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_members x SET status='suspended',administratively_suspended=true,administratively_removed=true,effective_membership_source_id=NULL,updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"teams", `SELECT id FROM workspace_teams WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE workspace_teams x SET name='Deleted team '||x.id,slug='deleted-team-'||x.id,status='archived',description='',updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
	{"projects", `SELECT id FROM projects WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`,
		`UPDATE projects x SET status='archived',name='Deleted project '||x.id,slug='deleted-'||x.id,description='',updated_at=now() FROM batch b WHERE x.workspace_id=$1 AND x.id=b.id`},
}

func (r *workspaceRepository) RunLifecyclePurgeBatch(ctx context.Context, claim *service.LifecycleDeletionJob, limit int) error {
	if claim == nil || claim.LeaseToken == "" || limit < 1 || limit > 200 {
		return service.ErrWorkspaceInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='20s'; SET LOCAL lock_timeout='5s'`); err != nil {
		return err
	}
	if err = lockWorkspace(ctx, tx, claim.WorkspaceID, true); err != nil {
		return err
	}
	// Workspace locks fence tenant policy and holds; these fixed policy rows
	// also fence an operator raising a platform floor during this batch.
	if _, err = tx.ExecContext(ctx, `SELECT category FROM platform_retention_policies WHERE category IN ('security','operational') ORDER BY category FOR SHARE`); err != nil {
		return err
	}
	job, err := scanLifecycleDeletion(tx.QueryRowContext(ctx, `SELECT `+lifecycleDeletionColumns+` FROM workspace_deletion_jobs WHERE workspace_id=$1 AND id=$2 AND state='running' AND lease_token=$3 AND lease_expires_at>clock_timestamp() AND earliest_purge_at<=now() FOR UPDATE`, claim.WorkspaceID, claim.ID, claim.LeaseToken))
	if err != nil {
		return service.ErrLifecycleLeaseLost.WithCause(err)
	}
	w, err := scanWorkspace(tx.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1`, job.WorkspaceID))
	if err != nil {
		return err
	}
	preflight, err := lifecyclePreflightTx(ctx, tx, w)
	if err != nil {
		return err
	}
	if !preflight.Eligible {
		reasons, e := json.Marshal(preflight.BlockingReasons)
		if e != nil {
			return e
		}
		state := "blocked"
		updated, updateErr := tx.ExecContext(ctx, `UPDATE workspace_deletion_jobs SET state=$4,attempts=0,failure_code=$5,blocking_reasons=$6,lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,available_at=now()+interval '5 minutes',updated_at=now() WHERE workspace_id=$1 AND id=$2 AND lease_token=$3 AND state='running' AND lease_expires_at>clock_timestamp()`, job.WorkspaceID, job.ID, job.LeaseToken, state, preflight.BlockingReasons[0].Code, reasons)
		if err = requireLifecycleAffected(updated, updateErr); err != nil {
			return err
		}
		if err = lifecycleEvent(ctx, tx, job.WorkspaceID, job.RequestedByUserID, service.EventWorkspaceDeletionBlocked, job.ID, state+":"+preflight.BlockingReasons[0].Code); err != nil {
			return err
		}
		return tx.Commit()
	}
	if w.Status == "pending_deletion" {
		if _, err = tx.ExecContext(ctx, `UPDATE workspaces SET status='purging',updated_at=now() WHERE id=$1`, w.ID); err != nil {
			return err
		}
	} else if w.Status != "purging" {
		return service.ErrWorkspaceConflict
	}
	stepIndex := -1
	for i, step := range lifecyclePurgeSteps {
		if step.phase == job.Phase {
			stepIndex = i
			break
		}
	}
	if stepIndex < 0 {
		return service.ErrWorkspaceInvalid
	}
	step := lifecyclePurgeSteps[stepIndex]
	var nextCursor, count int64
	query := `WITH batch AS MATERIALIZED (` + step.selection + `), changed AS (` + step.mutation + ` RETURNING 1) SELECT COALESCE((SELECT max(id) FROM batch),0),(SELECT count(*) FROM changed)`
	if err = tx.QueryRowContext(ctx, query, w.ID, job.Cursor, limit).Scan(&nextCursor, &count); err != nil {
		return err
	}
	phase, cursor := job.Phase, nextCursor
	if count == 0 {
		if stepIndex == len(lifecyclePurgeSteps)-1 {
			if err = lifecycleVerifyPurge(ctx, tx, w.ID); err != nil {
				return err
			}
			updated, updateErr := tx.ExecContext(ctx, `UPDATE workspace_deletion_jobs SET state='completed',business_closed=true,protected_evidence_retained=true,lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,failure_code='',blocking_reasons='[]',completed_at=now(),updated_at=now() WHERE workspace_id=$1 AND id=$2 AND lease_token=$3 AND state='running' AND lease_expires_at>clock_timestamp()`, w.ID, job.ID, job.LeaseToken)
			if err = requireLifecycleAffected(updated, updateErr); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE workspaces SET status='deleted',name='Deleted workspace '||id,slug='deleted-workspace-'||id,updated_at=now() WHERE id=$1`, w.ID); err != nil {
				return err
			}
			if err = lifecycleEvent(ctx, tx, w.ID, job.RequestedByUserID, service.EventWorkspaceDeletionCompleted, job.ID, "completed"); err != nil {
				return err
			}
			return tx.Commit()
		}
		phase = lifecyclePurgeSteps[stepIndex+1].phase
		cursor = 0
	}
	result, err := tx.ExecContext(ctx, `UPDATE workspace_deletion_jobs SET phase=$4,cursor=$5,progress=progress+$6,attempts=0,failure_code='',blocking_reasons='[]',lease_expires_at=clock_timestamp()+interval '60 seconds',updated_at=now() WHERE workspace_id=$1 AND id=$2 AND lease_token=$3 AND lease_expires_at>clock_timestamp()`, w.ID, job.ID, job.LeaseToken, phase, cursor, count)
	if err = requireLifecycleAffected(result, err); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *workspaceRepository) ReleaseLifecycleDeletion(ctx context.Context, claim *service.LifecycleDeletionJob, reason error) error {
	if claim == nil {
		return service.ErrWorkspaceInvalid
	}
	state, code, delay := "running", "", int64(0)
	if reason != nil {
		code = "PURGE_BATCH_FAILED"
		delay = 30
		if claim.Attempts >= 5 {
			state = "failed"
		}
	}
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_deletion_jobs SET state=$4,failure_code=$5,lease_owner=NULL,lease_token=NULL,lease_expires_at=NULL,available_at=now()+$6*interval '1 second',updated_at=now() WHERE workspace_id=$1 AND id=$2 AND lease_token=$3 AND lease_expires_at>clock_timestamp() AND state='running'`, claim.WorkspaceID, claim.ID, claim.LeaseToken, state, code, delay)
	return requireLifecycleAffected(result, err)
}
