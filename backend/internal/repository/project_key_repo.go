package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func legacyTenantReadPredicate(actor int64) predicate.APIKey {
	return func(s *entsql.Selector) {
		s.Where(entsql.Or(entsql.IsNull(s.C(apikey.FieldProjectID)), entsql.P(func(b *entsql.Builder) {
			b.WriteString(`EXISTS(SELECT 1 FROM projects p JOIN workspaces w ON w.id=p.workspace_id JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE p.id=api_keys.project_id AND m.user_id=`).Arg(actor).WriteString(` AND m.status='active' AND m.role IN ('owner','admin','developer') AND u.status='active' AND u.deleted_at IS NULL)`)
		})))
	}
}
func (r *apiKeyRepository) maskLegacyOrganizationKeys(ctx context.Context, a int64, keys []service.APIKey) error {
	ids := make([]int64, 0, len(keys))
	for _, k := range keys {
		if k.ProjectID != nil {
			ids = append(ids, k.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, e := r.sql.QueryContext(ctx, `SELECT k.id FROM api_keys k JOIN projects p ON p.id=k.project_id JOIN workspaces w ON w.id=p.workspace_id WHERE k.user_id=$1 AND k.id=ANY($2::bigint[]) AND w.type='organization'`, a, pq.Array(ids))
	if e != nil {
		return e
	}
	defer func() { _ = rows.Close() }()
	org := map[int64]bool{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			return e
		}
		org[id] = true
	}
	if e = rows.Err(); e != nil {
		return e
	}
	for i := range keys {
		if org[keys[i].ID] {
			secret := keys[i].Key
			keys[i].Key = "****"
			if len(secret) > 4 {
				keys[i].Key += secret[len(secret)-4:]
			}
		}
	}
	return nil
}

func (r *apiKeyRepository) PersonalProject(ctx context.Context, a int64) (int64, int64, error) {
	var w, p int64
	rows, e := r.sql.QueryContext(ctx, `SELECT w.id,p.id FROM workspaces w JOIN projects p ON p.workspace_id=w.id AND p.is_default WHERE w.id=ensure_personal_workspace($1) AND w.owner_user_id=$1 AND w.type='personal'`, a)
	if e != nil {
		return 0, 0, e
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, 0, service.ErrWorkspaceNotFound
	}
	e = rows.Scan(&w, &p)
	return w, p, e
}
func (r *apiKeyRepository) ProjectForKey(ctx context.Context, id int64) (int64, int64, error) {
	var w, p sql.NullInt64
	rows, e := r.sql.QueryContext(ctx, `SELECT p.workspace_id,k.project_id FROM api_keys k LEFT JOIN projects p ON p.id=k.project_id WHERE k.id=$1 AND k.deleted_at IS NULL`, id)
	if e != nil {
		return 0, 0, e
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, 0, service.ErrAPIKeyNotFound
	}
	e = rows.Scan(&w, &p)
	return w.Int64, p.Int64, e
}

// One joined, authoritative admission on every request, including cache hits.
// A missing legacy assignment takes a separate bounded, conditional UPDATE;
// already-assigned keys execute only the joined gate and principal load.
func (r *apiKeyRepository) ResolveTenant(ctx context.Context, k *service.APIKey) (*service.TenantContext, error) {
	if k.ProjectID == nil {
		_, e := r.sql.ExecContext(ctx, `UPDATE api_keys k SET project_id=p.id FROM projects p,workspaces w WHERE k.id=$1 AND k.key=$2 AND k.project_id IS NULL AND k.deleted_at IS NULL AND w.id=ensure_personal_workspace(k.user_id) AND w.type='personal' AND w.owner_user_id=k.user_id AND p.workspace_id=w.id AND p.is_default`, k.ID, k.Key)
		if e != nil {
			return nil, e
		}
	}
	rows, e := r.sql.QueryContext(ctx, `SELECT p.workspace_id,p.id,w.billing_owner_user_id,p.allowed_group_ids,p.allowed_models,k.status,k.group_id,u.status,m.status,w.status,p.status,b.status,bm.status,to_jsonb(g)
 FROM api_keys k JOIN projects p ON p.id=k.project_id JOIN workspaces w ON w.id=p.workspace_id
 JOIN users u ON u.id=k.user_id AND u.deleted_at IS NULL
 JOIN workspace_members m ON m.workspace_id=w.id AND m.user_id=k.user_id
 JOIN users b ON b.id=w.billing_owner_user_id AND b.deleted_at IS NULL
 JOIN workspace_members bm ON bm.workspace_id=w.id AND bm.user_id=b.id
 LEFT JOIN groups g ON g.id=k.group_id AND g.deleted_at IS NULL
 WHERE k.id=$1 AND k.key=$2 AND k.user_id=$3 AND k.deleted_at IS NULL`, k.ID, k.Key, k.UserID)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if e = rows.Err(); e != nil {
			return nil, e
		}
		return nil, service.ErrAPIKeyNotFound
	}
	tenant := &service.TenantContext{}
	var actor, member, workspace, project, payer, billingMember string
	var groupJSON []byte
	e = rows.Scan(&tenant.WorkspaceID, &tenant.ProjectID, &tenant.BillingPrincipalUserID, pq.Array(&tenant.AllowedGroupIDs), pq.Array(&tenant.AllowedModels), &k.Status, &k.GroupID, &actor, &member, &workspace, &project, &payer, &billingMember, &groupJSON)
	if e != nil {
		return nil, e
	}
	if actor != "active" || member != "active" || payer != "active" || billingMember != "active" {
		return nil, service.ErrWorkspaceForbidden
	}
	if workspace != "active" || project != "active" {
		return nil, service.ErrWorkspaceConflict
	}
	if k.Status != service.StatusActive && k.Status != service.StatusAPIKeyExpired && k.Status != service.StatusAPIKeyQuotaExhausted {
		return nil, service.ErrAPIKeyNotFound
	}
	if (k.GroupID == nil && tenant.AllowedGroupIDs != nil) || (k.GroupID != nil && !tenant.AllowsGroup(*k.GroupID)) {
		return nil, service.ErrGroupNotAllowed
	}
	k.Group = nil
	if k.GroupID != nil {
		if len(groupJSON) == 0 {
			return nil, service.ErrGroupNotAllowed
		}
		var entity dbent.Group
		if e = json.Unmarshal(groupJSON, &entity); e != nil {
			return nil, e
		}
		k.Group = groupEntityToService(&entity)
		if !k.Group.IsActive() {
			return nil, service.ErrGroupNotAllowed
		}
	}
	p := tenant.ProjectID
	k.ProjectID = &p
	return tenant, nil
}

// Reads constrain membership, workspace, project and key in SQL, even if an
// earlier service permission check saw a different membership state.
const projectKeyReadWhere = ` FROM api_keys k JOIN projects p ON p.id=k.project_id JOIN workspaces w ON w.id=p.workspace_id JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.status='active' AND m.role IN ('owner','admin','developer') AND u.status='active' AND u.deleted_at IS NULL AND w.id=$2 AND p.id=$3 AND k.deleted_at IS NULL`

const projectKeyColumns = `k.id,k.user_id,k.project_id,k.key,k.name,k.group_id,k.status,k.ip_whitelist,k.ip_blacklist,k.quota,k.quota_used,k.expires_at,k.rate_limit_5h,k.rate_limit_1d,k.rate_limit_7d,k.usage_5h,k.usage_1d,k.usage_7d,k.window_5h_start,k.window_1d_start,k.window_7d_start,k.last_used_at,k.created_at,k.updated_at`

func scanProjectKey(row workspaceScanner) (*service.APIKey, error) {
	k := &service.APIKey{}
	var white, black []byte
	e := row.Scan(&k.ID, &k.UserID, &k.ProjectID, &k.Key, &k.Name, &k.GroupID, &k.Status, &white, &black, &k.Quota, &k.QuotaUsed, &k.ExpiresAt, &k.RateLimit5h, &k.RateLimit1d, &k.RateLimit7d, &k.Usage5h, &k.Usage1d, &k.Usage7d, &k.Window5hStart, &k.Window1dStart, &k.Window7dStart, &k.LastUsedAt, &k.CreatedAt, &k.UpdatedAt)
	if e != nil {
		return nil, e
	}
	if len(white) > 0 {
		if e = json.Unmarshal(white, &k.IPWhitelist); e != nil {
			return nil, e
		}
	}
	if len(black) > 0 {
		if e = json.Unmarshal(black, &k.IPBlacklist); e != nil {
			return nil, e
		}
	}
	return k, nil
}
func (r *apiKeyRepository) ListProjectKeys(ctx context.Context, a, w, p int64, params pagination.PaginationParams) ([]service.APIKey, int64, error) {
	client := clientFromContext(ctx, r.client)
	rows, e := client.QueryContext(ctx, `SELECT count(*)`+projectKeyReadWhere, a, w, p)
	if e != nil {
		return nil, 0, e
	}
	var total int64
	if !rows.Next() {
		_ = rows.Close()
		return nil, 0, service.ErrWorkspaceNotFound
	}
	e = rows.Scan(&total)
	_ = rows.Close()
	if e != nil {
		return nil, 0, e
	}
	rows, e = client.QueryContext(ctx, `SELECT `+projectKeyColumns+projectKeyReadWhere+` ORDER BY k.id DESC LIMIT $4 OFFSET $5`, a, w, p, params.Limit(), params.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	keys := []service.APIKey{}
	for rows.Next() {
		k, e := scanProjectKey(rows)
		if e != nil {
			return nil, 0, e
		}
		keys = append(keys, *k)
	}
	return keys, total, rows.Err()
}
func (r *apiKeyRepository) GetProjectKey(ctx context.Context, a, w, p, id int64) (*service.APIKey, error) {
	rows, e := clientFromContext(ctx, r.client).QueryContext(ctx, `SELECT `+projectKeyColumns+projectKeyReadWhere+` AND k.id=$4`, a, w, p, id)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if e = rows.Err(); e != nil {
			return nil, e
		}
		return nil, service.ErrWorkspaceNotFound
	}
	return scanProjectKey(rows)
}

func (r *apiKeyRepository) WithProjectKeyMutation(ctx context.Context, a, w, p int64, permission string, fn func(context.Context) error) error {
	tx, e := r.client.Tx(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	// User lifecycle locks precede tenant locks, matching foundation mutations.
	rows, e := client.QueryContext(ctx, `SELECT id FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR SHARE`, a)
	if e != nil {
		return e
	}
	active := rows.Next()
	_ = rows.Close()
	if !active {
		return service.ErrWorkspaceNotFound
	}
	rows, e = client.QueryContext(ctx, `SELECT w.status,p.status,m.role,m.status FROM workspaces w JOIN projects p ON p.workspace_id=w.id JOIN workspace_members m ON m.workspace_id=w.id WHERE w.id=$1 AND p.id=$2 AND m.user_id=$3 FOR UPDATE OF w`, w, p, a)
	if e != nil {
		return e
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return service.ErrWorkspaceNotFound
	}
	var ws, ps, role, ms string
	if e = rows.Scan(&ws, &ps, &role, &ms); e != nil {
		return e
	}
	_ = rows.Close()
	ac := &service.WorkspaceAccess{Workspace: &service.Workspace{Status: ws}, Project: &service.Project{Status: ps}, Member: &service.WorkspaceMember{Role: role, Status: ms}}
	if e = service.CheckWorkspacePermission(ac, permission); e != nil {
		return e
	}
	txctx := dbent.NewTxContext(ctx, tx)
	if e = fn(txctx); e != nil {
		return e
	}
	target := service.ProjectKeyMutationTargetFromContext(txctx)
	if target <= 0 {
		return service.ErrWorkspaceInvalid
	}
	if _, e = client.ExecContext(ctx, `INSERT INTO workspace_audit_logs(workspace_id,project_id,actor_user_id,action,target_type,target_id,metadata) VALUES($1,$2,$3,$4,'api_key',$5,'{}'::jsonb)`, w, p, a, permission, target); e != nil {
		return e
	}
	return tx.Commit()
}
func (r *apiKeyRepository) ListWorkspaceKeySecrets(ctx context.Context, w int64) ([]string, error) {
	rows, e := r.sql.QueryContext(ctx, `SELECT k.key FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1 AND k.deleted_at IS NULL`, w)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var key string
		if e = rows.Scan(&key); e != nil {
			return nil, e
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (r *apiKeyRepository) checkProjectKeyBinding(ctx context.Context, k *service.APIKey) error {
	_, w, p, _, scoped := service.ProjectKeyScopeFromContext(ctx)
	if !scoped {
		return nil
	}
	rows, e := clientFromContext(ctx, r.client).QueryContext(ctx, `SELECT CASE WHEN $3::bigint IS NULL THEN p.allowed_group_ids IS NULL ELSE
 (p.allowed_group_ids IS NULL OR $3=ANY(p.allowed_group_ids)) AND EXISTS(
 SELECT 1 FROM groups g JOIN users b ON b.id=w.billing_owner_user_id
 WHERE g.id=$3 AND g.status='active' AND g.deleted_at IS NULL AND b.status='active' AND b.deleted_at IS NULL
 AND CASE WHEN g.subscription_type='subscription' THEN EXISTS(SELECT 1 FROM user_subscriptions s WHERE s.user_id=b.id AND s.group_id=g.id AND s.status='active' AND s.expires_at>now() AND s.deleted_at IS NULL)
 ELSE (NOT g.is_exclusive AND NOT b.restrict_public_groups) OR EXISTS(SELECT 1 FROM user_allowed_groups ag WHERE ag.user_id=b.id AND ag.group_id=g.id) END) END
 FROM projects p JOIN workspaces w ON w.id=p.workspace_id WHERE p.id=$1 AND w.id=$2`, p, w, k.GroupID)
	if e != nil {
		return e
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return service.ErrWorkspaceNotFound
	}
	var allowed bool
	if e = rows.Scan(&allowed); e != nil {
		return e
	}
	if !allowed {
		return service.ErrGroupNotAllowed
	}
	return nil
}
