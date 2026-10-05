package handler

import (
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"time"
)

type WorkspaceHandler struct {
	workspaces *service.WorkspaceService
	keys       *service.APIKeyService
	webhooks   *service.WorkspaceWebhookService
}

func finopsRange(c *gin.Context) (time.Time, time.Time, string, error) {
	tz := c.DefaultQuery("timezone", "UTC")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, time.Time{}, "", service.ErrWorkspaceInvalid
	}
	end := time.Now().UTC()
	if raw := c.Query("end"); raw != "" {
		end, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, "", service.ErrWorkspaceInvalid
		}
	}
	local := end.In(loc)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc).UTC()
	if raw := c.Query("start"); raw != "" {
		var err error
		start, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, "", service.ErrWorkspaceInvalid
		}
	}
	if !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, "", service.ErrWorkspaceInvalid
	}
	return start, end, tz, nil
}

func NewWorkspaceHandler(w *service.WorkspaceService, k *service.APIKeyService) *WorkspaceHandler {
	return &WorkspaceHandler{workspaces: w, keys: k}
}

func (h *WorkspaceHandler) SetWebhookService(webhooks *service.WorkspaceWebhookService) {
	h.webhooks = webhooks
}
func (h *WorkspaceHandler) RegisterTenantRoutes(v1 *gin.RouterGroup) {
	routes := []struct{ method, path, action string }{
		{"GET", "/workspaces", "workspace.list"}, {"POST", "/workspaces", "workspace.create"}, {"GET", "/workspaces/:id", "workspace.get"}, {"PATCH", "/workspaces/:id", "workspace.update"}, {"DELETE", "/workspaces/:id", "workspace.archive"},
		{"GET", "/workspaces/:id/members", "member.list"}, {"PATCH", "/workspaces/:id/members/:member_id", "member.update"}, {"DELETE", "/workspaces/:id/members/:member_id", "member.remove"},
		{"GET", "/workspaces/:id/invitations", "invitation.list"}, {"POST", "/workspaces/:id/invitations", "invitation.create"}, {"DELETE", "/workspaces/:id/invitations/:invitation_id", "invitation.revoke"}, {"POST", "/workspace-invitations/accept", "invitation.accept"},
		{"GET", "/workspaces/:id/projects", "project.list"}, {"POST", "/workspaces/:id/projects", "project.create"}, {"GET", "/workspaces/:id/projects/:project_id", "project.get"}, {"PATCH", "/workspaces/:id/projects/:project_id", "project.update"}, {"DELETE", "/workspaces/:id/projects/:project_id", "project.archive"},
		{"GET", "/workspaces/:id/projects/:project_id/keys", "key.list"}, {"POST", "/workspaces/:id/projects/:project_id/keys", "key.create"}, {"GET", "/workspaces/:id/projects/:project_id/keys/:key_id", "key.get"}, {"PATCH", "/workspaces/:id/projects/:project_id/keys/:key_id", "key.update"}, {"DELETE", "/workspaces/:id/projects/:project_id/keys/:key_id", "key.revoke"}, {"GET", "/workspaces/:id/projects/:project_id/groups/available", "group.available"}, {"GET", "/workspaces/:id/audit", "audit.list"},
		{"GET", "/workspaces/:id/webhooks", "webhook.list"}, {"POST", "/workspaces/:id/webhooks", "webhook.create"}, {"PATCH", "/workspaces/:id/webhooks/:webhook_id", "webhook.update"}, {"DELETE", "/workspaces/:id/webhooks/:webhook_id", "webhook.delete"}, {"POST", "/workspaces/:id/webhooks/:webhook_id/rotate", "webhook.rotate"}, {"POST", "/workspaces/:id/webhooks/:webhook_id/test", "webhook.test"}, {"GET", "/workspaces/:id/webhooks/:webhook_id/deliveries", "webhook.deliveries"}, {"POST", "/workspaces/:id/webhooks/:webhook_id/deliveries/:delivery_id/retry", "webhook.retry"},
		{"GET", "/workspaces/:id/usage", "finops.workspace.usage"}, {"GET", "/workspaces/:id/overview", "finops.workspace.overview"}, {"GET", "/workspaces/:id/budget", "finops.workspace.budget.get"}, {"PUT", "/workspaces/:id/budget", "finops.workspace.budget.put"},
		{"GET", "/workspaces/:id/projects/:project_id/usage", "finops.project.usage"}, {"GET", "/workspaces/:id/projects/:project_id/overview", "finops.project.overview"}, {"GET", "/workspaces/:id/projects/:project_id/budget", "finops.project.budget.get"}, {"PUT", "/workspaces/:id/projects/:project_id/budget", "finops.project.budget.put"},
	}
	for _, r := range routes {
		v1.Handle(r.method, r.path, h.handle(r.action))
	}
}
func (h *WorkspaceHandler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/workspaces", h.handle("admin.list"))
	admin.GET("/workspaces/:id", h.handle("admin.inspect"))
	admin.PATCH("/workspaces/:id/status", h.handle("admin.status"))
}

