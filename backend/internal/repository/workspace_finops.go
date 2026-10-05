package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *workspaceRepository) GetUsageSummary(ctx context.Context, scope service.FinOpsScope, start, end time.Time, timezone string) (*service.FinOpsUsageSummary, error) {
	return r.getUsageSummary(ctx, r.db, scope, start, end, timezone)
}

func (r *workspaceRepository) getUsageSummary(ctx context.Context, q workspaceSQL, scope service.FinOpsScope, start, end time.Time, timezone string) (*service.FinOpsUsageSummary, error) {
	start, end, timezone, err := finopsWindow(start, end, timezone)
	if err != nil {
		return nil, err
	}
	cte, args, err := finopsUsageCTE(scope, start, end)
	if err != nil {
		return nil, err
	}
	var out service.FinOpsUsageSummary
	out.WorkspaceID, out.ProjectID, out.Start, out.End, out.Timezone = scope.WorkspaceID, scope.ProjectID, start, end, timezone
	if err = q.QueryRowContext(ctx, cte+` SELECT COALESCE(SUM(request_count),0)::bigint,COALESCE(SUM(actual_cost),0) FROM finops_usage`, args...).Scan(&out.Requests, &out.Spend); err != nil {
		return nil, err
	}
	if scope.ProjectID == 0 {
		if err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspace_members WHERE workspace_id=$1`, scope.WorkspaceID).Scan(&out.Members); err != nil {
			return nil, err
		}
		if err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE workspace_id=$1`, scope.WorkspaceID).Scan(&out.Projects); err != nil {
			return nil, err
		}
		if err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1 AND k.deleted_at IS NULL AND k.status='active' AND (k.expires_at IS NULL OR k.expires_at>now())`, scope.WorkspaceID).Scan(&out.APIKeys); err != nil {
			return nil, err
		}
	} else if err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1 AND k.project_id=$2 AND k.deleted_at IS NULL AND k.status='active' AND (k.expires_at IS NULL OR k.expires_at>now())`, scope.WorkspaceID, scope.ProjectID).Scan(&out.APIKeys); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *workspaceRepository) GetOverview(ctx context.Context, scope service.FinOpsScope, start, end time.Time, timezone string) (*service.FinOpsOverview, error) {
	// Summary, daily spend and dimensions see the same usage/rollup snapshot.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	summary, err := r.getUsageSummary(ctx, tx, scope, start, end, timezone)
	if err != nil {
		return nil, err
	}
	out := &service.FinOpsOverview{Summary: *summary}
	dailyQuery, dailyArgs, err := finopsDailyQuery(scope, summary.Start, summary.End, summary.Timezone)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, dailyQuery, dailyArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var point service.FinOpsDailySpendPoint
		if err := rows.Scan(&point.Date, &point.Requests, &point.Spend); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.DailySpend = append(out.DailySpend, point)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	cte, args, err := finopsUsageCTE(scope, summary.Start, summary.End)
	if err != nil {
		return nil, err
	}
	load := func(query string, target *[]service.FinOpsBreakdown) error {
		rows, e := tx.QueryContext(ctx, cte+query, args...)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v service.FinOpsBreakdown
			if e = rows.Scan(&v.ID, &v.Name, &v.Requests, &v.Spend); e != nil {
				return e
			}
			*target = append(*target, v)
		}
		return rows.Err()
	}
	if err = load(` SELECT u.project_id::text,COALESCE(p.name,''),SUM(u.request_count)::bigint,SUM(u.actual_cost) FROM finops_usage u LEFT JOIN projects p ON p.id=u.project_id AND p.workspace_id=$1 GROUP BY 1,2 ORDER BY 4 DESC LIMIT 100`, &out.Projects); err != nil {
		return nil, err
	}
	if err = load(` SELECT COALESCE(resolved_platform,''),COALESCE(resolved_platform,''),SUM(request_count)::bigint,SUM(actual_cost) FROM finops_usage GROUP BY 1 ORDER BY 4 DESC LIMIT 100`, &out.Platforms); err != nil {
		return nil, err
	}
	if err = load(` SELECT model,model,SUM(request_count)::bigint,SUM(actual_cost) FROM finops_usage GROUP BY 1 ORDER BY 4 DESC LIMIT 100`, &out.Models); err != nil {
		return nil, err
	}
	if err = load(` SELECT api_key_id::text,api_key_id::text,SUM(request_count)::bigint,SUM(actual_cost) FROM finops_usage GROUP BY 1 ORDER BY 4 DESC LIMIT 100`, &out.APIKeys); err != nil {
		return nil, err
	}
	out.ServiceAccounts, err = loadServiceAccountFinOps(ctx, tx, scope, summary.Start, summary.End, *summary)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

