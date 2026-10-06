package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PolicyHandler struct {
	workspaces      *service.WorkspaceService
	serviceAccounts *service.ServiceAccountService
	repository      domain.PolicyRepository
	resolver        *domain.EffectivePolicyResolver
}

func NewPolicyHandler(workspaces *service.WorkspaceService, serviceAccounts *service.ServiceAccountService, repository domain.PolicyRepository, resolver *domain.EffectivePolicyResolver) *PolicyHandler {
	if resolver == nil && repository != nil {
		resolver = domain.NewEffectivePolicyResolver(repository)
	}
	return &PolicyHandler{workspaces: workspaces, serviceAccounts: serviceAccounts, repository: repository, resolver: resolver}
}

func (h *PolicyHandler) RegisterTenantRoutes(v1 *gin.RouterGroup) {
	routes := []struct{ method, path, action string }{
		{"GET", "/workspaces/:id/policy", "workspace.get"},
		{"PATCH", "/workspaces/:id/policy", "workspace.update"},
		{"GET", "/workspaces/:id/projects/:project_id/policy", "project.get"},
		{"PATCH", "/workspaces/:id/projects/:project_id/policy", "project.update"},
		{"GET", "/workspaces/:id/projects/:project_id/effective-policy", "project.effective"},
		{"GET", "/workspaces/:id/projects/:project_id/service-accounts/:service_account_id/policy", "service_account.get"},
		{"PATCH", "/workspaces/:id/projects/:project_id/service-accounts/:service_account_id/policy", "service_account.update"},
		{"GET", "/workspaces/:id/projects/:project_id/service-accounts/:service_account_id/effective-policy", "service_account.effective"},
	}
	for _, route := range routes {
		v1.Handle(route.method, route.path, h.handle(route.action))
	}
}

func (h *PolicyHandler) handle(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		ids := map[string]int64{}
		for _, name := range []string{"id", "project_id", "service_account_id"} {
			if raw := c.Param(name); raw != "" {
				id, err := strconv.ParseInt(raw, 10, 64)
				if err != nil || id <= 0 {
					response.ErrorFrom(c, service.ErrWorkspaceNotFound)
					return
				}
				ids[name] = id
			}
		}
		workspaceID, projectID, serviceAccountID := ids["id"], ids["project_id"], ids["service_account_id"]
		if h.repository == nil || h.workspaces == nil {
			response.ErrorFrom(c, domain.ErrInvalidPolicy)
			return
		}
		ctx := c.Request.Context()
		var ref domain.PolicyRef
		var authorizeErr error
		switch action {
		case "workspace.get", "workspace.update":
			permission := "policy.read"
			if action == "workspace.update" {
				permission = "workspace_policy.update"
			}
			authorizeErr = h.workspaces.RequirePolicyWorkspace(ctx, subject.UserID, workspaceID, permission)
			ref = domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: workspaceID}
		case "project.get", "project.update", "project.effective":
			permission := "policy.read"
			if action == "project.update" {
				permission = "project_policy.update"
			}
			authorizeErr = h.workspaces.RequirePolicyProject(ctx, subject.UserID, workspaceID, projectID, permission)
			ref = domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: projectID}
		case "service_account.get", "service_account.update", "service_account.effective":
			permission := "policy.read"
			if action == "service_account.update" {
				permission = "service_account_policy.update"
			}
			if h.serviceAccounts == nil {
				authorizeErr = service.ErrWorkspaceNotFound
			} else {
				authorizeErr = h.serviceAccounts.RequirePolicy(ctx, subject.UserID, workspaceID, projectID, serviceAccountID, permission)
			}
			ref = domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: serviceAccountID}
		default:
			response.ErrorFrom(c, service.ErrWorkspaceNotFound)
			return
		}
		if authorizeErr != nil {
			response.ErrorFrom(c, authorizeErr)
			return
		}

		if action == "project.effective" || action == "service_account.effective" {
			if h.resolver == nil {
				writePolicyError(c, domain.ErrInvalidPolicy)
				return
			}
			identity := domain.PolicyContext{WorkspaceID: workspaceID, ProjectID: projectID}
			if action == "service_account.effective" {
				identity.ServiceAccountID = serviceAccountID
			}
			out, err := h.resolver.Resolve(ctx, identity)
			if err != nil {
				writePolicyError(c, err)
				return
			}
			response.Success(c, out)
			return
		}

		if action == "workspace.update" || action == "project.update" || action == "service_account.update" {
			updated, err := h.update(c, ref, subject.UserID)
			if err != nil {
				writePolicyError(c, err)
				return
			}
			response.Success(c, updated)
			return
		}

		out, err := h.repository.GetPolicy(ctx, ref)
		if err != nil {
			writePolicyError(c, err)
			return
		}
		if out == nil {
			out = &domain.Policy{Scope: ref.Scope, ScopeID: ref.ScopeID}
		}
		response.Success(c, out)
	}
}

func writePolicyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrPolicyRevisionConflict):
		response.ErrorWithDetails(c, http.StatusConflict, "policy revision conflict", "POLICY_CONFLICT", nil)
	case errors.Is(err, domain.ErrPolicyNotFound):
		response.ErrorWithDetails(c, http.StatusNotFound, "policy resource not found", "POLICY_NOT_FOUND", nil)
	case errors.Is(err, domain.ErrInvalidPolicy):
		response.ErrorWithDetails(c, http.StatusBadRequest, "invalid policy", "POLICY_INVALID", nil)
	case errors.Is(err, service.ErrWorkspaceInvalid):
		response.ErrorWithDetails(c, http.StatusBadRequest, "invalid policy input", "POLICY_INVALID", nil)
	default:
		response.ErrorFrom(c, err)
	}
}

func (h *PolicyHandler) update(c *gin.Context, ref domain.PolicyRef, actorID int64) (*domain.Policy, error) {
	ctx := service.WithPolicyActor(c.Request.Context(), actorID)
	var payload map[string]json.RawMessage
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, service.ErrWorkspaceInvalid
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, service.ErrWorkspaceInvalid
	}
	allowed := map[string]bool{
		"expected_revision": true, "allowed_models": true, "allowed_platforms": true,
		"rpm_limit": true, "daily_request_limit": true, "monthly_request_limit": true,
		"daily_token_limit": true, "monthly_token_limit": true,
	}
	for key := range payload {
		if !allowed[key] {
			return nil, service.ErrWorkspaceInvalid
		}
	}
	var expectedRevision int64
	rawRevision, ok := payload["expected_revision"]
	if !ok || json.Unmarshal(rawRevision, &expectedRevision) != nil || expectedRevision < 0 {
		return nil, service.ErrWorkspaceInvalid
	}
	current, err := h.repository.GetPolicy(ctx, ref)
	if err != nil {
		return nil, err
	}
	value := domain.Policy{Scope: ref.Scope, ScopeID: ref.ScopeID}
	if current != nil {
		value = *current
	}
	if err = decodeOptionalPolicyField(payload, "allowed_models", &value.AllowedModels); err != nil {
		return nil, service.ErrWorkspaceInvalid
	}
	if err = decodeOptionalPolicyField(payload, "allowed_platforms", &value.AllowedPlatforms); err != nil {
		return nil, service.ErrWorkspaceInvalid
	}
	for key, target := range map[string]**int64{
		"rpm_limit":             &value.RPMLimit,
		"daily_request_limit":   &value.DailyRequestLimit,
		"monthly_request_limit": &value.MonthlyRequestLimit,
		"daily_token_limit":     &value.DailyTokenLimit,
		"monthly_token_limit":   &value.MonthlyTokenLimit,
	} {
		if err = decodeOptionalPolicyField(payload, key, target); err != nil {
			return nil, service.ErrWorkspaceInvalid
		}
	}
	if err = value.Validate(); err != nil {
		return nil, service.ErrWorkspaceInvalid
	}
	return h.repository.UpdatePolicy(ctx, ref, expectedRevision, value)
}

func decodeOptionalPolicyField(payload map[string]json.RawMessage, key string, target any) error {
	raw, exists := payload[key]
	if !exists {
		return nil
	}
	if string(raw) == "null" {
		switch value := target.(type) {
		case *[]string:
			*value = nil
		case **int64:
			*value = nil
		default:
			return fmt.Errorf("unsupported nullable policy field %T", target)
		}
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}
