package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *enterpriseSCIMRepository) RecordSyncOutcome(ctx context.Context, p *service.SCIMPrincipal, success bool, code string) error {
	switch code {
	case "", "invalidValue", "invalidPath", "uniqueness", "mutability", "invalidFilter", "tooLarge", "not_found", "precondition", "sync_failed", "security_conflict":
	default:
		code = "sync_failed"
	}
	tx, e := r.beginSync(ctx, p, true)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	if success {
		_, e = tx.ExecContext(ctx, `UPDATE workspace_scim_connectors SET last_sync_at=clock_timestamp(),last_error_code=NULL,failure_count=0 WHERE workspace_id=$1 AND id=$2 AND (last_sync_at IS NULL OR last_sync_at<clock_timestamp()-interval '1 minute' OR failure_count<>0)`, p.WorkspaceID, p.ConnectorID)
		if e != nil {
			return e
		}
	} else {
		var count int
		var notify, conflict bool
		e = tx.QueryRowContext(ctx, `UPDATE workspace_scim_connectors SET failure_count=LEAST(failure_count+1,1000000),last_error_code=$3 WHERE workspace_id=$1 AND id=$2 RETURNING failure_count,failure_count>=3 AND (failure_notified_at IS NULL OR failure_notified_at<clock_timestamp()-interval '15 minutes'),conflict_notified_at IS NULL OR conflict_notified_at<clock_timestamp()-interval '15 minutes'`, p.WorkspaceID, p.ConnectorID, code).Scan(&count, &notify, &conflict)
		if e != nil {
			return e
		}
		if notify {
			if _, e = tx.ExecContext(ctx, `UPDATE workspace_scim_connectors SET failure_notified_at=clock_timestamp() WHERE workspace_id=$1 AND id=$2`, p.WorkspaceID, p.ConnectorID); e != nil {
				return e
			}
			if e = appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.sync_failed", service.EventSCIMSyncFailed, "scim_connector", p.ConnectorID, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "reason_code": code, "failure_count": count}); e != nil {
				return e
			}
		}
		if (code == "mutability" || code == "security_conflict") && conflict {
			if _, e = tx.ExecContext(ctx, `UPDATE workspace_scim_connectors SET conflict_notified_at=clock_timestamp() WHERE workspace_id=$1 AND id=$2`, p.WorkspaceID, p.ConnectorID); e != nil {
				return e
			}
			if e = appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.security_conflict", service.EventSCIMSecurityConflict, "scim_connector", p.ConnectorID, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "reason_code": code}); e != nil {
				return e
			}
		}
	}
	var expiry bool
	e = tx.QueryRowContext(ctx, `SELECT expires_at IS NOT NULL AND expires_at<=clock_timestamp()+interval '7 days' AND (expiry_notified_at IS NULL OR expiry_notified_at<clock_timestamp()-interval '1 day') FROM workspace_scim_tokens WHERE workspace_id=$1 AND connector_id=$2 AND id=$3`, p.WorkspaceID, p.ConnectorID, p.TokenID).Scan(&expiry)
	if e != nil {
		return e
	}
	if expiry {
		if _, e = tx.ExecContext(ctx, `UPDATE workspace_scim_tokens SET expiry_notified_at=clock_timestamp() WHERE id=$1`, p.TokenID); e != nil {
			return e
		}
		if e = appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.token_expiring", service.EventSCIMTokenExpiring, "scim_token", p.TokenID, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "token_id": p.TokenID}); e != nil {
			return e
		}
	}
	return tx.Commit()
}

// NotifyExpiringTokens is bounded and rechecks state after acquiring the same
// Workspace lock as revocation and provisioning. Dormant credentials can alert.
func (r *enterpriseSCIMRepository) NotifyExpiringTokens(ctx context.Context, batchSize int) error {
	if batchSize < 1 || batchSize > 100 {
		batchSize = 100
	}
	rows, e := r.db.QueryContext(ctx, `SELECT t.workspace_id,t.connector_id,t.id FROM workspace_scim_tokens t JOIN workspace_scim_connectors c ON c.workspace_id=t.workspace_id AND c.id=t.connector_id JOIN workspaces w ON w.id=t.workspace_id WHERE w.type='organization' AND w.status='active' AND c.status='active' AND t.status='active' AND t.expires_at>clock_timestamp() AND t.expires_at<=clock_timestamp()+interval '7 days' AND (t.expiry_notified_at IS NULL OR t.expiry_notified_at<clock_timestamp()-interval '1 day') ORDER BY t.expires_at,t.id LIMIT $1`, batchSize)
	if e != nil {
		return e
	}
	type target struct{ w, c, t int64 }
	targets := []target{}
	for rows.Next() {
		var v target
		if e = rows.Scan(&v.w, &v.c, &v.t); e != nil {
			_ = rows.Close()
			return e
		}
		targets = append(targets, v)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return e
	}
	for _, v := range targets {
		tx, e := r.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		if e = lockWorkspace(ctx, tx, v.w, true); e != nil {
			_ = tx.Rollback()
			return e
		}
		result, e := tx.ExecContext(ctx, `UPDATE workspace_scim_tokens t SET expiry_notified_at=clock_timestamp() FROM workspace_scim_connectors c,workspaces w WHERE t.workspace_id=$1 AND t.connector_id=$2 AND t.id=$3 AND c.workspace_id=t.workspace_id AND c.id=t.connector_id AND w.id=t.workspace_id AND w.type='organization' AND w.status='active' AND c.status='active' AND t.status='active' AND t.expires_at>clock_timestamp() AND t.expires_at<=clock_timestamp()+interval '7 days' AND (t.expiry_notified_at IS NULL OR t.expiry_notified_at<clock_timestamp()-interval '1 day')`, v.w, v.c, v.t)
		if e != nil {
			_ = tx.Rollback()
			return e
		}
		n, e := result.RowsAffected()
		if e == nil && n > 0 {
			e = appendSCIMMutation(ctx, tx, v.w, 0, "scim.token_expiring", service.EventSCIMTokenExpiring, "scim_token", v.t, service.DomainEventData{"category": "scim", "connector_id": v.c, "token_id": v.t})
		}
		if e != nil {
			_ = tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return nil
}
