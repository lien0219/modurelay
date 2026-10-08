package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func allocationTx(ctx context.Context, r *workspaceRepository, actor, workspace, project int64, permission string, write bool) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err = lockWorkspace(ctx, tx, workspace, write); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	access, err := workspaceAccess(ctx, tx, actor, workspace, project)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err = service.CheckWorkspacePermission(access, permission); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func scanCostCenter(row workspaceScanner) (*service.WorkspaceCostCenter, error) {
	v := &service.WorkspaceCostCenter{}
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.Code, &v.Name, &v.Description, &v.Status)
	return v, workspaceError(err)
}
func scanAllocationTag(row workspaceScanner) (*service.WorkspaceAllocationTag, error) {
	v := &service.WorkspaceAllocationTag{}
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.Key, &v.Value, &v.Description, &v.Status)
	return v, workspaceError(err)
}

func (r *workspaceRepository) ListAllocationCostCenters(ctx context.Context, actor, workspace int64, archived bool) ([]service.WorkspaceCostCenter, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.read", false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	query, args := `SELECT id,workspace_id,code,name,description,status FROM workspace_cost_centers WHERE workspace_id=$1`, []any{workspace}
	if !archived {
		query += ` AND status='active'`
	}
	query += ` ORDER BY code,id`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, workspaceError(err)
	}
	defer func() { _ = rows.Close() }()
	out := []service.WorkspaceCostCenter{}
	for rows.Next() {
		v, e := scanCostCenter(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *v)
	}
	return out, workspaceError(rows.Err())
}
func (r *workspaceRepository) CreateAllocationCostCenter(ctx context.Context, actor, workspace int64, code, name, description string) (*service.WorkspaceCostCenter, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.update", true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	v, err := scanCostCenter(tx.QueryRowContext(ctx, `INSERT INTO workspace_cost_centers(workspace_id,code,name,description,created_by_user_id) VALUES($1,$2,$3,$4,$5) RETURNING id,workspace_id,code,name,description,status`, workspace, code, name, description, actor))
	if err != nil {
		return nil, err
	}
	if err = recordAllocationMutation(ctx, tx, workspace, 0, actor, "allocation_center_created", "cost_center", v.ID, map[string]any{"operation": "create", "resource_id": v.ID, "slug": v.Code, "name": v.Name, "status": v.Status}); err != nil {
		return nil, err
	}
	return v, workspaceError(tx.Commit())
}
func (r *workspaceRepository) UpdateAllocationCostCenter(ctx context.Context, actor, workspace, id int64, code, name, description string) (*service.WorkspaceCostCenter, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.update", true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	v, err := scanCostCenter(tx.QueryRowContext(ctx, `UPDATE workspace_cost_centers SET code=$3,name=$4,description=$5,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status='active' RETURNING id,workspace_id,code,name,description,status`, workspace, id, code, name, description))
	if err != nil {
		return nil, err
	}
	if err = recordAllocationMutation(ctx, tx, workspace, 0, actor, "allocation_center_updated", "cost_center", v.ID, map[string]any{"operation": "update", "resource_id": v.ID, "slug": v.Code, "name": v.Name, "status": v.Status}); err != nil {
		return nil, err
	}
	return v, workspaceError(tx.Commit())
}
func (r *workspaceRepository) ArchiveAllocationCostCenter(ctx context.Context, actor, workspace, id int64) error {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.update", true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE workspace_cost_centers SET status='archived',archived_at=now(),updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status='active'`, workspace, id)
	if err != nil {
		return workspaceError(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrWorkspaceNotFound
	}
	if err = recordAllocationMutation(ctx, tx, workspace, 0, actor, "allocation_center_archived", "cost_center", id, map[string]any{"operation": "archive", "resource_id": id, "status": "archived"}); err != nil {
		return err
	}
	return workspaceError(tx.Commit())
}

func (r *workspaceRepository) ListAllocationTags(ctx context.Context, actor, workspace int64, archived bool) ([]service.WorkspaceAllocationTag, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.read", false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	query, args := `SELECT id,workspace_id,tag_key,tag_value,description,status FROM workspace_allocation_tags WHERE workspace_id=$1`, []any{workspace}
	if !archived {
		query += ` AND status='active'`
	}
	query += ` ORDER BY tag_key,tag_value,id`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, workspaceError(err)
	}
	defer func() { _ = rows.Close() }()
	out := []service.WorkspaceAllocationTag{}
	for rows.Next() {
		v, e := scanAllocationTag(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *v)
	}
	return out, workspaceError(rows.Err())
}
func (r *workspaceRepository) CreateAllocationTag(ctx context.Context, actor, workspace int64, key, value, description string) (*service.WorkspaceAllocationTag, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.update", true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	v, err := scanAllocationTag(tx.QueryRowContext(ctx, `INSERT INTO workspace_allocation_tags(workspace_id,tag_key,tag_value,description,created_by_user_id) VALUES($1,$2,$3,$4,$5) RETURNING id,workspace_id,tag_key,tag_value,description,status`, workspace, key, value, description, actor))
	if err != nil {
		return nil, err
	}
	if err = recordAllocationMutation(ctx, tx, workspace, 0, actor, "allocation_tag_created", "allocation_tag", v.ID, map[string]any{"operation": "create", "resource_id": v.ID, "name": v.Key + "=" + v.Value, "status": v.Status}); err != nil {
		return nil, err
	}
	return v, workspaceError(tx.Commit())
}
func (r *workspaceRepository) UpdateAllocationTag(ctx context.Context, actor, workspace, id int64, key, value, description string) (*service.WorkspaceAllocationTag, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.update", true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	v, err := scanAllocationTag(tx.QueryRowContext(ctx, `UPDATE workspace_allocation_tags SET tag_key=$3,tag_value=$4,description=$5,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status='active' RETURNING id,workspace_id,tag_key,tag_value,description,status`, workspace, id, key, value, description))
	if err != nil {
		return nil, err
	}
	if err = recordAllocationMutation(ctx, tx, workspace, 0, actor, "allocation_tag_updated", "allocation_tag", v.ID, map[string]any{"operation": "update", "resource_id": v.ID, "name": v.Key + "=" + v.Value, "status": v.Status}); err != nil {
		return nil, err
	}
	return v, workspaceError(tx.Commit())
}
func (r *workspaceRepository) ArchiveAllocationTag(ctx context.Context, actor, workspace, id int64) error {
	tx, err := allocationTx(ctx, r, actor, workspace, 0, "workspace.update", true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE workspace_allocation_tags SET status='archived',archived_at=now(),updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status='active'`, workspace, id)
	if err != nil {
		return workspaceError(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrWorkspaceNotFound
	}
	if err = recordAllocationMutation(ctx, tx, workspace, 0, actor, "allocation_tag_archived", "allocation_tag", id, map[string]any{"operation": "archive", "resource_id": id, "status": "archived"}); err != nil {
		return err
	}
	return workspaceError(tx.Commit())
}

func allocationTags(ctx context.Context, q workspaceSQL, table, scopeColumn string, workspace, id int64) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT t.tag_key,t.tag_value FROM `+table+` x JOIN workspace_allocation_tags t ON t.workspace_id=x.workspace_id AND t.id=x.tag_id WHERE x.workspace_id=$1 AND x.`+scopeColumn+`=$2 ORDER BY t.tag_key,t.tag_value`, workspace, id)
	if err != nil {
		return nil, workspaceError(err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, workspaceError(rows.Err())
}
func validateAllocationRefs(ctx context.Context, tx *sql.Tx, workspace int64, c service.AllocationConfig) error {
	if c.CostCenterID != nil {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM workspace_cost_centers WHERE workspace_id=$1 AND id=$2`, workspace, *c.CostCenterID).Scan(&status); err != nil {
			return workspaceError(err)
		}
		if status != "active" {
			return service.ErrWorkspaceConflict
		}
	}
	for k, v := range c.Tags {
		var id int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workspace_allocation_tags WHERE workspace_id=$1 AND tag_key=$2 AND tag_value=$3 AND status='active'`, workspace, k, v).Scan(&id); err != nil {
			return workspaceError(err)
		}
	}
	return nil
}
func replaceAllocationTags(ctx context.Context, tx *sql.Tx, table, scopeColumn string, workspace, id int64, tags map[string]string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE workspace_id=$1 AND `+scopeColumn+`=$2`, workspace, id); err != nil {
		return err
	}
	for k, v := range tags {
		var tagID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM workspace_allocation_tags WHERE workspace_id=$1 AND tag_key=$2 AND tag_value=$3 AND status='active'`, workspace, k, v).Scan(&tagID); err != nil {
			return workspaceError(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+`(workspace_id,`+scopeColumn+`,tag_id) VALUES($1,$2,$3)`, workspace, id, tagID); err != nil {
			return workspaceError(err)
		}
	}
	return nil
}