type workspaceKeyDTO struct {
	*dto.APIKey
	ProjectID *int64 `json:"project_id"`
}

func projectKeyDTO(k *service.APIKey) any {
	if k == nil {
		return nil
	}
	masked := *k
	secret := masked.Key
	masked.Key = "****"
	if len(secret) > 4 {
		masked.Key += secret[len(secret)-4:]
	}
	out := dto.APIKeyFromService(&masked)
	out.User = nil
	return workspaceKeyDTO{out, masked.ProjectID}
}

// projectKeyCreateDTO intentionally preserves the one-time secret returned by
// project-key creation. All subsequent project-key reads use projectKeyDTO,
// whose service inputs are masked before they reach this handler.
func projectKeyCreateDTO(k *service.APIKey) any {
	if k == nil {
		return nil
	}
	out := dto.APIKeyFromService(k)
	out.User = nil
	return workspaceKeyDTO{out, k.ProjectID}
}
func (h *WorkspaceHandler) handle(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		ids := map[string]int64{}
		for _, name := range []string{"id", "project_id", "member_id", "invitation_id", "key_id", "webhook_id", "delivery_id"} {
			if raw := c.Param(name); raw != "" {
				id, e := strconv.ParseInt(raw, 10, 64)
				if e != nil || id <= 0 {
					response.ErrorFrom(c, service.ErrWorkspaceNotFound)
					return
				}
				ids[name] = id
			}
		}
		a, w, p := subject.UserID, ids["id"], ids["project_id"]
		ctx := c.Request.Context()
		page, size := response.ParsePagination(c)
		params := pagination.PaginationParams{Page: page, PageSize: size}
		var out any
		var err error
		var total int64
		list := false
		switch action {
		case "workspace.list":
			out, total, err = h.workspaces.ListWorkspaces(ctx, a, params)
			list = true
		case "workspace.create":
			var req struct {
				Name string `json:"name"`
				Slug string `json:"slug"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			out, err = h.workspaces.CreateOrganization(ctx, a, req.Name, req.Slug)
		case "workspace.get":
			out, err = h.workspaces.GetWorkspace(ctx, a, w)
		case "workspace.update":
			var req struct {
				Name         *string `json:"name"`
				Slug         *string `json:"slug"`
				BillingOwner *int64  `json:"billing_owner_user_id"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			if req.BillingOwner != nil {
				if req.Name != nil || req.Slug != nil {
					response.BadRequest(c, "Change billing owner separately from workspace details")
					return
				}
				err = h.workspaces.ChangeBillingOwner(ctx, a, w, *req.BillingOwner)
				if err == nil {
					out, err = h.workspaces.GetWorkspace(ctx, a, w)
				}
			} else {
				var current *service.Workspace
				current, err = h.workspaces.GetWorkspace(ctx, a, w)
				if err == nil {
					name, slug := current.Name, current.Slug
					if req.Name != nil {
						name = *req.Name
					}
					if req.Slug != nil {
						slug = *req.Slug
					}
					out, err = h.workspaces.UpdateWorkspace(ctx, a, w, name, slug)
				}
			}
		case "workspace.archive":
			err = h.workspaces.ArchiveWorkspace(ctx, a, w)
		case "member.list":
			out, total, err = h.workspaces.ListMembers(ctx, a, w, params)
			list = true
		case "member.update":
			var req struct {
				Role   string `json:"role"`
				Status string `json:"status"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			err = h.workspaces.UpdateMember(ctx, a, w, ids["member_id"], req.Role, req.Status)
		case "member.remove":
			err = h.workspaces.RemoveMember(ctx, a, w, ids["member_id"])
		case "invitation.list":
			out, total, err = h.workspaces.ListInvitations(ctx, a, w, params)
			list = true
		case "invitation.create":
			var req struct {
				Email          string `json:"email"`
				Role           string `json:"role"`
				ExpiresInHours *int   `json:"expires_in_hours"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			hours := 168
			if req.ExpiresInHours != nil {
				hours = *req.ExpiresInHours
			}
			if hours < 1 || hours > 720 {
				response.ErrorFrom(c, service.ErrWorkspaceInvalid)
				return
			}
			var inv *service.WorkspaceInvitation
			var token string
			inv, token, err = h.workspaces.CreateInvitation(ctx, a, w, req.Email, req.Role, time.Duration(hours)*time.Hour)
			out = gin.H{"invitation": inv, "token": token}
		case "invitation.revoke":
			err = h.workspaces.RevokeInvitation(ctx, a, w, ids["invitation_id"])
		case "invitation.accept":
			var req struct {
				Token string `json:"token"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			out, err = h.workspaces.AcceptInvitation(ctx, a, req.Token)
		case "project.list":
			out, total, err = h.workspaces.ListProjects(ctx, a, w, params)
			list = true
		case "project.get":
			out, err = h.workspaces.GetProject(ctx, a, w, p)
		case "project.create":
			var req service.ProjectInput
			if !workspaceBind(c, &req) {
				return
			}
			out, err = h.workspaces.CreateProject(ctx, a, w, req)
		case "project.update":
			var current *service.Project
			current, err = h.workspaces.GetProject(ctx, a, w, p)
			if err != nil {
				break
			}
			var req struct {
				Name        *string         `json:"name"`
				Slug        *string         `json:"slug"`
				Description *string         `json:"description"`
				Groups      json.RawMessage `json:"allowed_group_ids"`
				Models      json.RawMessage `json:"allowed_models"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			input := service.ProjectInput{Name: current.Name, Slug: current.Slug, Description: current.Description, AllowedGroupIDs: current.AllowedGroupIDs, AllowedModels: current.AllowedModels}
			if req.Name != nil {
				input.Name = *req.Name
			}
			if req.Slug != nil {
				input.Slug = *req.Slug
			}
			if req.Description != nil {
				input.Description = *req.Description
			}
			if len(req.Groups) > 0 {
				if e := json.Unmarshal(req.Groups, &input.AllowedGroupIDs); e != nil {
					response.ErrorFrom(c, service.ErrWorkspaceInvalid)
					return
				}
			}
			if len(req.Models) > 0 {
				if e := json.Unmarshal(req.Models, &input.AllowedModels); e != nil {
					response.ErrorFrom(c, service.ErrWorkspaceInvalid)
					return
				}
			}
			out, err = h.workspaces.UpdateProject(ctx, a, w, p, input)
		case "project.archive":
			err = h.workspaces.ArchiveProject(ctx, a, w, p)
		case "key.list":
			var keys []service.APIKey
			keys, total, err = h.keys.ListForProject(ctx, a, w, p, params)
			items := []any{}
			for i := range keys {
				items = append(items, projectKeyDTO(&keys[i]))
			}
			out = items
			list = true
		case "key.get":
			var key *service.APIKey
			key, err = h.keys.GetForProject(ctx, a, w, p, ids["key_id"])
			out = projectKeyDTO(key)
		case "key.create":
			var req CreateAPIKeyRequest
			if !workspaceBind(c, &req) {
				return
			}
			if validateAPIKeyCreateRequest(req) != nil {
				response.ErrorFrom(c, service.ErrWorkspaceInvalid)
				return
			}
			input := service.CreateAPIKeyRequest{Name: req.Name, GroupID: req.GroupID, CustomKey: req.CustomKey, IPWhitelist: req.IPWhitelist, IPBlacklist: req.IPBlacklist, ExpiresInDays: req.ExpiresInDays}
			if req.Quota != nil {
				input.Quota = *req.Quota
			}
			if req.RateLimit5h != nil {
				input.RateLimit5h = *req.RateLimit5h
			}
			if req.RateLimit1d != nil {
				input.RateLimit1d = *req.RateLimit1d
			}
			if req.RateLimit7d != nil {
				input.RateLimit7d = *req.RateLimit7d
			}
			var key *service.APIKey
			key, err = h.keys.CreateForProject(ctx, a, w, p, input)
			out = projectKeyCreateDTO(key)
		case "key.update":
			var req UpdateAPIKeyRequest
			if !workspaceBind(c, &req) {
				return
			}
			if validateAPIKeyUpdateRequest(req) != nil {
				response.ErrorFrom(c, service.ErrWorkspaceInvalid)
				return
			}
			input := service.UpdateAPIKeyRequest{GroupID: req.GroupID, IPWhitelist: req.IPWhitelist, IPBlacklist: req.IPBlacklist, Quota: req.Quota, ResetQuota: req.ResetQuota, RateLimit5h: req.RateLimit5h, RateLimit1d: req.RateLimit1d, RateLimit7d: req.RateLimit7d, ResetRateLimitUsage: req.ResetRateLimitUsage}
			if req.Name != "" {
				input.Name = &req.Name
			}
			if req.Status != "" {
				input.Status = &req.Status
			}
			if req.ExpiresAt != nil {
				if *req.ExpiresAt == "" {
					input.ClearExpiration = true
				} else {
					t, e := time.Parse(time.RFC3339, *req.ExpiresAt)
					if e != nil {
						response.ErrorFrom(c, service.ErrWorkspaceInvalid)
						return
					}
					input.ExpiresAt = &t
				}
			}
			var key *service.APIKey
			key, err = h.keys.UpdateForProject(ctx, a, w, p, ids["key_id"], input)
			out = projectKeyDTO(key)
		case "key.revoke":
			err = h.keys.DeleteForProject(ctx, a, w, p, ids["key_id"])
		case "group.available":
			var groups []service.Group
			groups, err = h.keys.GetAvailableGroupsForProject(ctx, a, w, p)
			items := []*dto.Group{}
			for i := range groups {
				items = append(items, dto.GroupFromService(&groups[i]))
			}
			out = items
		case "audit.list":
			out, total, err = h.workspaces.ListAudit(ctx, a, w, params)
			list = true
		case "webhook.list":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			out, err = h.webhooks.List(ctx, a, w)
		case "webhook.create":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			var req struct {
				Name       string   `json:"name"`
				URL        string   `json:"url"`
				EventTypes []string `json:"event_types"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			var webhook *service.WorkspaceWebhook
			var secret string
			webhook, secret, err = h.webhooks.Create(ctx, a, w, service.CreateWorkspaceWebhookInput{Name: req.Name, URL: req.URL, EventTypes: req.EventTypes})
			out = gin.H{"webhook": webhook, "secret": secret}
		case "webhook.update":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			var req struct {
				Name       *string   `json:"name"`
				URL        *string   `json:"url"`
				Enabled    *bool     `json:"enabled"`
				EventTypes *[]string `json:"event_types"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			out, err = h.webhooks.Update(ctx, a, w, ids["webhook_id"], service.UpdateWorkspaceWebhookInput{Name: req.Name, URL: req.URL, Enabled: req.Enabled, EventTypes: req.EventTypes})
		case "webhook.delete":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			err = h.webhooks.Delete(ctx, a, w, ids["webhook_id"])
		case "webhook.rotate":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			var webhook *service.WorkspaceWebhook
			var secret string
			webhook, secret, err = h.webhooks.Rotate(ctx, a, w, ids["webhook_id"])
			out = gin.H{"webhook": webhook, "secret": secret}
		case "webhook.test":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			out, err = h.webhooks.Test(ctx, a, w, ids["webhook_id"])
			if err == nil {
				response.Accepted(c, out)
				return
			}
		case "webhook.deliveries":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			out, total, err = h.webhooks.Deliveries(ctx, a, w, ids["webhook_id"], page, size)
			list = true
		case "webhook.retry":
			if h.webhooks == nil {
				err = service.ErrWorkspaceNotFound
				break
			}
			out, err = h.webhooks.Retry(ctx, a, w, ids["webhook_id"], ids["delivery_id"])
		case "finops.workspace.usage", "finops.project.usage", "finops.workspace.overview", "finops.project.overview":
			start, end, tz, parseErr := finopsRange(c)
			if parseErr != nil {
				response.ErrorFrom(c, parseErr)
				return
			}
			projectID := int64(0)
			if strings.Contains(action, "project") {
				projectID = p
			}
			if strings.HasSuffix(action, "overview") {
				out, err = h.workspaces.GetOverview(ctx, a, w, projectID, start, end, tz)
			} else {
				out, err = h.workspaces.GetUsageSummary(ctx, a, w, projectID, start, end, tz)
			}
		case "finops.workspace.budget.get", "finops.project.budget.get":
			projectID := int64(0)
			if strings.Contains(action, "project") {
				projectID = p
			}
			out, err = h.workspaces.GetBudget(ctx, a, w, projectID)
		case "finops.workspace.budget.put", "finops.project.budget.put":
			var req service.BudgetPolicyInput
			if !workspaceBind(c, &req) {
				return
			}
			projectID := int64(0)
			if strings.Contains(action, "project") {
				projectID = p
			}
			out, err = h.workspaces.SetBudget(ctx, a, w, projectID, req)
		case "admin.list":
			out, total, err = h.workspaces.AdminList(ctx, a, params)
			list = true
		case "admin.inspect":
			out, err = h.workspaces.AdminInspect(ctx, a, w)
		case "admin.status":
			var req struct {
				Status string `json:"status"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			err = h.workspaces.AdminSetStatus(ctx, a, w, req.Status)
			if err == nil {
				out, err = h.workspaces.AdminInspect(ctx, a, w)
			}
		}
		if response.ErrorFrom(c, err) {
			return
		}
		if w > 0 && c.Request.Method != "GET" {
			h.keys.InvalidateWorkspaceAuth(ctx, w)
		}
		if list {
			response.Paginated(c, out, total, page, size)
		} else {
			if out == nil {
				out = gin.H{"success": true}
			}
			response.Success(c, out)
		}
	}
}
func workspaceBind(c *gin.Context, v any) bool {
	if e := c.ShouldBindJSON(v); e != nil {
		response.BadRequest(c, "Invalid request")
		return false
	}
	return true
}
