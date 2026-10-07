package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type workspaceRepository struct{ db *sql.DB }

func NewWorkspaceRepository(db *sql.DB) service.WorkspaceRepository {
	return &workspaceRepository{db: db}
}

type workspaceSQL interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
type workspaceScanner interface{ Scan(...any) error }

const workspaceColumns = `id,name,slug,type,status,project_access_mode,owner_user_id,billing_owner_user_id,created_at,updated_at`
const projectColumns = `id,workspace_id,name,slug,description,status,is_default,created_by_user_id,allowed_group_ids,allowed_models,created_at,updated_at`
const memberColumns = `id,workspace_id,user_id,role,status,invited_by_user_id,joined_at,created_at,updated_at`
const invitationColumns = `id,workspace_id,email,role,invited_by_user_id,expires_at,accepted_at,revoked_at,created_at`

func scanWorkspace(s workspaceScanner) (*service.Workspace, error) {
	w := &service.Workspace{}
	e := s.Scan(&w.ID, &w.Name, &w.Slug, &w.Type, &w.Status, &w.ProjectAccessMode, &w.OwnerUserID, &w.BillingOwnerUserID, &w.CreatedAt, &w.UpdatedAt)
	return w, workspaceError(e)
}
func scanProject(s workspaceScanner) (*service.Project, error) {
	p := &service.Project{}
	e := s.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.Status, &p.IsDefault, &p.CreatedByUserID, pq.Array(&p.AllowedGroupIDs), pq.Array(&p.AllowedModels), &p.CreatedAt, &p.UpdatedAt)
	return p, workspaceError(e)
}
func scanMember(s workspaceScanner) (*service.WorkspaceMember, error) {
	m := &service.WorkspaceMember{}
	e := s.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.Role, &m.Status, &m.InvitedByUserID, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
	return m, workspaceError(e)
}
func scanInvitation(s workspaceScanner) (*service.WorkspaceInvitation, error) {
	i := &service.WorkspaceInvitation{}
	e := s.Scan(&i.ID, &i.WorkspaceID, &i.Email, &i.Role, &i.InvitedByUserID, &i.ExpiresAt, &i.AcceptedAt, &i.RevokedAt, &i.CreatedAt)
	return i, workspaceError(e)
}
func workspaceError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, sql.ErrNoRows) {
		return service.ErrWorkspaceNotFound
	}
	var p *pq.Error
	if errors.As(e, &p) {
		switch p.Code {
		case "23505", "23503", "23514", "40001", "40P01":
			return service.ErrWorkspaceConflict.WithCause(e)
		case "P0002":
			return service.ErrWorkspaceNotFound
		}
	}
	return e
}
func workspaceAccess(ctx context.Context, q workspaceSQL, a, w, p int64) (*service.WorkspaceAccess, error) {
	// Both membership and the global user must still be usable. A suspended member
	// remains distinguishable from an unrelated user for the 403 contract.
	m, e := scanMember(q.QueryRowContext(ctx, `SELECT m.id,m.workspace_id,m.user_id,m.role,m.status,m.invited_by_user_id,m.joined_at,m.created_at,m.updated_at FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND u.deleted_at IS NULL AND u.status='active' AND COALESCE((to_jsonb(m)->>'administratively_removed')::boolean,false)=false`, w, a))
	if e != nil {
		return nil, e
	}
	ws, e := scanWorkspace(q.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1 AND EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id=$1 AND user_id=$2)`, w, a))
	if e != nil {
		return nil, e
	}
	ac := &service.WorkspaceAccess{Workspace: ws, Member: m, Permissions: service.WorkspacePermissions(m.Role)}
	if m.Status != "active" {
		ac.Permissions = []string{}
	}
	if p != 0 {
		ac.Project, e = scanProject(q.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE workspace_id=$1 AND id=$2`, w, p))
		if e != nil {
			return nil, e
		}
		// Project access is derived from the member's immutable workspace-member
		// row and active team memberships in this same tenant-locked snapshot.
		// A direct grant is an explicit override; otherwise the strongest active
		// team grant applies. Owners/admins and billing retain the documented
		// all-project compatibility semantics.
		if m.Role == "owner" || m.Role == "admin" {
			ac.ProjectRole = service.ProjectAccessRoleAdmin
		} else if m.Role == "billing" {
			ac.ProjectRole = service.ProjectAccessRoleViewer
		} else if ws.ProjectAccessMode == service.ProjectAccessModeAssigned {
			var direct sql.NullString
			if e = q.QueryRowContext(ctx, `SELECT role FROM project_access_grants WHERE workspace_id=$1 AND project_id=$2 AND subject_type='member' AND subject_id=$3`, w, p, m.ID).Scan(&direct); e != nil && !errors.Is(e, sql.ErrNoRows) {
				return nil, workspaceError(e)
			}
			if direct.Valid {
				ac.ProjectRole = direct.String
			} else {
				var teamRole sql.NullString
				e = q.QueryRowContext(ctx, `SELECT g.role FROM project_access_grants g JOIN workspace_team_members tm ON tm.workspace_id=g.workspace_id AND tm.team_id=g.subject_id AND tm.workspace_member_id=$3 JOIN workspace_teams t ON t.workspace_id=g.workspace_id AND t.id=g.subject_id AND t.status='active' WHERE g.workspace_id=$1 AND g.project_id=$2 AND g.subject_type='team' ORDER BY CASE g.role WHEN 'admin' THEN 3 WHEN 'developer' THEN 2 ELSE 1 END DESC, g.id DESC LIMIT 1`, w, p, m.ID).Scan(&teamRole)
				if e != nil && !errors.Is(e, sql.ErrNoRows) {
					return nil, workspaceError(e)
				}
				if teamRole.Valid {
					ac.ProjectRole = teamRole.String
				}
			}
		}
		if ac.ProjectRole != "" {
			ac.ProjectPermissions = service.ProjectRolePermissions(ac.ProjectRole)
		}
	}
	ac.Permissions = service.WorkspaceEffectivePermissions(ac)
	ws.Permissions = ac.Permissions
	return ac, nil
}
func lockWorkspace(ctx context.Context, q workspaceSQL, w int64, write bool) error {
	lock := " FOR SHARE"
	if write {
		lock = " FOR UPDATE"
	}
	var id int64
	return workspaceError(q.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE id=$1`+lock, w).Scan(&id))
}
func (r *workspaceRepository) GetAccess(ctx context.Context, a, w, p int64) (*service.WorkspaceAccess, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	if e = lockWorkspace(ctx, tx, w, false); e != nil {
		return nil, e
	}
	ac, e := workspaceAccess(ctx, tx, a, w, p)
	if e != nil {
		return nil, e
	}
	return ac, tx.Commit()
}
func (r *workspaceRepository) EnsurePersonalWorkspace(ctx context.Context, userID int64) (*service.Workspace, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	if e = tx.QueryRowContext(ctx, `SELECT ensure_personal_workspace($1)`, userID).Scan(&id); e != nil {
		return nil, workspaceError(e)
	}
	w, e := scanWorkspace(tx.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1 AND owner_user_id=$2 AND type='personal'`, id, userID))
	if e != nil {
		return nil, e
	}
	// The personal-workspace insert trigger records creation atomically for
	// both registration and lazy bootstrap, including concurrent first access.
	// Lazy first access fills one bounded page. Remaining keys are picked up by
	// Bootstrap; never rewrite already assigned keys or any historical usage.
	_, e = tx.ExecContext(ctx, `UPDATE api_keys SET project_id=(SELECT id FROM projects WHERE workspace_id=$1 AND is_default) WHERE id IN(SELECT id FROM api_keys WHERE user_id=$2 AND project_id IS NULL ORDER BY id LIMIT 100) AND project_id IS NULL`, id, userID)
	if e != nil {
		return nil, e
	}
	w.Permissions = service.WorkspacePermissions("owner")
	return w, tx.Commit()
}
func (r *workspaceRepository) Bootstrap(ctx context.Context, batchSize int) error {
	if batchSize < 1 || batchSize > 1000 {
		return service.ErrWorkspaceInvalid
	}
	// Separate bounded transactions avoid retaining all user/tenant locks for a
	// large legacy installation. Missing scopes and NULL keys act as the cursor.
	rows, e := r.db.QueryContext(ctx, `SELECT u.id FROM users u WHERE u.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM workspaces w WHERE w.type='personal' AND w.owner_user_id=u.id) ORDER BY u.id LIMIT $1`, batchSize)
	if e != nil {
		return e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			_ = rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if _, e = r.EnsurePersonalWorkspace(ctx, id); e != nil {
			return e
		}
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	_, e = tx.ExecContext(ctx, `WITH batch AS (SELECT k.id,p.id AS project_id FROM api_keys k JOIN users u ON u.id=k.user_id AND u.deleted_at IS NULL JOIN workspaces w ON w.type='personal' AND w.owner_user_id=k.user_id JOIN projects p ON p.workspace_id=w.id AND p.is_default WHERE k.project_id IS NULL ORDER BY k.id LIMIT $1 FOR UPDATE OF k SKIP LOCKED) UPDATE api_keys k SET project_id=b.project_id FROM batch b WHERE k.id=b.id AND k.project_id IS NULL`, batchSize)
	if e != nil {
		return workspaceError(e)
	}
	return tx.Commit()
}
func appendWorkspaceAudit(ctx context.Context, q workspaceSQL, w, a int64, project *int64, action, target string, id int64, metadata map[string]any) error {
	// Metadata is built from a closed field allowlist by mutation code; raw
	// requests, credentials and invitation hashes never reach this function.
	if metadata == nil {
		metadata = map[string]any{}
	}
	b, e := json.Marshal(metadata)
	if e != nil {
		return e
	}
	_, e = q.ExecContext(ctx, `INSERT INTO workspace_audit_logs(workspace_id,project_id,actor_user_id,action,target_type,target_id,metadata) VALUES($1,$2,$3,$4,$5,$6,$7)`, w, project, a, action, target, id, b)
	return workspaceError(e)
}
func (r *workspaceRepository) CreateOrganization(ctx context.Context, a int64, name, slug string) (*service.Workspace, error) {
	if e := service.ValidateOrganizationNameSlug(name, slug); e != nil {
		return nil, e
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	if e = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' FOR UPDATE`, a).Scan(&id); e != nil {
		return nil, workspaceError(e)
	}
	w, e := scanWorkspace(tx.QueryRowContext(ctx, `INSERT INTO workspaces(name,slug,type,owner_user_id,billing_owner_user_id) VALUES($1,$2,'organization',$3,$3) RETURNING `+workspaceColumns, name, slug, a))
	if e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner')`, w.ID, a)
	if e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO projects(workspace_id,name,slug,is_default,created_by_user_id) VALUES($1,'Default','default',true,$2)`, w.ID, a)
	if e != nil {
		return nil, e
	}
	if e = appendWorkspaceAudit(ctx, tx, w.ID, a, nil, "workspace_created", "workspace", w.ID, nil); e != nil {
		return nil, e
	}
	if e = insertWorkspaceMutationEvent(ctx, tx, w.ID, 0, a, "workspace_created", "workspace", w.ID, service.DomainEventData{"name": w.Name, "slug": w.Slug, "status": w.Status}); e != nil {
		return nil, e
	}
	w.Permissions = service.WorkspacePermissions("owner")
	return w, tx.Commit()
}
