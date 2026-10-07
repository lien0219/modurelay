package handler

import (
	"errors"
	"strconv"

	rate "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *WorkspaceHandler) SetEnterpriseSCIMService(scim *service.EnterpriseSCIMService) {
	h.scim = scim
}

func (h *WorkspaceHandler) RegisterPublicSCIMRoutes(r *gin.Engine, limiter *rate.RateLimiter) {
	if h.scim == nil {
		return
	}
	var configured scimRateLimiter
	if limiter != nil {
		configured = limiter
	}
	NewEnterpriseSCIMHandler(h.scim, configured, h.identityRedirectURL).Register(r)
}

type workspaceSCIMConnectorDTO struct {
	service.SCIMConnector
	BaseURL string `json:"base_url"`
}

func (h *WorkspaceHandler) registerSCIMControlRoutes(v1 *gin.RouterGroup) {
	if h.scim == nil {
		return
	}
	for _, route := range []struct{ method, path, action string }{
		{"GET", "/scim-connectors", "connector.list"}, {"POST", "/scim-connectors", "connector.create"},
		{"PATCH", "/scim-connectors/:connector_id", "connector.update"}, {"POST", "/scim-connectors/:connector_id/disable", "connector.disable"},
		{"GET", "/scim-connectors/:connector_id/tokens", "token.list"}, {"POST", "/scim-connectors/:connector_id/tokens", "token.create"},
		{"DELETE", "/scim-connectors/:connector_id/tokens/:token_id", "token.revoke"},
		{"GET", "/scim-connectors/:connector_id/groups", "group.list"}, {"PUT", "/scim-connectors/:connector_id/groups/:group_id/team", "group.bind"},
	} {
		v1.Handle(route.method, "/workspaces/:id"+route.path, h.scimControl(route.action))
	}
}

func workspaceSCIMError(c *gin.Context, err error) {
	var typed *service.SCIMError
	if errors.As(err, &typed) {
		reason := "SCIM_OPERATION_FAILED"
		switch typed.Status {
		case 400:
			reason = "SCIM_INVALID_VALUE"
		case 401, 403:
			reason = "SCIM_FORBIDDEN"
		case 404:
			reason = "SCIM_NOT_FOUND"
		case 409, 412:
			reason = "SCIM_REVISION_CONFLICT"
		case 503:
			reason = "SCIM_UNAVAILABLE"
		}
		response.ErrorWithDetails(c, typed.Status, typed.Detail, reason, nil)
		return
	}
	response.ErrorFrom(c, err)
}
func (h *WorkspaceHandler) scimControl(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 || subject.PrincipalType != "" && subject.PrincipalType != service.PrincipalHuman {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		ids := map[string]int64{}
		for _, name := range []string{"id", "connector_id", "token_id"} {
			if raw := c.Param(name); raw != "" {
				value, err := strconv.ParseInt(raw, 10, 64)
				if err != nil || value <= 0 {
					response.ErrorFrom(c, service.ErrWorkspaceNotFound)
					return
				}
				ids[name] = value
			}
		}
		if !checkEnterpriseWorkspaceAccess(c, h.workspaces, h.identity, subject, ids["id"]) {
			return
		}
		mutation := action != "connector.list" && action != "token.list" && action != "group.list"
		if mutation {
			if h.identityRecentAuth == nil {
				response.ErrorFrom(c, service.ErrWorkspaceForbidden)
				return
			}
			if !h.identityRecentAuth(c) {
				return
			}
		}
		ctx := c.Request.Context()
		w, a, connector := ids["id"], subject.UserID, ids["connector_id"]
		var out any
		var err error
		switch action {
		case "connector.list":
			var rows []service.SCIMConnector
			rows, err = h.scim.ListConnectors(ctx, w, a)
			result := make([]workspaceSCIMConnectorDTO, 0, len(rows))
			for _, row := range rows {
				base, _ := SCIMBaseURL(row.PublicID, h.identityRedirectURL)
				result = append(result, workspaceSCIMConnectorDTO{row, base})
			}
			out = result
		case "connector.create", "connector.update":
			var input service.SCIMConnectorInput
			if err = readSCIMBody(c, &input); err != nil {
				break
			}
			if action == "connector.create" {
				if _, err = SCIMBaseURL("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", h.identityRedirectURL); err != nil {
					break
				}
			}
			var row *service.SCIMConnector
			if action == "connector.create" {
				row, err = h.scim.CreateConnector(ctx, w, a, input)
			} else {
				row, err = h.scim.UpdateConnector(ctx, w, a, connector, input)
			}
			if err == nil && row != nil {
				base, _ := SCIMBaseURL(row.PublicID, h.identityRedirectURL)
				out = workspaceSCIMConnectorDTO{*row, base}
			}
		case "connector.disable":
			var input struct {
				Revision int64 `json:"revision"`
			}
			if err = readSCIMBody(c, &input); err == nil {
				err = h.scim.DisableConnector(ctx, w, a, connector, input.Revision)
			}
		case "token.list":
			out, err = h.scim.ListTokens(ctx, w, a, connector)
		case "token.create":
			var input service.SCIMTokenInput
			if err = readSCIMBody(c, &input); err == nil {
				out, err = h.scim.CreateToken(ctx, w, a, connector, input)
			}
		case "token.revoke":
			err = h.scim.RevokeToken(ctx, w, a, connector, ids["token_id"])
		case "group.list":
			out, err = h.scim.ListGroupBindings(ctx, w, a, connector)
		case "group.bind":
			var input service.SCIMGroupBindingInput
			if err = readSCIMBody(c, &input); err == nil {
				err = h.scim.BindGroup(ctx, w, a, connector, c.Param("group_id"), input)
			}
		}
		if err != nil {
			workspaceSCIMError(c, err)
			return
		}
		if action == "connector.create" || action == "token.create" {
			response.Created(c, out)
			return
		}
		response.Success(c, out)
	}
}
