package service

import "context"

// RequirePolicyWorkspace and RequirePolicyProject keep policy authorization in
// the same role map as the rest of the tenant API. Handlers must not inspect
// role strings or trust caller-supplied tenant metadata themselves.
func (s *WorkspaceService) RequirePolicyWorkspace(ctx context.Context, actorID, workspaceID int64, permission string) error {
	if s == nil || s.access == nil {
		return ErrWorkspaceNotFound
	}
	_, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, permission)
	return err
}

func (s *WorkspaceService) RequirePolicyProject(ctx context.Context, actorID, workspaceID, projectID int64, permission string) error {
	if s == nil || s.access == nil {
		return ErrWorkspaceNotFound
	}
	access, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, permission)
	if err != nil {
		return err
	}
	if access == nil || access.Workspace == nil || access.Workspace.ID != workspaceID || access.Project == nil || access.Project.ID != projectID || access.Project.WorkspaceID != workspaceID {
		return ErrWorkspaceNotFound
	}
	return nil
}

// RequirePolicy verifies both the central project role and that the service
// account belongs to the supplied project/workspace. This prevents policy IDOR
// when two tenants happen to use the same numeric service-account ID.
func (s *ServiceAccountService) RequirePolicy(ctx context.Context, actorID, workspaceID, projectID, serviceAccountID int64, permission string) error {
	if s == nil || s.access == nil || s.repo == nil {
		return ErrWorkspaceNotFound
	}
	access, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, permission)
	if err != nil {
		return err
	}
	if access == nil || access.Workspace == nil || access.Workspace.ID != workspaceID || access.Project == nil || access.Project.ID != projectID || access.Project.WorkspaceID != workspaceID {
		return ErrWorkspaceNotFound
	}
	account, err := s.repo.Get(ctx, actorID, workspaceID, projectID, serviceAccountID)
	if err != nil {
		return err
	}
	if account == nil || account.ID != serviceAccountID || account.WorkspaceID != workspaceID || account.ProjectID != projectID {
		return ErrWorkspaceNotFound
	}
	return nil
}
