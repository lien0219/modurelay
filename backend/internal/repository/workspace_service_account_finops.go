package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"time"
)

func (r *workspaceRepository) ValidateServiceAccountUsageScope(ctx context.Context, actor, workspace, project, machine int64) error {
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT s.id FROM service_accounts s JOIN projects p ON p.id=s.project_id AND p.workspace_id=s.workspace_id JOIN workspace_members m ON m.workspace_id=p.workspace_id JOIN users u ON u.id=m.user_id WHERE s.workspace_id=$1 AND ($2::bigint=0 OR s.project_id=$2) AND s.id=$3 AND m.user_id=$4 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL`, workspace, project, machine, actor).Scan(&id)
	return workspaceError(err)
}

func serviceAccountFinOpsWhere(scope service.FinOpsScope, alias string) (string, []any, error) {
	where, args, err := finopsScopeWhere(scope, alias)
	if err != nil || scope.ServiceAccountID < 0 {
		return "", nil, service.ErrWorkspaceNotFound
	}
	if scope.ServiceAccountID > 0 {
		args = append(args, scope.ServiceAccountID)
		where += fmt.Sprintf(" AND %s.service_account_id=$%d", alias, len(args))
	} else {
		where += " AND " + alias + ".service_account_id IS NOT NULL"
	}
	return where, args, nil
}

func serviceAccountUsageCTE(scope service.FinOpsScope, start, end time.Time) (string, []any, error) {
	where, args, err := serviceAccountFinOpsWhere(scope, "u")
	if err != nil {
		return "", nil, err
	}
	first, last := finopsWholeHours(start, end)
	n := len(args)
	args = append(args, start, end, first, last)
	query := fmt.Sprintf(`WITH machine_usage AS (
 SELECT service_account_id,model,request_count,actual_cost,input_tokens,output_tokens
 FROM usage_service_account_hourly_rollups u WHERE %s AND bucket_start >= $%d AND bucket_start < $%d
 UNION ALL
 SELECT service_account_id,model,1::bigint,actual_cost,input_tokens,output_tokens
 FROM usage_logs u WHERE %s AND created_at >= $%d AND created_at < LEAST($%d::timestamptz,$%d::timestamptz)
 UNION ALL
 SELECT service_account_id,model,1::bigint,actual_cost,input_tokens,output_tokens
 FROM usage_logs u WHERE %s AND created_at >= GREATEST($%d::timestamptz,$%d::timestamptz,$%d::timestamptz) AND created_at < $%d
 )`, where, n+3, n+4, where, n+1, n+2, n+3, where, n+1, n+3, n+4, n+2)
	return query, args, nil
}

func loadServiceAccountFinOps(ctx context.Context, q workspaceSQL, scope service.FinOpsScope, start, end time.Time, summary service.FinOpsUsageSummary) ([]service.FinOpsServiceAccountBreakdown, error) {
	cte, args, err := serviceAccountUsageCTE(scope, start, end)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, cte+` SELECT u.service_account_id::text,COALESCE(s.name,''),SUM(u.request_count)::bigint,SUM(u.actual_cost),SUM(u.input_tokens+u.output_tokens)::bigint,array_agg(DISTINCT u.model ORDER BY u.model) FROM machine_usage u LEFT JOIN service_accounts s ON s.id=u.service_account_id AND s.workspace_id=$1 GROUP BY 1,2 ORDER BY 4 DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []service.FinOpsServiceAccountBreakdown{}
	var requests int64
	var spend float64
	for rows.Next() {
		var v service.FinOpsServiceAccountBreakdown
		if err = rows.Scan(&v.ID, &v.Name, &v.Requests, &v.Spend, &v.Tokens, pq.Array(&v.Models)); err != nil {
			return nil, err
		}
		requests += v.Requests
		spend += v.Spend
		result = append(result, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if scope.ServiceAccountID == 0 && summary.Requests > requests {
		result = append(result, service.FinOpsServiceAccountBreakdown{ID: "legacy", Name: "Human / Legacy", Requests: summary.Requests - requests, Spend: service.QuantizeUsageBillingAmount(summary.Spend - spend), Models: []string{}})
	}
	return result, nil
}
