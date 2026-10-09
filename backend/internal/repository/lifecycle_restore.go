package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func lifecycleRestoreChecks(ctx context.Context, tx *sql.Tx, a, w int64, ac *service.WorkspaceAccess) error {
	if err := lifecycleStrongAuthTx(ctx, tx, a, w); err != nil {
		return err
	}
	var safe bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM workspace_deletion_jobs WHERE workspace_id=$1 AND state IN ('pending','running','blocked','failed'))`, w, ac.Workspace.BillingOwnerUserID).Scan(&safe)
	if err != nil {
		return err
	}
	if !safe {
		return service.ErrWorkspaceConflict
	}
	// Admitted tasks retain frozen settlement context. Restore only reopens new
	// admission; it never reactivates credentials, grants or global identities.
	return nil
}
