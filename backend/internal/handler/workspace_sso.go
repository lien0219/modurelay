package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func checkEnterpriseWorkspaceAccess(c *gin.Context, workspaces *service.WorkspaceService, identity *service.EnterpriseIdentityService, subject middleware.AuthSubject, workspaceID int64) bool {
	if identity == nil || workspaceID <= 0 {
		return true
	}
	workspace, err := workspaces.GetWorkspace(c.Request.Context(), subject.UserID, workspaceID)
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	principal := subject.PrincipalType
	if principal == "" {
		principal = service.PrincipalHuman
	}
	err = identity.CheckWorkspaceAccess(c.Request.Context(), workspaceID, workspace.Type, principal, service.WorkspaceAssurance{WorkspaceID: subject.OIDCWorkspaceID, ProviderID: subject.OIDCProviderID, ProviderRevision: subject.OIDCProviderRevision, AuthenticatedAt: subject.OIDCAuthenticatedAt, AuthMethod: subject.AuthMethod})
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}
