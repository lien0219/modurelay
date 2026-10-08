package service

import (
	"context"
	"strings"
)

// The only tenant role map. Global admin is deliberately not a tenant role.
var workspaceRolePermissions = map[string][]string{
	"owner":     {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "workspace.update", "workspace.archive", "member.read", "invitation.read", "member.invite", "member.update", "member.remove", "owner.manage", "billing.owner.update", "project.read", "project.create", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "billing.read", "budget.read", "budget.update", "finops_anomaly.read", "finops_anomaly.manage", "audit.read", "webhook.read", "webhook.create", "webhook.update", "webhook.delete", "webhook.secret.rotate", "webhook.test", "webhook.delivery.read", "webhook.delivery.retry", "team.read", "team.create", "team.update", "team.archive", "team.member.update", "project_access.read", "project_access.update", "workspace.project_access.update", "identity.read", "identity.manage", "workspace_sso.update"},
	"admin":     {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "workspace.update", "member.read", "invitation.read", "member.invite", "member.update", "member.remove", "project.read", "project.create", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "budget.read", "budget.update", "finops_anomaly.read", "finops_anomaly.manage", "audit.read", "webhook.read", "webhook.create", "webhook.update", "webhook.delete", "webhook.secret.rotate", "webhook.test", "webhook.delivery.read", "webhook.delivery.retry", "team.read", "team.create", "team.update", "team.archive", "team.member.update", "project_access.read", "project_access.update", "workspace.project_access.update", "identity.read", "identity.manage"},
	"developer": {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "member.read", "project.read", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "budget.read", "finops_anomaly.read", "webhook.read", "webhook.delivery.read"},
	"billing":   {"service_account.read", "workspace.read", "project.read", "usage.read", "billing.read", "budget.read", "budget.update", "finops_anomaly.read", "finops_anomaly.manage", "webhook.read"},
	"viewer":    {"service_account.read", "workspace.read", "project.read", "usage.read", "budget.read", "finops_anomaly.read"},
}

var projectRolePermissions = map[string][]string{
	ProjectAccessRoleViewer: {
		"project.read", "key.read", "usage.read", "billing.read", "budget.read", "finops_anomaly.read",
		"service_account.read", "service_account.credential.read", "policy.read",
	},
	ProjectAccessRoleDeveloper: {
		"project.read", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "billing.read", "budget.read", "finops_anomaly.read",
		"service_account.read", "service_account.create", "service_account.update", "service_account.disable",
		"service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "policy.read",
	},
	ProjectAccessRoleAdmin: {
		"project.read", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "billing.read", "budget.read", "budget.update", "finops_anomaly.read", "finops_anomaly.manage",
		"service_account.read", "service_account.create", "service_account.update", "service_account.disable",
		"service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "project_policy.update", "policy.read",
	},
}

var projectScopedPermissions = map[string]struct{}{
	"project.read": {}, "project.update": {}, "project.archive": {},
	"key.read": {}, "key.create": {}, "key.update": {}, "key.revoke": {},
	"usage.read": {}, "billing.read": {}, "budget.read": {}, "budget.update": {}, "finops_anomaly.read": {}, "finops_anomaly.manage": {},
	"service_account.read": {}, "service_account.create": {}, "service_account.update": {}, "service_account.disable": {},
	"service_account.credential.read": {}, "service_account.credential.create": {}, "service_account.credential.update": {}, "service_account.credential.revoke": {}, "service_account.credential.rotate": {},
	"project_policy.update": {}, "policy.read": {},
}

func init() {
	// Policy permissions live in the same central role map as every other
	// tenant capability. Read access is available to all tenant roles; policy
	// mutation is intentionally narrower because it affects all credentials in
	// a scope.
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		workspaceRolePermissions[role] = append(workspaceRolePermissions[role], "policy.read", "workspace_security.read")
	}
	for _, role := range []string{"owner", "admin"} {
		workspaceRolePermissions[role] = append(workspaceRolePermissions[role], "workspace_policy.update", "project_policy.update", "provisioning.read", "provisioning.manage", "provisioning.token.rotate")
	}
	workspaceRolePermissions["owner"] = append(workspaceRolePermissions["owner"], "service_account_policy.update")
	workspaceRolePermissions["owner"] = append(workspaceRolePermissions["owner"], "workspace_security.update")
	workspaceRolePermissions["admin"] = append(workspaceRolePermissions["admin"], "service_account_policy.update")
	workspaceRolePermissions["developer"] = append(workspaceRolePermissions["developer"], "service_account_policy.update")
}

func ValidWorkspaceRole(role string) bool { _, ok := workspaceRolePermissions[role]; return ok }
func WorkspacePermissions(role string) []string {
	return append([]string{}, workspaceRolePermissions[role]...)
}

func ValidProjectAccessMode(mode string) bool {
	return mode == ProjectAccessModeAllProjects || mode == ProjectAccessModeAssigned
}

func ValidProjectAccessRole(role string) bool {
	_, ok := projectRolePermissions[role]
	return ok
}

func ProjectRolePermissions(role string) []string {
	return append([]string{}, projectRolePermissions[role]...)
}

func isProjectScopedPermission(permission string) bool {
	_, ok := projectScopedPermissions[permission]
	return ok
}

func hasPermission(permissions []string, permission string) bool {
	for _, candidate := range permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

// WorkspaceMutationPermission maps closed mutation commands to the central
// permission registry. Handlers never need to inspect role strings.
func WorkspaceMutationPermission(m WorkspaceMutation) string {
	switch m.Action {
	case "workspace.project_access_mode.update":
		return "workspace.project_access.update"
	case "team.create":
		return "team.create"
	case "team.update":
		return "team.update"
	case "team.archive":
		return "team.archive"
	case "team.member.add", "team.member.remove":
		return "team.member.update"
	case "project_access.grant.create", "project_access.grant.update", "project_access.grant.delete":
		return "project_access.update"
	default:
		return m.Action
	}
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
	if a.Member.Status != "active" {
		return ErrWorkspaceForbidden
	}
	if a.Project != nil && isProjectScopedPermission(permission) && a.Workspace.ProjectAccessMode == ProjectAccessModeAssigned {
		switch a.Member.Role {
		case "owner", "admin", "billing":
			// These built-in workspace roles keep their existing permission
			// boundary. Billing remains read-only even though it can see every
			// project in assigned mode.
			if !HasWorkspacePermission(a.Member.Role, permission) {
				return ErrWorkspaceForbidden
			}
		default:
			if !hasPermission(a.ProjectPermissions, permission) {
				return ErrWorkspaceForbidden
			}
		}
	} else if !HasWorkspacePermission(a.Member.Role, permission) {
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
