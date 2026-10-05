package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func finopsScopeWhere(scope service.FinOpsScope, alias string) (string, []any, error) {
	if scope.WorkspaceID <= 0 || scope.ProjectID < 0 {
		return "", nil, service.ErrWorkspaceNotFound
	}
	if scope.ProjectID > 0 {
		return fmt.Sprintf("%s.workspace_id=$1 AND %s.project_id=$2", alias, alias), []any{scope.WorkspaceID, scope.ProjectID}, nil
	}
	return fmt.Sprintf("%s.workspace_id=$1", alias), []any{scope.WorkspaceID}, nil
}

func finopsWindow(start, end time.Time, timezone string) (time.Time, time.Time, string, error) {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return start, end, timezone, service.ErrWorkspaceInvalid
	}
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if start.IsZero() {
		local := end.In(loc)
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	}
	if !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return start, end, timezone, service.ErrWorkspaceInvalid
	}
	return start.UTC(), end.UTC(), timezone, nil
}

func finopsWholeHours(start, end time.Time) (time.Time, time.Time) {
	first := start.UTC().Truncate(time.Hour)
	if first.Before(start) {
		first = first.Add(time.Hour)
	}
	return first, end.UTC().Truncate(time.Hour)
}

// Whole hours come from durable aggregates. Two disjoint, indexed edge ranges
// preserve exact arbitrary start/end times without rescanning the whole period.
func finopsUsageCTE(scope service.FinOpsScope, start, end time.Time) (string, []any, error) {
	where, args, err := finopsScopeWhere(scope, "u")
	if err != nil {
		return "", nil, err
	}
	rollupTable := "usage_tenant_hourly_rollups"
	if scope.ServiceAccountID > 0 {
		where, args, err = serviceAccountFinOpsWhere(scope, "u")
		if err != nil {
			return "", nil, err
		}
		rollupTable = "usage_service_account_hourly_rollups"
	}
	first, last := finopsWholeHours(start, end)
	n := len(args)
	args = append(args, start, end, first, last)
	query := fmt.Sprintf(`WITH finops_usage AS (
 SELECT project_id,api_key_id,resolved_platform,model,request_count,actual_cost
 FROM %s u WHERE %s AND bucket_start >= $%d AND bucket_start < $%d
 UNION ALL
 SELECT project_id,api_key_id,resolved_platform,model,1::bigint,actual_cost
 FROM usage_logs u WHERE %s AND created_at >= $%d AND created_at < LEAST($%d::timestamptz,$%d::timestamptz)
 UNION ALL
 SELECT project_id,api_key_id,resolved_platform,model,1::bigint,actual_cost
 FROM usage_logs u WHERE %s AND created_at >= GREATEST($%d::timestamptz,$%d::timestamptz,$%d::timestamptz) AND created_at < $%d
)`, rollupTable, where, n+3, n+4, where, n+1, n+2, n+3, where, n+1, n+3, n+4, n+2)
	return query, args, nil
}

// Local day ranges are computed with the IANA timezone, including DST and
// quarter-hour offsets. One SQL query aggregates every day; no N-query loop.
func finopsDailyQuery(scope service.FinOpsScope, start, end time.Time, timezone string) (string, []any, error) {
	where, args, err := finopsScopeWhere(scope, "u")
	if err != nil {
		return "", nil, err
	}
	rollupTable := "usage_tenant_hourly_rollups"
	if scope.ServiceAccountID > 0 {
		where, args, err = serviceAccountFinOpsWhere(scope, "u")
		if err != nil {
			return "", nil, err
		}
		rollupTable = "usage_service_account_hourly_rollups"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", nil, service.ErrWorkspaceInvalid
	}
	var dates, starts, ends, firsts, lasts []string
	for current := start; current.Before(end); {
		local := current.In(loc)
		next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
		if !next.After(current) {
			next = current.Add(24 * time.Hour)
		}
		if next.After(end) {
			next = end
		}
		first, last := finopsWholeHours(current, next)
		dates = append(dates, local.Format("2006-01-02"))
		starts = append(starts, current.Format(time.RFC3339Nano))
		ends = append(ends, next.Format(time.RFC3339Nano))
		firsts = append(firsts, first.Format(time.RFC3339Nano))
		lasts = append(lasts, last.Format(time.RFC3339Nano))
		current = next
	}
	n := len(args)
	args = append(args, pq.Array(dates), pq.Array(starts), pq.Array(ends), pq.Array(firsts), pq.Array(lasts))
	query := fmt.Sprintf(`WITH ranges AS (
 SELECT * FROM unnest($%d::text[],$%d::timestamptz[],$%d::timestamptz[],$%d::timestamptz[],$%d::timestamptz[])
 AS b(date,start_at,end_at,full_start,full_end)
), edges AS (
 SELECT date,start_at,LEAST(end_at,full_start) AS end_at FROM ranges WHERE start_at < LEAST(end_at,full_start)
 UNION ALL
 SELECT date,GREATEST(start_at,full_start,full_end),end_at FROM ranges WHERE GREATEST(start_at,full_start,full_end) < end_at
), daily_usage AS (
 SELECT b.date,u.request_count,u.actual_cost FROM ranges b JOIN %s u
 ON u.bucket_start >= b.full_start AND u.bucket_start < b.full_end WHERE %s
 UNION ALL
 SELECT b.date,1::bigint,u.actual_cost FROM edges b JOIN usage_logs u
 ON u.created_at >= b.start_at AND u.created_at < b.end_at WHERE %s
)
SELECT date,SUM(request_count)::bigint,SUM(actual_cost) FROM daily_usage GROUP BY 1 ORDER BY 1`, n+1, n+2, n+3, n+4, n+5, rollupTable, where, where)
	return query, args, nil
}