func (r *workspaceRepository) GetProjectAllocation(ctx context.Context, actor, workspace, project int64) (*service.ProjectAllocation, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, project, "project.read", false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var c service.AllocationConfig
	var center, updatedBy sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT cost_center_id,environment,policy_revision,updated_by_user_id FROM project_cost_allocations WHERE workspace_id=$1 AND project_id=$2`, workspace, project).Scan(&center, &c.Environment, &c.PolicyRevision, &updatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, workspaceError(err)
	}
	if center.Valid {
		c.CostCenterID = &center.Int64
	}
	c.Tags, err = allocationTags(ctx, tx, "project_cost_allocation_tags", "project_id", workspace, project)
	if err != nil {
		return nil, err
	}
	c.AllocationTags = c.Tags
	out := &service.ProjectAllocation{WorkspaceID: workspace, ProjectID: project, Allocation: c}
	if updatedBy.Valid {
		out.UpdatedByUserID = updatedBy.Int64
	}
	return out, nil
}

func (r *workspaceRepository) SetProjectAllocation(ctx context.Context, actor, workspace, project int64, c service.AllocationConfig) (*service.ProjectAllocation, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, project, "project.update", true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = validateAllocationRefs(ctx, tx, workspace, c); err != nil {
		return nil, err
	}
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT policy_revision FROM project_cost_allocations WHERE workspace_id=$1 AND project_id=$2`, workspace, project).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		if c.PolicyRevision != 1 {
			return nil, service.ErrWorkspaceAllocationConflict
		}
		current = 0
	} else if err != nil {
		return nil, workspaceError(err)
	} else if c.PolicyRevision != current {
		return nil, service.ErrWorkspaceAllocationConflict
	}
	next := c.PolicyRevision + 1
	if current == 0 {
		next = c.PolicyRevision
	}
	var center any = nil
	if c.CostCenterID != nil {
		center = *c.CostCenterID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO project_cost_allocations(workspace_id,project_id,cost_center_id,environment,policy_revision,updated_by_user_id) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(workspace_id,project_id) DO UPDATE SET cost_center_id=EXCLUDED.cost_center_id,environment=EXCLUDED.environment,policy_revision=EXCLUDED.policy_revision,updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=now()`, workspace, project, center, c.Environment, next, actor)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = replaceAllocationTags(ctx, tx, "project_cost_allocation_tags", "project_id", workspace, project, c.Tags); err != nil {
		return nil, err
	}
	if err = recordAllocationMutation(ctx, tx, workspace, project, actor, "allocation_project_updated", "project", project, map[string]any{"operation": "update", "resource_id": project, "project_id": project, "policy_revision": next, "status": "active"}); err != nil {
		return nil, err
	}
	c.PolicyRevision = next
	c.AllocationTags = c.Tags
	out := &service.ProjectAllocation{WorkspaceID: workspace, ProjectID: project, Allocation: c, UpdatedByUserID: actor}
	return out, workspaceError(tx.Commit())
}

func (r *workspaceRepository) GetAPIKeyAllocationOverride(ctx context.Context, actor, workspace, project, key int64) (*service.APIKeyAllocationOverride, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, project, "project.read", false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureAllocationCredential(ctx, tx, true, workspace, project, key); err != nil {
		return nil, err
	}
	var c service.AllocationConfig
	var center sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT cost_center_id,environment,policy_revision FROM api_key_allocation_overrides WHERE workspace_id=$1 AND project_id=$2 AND api_key_id=$3`, workspace, project, key).Scan(&center, &c.Environment, &c.PolicyRevision)
	if err != nil {
		return nil, workspaceError(err)
	}
	if center.Valid {
		c.CostCenterID = &center.Int64
	}
	c.Tags, err = allocationTags(ctx, tx, "api_key_allocation_override_tags", "api_key_id", workspace, key)
	if err != nil {
		return nil, err
	}
	c.AllocationTags = c.Tags
	return &service.APIKeyAllocationOverride{WorkspaceID: workspace, ProjectID: project, APIKeyID: key, Allocation: c}, nil
}
func (r *workspaceRepository) SetAPIKeyAllocationOverride(ctx context.Context, actor, workspace, project, key int64, c service.AllocationConfig) (*service.APIKeyAllocationOverride, error) {
	v, err := r.setCredentialOverride(ctx, actor, workspace, project, key, c, true)
	if err != nil {
		return nil, err
	}
	override, ok := v.(*service.APIKeyAllocationOverride)
	if !ok {
		return nil, fmt.Errorf("unexpected API key allocation override type %T", v)
	}
	return override, nil
}
func (r *workspaceRepository) GetServiceAccountAllocationOverride(ctx context.Context, actor, workspace, project, account int64) (*service.ServiceAccountAllocationOverride, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, project, "project.read", false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureAllocationCredential(ctx, tx, false, workspace, project, account); err != nil {
		return nil, err
	}
	var c service.AllocationConfig
	var center sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT cost_center_id,environment,policy_revision FROM service_account_allocation_overrides WHERE workspace_id=$1 AND project_id=$2 AND service_account_id=$3`, workspace, project, account).Scan(&center, &c.Environment, &c.PolicyRevision)
	if err != nil {
		return nil, workspaceError(err)
	}
	if center.Valid {
		c.CostCenterID = &center.Int64
	}
	c.Tags, err = allocationTags(ctx, tx, "service_account_allocation_override_tags", "service_account_id", workspace, account)
	if err != nil {
		return nil, err
	}
	c.AllocationTags = c.Tags
	return &service.ServiceAccountAllocationOverride{WorkspaceID: workspace, ProjectID: project, ServiceAccountID: account, Allocation: c}, nil
}
func (r *workspaceRepository) SetServiceAccountAllocationOverride(ctx context.Context, actor, workspace, project, account int64, c service.AllocationConfig) (*service.ServiceAccountAllocationOverride, error) {
	v, err := r.setCredentialOverride(ctx, actor, workspace, project, account, c, false)
	if err != nil {
		return nil, err
	}
	override, ok := v.(*service.ServiceAccountAllocationOverride)
	if !ok {
		return nil, fmt.Errorf("unexpected service account allocation override type %T", v)
	}
	return override, nil
}
func (r *workspaceRepository) setCredentialOverride(ctx context.Context, actor, workspace, project, id int64, c service.AllocationConfig, api bool) (any, error) {
	tx, err := allocationTx(ctx, r, actor, workspace, project, "project.update", true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureAllocationCredential(ctx, tx, api, workspace, project, id); err != nil {
		return nil, err
	}
	if err = validateAllocationRefs(ctx, tx, workspace, c); err != nil {
		return nil, err
	}
	table, tags := `service_account_allocation_overrides`, `service_account_allocation_override_tags`
	if api {
		table, tags = `api_key_allocation_overrides`, `api_key_allocation_override_tags`
	}
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT policy_revision FROM `+table+` WHERE workspace_id=$1 AND project_id=$2 AND `+map[bool]string{true: `api_key_id`, false: `service_account_id`}[api]+`=$3`, workspace, project, id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		if c.PolicyRevision != 1 {
			return nil, service.ErrWorkspaceAllocationConflict
		}
		current = 0
	} else if err != nil {
		return nil, workspaceError(err)
	} else if c.PolicyRevision != current {
		return nil, service.ErrWorkspaceAllocationConflict
	}
	next := c.PolicyRevision + 1
	if current == 0 {
		next = c.PolicyRevision
	}
	var center any = nil
	if c.CostCenterID != nil {
		center = *c.CostCenterID
	}
	col := map[bool]string{true: `api_key_id`, false: `service_account_id`}[api]
	_, err = tx.ExecContext(ctx, `INSERT INTO `+table+`(workspace_id,project_id,`+col+`,cost_center_id,environment,policy_revision,updated_by_user_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(`+col+`) DO UPDATE SET cost_center_id=EXCLUDED.cost_center_id,environment=EXCLUDED.environment,policy_revision=EXCLUDED.policy_revision,updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=now()`, workspace, project, id, center, c.Environment, next, actor)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = replaceAllocationTags(ctx, tx, tags, col, workspace, id, c.Tags); err != nil {
		return nil, err
	}
	action := "allocation_service_account_updated"
	target := "service_account"
	data := map[string]any{"operation": "update", "resource_id": id, "project_id": project, "service_account_id": id, "policy_revision": next, "status": "active"}
	if api {
		action = "allocation_key_updated"
		target = "api_key"
		data = map[string]any{"operation": "update", "resource_id": id, "project_id": project, "key_id": id, "policy_revision": next, "status": "active"}
	}
	if err = recordAllocationMutation(ctx, tx, workspace, project, actor, action, target, id, data); err != nil {
		return nil, err
	}
	c.PolicyRevision = next
	c.AllocationTags = c.Tags
	if api {
		return &service.APIKeyAllocationOverride{WorkspaceID: workspace, ProjectID: project, APIKeyID: id, Allocation: c}, workspaceError(tx.Commit())
	}
	return &service.ServiceAccountAllocationOverride{WorkspaceID: workspace, ProjectID: project, ServiceAccountID: id, Allocation: c}, workspaceError(tx.Commit())
}

