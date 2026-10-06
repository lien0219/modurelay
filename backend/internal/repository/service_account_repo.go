package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type serviceAccountRepository struct{ db *sql.DB }

func NewServiceAccountRepository(db *sql.DB) service.ServiceAccountRepository {
	return &serviceAccountRepository{db}
}

const serviceAccountColumns = `s.id,s.workspace_id,s.project_id,s.name,s.slug,s.description,s.status,s.created_by_user_id,s.disabled_at,s.created_at,s.updated_at`
const serviceAccountReadScope = ` FROM service_accounts s JOIN projects p ON p.id=s.project_id AND p.workspace_id=s.workspace_id JOIN workspaces w ON w.id=s.workspace_id JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.status='active' AND m.role IN ('owner','admin','developer','billing','viewer') AND u.status='active' AND u.deleted_at IS NULL AND s.workspace_id=$2 AND s.project_id=$3 AND ` + projectAccessVisibilitySQL

func scanServiceAccount(row workspaceScanner) (*service.ServiceAccount, error) {
	s := &service.ServiceAccount{}
	e := row.Scan(&s.ID, &s.WorkspaceID, &s.ProjectID, &s.Name, &s.Slug, &s.Description, &s.Status, &s.CreatedByUserID, &s.DisabledAt, &s.CreatedAt, &s.UpdatedAt)
	return s, workspaceError(e)
}
func (r *serviceAccountRepository) List(ctx context.Context, a, w, p int64, params pagination.PaginationParams) ([]service.ServiceAccount, int64, error) {
	var total int64
	if e := r.db.QueryRowContext(ctx, `SELECT count(*)`+serviceAccountReadScope, a, w, p).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT `+serviceAccountColumns+serviceAccountReadScope+` ORDER BY s.id DESC LIMIT $4 OFFSET $5`, a, w, p, params.Limit(), params.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	out := []service.ServiceAccount{}
	for rows.Next() {
		s, e := scanServiceAccount(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}
func (r *serviceAccountRepository) Get(ctx context.Context, a, w, p, id int64) (*service.ServiceAccount, error) {
	return scanServiceAccount(r.db.QueryRowContext(ctx, `SELECT `+serviceAccountColumns+serviceAccountReadScope+` AND s.id=$4`, a, w, p, id))
}

const serviceAccountKeyColumns = `k.id,COALESCE(k.user_id,0),k.project_id,k.key,k.name,k.group_id,k.status,k.ip_whitelist,k.ip_blacklist,k.quota,k.quota_used,k.expires_at,k.rate_limit_5h,k.rate_limit_1d,k.rate_limit_7d,k.usage_5h,k.usage_1d,k.usage_7d,k.window_5h_start,k.window_1d_start,k.window_7d_start,k.last_used_at,k.created_at,k.updated_at,k.service_account_id,k.key_suffix`

func scanServiceAccountKey(row workspaceScanner) (*service.APIKey, error) {
	k := &service.APIKey{}
	var white, black []byte
	e := row.Scan(&k.ID, &k.UserID, &k.ProjectID, &k.Key, &k.Name, &k.GroupID, &k.Status, &white, &black, &k.Quota, &k.QuotaUsed, &k.ExpiresAt, &k.RateLimit5h, &k.RateLimit1d, &k.RateLimit7d, &k.Usage5h, &k.Usage1d, &k.Usage7d, &k.Window5hStart, &k.Window1dStart, &k.Window7dStart, &k.LastUsedAt, &k.CreatedAt, &k.UpdatedAt, &k.ServiceAccountID, &k.KeySuffix)
	if e != nil {
		return nil, workspaceError(e)
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
func serviceAccountKeys(ctx context.Context, q workspaceSQL, w, p, id int64) ([]service.APIKey, error) {
	rows, e := q.QueryContext(ctx, `SELECT `+serviceAccountKeyColumns+` FROM api_keys k JOIN service_accounts s ON s.id=k.service_account_id AND s.project_id=k.project_id WHERE s.workspace_id=$1 AND s.project_id=$2 AND s.id=$3 AND k.deleted_at IS NULL ORDER BY k.id DESC`, w, p, id)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []service.APIKey{}
	for rows.Next() {
		k, e := scanServiceAccountKey(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}
func (r *serviceAccountRepository) Credentials(ctx context.Context, a, w, p, id int64) ([]service.APIKey, error) {
	// Membership, central role and parent scope are checked in the same SQL
	// snapshot as the credentials, including empty credential collections.
	tx, e := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	ac, e := workspaceAccess(ctx, tx, a, w, p)
	if e != nil {
		return nil, e
	}
	if e = service.CheckWorkspacePermission(ac, "service_account.credential.read"); e != nil {
		return nil, e
	}
	if _, e = scanServiceAccount(tx.QueryRowContext(ctx, `SELECT `+serviceAccountColumns+serviceAccountReadScope+` AND s.id=$4`, a, w, p, id)); e != nil {
		return nil, e
	}
	keys, e := serviceAccountKeys(ctx, tx, w, p, id)
	if e != nil {
		return nil, e
	}
	return keys, tx.Commit()
}

type serviceAccountTransaction struct {
	tx                                 *sql.Tx
	actor, workspace, project, account int64
	admin                              bool
}

func (r *serviceAccountRepository) WithMutation(ctx context.Context, a, w, p, id int64, permission string, admin bool, fn func(*service.ServiceAccount, *service.WorkspaceAccess, service.ServiceAccountTransaction) error) error {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	if admin {
		if e = requireServiceAccountAdmin(ctx, tx, a); e != nil {
			return e
		}
		// The privileged path derives the tenant from the target, never from a
		// caller-supplied scope; the same parent lock ordering is then used.
		if e = tx.QueryRowContext(ctx, `SELECT workspace_id,project_id FROM service_accounts WHERE id=$1`, id).Scan(&w, &p); e != nil {
			return workspaceError(e)
		}
	}
	if e = lockWorkspace(ctx, tx, w, true); e != nil {
		return e
	}
	var ac *service.WorkspaceAccess
	if !admin {
		ac, e = workspaceAccess(ctx, tx, a, w, p)
		if e != nil {
			return e
		}
		if e = service.CheckWorkspacePermission(ac, permission); e != nil {
			return e
		}
		if permission == "service_account.credential.create" || permission == "service_account.credential.update" || permission == "service_account.credential.rotate" {
			var payerActive bool
			if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspaces w JOIN users b ON b.id=w.billing_owner_user_id JOIN workspace_members bm ON bm.workspace_id=w.id AND bm.user_id=b.id WHERE w.id=$1 AND b.status='active' AND b.deleted_at IS NULL AND bm.status='active')`, w).Scan(&payerActive); e != nil {
				return e
			}
			if !payerActive {
				return service.ErrWorkspaceForbidden
			}
		}
	}
	var account *service.ServiceAccount
	if id > 0 {
		account, e = scanServiceAccount(tx.QueryRowContext(ctx, `SELECT `+serviceAccountColumns+` FROM service_accounts s WHERE s.workspace_id=$1 AND s.project_id=$2 AND s.id=$3 FOR UPDATE`, w, p, id))
		if e != nil {
			return e
		}
	}
	scoped := &serviceAccountTransaction{tx, a, w, p, id, admin}
	if e = fn(account, ac, scoped); e != nil {
		return workspaceError(e)
	}
	return workspaceError(tx.Commit())
}
func (t *serviceAccountTransaction) CountAccounts(ctx context.Context) (int, error) {
	var n int
	e := t.tx.QueryRowContext(ctx, `SELECT count(*) FROM service_accounts WHERE workspace_id=$1 AND project_id=$2`, t.workspace, t.project).Scan(&n)
	return n, e
}
func (t *serviceAccountTransaction) CreateAccount(ctx context.Context, s *service.ServiceAccount) error {
	e := t.tx.QueryRowContext(ctx, `INSERT INTO service_accounts(workspace_id,project_id,name,slug,description,status,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,created_at,updated_at`, t.workspace, t.project, s.Name, s.Slug, s.Description, s.Status, t.actor).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	t.account = s.ID
	return workspaceError(e)
}
func (t *serviceAccountTransaction) SaveAccount(ctx context.Context, s *service.ServiceAccount) error {
	return workspaceError(t.tx.QueryRowContext(ctx, `UPDATE service_accounts SET name=$4,description=$5,status=$6,disabled_at=$7,updated_at=now() WHERE workspace_id=$1 AND project_id=$2 AND id=$3 RETURNING updated_at`, t.workspace, t.project, t.account, s.Name, s.Description, s.Status, s.DisabledAt).Scan(&s.UpdatedAt))
}
func (t *serviceAccountTransaction) Credentials(ctx context.Context) ([]service.APIKey, error) {
	return serviceAccountKeys(ctx, t.tx, t.workspace, t.project, t.account)
}
func (t *serviceAccountTransaction) Credential(ctx context.Context, id int64) (*service.APIKey, error) {
	return scanServiceAccountKey(t.tx.QueryRowContext(ctx, `SELECT `+serviceAccountKeyColumns+` FROM api_keys k JOIN service_accounts s ON s.id=k.service_account_id AND s.project_id=k.project_id WHERE s.workspace_id=$1 AND s.project_id=$2 AND s.id=$3 AND k.id=$4 AND k.deleted_at IS NULL FOR UPDATE OF k`, t.workspace, t.project, t.account, id))
}
func (t *serviceAccountTransaction) CountActiveCredentials(ctx context.Context) (int, error) {
	var n int
	e := t.tx.QueryRowContext(ctx, `SELECT count(*) FROM api_keys WHERE service_account_id=$1 AND project_id=$2 AND deleted_at IS NULL AND status IN ('active','quota_exhausted') AND (expires_at IS NULL OR expires_at>now())`, t.account, t.project).Scan(&n)
	return n, e
}

