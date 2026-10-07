package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseSCIMRepository struct{ db *sql.DB }

func NewEnterpriseSCIMRepository(db *sql.DB) service.SCIMRepository {
	return &enterpriseSCIMRepository{db: db}
}

var _ service.SCIMRepository = (*enterpriseSCIMRepository)(nil)

func scimError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, sql.ErrNoRows) {
		return service.NewSCIMError(404, "", "resource not found")
	}
	var p *pq.Error
	if errors.As(e, &p) {
		switch p.Code {
		case "23505":
			return service.NewSCIMError(409, "uniqueness", "resource already exists")
		case "23503", "23514":
			return service.NewSCIMError(400, "invalidValue", "invalid scoped resource reference")
		}
	}
	return e
}
func (r *enterpriseSCIMRepository) beginControl(ctx context.Context, w, a int64, permission string) (*sql.Tx, error) {
	tx, ac, e := (&workspaceWebhookRepository{db: r.db}).beginScoped(ctx, a, w, permission)
	if e != nil {
		return nil, e
	}
	if ac.Workspace.Type != service.WorkspaceTypeOrganization || ac.Workspace.Status != "active" {
		_ = tx.Rollback()
		return nil, service.ErrWorkspaceForbidden
	}
	return tx, nil
}
func (r *enterpriseSCIMRepository) beginSync(ctx context.Context, p *service.SCIMPrincipal, write bool) (*sql.Tx, error) {
	if p == nil || p.WorkspaceID <= 0 || p.ConnectorID <= 0 || p.TokenID <= 0 || p.ConnectorRevision <= 0 {
		return nil, service.NewSCIMError(401, "", "invalid provisioning credential")
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = lockWorkspace(ctx, tx, p.WorkspaceID, write); e != nil {
		_ = tx.Rollback()
		return nil, e
	}
	var live bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_scim_connectors c JOIN workspaces w ON w.id=c.workspace_id JOIN workspace_scim_tokens t ON t.workspace_id=c.workspace_id AND t.connector_id=c.id WHERE c.workspace_id=$1 AND c.id=$2 AND c.revision=$3 AND c.public_endpoint_id=$4 AND c.status='active' AND w.type='organization' AND w.status='active' AND t.id=$5 AND t.status='active' AND (t.expires_at IS NULL OR t.expires_at>clock_timestamp()))`, p.WorkspaceID, p.ConnectorID, p.ConnectorRevision, p.PublicID, p.TokenID).Scan(&live)
	if e != nil || !live {
		_ = tx.Rollback()
		if e != nil {
			return nil, e
		}
		return nil, service.NewSCIMError(401, "", "invalid provisioning credential")
	}
	return tx, nil
}

const scimConnectorColumns = `id,workspace_id,revision,public_endpoint_id,name,status,default_role,group_mode,created_at,updated_at,disabled_at,last_sync_at,COALESCE(last_error_code,''),failure_count`

func scanSCIMConnector(row workspaceScanner) (*service.SCIMConnector, error) {
	c := &service.SCIMConnector{}
	e := row.Scan(&c.ID, &c.WorkspaceID, &c.Revision, &c.PublicID, &c.Name, &c.Status, &c.DefaultRole, &c.GroupMode, &c.CreatedAt, &c.UpdatedAt, &c.DisabledAt, &c.LastSyncAt, &c.LastErrorCode, &c.FailureCount)
	return c, scimError(e)
}
func scimConnector(ctx context.Context, q workspaceSQL, w, c int64) (*service.SCIMConnector, error) {
	return scanSCIMConnector(q.QueryRowContext(ctx, `SELECT `+scimConnectorColumns+` FROM workspace_scim_connectors WHERE workspace_id=$1 AND id=$2`, w, c))
}
func appendSCIMMutation(ctx context.Context, tx *sql.Tx, w, a int64, action, event, target string, id int64, data service.DomainEventData) error {
	raw, e := json.Marshal(data)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspace_audit_logs(workspace_id,actor_user_id,action,target_type,target_id,metadata) VALUES($1,NULLIF($2,0),$3,$4,$5,$6)`, w, a, action, target, id, raw)
	if e != nil {
		return e
	}
	if event == "" {
		return nil
	}
	entry, e := service.NewDomainEvent(event, w, 0, a, target, fmt.Sprint(id), data)
	if e != nil {
		return e
	}
	return insertDomainEventTx(ctx, tx, entry, "")
}
func (r *enterpriseSCIMRepository) ListConnectors(ctx context.Context, w, a int64) ([]service.SCIMConnector, error) {
	tx, e := r.beginControl(ctx, w, a, "provisioning.read")
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	rows, e := tx.QueryContext(ctx, `SELECT `+scimConnectorColumns+` FROM workspace_scim_connectors WHERE workspace_id=$1 ORDER BY id LIMIT 100`, w)
	if e != nil {
		return nil, e
	}
	out := []service.SCIMConnector{}
	for rows.Next() {
		c, e := scanSCIMConnector(rows)
		if e != nil {
			_ = rows.Close()
			return nil, e
		}
		out = append(out, *c)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, e
	}
	return out, tx.Commit()
}
func (r *enterpriseSCIMRepository) CreateConnector(ctx context.Context, w, a int64, in service.SCIMConnectorInput) (*service.SCIMConnector, error) {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 120 || !validOIDCRoleForRepository(in.DefaultRole) {
		return nil, service.NewSCIMError(400, "invalidValue", "invalid connector configuration")
	}
	tx, e := r.beginControl(ctx, w, a, "provisioning.manage")
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	id, _, e := service.NewSecureToken(32)
	if e != nil {
		return nil, e
	}
	c, e := scanSCIMConnector(tx.QueryRowContext(ctx, `INSERT INTO workspace_scim_connectors(workspace_id,public_endpoint_id,name,default_role,created_by_user_id) VALUES($1,$2,$3,$4,$5) RETURNING `+scimConnectorColumns, w, id, in.Name, in.DefaultRole, a))
	if e != nil {
		return nil, e
	}
	if e = appendSCIMMutation(ctx, tx, w, a, "scim.connector_created", service.EventSCIMConnectorCreated, "scim_connector", c.ID, service.DomainEventData{"category": "scim", "connector_id": c.ID}); e != nil {
		return nil, e
	}
	return c, tx.Commit()
}
func (r *enterpriseSCIMRepository) UpdateConnector(ctx context.Context, w, a, id int64, in service.SCIMConnectorInput) (*service.SCIMConnector, error) {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 120 || !validOIDCRoleForRepository(in.DefaultRole) {
		return nil, service.NewSCIMError(400, "invalidValue", "invalid connector configuration")
	}
	tx, e := r.beginControl(ctx, w, a, "provisioning.manage")
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	current, e := scimConnector(ctx, tx, w, id)
	if e != nil {
		return nil, e
	}
	if current.Revision != in.Revision {
		return nil, service.ErrWorkspaceConflict
	}
	if current.DefaultRole != in.DefaultRole {
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_membership_sources WHERE workspace_id=$1 AND connector_id=$2`, w, id).Scan(&count); e != nil {
			return nil, e
		}
		if count > 5000 {
			return nil, service.ErrWorkspaceConflict
		}
		if _, e = tx.ExecContext(ctx, `UPDATE workspace_membership_sources SET role=$3 WHERE workspace_id=$1 AND connector_id=$2 AND source_type='scim'`, w, id, in.DefaultRole); e != nil {
			return nil, e
		}
		rows, e := tx.QueryContext(ctx, `SELECT member_id FROM workspace_membership_sources WHERE workspace_id=$1 AND connector_id=$2`, w, id)
		if e != nil {
			return nil, e
		}
		ids := []int64{}
		for rows.Next() {
			var m int64
			if e = rows.Scan(&m); e != nil {
				_ = rows.Close()
				return nil, e
			}
			ids = append(ids, m)
		}
		e = rows.Err()
		_ = rows.Close()
		if e != nil {
			return nil, e
		}
		for _, m := range ids {
			if e = reconcileSCIMMember(ctx, tx, &service.SCIMPrincipal{WorkspaceID: w, ConnectorID: id}, m); e != nil {
				return nil, e
			}
		}
	}
	c, e := scanSCIMConnector(tx.QueryRowContext(ctx, `UPDATE workspace_scim_connectors SET name=$3,default_role=$4,revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+scimConnectorColumns, w, id, in.Name, in.DefaultRole))
	if e != nil {
		return nil, e
	}
	if e = appendSCIMMutation(ctx, tx, w, a, "scim.connector_updated", "", "scim_connector", id, service.DomainEventData{"category": "scim", "connector_id": id}); e != nil {
		return nil, e
	}
	return c, tx.Commit()
}
func (r *enterpriseSCIMRepository) DisableConnector(ctx context.Context, w, a, id, revision int64) error {
	tx, e := r.beginControl(ctx, w, a, "provisioning.manage")
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	c, e := scimConnector(ctx, tx, w, id)
	if e != nil {
		return e
	}
	if c.Revision != revision {
		return service.ErrWorkspaceConflict
	}
	if c.Status == "disabled" {
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `UPDATE workspace_scim_connectors SET status='disabled',disabled_at=now(),updated_at=now(),revision=revision+1 WHERE workspace_id=$1 AND id=$2`, w, id); e != nil {
		return e
	}
	if e = appendSCIMMutation(ctx, tx, w, a, "scim.connector_disabled", service.EventSCIMConnectorDisabled, "scim_connector", id, service.DomainEventData{"category": "scim", "connector_id": id}); e != nil {
		return e
	}
	return tx.Commit()
}

