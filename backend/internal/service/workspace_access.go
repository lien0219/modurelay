package service

import (
	"context"
	"strings"
)

// The only tenant role map. Global admin is deliberately not a tenant role.
var workspaceRolePermissions = map[string][]string{
	"owner":     {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "workspace.update", "workspace.archive", "member.read", "invitation.read", "member.invite", "member.update", "member.remove", "owner.manage", "billing.owner.update", "project.read", "project.create", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "billing.read", "budget.read", "budget.update", "audit.read", "webhook.read", "webhook.create", "webhook.update", "webhook.delete", "webhook.secret.rotate", "webhook.test", "webhook.delivery.read", "webhook.delivery.retry"},
	"admin":     {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "workspace.update", "member.read", "invitation.read", "member.invite", "member.update", "member.remove", "project.read", "project.create", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "budget.read", "budget.update", "audit.read", "webhook.read", "webhook.create", "webhook.update", "webhook.delete", "webhook.secret.rotate", "webhook.test", "webhook.delivery.read", "webhook.delivery.retry"},
	"developer": {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "member.read", "project.read", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "budget.read", "webhook.read", "webhook.delivery.read"},
	"billing":   {"service_account.read", "workspace.read", "project.read", "usage.read", "billing.read", "budget.read", "budget.update", "webhook.read"},
	"viewer":    {"service_account.read", "workspace.read", "project.read", "usage.read", "budget.read"},
}

func init() {
	// Policy permissions live in the same central role map as every other
	// tenant capability. Read access is available to all tenant roles; policy
	// mutation is intentionally narrower because it affects all credentials in
	// a scope.
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		workspaceRolePermissions[role] = append(workspaceRolePermissions[role], "policy.read")
	}
	for _, role := range []string{"owner", "admin"} {
		workspaceRolePermissions[role] = append(workspaceRolePermissions[role], "workspace_policy.update", "project_policy.update")
	}
	workspaceRolePermissions["owner"] = append(workspaceRolePermissions["owner"], "service_account_policy.update")
	workspaceRolePermissions["admin"] = append(workspaceRolePermissions["admin"], "service_account_policy.update")
	workspaceRolePermissions["developer"] = append(workspaceRolePermissions["developer"], "service_account_policy.update")
}

func ValidWorkspaceRole(role string) bool { _, ok := workspaceRolePermissions[role]; return ok }
func WorkspacePermissions(role string) []string {
	return append([]string{}, workspaceRolePermissions[role]...)
}

// Effective permissions let panels use the same central RBAC map while
// suspended/archived scopes expose history without advertising write actions.
func WorkspaceEffectivePermissions(a *WorkspaceAccess) []string {
	if a == nil || a.Workspace == nil || a.Member == nil || a.Member.Status != "active" {
		return []string{}
	}
	permissions := WorkspacePermissions(a.Member.Role)
	if a.Workspace.Status == "active" && (a.Project == nil || a.Project.Status == "active") {
		return permissions
	}
	reads := []string{}
	for _, p := range permissions {
		if strings.HasSuffix(p, ".read") {
			reads = append(reads, p)
		}
	}
	return reads
}
func HasWorkspacePermission(role, permission string) bool {
	for _, p := range workspaceRolePermissions[role] {
		if p == permission {
			return true
		}
	}
	return false
}

// Archived and suspended scopes retain read access to history. Mutations require
// an active scope. Gateway admission separately requires both scopes active.
func CheckWorkspacePermission(a *WorkspaceAccess, permission string) error {
	if a == nil || a.Member == nil || a.Workspace == nil {
		return ErrWorkspaceNotFound
	}
	if a.Member.Status != "active" || !HasWorkspacePermission(a.Member.Role, permission) {
		return ErrWorkspaceForbidden
	}
	if !strings.HasSuffix(permission, ".read") && (a.Workspace.Status != "active" || (a.Project != nil && a.Project.Status != "active")) {
		return ErrWorkspaceConflict
	}
	return nil
}

type WorkspaceAccessService struct{ repo WorkspaceRepository }

func NewWorkspaceAccessService(repo WorkspaceRepository) *WorkspaceAccessService {
	return &WorkspaceAccessService{repo: repo}
}
func (s *WorkspaceAccessService) RequireWorkspace(ctx context.Context, actorID, workspaceID int64, permission string) (*WorkspaceAccess, error) {
	a, e := s.repo.GetAccess(ctx, actorID, workspaceID, 0)
	if e != nil {
		return nil, e
	}
	if e = CheckWorkspacePermission(a, permission); e != nil {
		return nil, e
	}
	return a, nil
}
func (s *WorkspaceAccessService) RequireProject(ctx context.Context, actorID, workspaceID, projectID int64, permission string) (*WorkspaceAccess, error) {
	if projectID <= 0 {
		return nil, ErrWorkspaceNotFound
	}
	a, e := s.repo.GetAccess(ctx, actorID, workspaceID, projectID)
	if e != nil {
		return nil, e
	}
	if e = CheckWorkspacePermission(a, permission); e != nil {
		return nil, e
	}
	return a, nil
}
