package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The candidates and their tags are read in one MVCC statement. Reservation
// uses this same query under the existing workspace lock, which serializes all
// allocation mutations. No cache or caller supplied allocation is authoritative.
const resolvedAllocationSQL = `WITH candidates AS (
 SELECT 1 AS priority,'api_key_override' AS source,api_key_id AS scope_id,workspace_id,cost_center_id,environment,policy_revision
 FROM api_key_allocation_overrides WHERE workspace_id=$1 AND project_id=$2 AND api_key_id=$3
 UNION ALL
 SELECT 2,'service_account_override',service_account_id,workspace_id,cost_center_id,environment,policy_revision
 FROM service_account_allocation_overrides WHERE workspace_id=$1 AND project_id=$2 AND service_account_id=$4
 UNION ALL
 SELECT 3,'project_default',project_id,workspace_id,cost_center_id,environment,policy_revision
 FROM project_cost_allocations WHERE workspace_id=$1 AND project_id=$2
), candidate_tags AS (
 SELECT c.priority,c.source,c.scope_id,x.tag_id
 FROM candidates c JOIN api_key_allocation_override_tags x
   ON c.source='api_key_override' AND x.workspace_id=c.workspace_id AND x.api_key_id=c.scope_id
 UNION ALL
 SELECT c.priority,c.source,c.scope_id,x.tag_id
 FROM candidates c JOIN service_account_allocation_override_tags x
   ON c.source='service_account_override' AND x.workspace_id=c.workspace_id AND x.service_account_id=c.scope_id
 UNION ALL
 SELECT c.priority,c.source,c.scope_id,x.tag_id
 FROM candidates c JOIN project_cost_allocation_tags x
   ON c.source='project_default' AND x.workspace_id=c.workspace_id AND x.project_id=c.scope_id
), candidate_valid AS (
 SELECT c.priority,c.source,c.scope_id,c.cost_center_id,c.environment,c.policy_revision,
        COALESCE(cc.code,'') AS cost_center_code,COALESCE(cc.name,'') AS cost_center_name,
        COALESCE(jsonb_object_agg(t.tag_key,t.tag_value) FILTER (WHERE t.id IS NOT NULL),'{}'::jsonb) AS allocation_tags,
        (c.cost_center_id IS NULL OR (cc.id IS NOT NULL AND cc.status='active'))
        AND COALESCE(bool_and(ct.tag_id IS NULL OR (t.id IS NOT NULL AND t.status='active')),true) AS valid
 FROM candidates c
 LEFT JOIN candidate_tags ct ON ct.priority=c.priority AND ct.source=c.source AND ct.scope_id=c.scope_id
 LEFT JOIN workspace_allocation_tags t ON t.workspace_id=$1 AND t.id=ct.tag_id
 LEFT JOIN workspace_cost_centers cc ON cc.workspace_id=c.workspace_id AND cc.id=c.cost_center_id
 GROUP BY c.priority,c.source,c.scope_id,c.cost_center_id,c.environment,c.policy_revision,cc.id,cc.code,cc.name,cc.status
), chosen AS (SELECT * FROM candidate_valid ORDER BY priority LIMIT 1), invalid AS (
 SELECT EXISTS (SELECT 1 FROM candidate_valid WHERE NOT valid) AS has_invalid
)
SELECT c.cost_center_id,c.cost_center_code,c.cost_center_name,c.environment,c.policy_revision,c.source,c.allocation_tags,
       c.valid AND NOT invalid.has_invalid
FROM chosen c CROSS JOIN invalid`

func resolveAllocationSQL(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, workspace, project, key, account int64) (*service.AllocationSnapshot, error) {
	rows, err := q.QueryContext(ctx, resolvedAllocationSQL, workspace, project, key, account)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		value := service.UnallocatedAllocation()
		return &value, nil
	}
	var center sql.NullInt64
	var raw []byte
	var valid bool
	value := service.AllocationSnapshot{}
	if err := rows.Scan(&center, &value.CostCenterCode, &value.CostCenterName, &value.Environment, &value.PolicyRevision, &value.Source, &raw, &valid); err != nil {
		return nil, err
	}
	if !valid {
		return nil, service.ErrWorkspaceConflict
	}
	if center.Valid {
		value.CostCenterID = &center.Int64
	}
	if err := json.Unmarshal(raw, &value.Tags); err != nil {
		return nil, service.ErrWorkspaceAllocationInvalid
	}
	value.AllocationTags = value.Tags
	normalized, err := value.NormalizeAndValidate()
	return &normalized, err
}