func (t *serviceAccountTransaction) CheckCredentialBinding(ctx context.Context, key *service.APIKey) error {
	var allowed bool
	if err := t.tx.QueryRowContext(ctx, projectKeyBindingSQL, t.project, t.workspace, key.GroupID).Scan(&allowed); err != nil {
		return workspaceError(err)
	}
	if !allowed {
		return service.ErrGroupNotAllowed
	}
	return nil
}
func serviceAccountACLJSON(k *service.APIKey) ([]byte, []byte, error) {
	white := k.IPWhitelist
	black := k.IPBlacklist
	if white == nil {
		white = []string{}
	}
	if black == nil {
		black = []string{}
	}
	w, e := json.Marshal(white)
	if e != nil {
		return nil, nil, e
	}
	b, e := json.Marshal(black)
	return w, b, e
}
func (t *serviceAccountTransaction) CreateCredential(ctx context.Context, k *service.APIKey) error {
	white, black, e := serviceAccountACLJSON(k)
	if e != nil {
		return e
	}
	return workspaceError(t.tx.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,service_account_id,project_id,key,key_suffix,name,group_id,status,ip_whitelist,ip_blacklist,quota,quota_used,expires_at,rate_limit_5h,rate_limit_1d,rate_limit_7d) VALUES(NULL,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0,$11,$12,$13,$14) RETURNING id,created_at,updated_at`, t.account, t.project, k.Key, k.KeySuffix, k.Name, k.GroupID, k.Status, white, black, k.Quota, k.ExpiresAt, k.RateLimit5h, k.RateLimit1d, k.RateLimit7d).Scan(&k.ID, &k.CreatedAt, &k.UpdatedAt))
}
func (t *serviceAccountTransaction) SaveCredential(ctx context.Context, k *service.APIKey) error {
	white, black, e := serviceAccountACLJSON(k)
	if e != nil {
		return e
	}
	return workspaceError(t.tx.QueryRowContext(ctx, `UPDATE api_keys SET name=$4,group_id=$5,status=$6,ip_whitelist=$7,ip_blacklist=$8,quota=$9,quota_used=$10,expires_at=$11,rate_limit_5h=$12,rate_limit_1d=$13,rate_limit_7d=$14,usage_5h=$15,usage_1d=$16,usage_7d=$17,window_5h_start=$18,window_1d_start=$19,window_7d_start=$20,updated_at=now() WHERE service_account_id=$1 AND project_id=$2 AND id=$3 AND deleted_at IS NULL AND status<>'revoked' RETURNING updated_at`, t.account, t.project, k.ID, k.Name, k.GroupID, k.Status, white, black, k.Quota, k.QuotaUsed, k.ExpiresAt, k.RateLimit5h, k.RateLimit1d, k.RateLimit7d, k.Usage5h, k.Usage1d, k.Usage7d, k.Window5hStart, k.Window1dStart, k.Window7dStart).Scan(&k.UpdatedAt))
}
func (t *serviceAccountTransaction) Event(ctx context.Context, name string, account *service.ServiceAccount, key *service.APIKey, oldID int64) error {
	data := service.DomainEventData{"service_account_id": account.ID, "project_id": t.project, "name": account.Name}
	target := "service_account"
	id := account.ID
	if key != nil {
		data["credential_id"] = key.ID
		data["credential_name"] = key.Name
		target = "service_account_credential"
		id = key.ID
		if key.ExpiresAt != nil {
			data["expires_at"] = key.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	if oldID > 0 {
		data["old_credential_id"] = oldID
		data["new_credential_id"] = key.ID
	}
	action := name
	if t.admin {
		action = "platform." + action
	}
	if e := appendWorkspaceAudit(ctx, t.tx, t.workspace, t.actor, &t.project, action, target, id, map[string]any(data)); e != nil {
		return e
	}
	if t.admin {
		meta, e := json.Marshal(data)
		if e != nil {
			return e
		}
		if _, e = t.tx.ExecContext(ctx, `INSERT INTO audit_logs(actor_user_id,actor_role,auth_method,action,extra,status_code) VALUES($1,'admin','human',$2,$3,200)`, t.actor, action, meta); e != nil {
			return e
		}
	}
	event, e := service.NewDomainEvent(name, t.workspace, t.project, t.actor, target, strconv.FormatInt(id, 10), data)
	if e != nil {
		return e
	}
	return insertDomainEventTx(ctx, t.tx, event, "")
}
func requireServiceAccountAdmin(ctx context.Context, q workspaceSQL, a int64) error {
	var ok bool
	e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)`, a).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return service.ErrWorkspaceForbidden
	}
	return nil
}
func (r *serviceAccountRepository) AdminList(ctx context.Context, a int64, search string, params pagination.PaginationParams) ([]service.ServiceAccount, int64, error) {
	if e := requireServiceAccountAdmin(ctx, r.db, a); e != nil {
		return nil, 0, e
	}
	const where = ` FROM service_accounts s WHERE EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL) AND ($2='' OR s.name ILIKE '%'||$2||'%' OR s.slug ILIKE '%'||$2||'%')`
	var total int64
	if e := r.db.QueryRowContext(ctx, `SELECT count(*)`+where, a, search).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT `+serviceAccountColumns+where+` ORDER BY s.id DESC LIMIT $3 OFFSET $4`, a, search, params.Limit(), params.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	out := []service.ServiceAccount{}
	for rows.Next() {
		s, e := scanServiceAccount(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}
func (r *serviceAccountRepository) AdminGet(ctx context.Context, a, id int64) (*service.ServiceAccount, []service.APIKey, error) {
	tx, e := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, nil, e
	}
	defer func() { _ = tx.Rollback() }()
	if e = requireServiceAccountAdmin(ctx, tx, a); e != nil {
		return nil, nil, e
	}
	s, e := scanServiceAccount(tx.QueryRowContext(ctx, `SELECT `+serviceAccountColumns+` FROM service_accounts s WHERE s.id=$1`, id))
	if e != nil {
		return nil, nil, e
	}
	keys, e := serviceAccountKeys(ctx, tx, s.WorkspaceID, s.ProjectID, id)
	if e != nil {
		return nil, nil, e
	}
	return s, keys, tx.Commit()
}
