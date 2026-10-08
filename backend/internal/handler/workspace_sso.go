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
	ctx := c.Request.Context()
	if _, ok := service.SessionAuthenticationFromContext(ctx); !ok {
		ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: subject.AuthMethod, AuthenticatedAt: subject.AuthenticatedAt, MFASatisfied: subject.MFASatisfied})
	}
	assurance := service.WorkspaceAssurance{WorkspaceID: subject.OIDCWorkspaceID, ProviderID: subject.OIDCProviderID, ProviderRevision: subject.OIDCProviderRevision, AuthenticatedAt: subject.OIDCAuthenticatedAt, ValidUntil: subject.OIDCValidUntil, AuthMethod: subject.AuthMethod}
	ctx = service.WithAuthenticationAssurance(ctx, assurance)
	c.Request = c.Request.WithContext(ctx)
	err = identity.CheckWorkspaceAccess(ctx, workspaceID, workspace.Type, principal, assurance)
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}
