package service

import "testing"

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
