package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *workspaceRepository) ListWorkspaces(ctx context.Context, a int64, p pagination.PaginationParams) ([]service.Workspace, int64, error) {
	const where = ` FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL`
	var total int64
	if e := r.db.QueryRowContext(ctx, `SELECT count(*)`+where, a).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT w.id,w.name,w.slug,w.type,w.status,w.owner_user_id,w.billing_owner_user_id,w.created_at,w.updated_at,m.role`+where+` ORDER BY w.id LIMIT $2 OFFSET $3`, a, p.Limit(), p.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.Workspace{}
	for rows.Next() {
		var w service.Workspace
		var role string
		if e = rows.Scan(&w.ID, &w.Name, &w.Slug, &w.Type, &w.Status, &w.OwnerUserID, &w.BillingOwnerUserID, &w.CreatedAt, &w.UpdatedAt, &role); e != nil {
			return nil, 0, e
		}
		w.Permissions = service.WorkspaceEffectivePermissions(&service.WorkspaceAccess{Workspace: &w, Member: &service.WorkspaceMember{Role: role, Status: "active"}})
		items = append(items, w)
	}
	return items, total, rows.Err()
}
func workspaceList[T any](ctx context.Context, r *workspaceRepository, a, w int64, p pagination.PaginationParams, permission, table, columns string, scan func(workspaceScanner) (*T, error)) ([]T, int64, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = tx.Rollback() }()
	if e = lockWorkspace(ctx, tx, w, false); e != nil {
		return nil, 0, e
	}
	ac, e := workspaceAccess(ctx, tx, a, w, 0)
	if e != nil {
		return nil, 0, e
	}
	if e = service.CheckWorkspacePermission(ac, permission); e != nil {
		return nil, 0, e
	}
	var total int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE workspace_id=$1`, w).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM `+table+` WHERE workspace_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, w, p.Limit(), p.Offset())
	if e != nil {
		return nil, 0, e
	}
	items := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			_ = rows.Close()
			return nil, 0, err
		}
		items = append(items, *v)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, 0, e
	}
	return items, total, tx.Commit()
}
func (r *workspaceRepository) ListProjects(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.Project, int64, error) {
	return workspaceList(ctx, r, a, w, p, "project.read", "projects", projectColumns, scanProject)
}
func (r *workspaceRepository) ListMembers(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceMember, int64, error) {
	return workspaceList(ctx, r, a, w, p, "member.read", "workspace_members", memberColumns, scanMember)
}
func (r *workspaceRepository) ListInvitations(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceInvitation, int64, error) {
	return workspaceList(ctx, r, a, w, p, "invitation.read", "workspace_invitations", invitationColumns, scanInvitation)
}
func (r *workspaceRepository) ListAudit(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceAudit, int64, error) {
	return workspaceList(ctx, r, a, w, p, "audit.read", "workspace_audit_logs", `id,workspace_id,project_id,actor_user_id,action,target_type,target_id,metadata,created_at`, func(s workspaceScanner) (*service.WorkspaceAudit, error) {
		v := &service.WorkspaceAudit{}
		var b []byte
		e := s.Scan(&v.ID, &v.WorkspaceID, &v.ProjectID, &v.ActorUserID, &v.Action, &v.TargetType, &v.TargetID, &b, &v.CreatedAt)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v.Metadata); e != nil {
			return nil, e
		}
		return v, nil
	})
}
func requireWorkspaceGlobalAdmin(ctx context.Context, q workspaceSQL, a int64) error {
	var ok bool
	if e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)`, a).Scan(&ok); e != nil {
		return e
	}
	if !ok {
		return service.ErrWorkspaceForbidden
	}
	return nil
}
func (r *workspaceRepository) AdminList(ctx context.Context, a int64, p pagination.PaginationParams) ([]service.Workspace, int64, error) {
	if e := requireWorkspaceGlobalAdmin(ctx, r.db, a); e != nil {
		return nil, 0, e
	}
	var total int64
	if e := r.db.QueryRowContext(ctx, `SELECT count(*) FROM workspaces`).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces ORDER BY id DESC LIMIT $1 OFFSET $2`, p.Limit(), p.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.Workspace{}
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, 0, err
		}
		w.Permissions = []string{}
		items = append(items, *w)
	}
	return items, total, rows.Err()
}
func (r *workspaceRepository) AdminInspect(ctx context.Context, a, w int64) (*service.Workspace, error) {
	if e := requireWorkspaceGlobalAdmin(ctx, r.db, a); e != nil {
		return nil, e
	}
	ws, e := scanWorkspace(r.db.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1`, w))
	if ws != nil {
		ws.Permissions = []string{}
	}
	return ws, e
}
func (r *workspaceRepository) AdminSetStatus(ctx context.Context, a, w int64, status string) error {
	if status != "active" && status != "suspended" && status != "archived" {
		return service.ErrWorkspaceInvalid
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	// Hold the global actor identity stable before the tenant lock.
	var adminID int64
	e = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL FOR SHARE`, a).Scan(&adminID)
	if e == sql.ErrNoRows {
		return service.ErrWorkspaceForbidden
	}
	if e != nil {
		return e
	}
	if e = lockWorkspace(ctx, tx, w, true); e != nil {
		return e
	}
	ws, e := scanWorkspace(tx.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1`, w))
	if e != nil {
		return e
	}
	if ws.Status == "archived" && status != "archived" {
		return service.ErrWorkspaceConflict
	}
	if status == "active" {
		var valid bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL) AND EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.role='owner' AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL)`, w, ws.BillingOwnerUserID).Scan(&valid)
		if e != nil {
			return e
		}
		if !valid {
			return service.ErrWorkspaceConflict
		}
	}
	_, e = tx.ExecContext(ctx, `UPDATE workspaces SET status=$2,updated_at=now() WHERE id=$1`, w, status)
	if e != nil {
		return e
	}
	if e = appendWorkspaceAudit(ctx, tx, w, a, nil, "workspace_"+status, "workspace", w, map[string]any{"previous_status": ws.Status, "status": status}); e != nil {
		return e
	}
	return tx.Commit()
}