func ensureAllocationCredential(ctx context.Context, tx *sql.Tx, api bool, workspace, project, id int64) error {
	table := "service_accounts"
	if api {
		table = "api_keys"
	}
	var got int64
	query := `SELECT c.id FROM ` + table + ` c JOIN projects p ON p.id=c.project_id AND p.workspace_id=$3 WHERE c.id=$1 AND c.project_id=$2`
	if api {
		query += ` AND c.deleted_at IS NULL`
	}
	if err := tx.QueryRowContext(ctx, query, id, project, workspace).Scan(&got); err != nil {
		return workspaceError(err)
	}
	_ = workspace
	return nil
}

func (r *workspaceRepository) GetAllocationReport(ctx context.Context, f service.AllocationFilter) (*service.AllocationReport, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	out := &service.AllocationReport{WorkspaceID: f.WorkspaceID, ProjectID: f.ProjectID, OverlappingTags: true}
	start, end := f.From, f.To
	if start == "" {
		start = "1970-01-01T00:00:00Z"
	}
	if end == "" {
		end = "9999-12-31T23:59:59Z"
	}
	reportCTE, reportArgs, reportFilters := allocationEvidenceQuery(f, start, end)
	// Base dimensions are served from the transactional hourly rollup whenever
	// no tag filter is active. Tag filters require immutable snapshots because
	// the rollup intentionally does not duplicate arbitrary tag dimensions.
	if f.TagKey == "" && f.TagValue == "" {
		reportCTE, reportArgs, reportFilters = allocationRollupQuery(f, start, end)
	}
	if err := r.db.QueryRowContext(ctx, reportCTE+` SELECT COALESCE(SUM(e.actual_cost),0),COALESCE(SUM(e.actual_cost) FILTER (WHERE e.cost_center_id IS NOT NULL),0),COALESCE(SUM(e.actual_cost) FILTER (WHERE e.cost_center_id IS NULL),0) FROM evidence e WHERE `+reportFilters, reportArgs...).Scan(&out.WorkspaceTotal, &out.Allocated, &out.Unallocated); err != nil {
		return nil, workspaceError(err)
	}
	load := func(cte string, args []any, filters, query string, target *[]service.AllocationReportGroup) error {
		rows, err := r.db.QueryContext(ctx, cte+query, args...)
		if err != nil {
			return workspaceError(err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var g service.AllocationReportGroup
			if err := rows.Scan(&g.Key, &g.Cost, &g.RequestCount); err != nil {
				return err
			}
			*target = append(*target, g)
		}
		return workspaceError(rows.Err())
	}
	if err := load(reportCTE, reportArgs, reportFilters, ` SELECT COALESCE(NULLIF(e.cost_center_code,''),c.code,'unallocated'),SUM(e.actual_cost),SUM(e.request_count)
 FROM evidence e LEFT JOIN workspace_cost_centers c ON c.workspace_id=e.workspace_id AND c.id=e.cost_center_id
	 WHERE `+reportFilters+` GROUP BY 1 ORDER BY 2 DESC,1 LIMIT 100`, &out.CostCenters); err != nil {
		return nil, err
	}
	if err := load(reportCTE, reportArgs, reportFilters, ` SELECT e.environment,SUM(e.actual_cost),SUM(e.request_count) FROM evidence e WHERE `+reportFilters+` GROUP BY 1 ORDER BY 2 DESC,1 LIMIT 100`, &out.Environments); err != nil {
		return nil, err
	}
	tagCTE, tagArgs, tagFilters := allocationEvidenceQuery(f, start, end)
	if err := load(tagCTE, tagArgs, tagFilters, ` SELECT tag.key || '=' || tag.value,SUM(e.actual_cost),SUM(e.request_count)
 FROM evidence e CROSS JOIN LATERAL jsonb_each_text(e.allocation_tags) AS tag(key,value)
	 WHERE `+tagFilters+` GROUP BY tag.key,tag.value ORDER BY 2 DESC,1 LIMIT 100`, &out.Tags); err != nil {
		return nil, err
	}
	return out, nil
}

// allocationEvidenceQuery reads immutable allocation snapshots and unions
// tenant-attributed legacy usage that predates migration 299. The latter is
// explicitly unallocated and is bounded by the same indexed workspace/project
// and time predicates; historical rows with no tenant scope cannot be assigned
// to a workspace and remain outside this tenant report.
func allocationEvidenceQuery(f service.AllocationFilter, start, end string) (string, []any, string) {
	args := []any{f.WorkspaceID, start, end}
	scope := `s.workspace_id=$1 AND s.created_at >= $2::timestamptz AND s.created_at < $3::timestamptz`
	legacyScope := `u.workspace_id=$1 AND u.created_at >= $2::timestamptz AND u.created_at < $3::timestamptz`
	if f.ProjectID > 0 {
		args = append(args, f.ProjectID)
		placeholder := fmt.Sprintf("$%d", len(args))
		scope += " AND s.project_id=" + placeholder
		legacyScope += " AND u.project_id=" + placeholder
	}
	cte := `WITH evidence AS (
		SELECT s.workspace_id,s.project_id,s.cost_center_id,s.environment,s.allocation_tags,
		        s.actual_cost,s.created_at,s.usage_log_id,s.cost_center_code,1::bigint AS request_count
   FROM usage_allocation_snapshots s WHERE ` + scope + `
 UNION ALL
		SELECT u.workspace_id,u.project_id,NULL::bigint,'unallocated'::varchar,'{}'::jsonb,
		        u.actual_cost,u.created_at,u.id,''::varchar,1::bigint AS request_count
   FROM usage_logs u
   LEFT JOIN usage_allocation_snapshots existing ON existing.usage_log_id=u.id
  WHERE ` + legacyScope + ` AND existing.usage_log_id IS NULL
)`
	filters := allocationFilterPredicates(f, &args)
	return cte, args, filters
}

// allocationRollupQuery serves the bounded base dimensions from the hourly
// rollup and unions tenant-scoped legacy rows that predate migration 299.
// Tag expansion remains snapshot-backed because tags are intentionally not
// copied into the low-cardinality rollup.
func allocationRollupQuery(f service.AllocationFilter, start, end string) (string, []any, string) {
	args := []any{f.WorkspaceID, start, end}
	scope := `r.workspace_id=$1 AND r.bucket_start >= $2::timestamptz AND r.bucket_start < $3::timestamptz`
	legacyScope := `u.workspace_id=$1 AND u.created_at >= $2::timestamptz AND u.created_at < $3::timestamptz`
	if f.ProjectID > 0 {
		args = append(args, f.ProjectID)
		placeholder := fmt.Sprintf("$%d", len(args))
		scope += " AND r.project_id=" + placeholder
		legacyScope += " AND u.project_id=" + placeholder
	}
	cte := `WITH evidence AS (
	 SELECT r.workspace_id,r.project_id,r.cost_center_id,r.environment,'{}'::jsonb AS allocation_tags,
	        r.actual_cost,r.bucket_start AS created_at,NULL::bigint AS usage_log_id,
	        COALESCE(c.code,'') AS cost_center_code,r.request_count
	   FROM usage_allocation_hourly_rollups r
	   LEFT JOIN workspace_cost_centers c ON c.workspace_id=r.workspace_id AND c.id=r.cost_center_id
	  WHERE ` + scope + `
	 UNION ALL
	 SELECT u.workspace_id,u.project_id,NULL::bigint,'unallocated'::varchar,'{}'::jsonb,
	        u.actual_cost,u.created_at,u.id,''::varchar,1::bigint
	   FROM usage_logs u
	   LEFT JOIN usage_allocation_snapshots existing ON existing.usage_log_id=u.id
	  WHERE ` + legacyScope + ` AND existing.usage_log_id IS NULL
	)`
	return cte, args, allocationFilterPredicates(f, &args)
}

func allocationFilterPredicates(f service.AllocationFilter, args *[]any) string {
	filters := "TRUE"
	if f.CostCenterID != nil {
		*args = append(*args, *f.CostCenterID)
		filters += fmt.Sprintf(" AND e.cost_center_id=$%d", len(*args))
	}
	if f.Environment != "" {
		*args = append(*args, f.Environment)
		filters += fmt.Sprintf(" AND e.environment=$%d", len(*args))
	}
	if f.TagKey != "" {
		*args = append(*args, f.TagKey)
		filters += fmt.Sprintf(" AND e.allocation_tags ? $%d::text", len(*args))
	}
	if f.TagValue != "" {
		*args = append(*args, f.TagValue)
		if f.TagKey != "" {
			filters += fmt.Sprintf(" AND e.allocation_tags @> jsonb_build_object($%d::text,$%d::text)", len(*args)-1, len(*args))
		} else {
			filters += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM jsonb_each_text(e.allocation_tags) AS tag_filter(k,v) WHERE tag_filter.v=$%d::text)", len(*args))
		}
	}
	return filters
}

var _ service.WorkspaceAllocationRepository = (*workspaceRepository)(nil)

func recordAllocationMutation(ctx context.Context, q workspaceSQL, workspace, project, actor int64, action, target string, id int64, metadata map[string]any) error {
	var projectRef *int64
	if project > 0 {
		projectRef = &project
	}
	if err := appendWorkspaceAudit(ctx, q, workspace, actor, projectRef, action, target, id, metadata); err != nil {
		return err
	}
	return insertWorkspaceMutationEvent(ctx, q, workspace, project, actor, action, target, id, service.DomainEventData{
		"operation": metadata["operation"], "resource_id": metadata["resource_id"], "project_id": metadata["project_id"],
		"key_id": metadata["key_id"], "service_account_id": metadata["service_account_id"], "policy_revision": metadata["policy_revision"],
		"slug": metadata["slug"], "name": metadata["name"], "status": metadata["status"],
	})
}
