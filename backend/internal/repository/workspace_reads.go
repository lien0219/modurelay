package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"strconv"
	"strings"
)

func (r *workspaceRepository) ListWorkspaces(ctx context.Context, a int64, p pagination.PaginationParams) ([]service.Workspace, int64, error) {
	const where = ` FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL`
	var total int64
	if e := r.db.QueryRowContext(ctx, `SELECT count(*)`+where, a).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT w.id,w.name,w.slug,w.type,w.status,w.project_access_mode,w.owner_user_id,w.billing_owner_user_id,w.created_at,w.updated_at,m.role`+where+` ORDER BY w.id LIMIT $2 OFFSET $3`, a, p.Limit(), p.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.Workspace{}
	for rows.Next() {
		var w service.Workspace
		var role string
		if e = rows.Scan(&w.ID, &w.Name, &w.Slug, &w.Type, &w.Status, &w.ProjectAccessMode, &w.OwnerUserID, &w.BillingOwnerUserID, &w.CreatedAt, &w.UpdatedAt, &role); e != nil {
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
	if e = service.CheckWorkspacePermission(ac, "project.read"); e != nil {
		return nil, 0, e
	}
	where := `workspace_id=$1`
	args := []any{w}
	if ac.Workspace.ProjectAccessMode == service.ProjectAccessModeAssigned && ac.Member.Role != "owner" && ac.Member.Role != "admin" && ac.Member.Role != "billing" {
		where += ` AND EXISTS (SELECT 1 FROM project_access_grants g WHERE g.workspace_id=projects.workspace_id AND g.project_id=projects.id AND ((g.subject_type='member' AND g.subject_id=$2) OR (g.subject_type='team' AND EXISTS (SELECT 1 FROM workspace_team_members tm JOIN workspace_teams t ON t.workspace_id=tm.workspace_id AND t.id=tm.team_id AND t.status='active' WHERE tm.workspace_id=g.workspace_id AND tm.team_id=g.subject_id AND tm.workspace_member_id=$2))))`
		args = append(args, ac.Member.ID)
	}
	var total int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM projects WHERE `+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, p.Limit(), p.Offset())
	rows, e := tx.QueryContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE `+where+` ORDER BY id DESC LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.Project{}
	for rows.Next() {
		project, scanErr := scanProject(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *project)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return items, total, tx.Commit()
}
func (r *workspaceRepository) ListMembers(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceMember, int64, error) {
	return workspaceList(ctx, r, a, w, p, "member.read", "workspace_members", memberColumns, scanMember)
}
func (r *workspaceRepository) ListInvitations(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceInvitation, int64, error) {
	return workspaceList(ctx, r, a, w, p, "invitation.read", "workspace_invitations", invitationColumns, scanInvitation)
}
func (r *workspaceRepository) ListAudit(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceAudit, int64, error) {
	return workspaceList(ctx, r, a, w, p, "audit.read", "workspace_audit_logs", `id,workspace_id,project_id,COALESCE(actor_user_id,0),action,target_type,target_id,metadata,created_at`, func(s workspaceScanner) (*service.WorkspaceAudit, error) {
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

const teamColumns = `id,workspace_id,name,slug,description,status,created_at,updated_at`
const grantColumns = `id,workspace_id,project_id,subject_type,subject_id,role,created_by_user_id,created_at,updated_at`

func scanTeam(s workspaceScanner) (*service.WorkspaceTeam, error) {
	t := &service.WorkspaceTeam{}
	e := s.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Slug, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, workspaceError(e)
}

func scanProjectAccessGrant(s workspaceScanner) (*service.ProjectAccessGrant, error) {
	g := &service.ProjectAccessGrant{}
	e := s.Scan(&g.ID, &g.WorkspaceID, &g.ProjectID, &g.SubjectType, &g.SubjectID, &g.Role, &g.CreatedByUserID, &g.CreatedAt, &g.UpdatedAt)
	return g, workspaceError(e)
}

func (r *workspaceRepository) ListTeams(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]service.WorkspaceTeam, int64, error) {
	return workspaceList(ctx, r, a, w, p, "team.read", "workspace_teams", teamColumns, scanTeam)
}

func (r *workspaceRepository) ListTeamMembers(ctx context.Context, a, w, teamID int64, p pagination.PaginationParams) ([]service.WorkspaceTeamMember, int64, error) {
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
	if e = service.CheckWorkspacePermission(ac, "team.read"); e != nil {
		return nil, 0, e
	}
	var total int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND team_id=$2`, w, teamID).Scan(&total); e != nil {
		return nil, 0, workspaceError(e)
	}
	rows, e := tx.QueryContext(ctx, `SELECT tm.team_id,tm.workspace_member_id,m.user_id,m.role,m.status,u.email,tm.created_at FROM workspace_team_members tm JOIN workspace_members m ON m.workspace_id=tm.workspace_id AND m.id=tm.workspace_member_id JOIN users u ON u.id=m.user_id WHERE tm.workspace_id=$1 AND tm.team_id=$2 ORDER BY tm.workspace_member_id LIMIT $3 OFFSET $4`, w, teamID, p.Limit(), p.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.WorkspaceTeamMember{}
	for rows.Next() {
		var item service.WorkspaceTeamMember
		if e = rows.Scan(&item.TeamID, &item.WorkspaceMemberID, &item.UserID, &item.Role, &item.Status, &item.Email, &item.CreatedAt); e != nil {
			return nil, 0, e
		}
		items = append(items, item)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return items, total, tx.Commit()
}

func (r *workspaceRepository) ListProjectAccessGrants(ctx context.Context, a, w, projectID int64, p pagination.PaginationParams) ([]service.ProjectAccessGrant, int64, error) {
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
	if e = service.CheckWorkspacePermission(ac, "project_access.read"); e != nil {
		return nil, 0, e
	}
	var total int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM project_access_grants WHERE workspace_id=$1 AND project_id=$2`, w, projectID).Scan(&total); e != nil {
		return nil, 0, workspaceError(e)
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+strings.TrimSpace(grantColumns)+` FROM project_access_grants WHERE workspace_id=$1 AND project_id=$2 ORDER BY id DESC LIMIT $3 OFFSET $4`, w, projectID, p.Limit(), p.Offset())
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.ProjectAccessGrant{}
	for rows.Next() {
		grant, scanErr := scanProjectAccessGrant(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *grant)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return items, total, tx.Commit()
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
	if e = insertWorkspaceMutationEvent(ctx, tx, w, 0, a, "workspace_"+status, "workspace", w, service.DomainEventData{"previous_status": ws.Status, "status": status}); e != nil {
		return e
	}
	return tx.Commit()
}
