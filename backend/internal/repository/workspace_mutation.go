package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"time"
)

func (r *workspaceRepository) Mutate(ctx context.Context, a, w int64, m service.WorkspaceMutation) (*service.WorkspaceMutationResult, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	if m.Action == "workspace.restore" || m.Action == "project.restore" {
		var userID int64
		if e = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR SHARE`, a).Scan(&userID); e != nil {
			return nil, workspaceError(e)
		}
	}
	if e = lockWorkspace(ctx, tx, w, true); e != nil {
		return nil, e
	}
	projectScope := int64(0)
	if m.Action == "project.update" || m.Action == "project.archive" || m.Action == "project.restore" {
		projectScope = m.TargetID
	}
	ac, e := workspaceAccess(ctx, tx, a, w, projectScope)
	if e != nil {
		return nil, e
	}
	if e = service.CheckWorkspacePermission(ac, service.WorkspaceMutationPermission(m)); e != nil {
		return nil, e
	}
	out := &service.WorkspaceMutationResult{}
	target, id, action := "workspace", w, ""
	var project *int64
	meta := map[string]any{}
	switch m.Action {
	case "workspace.restore", "project.restore":
		if e = lifecycleRestoreChecks(ctx, tx, a, w, ac); e != nil {
			return nil, e
		}
		if m.Action == "workspace.restore" {
			if ac.Workspace.Type != "organization" || ac.Workspace.Status != "archived" {
				return nil, service.ErrWorkspaceConflict
			}
			_, e = tx.ExecContext(ctx, `UPDATE workspaces SET status='active',updated_at=now() WHERE id=$1 AND status='archived'`, w)
			action = "workspace_restored"
		} else {
			if ac.Workspace.Status != "active" || ac.Project == nil || ac.Project.Status != "archived" {
				return nil, service.ErrWorkspaceConflict
			}
			_, e = tx.ExecContext(ctx, `UPDATE projects SET status='active',updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status='archived'`, w, m.TargetID)
			action, target, id = "project_restored", "project", m.TargetID
			project = &m.TargetID
		}
		meta["previous_status"], meta["status"] = "archived", "active"
	case "workspace.project_access_mode.update":
		if ac.Workspace.Type == "personal" || !service.ValidProjectAccessMode(m.ProjectAccessMode) {
			return nil, service.ErrWorkspaceInvalid
		}
		out.Workspace, e = scanWorkspace(tx.QueryRowContext(ctx, `UPDATE workspaces SET project_access_mode=$2,updated_at=now() WHERE id=$1 RETURNING `+workspaceColumns, w, m.ProjectAccessMode))
		action = "workspace_project_access_mode_updated"
		meta["project_access_mode"] = m.ProjectAccessMode
	case "workspace.update":
		if e = service.ValidateWorkspaceNameSlug(m.Name, m.Slug); e != nil {
			return nil, e
		}
		if ac.Workspace.Type == "personal" && m.Slug != ac.Workspace.Slug {
			return nil, service.ErrWorkspaceConflict
		}
		if ac.Workspace.Type == "organization" {
			if e = service.ValidateOrganizationNameSlug(m.Name, m.Slug); e != nil {
				return nil, e
			}
		}
		out.Workspace, e = scanWorkspace(tx.QueryRowContext(ctx, `UPDATE workspaces SET name=$2,slug=$3,updated_at=now() WHERE id=$1 RETURNING `+workspaceColumns, w, m.Name, m.Slug))
		action = "workspace_updated"
	case "workspace.archive":
		if ac.Workspace.Type == "personal" {
			return nil, service.ErrWorkspaceConflict
		}
		_, e = tx.ExecContext(ctx, `UPDATE workspaces SET status='archived',updated_at=now() WHERE id=$1`, w)
		action = "workspace_archived"
	case "project.create", "project.update":
		if e = service.ValidateProjectInput(m.Project); e != nil {
			return nil, e
		}
		p := m.Project
		// Group IDs are references to existing global groups, never provider ownership.
		if len(p.AllowedGroupIDs) > 0 {
			var n int
			e = tx.QueryRowContext(ctx, `SELECT count(DISTINCT id) FROM groups WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(p.AllowedGroupIDs)).Scan(&n)
			if e != nil {
				return nil, e
			}
			unique := map[int64]bool{}
			for _, id := range p.AllowedGroupIDs {
				unique[id] = true
			}
			if n != len(unique) {
				return nil, service.ErrWorkspaceInvalid
			}
		}
		if m.Action == "project.create" {
			out.Project, e = scanProject(tx.QueryRowContext(ctx, `INSERT INTO projects(workspace_id,name,slug,description,created_by_user_id,allowed_group_ids,allowed_models) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+projectColumns, w, p.Name, p.Slug, p.Description, a, pq.Array(p.AllowedGroupIDs), pq.Array(p.AllowedModels)))
			action = "project_created"
		} else {
			old, err := scanProject(tx.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE workspace_id=$1 AND id=$2`, w, m.TargetID))
			if err != nil {
				return nil, err
			}
			if old.Status != "active" {
				return nil, service.ErrWorkspaceConflict
			}
			out.Project, e = scanProject(tx.QueryRowContext(ctx, `UPDATE projects SET name=$3,slug=$4,description=$5,allowed_group_ids=$6,allowed_models=$7,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+projectColumns, w, m.TargetID, p.Name, p.Slug, p.Description, pq.Array(p.AllowedGroupIDs), pq.Array(p.AllowedModels)))
			action = "project_updated"
		}
		if e == nil {
			target, id = "project", out.Project.ID
			project = &out.Project.ID
		}
	case "project.archive":
		p, err := scanProject(tx.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE workspace_id=$1 AND id=$2`, w, m.TargetID))
		if err != nil {
			return nil, err
		}
		if p.IsDefault {
			return nil, service.ErrWorkspaceConflict
		}
		_, e = tx.ExecContext(ctx, `UPDATE projects SET status='archived',updated_at=now() WHERE workspace_id=$1 AND id=$2`, w, m.TargetID)
		action, target, id = "project_archived", "project", m.TargetID
		project = &m.TargetID
	case "team.create":
		if ac.Workspace.Type == "personal" || service.ValidateWorkspaceTeamInput(m.Team) != nil {
			return nil, service.ErrWorkspaceInvalid
		}
		out.Team, e = scanTeam(tx.QueryRowContext(ctx, `INSERT INTO workspace_teams(workspace_id,name,slug,description) VALUES($1,$2,$3,$4) RETURNING `+teamColumns, w, m.Team.Name, m.Team.Slug, m.Team.Description))
		action, target, id = "team_created", "team", out.Team.ID
		meta["team_id"] = out.Team.ID
	case "team.update":
		if ac.Workspace.Type == "personal" || service.ValidateWorkspaceTeamInput(m.Team) != nil {
			return nil, service.ErrWorkspaceInvalid
		}
		team, err := scanTeam(tx.QueryRowContext(ctx, `SELECT `+teamColumns+` FROM workspace_teams WHERE workspace_id=$1 AND id=$2`, w, m.TargetID))
		if err != nil {
			return nil, err
		}
		if team.Status != "active" {
			return nil, service.ErrWorkspaceConflict
		}
		out.Team, e = scanTeam(tx.QueryRowContext(ctx, `UPDATE workspace_teams SET name=$3,slug=$4,description=$5,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING `+teamColumns, w, m.TargetID, m.Team.Name, m.Team.Slug, m.Team.Description))
		action, target, id = "team_updated", "team", m.TargetID
		meta["team_id"] = m.TargetID
	case "team.archive":
		if ac.Workspace.Type == "personal" {
			return nil, service.ErrWorkspaceConflict
		}
		team, err := scanTeam(tx.QueryRowContext(ctx, `SELECT `+teamColumns+` FROM workspace_teams WHERE workspace_id=$1 AND id=$2`, w, m.TargetID))
		if err != nil {
			return nil, err
		}
		if team.Status != "active" {
			return nil, service.ErrWorkspaceConflict
		}
		_, e = tx.ExecContext(ctx, `UPDATE workspace_teams SET status='archived',updated_at=now() WHERE workspace_id=$1 AND id=$2`, w, m.TargetID)
		action, target, id = "team_archived", "team", m.TargetID
		meta["team_id"] = m.TargetID
	case "team.member.add", "team.member.remove":
		if ac.Workspace.Type == "personal" || m.TargetID <= 0 || m.SubjectID <= 0 {
			return nil, service.ErrWorkspaceInvalid
		}
		var teamStatus string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM workspace_teams WHERE workspace_id=$1 AND id=$2`, w, m.TargetID).Scan(&teamStatus); e != nil {
			return nil, workspaceError(e)
		}
		if teamStatus != "active" {
			return nil, service.ErrWorkspaceConflict
		}
		var memberStatus string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM workspace_members WHERE workspace_id=$1 AND id=$2`, w, m.SubjectID).Scan(&memberStatus); e != nil {
			return nil, workspaceError(e)
		}
		if memberStatus != "active" {
			return nil, service.ErrWorkspaceConflict
		}
		if m.Action == "team.member.add" {
			_, e = tx.ExecContext(ctx, `INSERT INTO workspace_team_membership_sources(workspace_id,team_id,member_id,source_type) VALUES($1,$2,$3,'manual') ON CONFLICT(workspace_id,member_id,team_id) WHERE source_type='manual' DO NOTHING`, w, m.TargetID, m.SubjectID)
			action = "team_member_added"
		} else {
			_, e = tx.ExecContext(ctx, `DELETE FROM workspace_team_membership_sources WHERE workspace_id=$1 AND team_id=$2 AND member_id=$3 AND source_type='manual'`, w, m.TargetID, m.SubjectID)
			action = "team_member_removed"
		}
		if e == nil {
			e = reconcileWorkspaceTeamSources(ctx, tx, w, m.SubjectID)
		}
		target, id, meta["team_id"], meta["member_id"] = "team", m.TargetID, m.TargetID, m.SubjectID
	case "project_access.grant.create", "project_access.grant.update", "project_access.grant.delete":
		if ac.Workspace.Type == "personal" || m.ProjectID <= 0 {
			return nil, service.ErrWorkspaceInvalid
		}
		var projectStatus string
		if e = tx.QueryRowContext(ctx, `SELECT status FROM projects WHERE workspace_id=$1 AND id=$2`, w, m.ProjectID).Scan(&projectStatus); e != nil {
			return nil, workspaceError(e)
		}
		if projectStatus != "active" {
			return nil, service.ErrWorkspaceConflict
		}
		if m.Action == "project_access.grant.delete" {
			if _, e = scanProjectAccessGrant(tx.QueryRowContext(ctx, `SELECT `+grantColumns+` FROM project_access_grants WHERE workspace_id=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, w, m.ProjectID, m.TargetID)); e != nil {
				return nil, e
			}
			_, e = tx.ExecContext(ctx, `DELETE FROM project_access_grants WHERE workspace_id=$1 AND project_id=$2 AND id=$3`, w, m.ProjectID, m.TargetID)
			action, target, id = "project_access_grant_deleted", "project_access_grant", m.TargetID
			meta["grant_id"], meta["project_id"] = m.TargetID, m.ProjectID
		} else {
			if service.ValidateProjectAccessGrantInput(m.Grant) != nil {
				return nil, service.ErrWorkspaceInvalid
			}
			if m.Grant.SubjectType == service.ProjectAccessSubjectMember {
				var status string
				if e = tx.QueryRowContext(ctx, `SELECT status FROM workspace_members WHERE workspace_id=$1 AND id=$2`, w, m.Grant.SubjectID).Scan(&status); e != nil {
					return nil, workspaceError(e)
				}
				if status != "active" {
					return nil, service.ErrWorkspaceConflict
				}
			} else {
				var status string
				if e = tx.QueryRowContext(ctx, `SELECT status FROM workspace_teams WHERE workspace_id=$1 AND id=$2`, w, m.Grant.SubjectID).Scan(&status); e != nil {
					return nil, workspaceError(e)
				}
				if status != "active" {
					return nil, service.ErrWorkspaceConflict
				}
			}
			if m.Action == "project_access.grant.create" {
				out.Grant, e = scanProjectAccessGrant(tx.QueryRowContext(ctx, `INSERT INTO project_access_grants(workspace_id,project_id,subject_type,subject_id,role,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+grantColumns, w, m.ProjectID, m.Grant.SubjectType, m.Grant.SubjectID, m.Grant.Role, a))
				action, target, id = "project_access_grant_created", "project_access_grant", out.Grant.ID
			} else {
				if _, err := scanProjectAccessGrant(tx.QueryRowContext(ctx, `SELECT `+grantColumns+` FROM project_access_grants WHERE workspace_id=$1 AND project_id=$2 AND id=$3 FOR UPDATE`, w, m.ProjectID, m.TargetID)); err != nil {
					return nil, err
				}
				out.Grant, e = scanProjectAccessGrant(tx.QueryRowContext(ctx, `UPDATE project_access_grants SET subject_type=$4,subject_id=$5,role=$6,updated_at=now() WHERE workspace_id=$1 AND project_id=$2 AND id=$3 RETURNING `+grantColumns, w, m.ProjectID, m.TargetID, m.Grant.SubjectType, m.Grant.SubjectID, m.Grant.Role))
				action, target, id = "project_access_grant_updated", "project_access_grant", m.TargetID
			}
			meta["grant_id"], meta["project_id"], meta["subject_type"] = id, m.ProjectID, m.Grant.SubjectType
		}
		project = &m.ProjectID
	case "member.update", "member.remove":
		if ac.Workspace.Type == "personal" {
			return nil, service.ErrWorkspaceConflict
		}
		member, err := scanMember(tx.QueryRowContext(ctx, `SELECT `+memberColumns+` FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, w, m.TargetID))
		if err != nil {
			return nil, err
		}
		if (member.Role == "owner" || m.Role == "owner") && !service.HasWorkspacePermission(ac.Member.Role, "owner.manage") {
			return nil, service.ErrWorkspaceForbidden
		}
		if m.Action == "member.update" && (!service.ValidWorkspaceRole(m.Role) || (m.Status != "active" && m.Status != "suspended")) {
			return nil, service.ErrWorkspaceInvalid
		}
		removing := m.Action == "member.remove" || m.Status != "active"
		if removing && m.TargetID == ac.Workspace.BillingOwnerUserID {
			return nil, service.ErrWorkspaceConflict
		}
		if member.Role == "owner" && (removing || m.Role != "owner") {
			var replacement int64
			err = tx.QueryRowContext(ctx, `SELECT m.user_id FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id<>$2 AND m.role='owner' AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL ORDER BY m.user_id LIMIT 1`, w, m.TargetID).Scan(&replacement)
			if err == sql.ErrNoRows {
				return nil, service.ErrWorkspaceConflict
			}
			if err != nil {
				return nil, err
			}
			if ac.Workspace.OwnerUserID == m.TargetID {
				if _, err = tx.ExecContext(ctx, `UPDATE workspaces SET owner_user_id=$2,updated_at=now() WHERE id=$1`, w, replacement); err != nil {
					return nil, err
				}
			}
		}
		if m.Action == "member.remove" {
			_, e = tx.ExecContext(ctx, `DELETE FROM workspace_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND source_type='manual'`, w, member.ID)
			if e == nil {
				_, e = tx.ExecContext(ctx, `UPDATE workspace_members SET administratively_suspended=true,administratively_removed=true,status='suspended',effective_membership_source_id=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2`, w, member.ID)
			}
			action = "member_removed"
		} else {
			if m.Status == "active" {
				var exists bool
				err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL)`, m.TargetID).Scan(&exists)
				if err != nil {
					return nil, err
				}
				if !exists {
					return nil, service.ErrWorkspaceConflict
				}
				if e = checkWorkspaceMemberAdmissionTx(ctx, tx, w, service.WorkspaceMemberAdmissionRequest{Source: service.AdmissionAdminRestore, MemberID: member.ID, UserID: m.TargetID}); e != nil {
					return nil, e
				}
			}
			e = upsertManualMemberSource(ctx, tx, w, member.ID, m.Role, true)
			if e == nil {
				_, e = tx.ExecContext(ctx, `UPDATE workspace_members SET administratively_suspended=$3,administratively_removed=CASE WHEN $3 THEN administratively_removed ELSE false END WHERE workspace_id=$1 AND id=$2`, w, member.ID, m.Status != "active")
			}
			if e == nil {
				e = reconcileWorkspaceMemberSources(ctx, tx, w, member.ID)
			}
			action = "member_role_changed"
			meta["role"] = m.Role
			meta["status"] = m.Status
		}
		target, id = "user", m.TargetID
	case "billing.owner.update":
		if ac.Workspace.Type == "personal" {
			return nil, service.ErrWorkspaceConflict
		}
		var exists bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL)`, w, m.TargetID).Scan(&exists)
		if e != nil {
			return nil, e
		}
		if !exists {
			return nil, service.ErrWorkspaceConflict
		}
		_, e = tx.ExecContext(ctx, `UPDATE workspaces SET billing_owner_user_id=$2,updated_at=now() WHERE id=$1`, w, m.TargetID)
		action = "billing_owner_changed"
		meta["previous_user_id"] = ac.Workspace.BillingOwnerUserID
		meta["user_id"] = m.TargetID
	case "member.invite":
		if ac.Workspace.Type == "personal" {
			return nil, service.ErrWorkspaceConflict
		}
		target = "invitation"
		if m.TargetID > 0 {
			inv, err := scanInvitation(tx.QueryRowContext(ctx, `SELECT `+invitationColumns+` FROM workspace_invitations WHERE workspace_id=$1 AND id=$2`, w, m.TargetID))
			if err != nil {
				return nil, err
			}
			if inv.Role == "owner" && !service.HasWorkspacePermission(ac.Member.Role, "owner.manage") {
				return nil, service.ErrWorkspaceForbidden
			}
			if inv.AcceptedAt != nil || inv.RevokedAt != nil {
				return nil, service.ErrWorkspaceConflict
			}
			_, e = tx.ExecContext(ctx, `UPDATE workspace_invitations SET revoked_at=now() WHERE workspace_id=$1 AND id=$2`, w, m.TargetID)
			action, id = "invitation_revoked", m.TargetID
		} else {
			normalized, err := service.NormalizeWorkspaceInvitationEmail(m.Email)
			if err != nil {
				return nil, err
			}
			if !service.ValidWorkspaceRole(m.Role) || len(m.TokenHash) != 32 || !m.ExpiresAt.After(time.Now()) || m.ExpiresAt.After(time.Now().Add(30*24*time.Hour)) {
				return nil, service.ErrWorkspaceInvalid
			}
			if m.Role == "owner" && !service.HasWorkspacePermission(ac.Member.Role, "owner.manage") {
				return nil, service.ErrWorkspaceForbidden
			}
			if e = checkWorkspaceMemberAdmissionTx(ctx, tx, w, service.WorkspaceMemberAdmissionRequest{Source: service.AdmissionInvitationCreate, Email: normalized}); e != nil {
				return nil, e
			}
			var exists bool
			e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND lower(trim(u.email))=$2 AND u.deleted_at IS NULL)`, w, normalized).Scan(&exists)
			if e != nil {
				return nil, e
			}
			if exists {
				return nil, service.ErrWorkspaceConflict
			}
			_, e = tx.ExecContext(ctx, `UPDATE workspace_invitations SET revoked_at=now() WHERE workspace_id=$1 AND email=$2 AND expires_at<=now() AND accepted_at IS NULL AND revoked_at IS NULL`, w, normalized)
			if e != nil {
				return nil, e
			}
			out.Invitation, e = scanInvitation(tx.QueryRowContext(ctx, `INSERT INTO workspace_invitations(workspace_id,email,role,token_hash,invited_by_user_id,expires_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+invitationColumns, w, normalized, m.Role, m.TokenHash, a, m.ExpiresAt))
			action = "member_invited"
			if e == nil {
				id = out.Invitation.ID
			}
			meta["role"] = m.Role
		}
	default:
		return nil, service.ErrWorkspaceInvalid
	}
	if e != nil {
		return nil, workspaceError(e)
	}
	if e = appendWorkspaceAudit(ctx, tx, w, a, project, action, target, id, meta); e != nil {
		return nil, e
	}
	if e = insertWorkspaceMutationEvent(ctx, tx, w, valueOrWorkspaceProject(project), a, action, target, id, mutationEventData(out, meta)); e != nil {
		return nil, e
	}
	if out.Workspace != nil {
		out.Workspace.Permissions = ac.Permissions
	}
	return out, workspaceError(tx.Commit())
}

func valueOrWorkspaceProject(project *int64) int64 {
	if project == nil {
		return 0
	}
	return *project
}

func mutationEventData(out *service.WorkspaceMutationResult, meta map[string]any) service.DomainEventData {
	data := service.DomainEventData{}
	for k, v := range meta {
		switch k {
		case "name", "slug", "status", "role", "user_id", "member_id", "invitation_id", "key_id", "key_name", "project_id", "workspace_id", "scope_type", "scope_id", "period_start", "policy_revision", "threshold", "amount", "spent", "reserved", "estimated_amount", "actual_amount", "reason_code", "request_id", "task_id", "model", "platform", "previous_status", "category", "team_id", "grant_id", "subject_type", "project_access_mode":
			data[k] = v
		case "previous_user_id":
			data["user_id"] = v
		}
	}
	if out != nil && out.Workspace != nil {
		data["name"], data["slug"], data["status"] = out.Workspace.Name, out.Workspace.Slug, out.Workspace.Status
	}
	if out != nil && out.Project != nil {
		data["name"], data["slug"], data["status"], data["project_id"] = out.Project.Name, out.Project.Slug, out.Project.Status, out.Project.ID
	}
	if out != nil && out.Invitation != nil {
		data["invitation_id"], data["role"] = out.Invitation.ID, out.Invitation.Role
	}
	return data
}

func (r *workspaceRepository) AcceptInvitation(ctx context.Context, a int64, hash []byte) (*service.Workspace, error) {
	if len(hash) != 32 {
		return nil, service.ErrWorkspaceNotFound
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	// Lock accepting user before workspace to serialize global deactivation and
	// membership creation. Token hash is the sole lookup credential, never an ID.
	var email string
	e = tx.QueryRowContext(ctx, `SELECT lower(trim(email)) FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR UPDATE`, a).Scan(&email)
	if e != nil {
		return nil, workspaceError(e)
	}
	var w int64
	e = tx.QueryRowContext(ctx, `SELECT workspace_id FROM workspace_invitations WHERE token_hash=$1`, hash).Scan(&w)
	if e != nil {
		return nil, workspaceError(e)
	}
	if e = lockWorkspace(ctx, tx, w, true); e != nil {
		return nil, e
	}
	inv, e := scanInvitation(tx.QueryRowContext(ctx, `SELECT `+invitationColumns+` FROM workspace_invitations WHERE workspace_id=$1 AND token_hash=$2 FOR UPDATE`, w, hash))
	if e != nil {
		return nil, e
	}
	if inv.Email != email {
		return nil, service.ErrWorkspaceForbidden
	}
	if inv.AcceptedAt != nil || inv.RevokedAt != nil || !inv.ExpiresAt.After(time.Now()) {
		return nil, service.ErrWorkspaceConflict
	}
	inviter, e := workspaceAccess(ctx, tx, inv.InvitedByUserID, w, 0)
	if e != nil {
		return nil, service.ErrWorkspaceConflict
	}
	if e = service.CheckWorkspacePermission(inviter, "member.invite"); e != nil {
		return nil, service.ErrWorkspaceConflict
	}
	if inv.Role == "owner" && !service.HasWorkspacePermission(inviter.Member.Role, "owner.manage") {
		return nil, service.ErrWorkspaceConflict
	}
	if e = checkWorkspaceMemberAdmissionTx(ctx, tx, w, service.WorkspaceMemberAdmissionRequest{Source: service.AdmissionInvitationAccept, UserID: a}); e != nil {
		return nil, e
	}
	if e = evaluateWorkspaceSecurityTx(ctx, tx, w, a); e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role,invited_by_user_id) VALUES($1,$2,$3,$4)`, w, a, inv.Role, inv.InvitedByUserID)
	if e != nil {
		return nil, workspaceError(e)
	}
	_, e = tx.ExecContext(ctx, `UPDATE workspace_invitations SET accepted_at=now() WHERE workspace_id=$1 AND id=$2 AND accepted_at IS NULL`, w, inv.ID)
	if e != nil {
		return nil, e
	}
	if e = appendWorkspaceAudit(ctx, tx, w, a, nil, "member_joined", "user", a, map[string]any{"role": inv.Role}); e != nil {
		return nil, e
	}
	if e = insertWorkspaceMutationEvent(ctx, tx, w, 0, a, "member_joined", "user", a, service.DomainEventData{"role": inv.Role, "user_id": a}); e != nil {
		return nil, e
	}
	ws := inviter.Workspace
	ws.Permissions = service.WorkspacePermissions(inv.Role)
	return ws, workspaceError(tx.Commit())
}
