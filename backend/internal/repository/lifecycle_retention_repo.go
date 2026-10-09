package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func lifecycleRetentionPolicies(ctx context.Context, tx *sql.Tx, workspace int64) ([]service.LifecycleRetentionPolicy, error) {
	rows, err := tx.QueryContext(ctx, `SELECT category,effective_lifecycle_retention_days($1,category),minimum_days,(protected OR effective_lifecycle_retention_days($1,category)=0) FROM platform_retention_policies ORDER BY category`, workspace)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	policies := []service.LifecycleRetentionPolicy{}
	for rows.Next() {
		var p service.LifecycleRetentionPolicy
		if err = rows.Scan(&p.Category, &p.RetentionDays, &p.MinimumDays, &p.Protected); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (r *workspaceRepository) LifecycleRetention(ctx context.Context, actor, workspace int64) ([]service.LifecycleRetentionPolicy, error) {
	tx, _, err := r.lifecycleTx(ctx, actor, workspace, "lifecycle.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	return lifecycleRetentionPolicies(ctx, tx, workspace)
}

func (r *workspaceRepository) UpdateLifecycleRetention(ctx context.Context, actor, workspace int64, category string, days int) ([]service.LifecycleRetentionPolicy, error) {
	if !service.ValidLifecycleRetentionCategory(category) || days < 0 || days > 36500 {
		return nil, service.ErrLifecycleRetentionInvalid
	}
	tx, _, err := r.lifecycleTx(ctx, actor, workspace, "lifecycle.manage")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO workspace_retention_policies(workspace_id,category,retention_days,updated_by_user_id) VALUES($1,$2,$3,$4) ON CONFLICT(workspace_id,category) DO UPDATE SET retention_days=EXCLUDED.retention_days,updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=now()`, workspace, category, days, actor)
	if err != nil {
		var pg *pq.Error
		if errors.As(err, &pg) && pg.Constraint == "workspace_retention_floor" {
			return nil, service.ErrLifecycleRetentionInvalid
		}
		return nil, workspaceError(err)
	}
	if err = appendWorkspaceAudit(ctx, tx, workspace, actor, nil, "retention_updated", "workspace", workspace, map[string]any{"category": category, "retention_days": days}); err != nil {
		return nil, err
	}
	if err = insertWorkspaceMutationEvent(ctx, tx, workspace, 0, actor, "retention_updated", "workspace", workspace, service.DomainEventData{"category": category, "retention_days": days}); err != nil {
		return nil, err
	}
	policies, err := lifecycleRetentionPolicies(ctx, tx, workspace)
	if err != nil {
		return nil, err
	}
	return policies, workspaceError(tx.Commit())
}