const scimTokenColumns = `id,connector_id,token_prefix,status,created_at,expires_at,revoked_at,last_used_at`

func scanSCIMToken(row workspaceScanner) (*service.SCIMToken, error) {
	t := &service.SCIMToken{}
	e := row.Scan(&t.ID, &t.ConnectorID, &t.Prefix, &t.Status, &t.CreatedAt, &t.ExpiresAt, &t.RevokedAt, &t.LastUsedAt)
	return t, scimError(e)
}
func (r *enterpriseSCIMRepository) ListTokens(ctx context.Context, w, a, c int64) ([]service.SCIMToken, error) {
	tx, e := r.beginControl(ctx, w, a, "provisioning.read")
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	if _, e = scimConnector(ctx, tx, w, c); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+scimTokenColumns+` FROM workspace_scim_tokens WHERE workspace_id=$1 AND connector_id=$2 ORDER BY id DESC LIMIT 100`, w, c)
	if e != nil {
		return nil, e
	}
	out := []service.SCIMToken{}
	for rows.Next() {
		t, e := scanSCIMToken(rows)
		if e != nil {
			_ = rows.Close()
			return nil, e
		}
		out = append(out, *t)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, e
	}
	return out, tx.Commit()
}
func (r *enterpriseSCIMRepository) CreateToken(ctx context.Context, w, a, c int64, prefix string, hash []byte, expiry *time.Time) (*service.SCIMToken, error) {
	if len(hash) != 32 || len(prefix) > 24 || !strings.HasPrefix(prefix, "mrc_scim_") || expiry != nil && (!expiry.After(time.Now()) || expiry.After(time.Now().Add(366*24*time.Hour))) {
		return nil, service.NewSCIMError(400, "invalidValue", "invalid token")
	}
	tx, e := r.beginControl(ctx, w, a, "provisioning.token.rotate")
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	connector, e := scimConnector(ctx, tx, w, c)
	if e != nil {
		return nil, e
	}
	if connector.Status != "active" {
		return nil, service.ErrWorkspaceConflict
	}
	var n int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_scim_tokens WHERE workspace_id=$1 AND connector_id=$2 AND status='active' AND (expires_at IS NULL OR expires_at>now())`, w, c).Scan(&n); e != nil {
		return nil, e
	}
	if n >= 8 {
		return nil, service.ErrWorkspaceConflict
	}
	t, e := scanSCIMToken(tx.QueryRowContext(ctx, `INSERT INTO workspace_scim_tokens(workspace_id,connector_id,token_prefix,token_hash,expires_at,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+scimTokenColumns, w, c, prefix, hash, expiry, a))
	if e != nil {
		return nil, e
	}
	if e = appendSCIMMutation(ctx, tx, w, a, "scim.token_created", service.EventSCIMTokenCreated, "scim_token", t.ID, service.DomainEventData{"category": "scim", "connector_id": c, "token_id": t.ID}); e != nil {
		return nil, e
	}
	return t, tx.Commit()
}
func (r *enterpriseSCIMRepository) RevokeToken(ctx context.Context, w, a, c, t int64) error {
	tx, e := r.beginControl(ctx, w, a, "provisioning.token.rotate")
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	if e = tx.QueryRowContext(ctx, `SELECT status FROM workspace_scim_tokens WHERE workspace_id=$1 AND connector_id=$2 AND id=$3`, w, c, t).Scan(&status); e != nil {
		return scimError(e)
	}
	if status == "revoked" {
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `UPDATE workspace_scim_tokens SET status='revoked',revoked_at=now() WHERE workspace_id=$1 AND connector_id=$2 AND id=$3`, w, c, t); e != nil {
		return e
	}
	if e = appendSCIMMutation(ctx, tx, w, a, "scim.token_revoked", service.EventSCIMTokenRevoked, "scim_token", t, service.DomainEventData{"category": "scim", "connector_id": c, "token_id": t}); e != nil {
		return e
	}
	return tx.Commit()
}
func (r *enterpriseSCIMRepository) Authenticate(ctx context.Context, id string, hash []byte, now time.Time) (*service.SCIMPrincipal, error) {
	if !service.ValidSCIMResourceID(id) || len(hash) != 32 {
		return nil, service.NewSCIMError(401, "", "invalid provisioning credential")
	}
	p := &service.SCIMPrincipal{PublicID: id}
	e := r.db.QueryRowContext(ctx, `SELECT c.workspace_id,c.id,t.id,c.revision FROM workspace_scim_connectors c JOIN workspaces w ON w.id=c.workspace_id JOIN workspace_scim_tokens t ON t.workspace_id=c.workspace_id AND t.connector_id=c.id WHERE c.public_endpoint_id=$1 AND t.token_hash=$2 AND c.status='active' AND w.type='organization' AND w.status='active' AND t.status='active' AND (t.expires_at IS NULL OR t.expires_at>$3)`, id, hash, now).Scan(&p.WorkspaceID, &p.ConnectorID, &p.TokenID, &p.ConnectorRevision)
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return nil, service.NewSCIMError(401, "", "invalid provisioning credential")
		}
		return nil, e
	}
	_, e = r.db.ExecContext(ctx, `UPDATE workspace_scim_tokens SET last_used_at=$2 WHERE id=$1 AND (last_used_at IS NULL OR last_used_at<$2::timestamptz-interval '5 minutes')`, p.TokenID, now)
	return p, e
}
