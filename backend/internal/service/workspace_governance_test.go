package service

import (
	"errors"
	"testing"
)

func TestProjectAccessRolePermissionsAndValidation(t *testing.T) {
	if !ValidProjectAccessMode("all_projects") || !ValidProjectAccessMode("assigned_projects") {
		t.Fatal("expected both built-in project access modes to be valid")
	}
	if ValidProjectAccessMode("custom") {
		t.Fatal("custom project access modes must be rejected")
	}
	if !ValidProjectAccessRole("viewer") || !ValidProjectAccessRole("developer") || !ValidProjectAccessRole("admin") {
		t.Fatal("expected built-in project access roles to be valid")
	}
	if ValidProjectAccessRole("owner") {
		t.Fatal("owner must never be a project grant role")
	}
	if !containsPermission(ProjectRolePermissions("viewer"), "project.read") || containsPermission(ProjectRolePermissions("viewer"), "key.create") {
		t.Fatal("viewer permissions must be read-only")
	}
	if !containsPermission(ProjectRolePermissions("developer"), "key.create") || !containsPermission(ProjectRolePermissions("developer"), "service_account.credential.rotate") {
		t.Fatal("developer permissions must include project credential management")
	}
	if !containsPermission(ProjectRolePermissions("admin"), "project.update") || !containsPermission(ProjectRolePermissions("admin"), "project_policy.update") {
		t.Fatal("admin permissions must include project administration")
	}
}

func TestAssignedProjectAccessRequiresGrant(t *testing.T) {
	access := &WorkspaceAccess{
		Workspace: &Workspace{Status: "active", ProjectAccessMode: "assigned_projects"},
		Project:   &Project{Status: "active"},
		Member:    &WorkspaceMember{Role: "developer", Status: "active"},
	}
	if err := CheckWorkspacePermission(access, "project.read"); err == nil {
		t.Fatal("a developer without a grant must not read an assigned project")
	}
	access.ProjectRole = "viewer"
	access.ProjectPermissions = ProjectRolePermissions("viewer")
	if err := CheckWorkspacePermission(access, "project.read"); err != nil {
		t.Fatalf("viewer grant should read project: %v", err)
	}
	if err := CheckWorkspacePermission(access, "key.create"); err == nil {
		t.Fatal("viewer grant must not create project keys")
	}
	access.ProjectRole = "developer"
	access.ProjectPermissions = ProjectRolePermissions("developer")
	if err := CheckWorkspacePermission(access, "key.create"); err != nil {
		t.Fatalf("developer grant should create project keys: %v", err)
	}
}

func TestAllProjectAccessRemainsBackwardCompatible(t *testing.T) {
	access := &WorkspaceAccess{
		Workspace: &Workspace{Status: "active", ProjectAccessMode: "all_projects"},
		Project:   &Project{Status: "active"},
		Member:    &WorkspaceMember{Role: "developer", Status: "active"},
	}
	if err := CheckWorkspacePermission(access, "key.create"); err != nil {
		t.Fatalf("all-project mode must preserve existing developer access: %v", err)
	}
}

func TestAssignedProjectBillingRemainsReadOnly(t *testing.T) {
	access := &WorkspaceAccess{
		Workspace: &Workspace{Status: "active", ProjectAccessMode: ProjectAccessModeAssigned},
		Project:   &Project{Status: "active"},
		Member:    &WorkspaceMember{Role: "billing", Status: "active"},
	}
	if err := CheckWorkspacePermission(access, "project.read"); err != nil {
		t.Fatalf("billing should retain project read access: %v", err)
	}
	if err := CheckWorkspacePermission(access, "key.create"); err == nil {
		t.Fatal("billing must not create keys in assigned-project mode")
	}
}

func TestServiceAccountPolicyRequiresProjectWriteGrantWithoutExpandingWorkspaceRoles(t *testing.T) {
	for _, tc := range []struct {
		name, mode, workspaceRole, projectRole string
		allowed                                bool
	}{
		{"developer_without_grant", ProjectAccessModeAssigned, "developer", "", false},
		{"developer_with_viewer_override", ProjectAccessModeAssigned, "developer", "viewer", false},
		{"developer_with_developer_grant", ProjectAccessModeAssigned, "developer", "developer", true},
		{"developer_with_admin_grant", ProjectAccessModeAssigned, "developer", "admin", true},
		{"viewer_with_developer_grant", ProjectAccessModeAssigned, "viewer", "developer", false},
		{"viewer_with_admin_grant", ProjectAccessModeAssigned, "viewer", "admin", false},
		{"billing_with_admin_grant", ProjectAccessModeAssigned, "billing", "admin", false},
		{"owner_without_grant", ProjectAccessModeAssigned, "owner", "", true},
		{"admin_without_grant", ProjectAccessModeAssigned, "admin", "", true},
		{"all_projects_developer", ProjectAccessModeAllProjects, "developer", "", true},
		{"all_projects_viewer", ProjectAccessModeAllProjects, "viewer", "admin", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			access := &WorkspaceAccess{
				Workspace:          &Workspace{Status: "active", ProjectAccessMode: tc.mode},
				Project:            &Project{Status: "active"},
				Member:             &WorkspaceMember{Role: tc.workspaceRole, Status: "active"},
				ProjectRole:        tc.projectRole,
				ProjectPermissions: ProjectRolePermissions(tc.projectRole),
			}
			err := CheckWorkspacePermission(access, "service_account_policy.update")
			if tc.allowed && err != nil {
				t.Fatalf("authorized policy write denied: %v", err)
			}
			if !tc.allowed && !errors.Is(err, ErrWorkspaceForbidden) {
				t.Fatalf("policy write must be forbidden, got %v", err)
			}
		})
	}
}