func (r *workspaceRepository) GetBudget(ctx context.Context, scope service.FinOpsScope) (*service.BudgetView, error) {
	if scope.WorkspaceID <= 0 {
		return nil, service.ErrWorkspaceNotFound
	}
	table, key := "workspace_budget_policies", scope.WorkspaceID
	if scope.ProjectID > 0 {
		var projectID int64
		if err := r.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE workspace_id=$1 AND id=$2`, scope.WorkspaceID, scope.ProjectID).Scan(&projectID); err != nil {
			return nil, workspaceError(err)
		}
		table, key = "project_budget_policies", scope.ProjectID
	}
	var policy service.BudgetPolicy
	policy.WorkspaceID, policy.ProjectID = scope.WorkspaceID, scope.ProjectID
	var updated time.Time
	if err := r.db.QueryRowContext(ctx, `SELECT amount,hard_limit,enabled,timezone,updated_at FROM `+table+` WHERE `+map[bool]string{true: "project_id", false: "workspace_id"}[scope.ProjectID > 0]+`=$1`, key).Scan(&policy.Amount, &policy.HardLimit, &policy.Enabled, &policy.Timezone, &updated); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	now := time.Now().UTC()
	if policy.Timezone == "" {
		policy.Timezone = "UTC"
	}
	loc, err := time.LoadLocation(policy.Timezone)
	if err != nil {
		policy.Timezone = "UTC"
		loc = time.UTC
	}
	local := now.In(loc)
	monthLabel := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	var spent, reserved float64
	if err = r.db.QueryRowContext(ctx, `SELECT spent,reserved FROM budget_counters WHERE scope_type=$1 AND scope_id=$2 AND period_start=$3`, map[bool]string{true: "project", false: "workspace"}[scope.ProjectID > 0], key, monthLabel).Scan(&spent, &reserved); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return &service.BudgetView{WorkspaceID: scope.WorkspaceID, ProjectID: scope.ProjectID, Policy: policy, PeriodStart: start.UTC(), PeriodEnd: end.UTC(), Spent: spent, Reserved: reserved, Remaining: policy.Amount - spent - reserved, OverBudget: policy.Enabled && spent > policy.Amount}, nil
}

func (r *workspaceRepository) SetBudget(ctx context.Context, actorID int64, scope service.FinOpsScope, in service.BudgetPolicyInput) (*service.BudgetView, error) {
	if scope.WorkspaceID <= 0 || in.Amount < 0 {
		return nil, service.ErrWorkspaceInvalid
	}
	if strings.TrimSpace(in.Timezone) == "" {
		in.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return nil, service.ErrWorkspaceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockWorkspace(ctx, tx, scope.WorkspaceID, true); err != nil {
		return nil, err
	}
	access, err := workspaceAccess(ctx, tx, actorID, scope.WorkspaceID, scope.ProjectID)
	if err != nil {
		return nil, err
	}
	if err = service.CheckWorkspacePermission(access, "budget.update"); err != nil {
		return nil, err
	}
	if scope.ProjectID > 0 {
		var id int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, scope.ProjectID, scope.WorkspaceID).Scan(&id); err != nil {
			return nil, err
		}
	}
	var revision int64
	if scope.ProjectID > 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO project_budget_policies(project_id,amount,hard_limit,enabled,timezone,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(project_id) DO UPDATE SET amount=EXCLUDED.amount,hard_limit=EXCLUDED.hard_limit,enabled=EXCLUDED.enabled,timezone=EXCLUDED.timezone,policy_revision=project_budget_policies.policy_revision+1,updated_at=now() RETURNING policy_revision`, scope.ProjectID, in.Amount, in.HardLimit, in.Enabled, in.Timezone).Scan(&revision)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO workspace_budget_policies(workspace_id,amount,hard_limit,enabled,timezone,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(workspace_id) DO UPDATE SET amount=EXCLUDED.amount,hard_limit=EXCLUDED.hard_limit,enabled=EXCLUDED.enabled,timezone=EXCLUDED.timezone,policy_revision=workspace_budget_policies.policy_revision+1,updated_at=now() RETURNING policy_revision`, scope.WorkspaceID, in.Amount, in.HardLimit, in.Enabled, in.Timezone).Scan(&revision)
	}
	if err != nil {
		return nil, err
	}
	scopeType := "workspace"
	if scope.ProjectID > 0 {
		scopeType = "project"
	}
	if in.Enabled && in.Amount > 0 {
		period, periodErr := budgetMonthStart(time.Now(), in.Timezone)
		if periodErr != nil {
			return nil, periodErr
		}
		// Editing a policy creates a new revision. Mark already-crossed actual
		// spend without replaying old consumption as new alerts.
		_, err = tx.ExecContext(ctx, `INSERT INTO budget_alert_transitions(scope_type,scope_id,period_start,policy_revision,threshold)
		 SELECT c.scope_type,c.scope_id,c.period_start,$4,t.threshold FROM budget_counters c CROSS JOIN (VALUES(50),(80),(100)) t(threshold)
		 WHERE c.scope_type=$1 AND c.scope_id=$2 AND c.period_start=$3 AND c.spent*100 >= $5::numeric*t.threshold
		 ON CONFLICT DO NOTHING`, scopeType, keyID(scope), period, revision, in.Amount)
		if err != nil {
			return nil, err
		}
	}
	if err = appendWorkspaceAudit(ctx, tx, scope.WorkspaceID, actorID, func() *int64 {
		if scope.ProjectID > 0 {
			p := scope.ProjectID
			return &p
		}
		return nil
	}(), "budget_updated", "budget", keyID(scope), map[string]any{"amount": in.Amount, "hard_limit": in.HardLimit, "enabled": in.Enabled, "timezone": in.Timezone}); err != nil {
		return nil, err
	}
	var projectID int64
	if scope.ProjectID > 0 {
		projectID = scope.ProjectID
	}
	if err = insertWorkspaceMutationEvent(ctx, tx, scope.WorkspaceID, projectID, actorID, "budget_updated", "budget", keyID(scope), service.DomainEventData{"amount": in.Amount, "status": "updated", "scope_type": scopeType, "scope_id": keyID(scope), "policy_revision": revision}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBudget(ctx, scope)
}

func keyID(scope service.FinOpsScope) int64 {
	if scope.ProjectID > 0 {
		return scope.ProjectID
	}
	return scope.WorkspaceID
}
