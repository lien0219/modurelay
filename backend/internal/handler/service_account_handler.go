package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type ServiceAccountHandler struct {
	service    *service.ServiceAccountService
	workspaces *service.WorkspaceService
	identity   *service.EnterpriseIdentityService
}

func NewServiceAccountHandler(s *service.ServiceAccountService) *ServiceAccountHandler {
	return &ServiceAccountHandler{service: s}
}
func (h *WorkspaceHandler) SetServiceAccountService(s *service.ServiceAccountService) {
	h.serviceAccounts = NewServiceAccountHandler(s)
}
func (h *ServiceAccountHandler) RegisterTenantRoutes(v1 *gin.RouterGroup) {
	base := "/workspaces/:id/projects/:project_id/service-accounts"
	routes := []struct{ method, path, action string }{
		{"GET", base, "list"}, {"POST", base, "create"}, {"GET", base + "/:service_account_id", "get"}, {"PATCH", base + "/:service_account_id", "update"},
		{"POST", base + "/:service_account_id/disable", "disable"}, {"POST", base + "/:service_account_id/enable", "enable"},
		{"GET", base + "/:service_account_id/credentials", "credentials"}, {"POST", base + "/:service_account_id/credentials", "credential.create"},
		{"PATCH", base + "/:service_account_id/credentials/:credential_id", "credential.update"}, {"POST", base + "/:service_account_id/credentials/:credential_id/revoke", "credential.revoke"}, {"POST", base + "/:service_account_id/credentials/:credential_id/rotate", "credential.rotate"},
	}
	for _, r := range routes {
		v1.Handle(r.method, r.path, h.handle(r.action, false))
	}
}
func (h *ServiceAccountHandler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/service-accounts", h.handle("list", true))
	admin.GET("/service-accounts/:service_account_id", h.handle("get", true))
	admin.POST("/service-accounts/:service_account_id/disable", h.handle("disable", true))
	admin.POST("/service-accounts/:service_account_id/credentials/:credential_id/revoke", h.handle("credential.revoke", true))
}
func (h *ServiceAccountHandler) handle(action string, admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		ids := map[string]int64{}
		for _, name := range []string{"id", "project_id", "service_account_id", "credential_id"} {
			if raw := c.Param(name); raw != "" {
				id, e := strconv.ParseInt(raw, 10, 64)
				if e != nil || id <= 0 {
					response.ErrorFrom(c, service.ErrWorkspaceNotFound)
					return
				}
				ids[name] = id
			}
		}
		a, w, p, id, k := subject.UserID, ids["id"], ids["project_id"], ids["service_account_id"], ids["credential_id"]
		ctx := c.Request.Context()
		if !admin && !checkEnterpriseWorkspaceAccess(c, h.workspaces, h.identity, subject, w) {
			return
		}
		page, size := response.ParsePagination(c)
		params := pagination.PaginationParams{Page: page, PageSize: size}
		var out any
		var err error
		var total int64
		list := false
		switch action {
		case "list":
			if admin {
				out, total, err = h.service.AdminList(ctx, a, c.Query("search"), params)
			} else {
				out, total, err = h.service.List(ctx, a, w, p, params)
			}
			list = true
		case "get":
			if admin {
				var account *service.ServiceAccount
				var credentials []service.ServiceAccountCredential
				account, credentials, err = h.service.AdminGet(ctx, a, id)
				out = gin.H{"service_account": account, "credentials": credentials}
			} else {
				out, err = h.service.Get(ctx, a, w, p, id)
			}
		case "create":
			var in service.ServiceAccountInput
			if !workspaceBind(c, &in) {
				return
			}
			out, err = h.service.Create(ctx, a, w, p, in)
		case "update":
			var in service.UpdateServiceAccountInput
			if !workspaceBind(c, &in) {
				return
			}
			out, err = h.service.Update(ctx, a, w, p, id, in)
		case "disable", "enable":
			out, err = h.service.SetStatus(ctx, a, w, p, id, action == "enable", admin)
		case "credentials":
			out, err = h.service.ListCredentials(ctx, a, w, p, id)
		case "credential.create":
			var in service.CreateAPIKeyRequest
			if !workspaceBind(c, &in) {
				return
			}
			if err = h.service.RequireCredentialMutation(ctx, a, w, p, id, "service_account.credential.create"); err != nil {
				response.ErrorFrom(c, err)
				return
			}
			// Scope IDs participate in the fingerprint: Gin's pattern alone would
			// otherwise permit replay across projects sharing the same route template.
			executeServiceAccountSecret(c, fmt.Sprintf("service-account:%d:%d:%d:create", w, p, id), gin.H{"workspace_id": w, "project_id": p, "service_account_id": id, "request": in}, func(execCtx context.Context) (*service.ServiceAccountSecret, error) {
				return h.service.CreateCredential(execCtx, a, w, p, id, in)
			})
			return
		case "credential.update":
			var in service.UpdateAPIKeyRequest
			if err = c.ShouldBindBodyWith(&in, binding.JSON); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			// Match the existing API's explicit expiration-null semantics.
			var raw map[string]json.RawMessage
			if cached, ok := c.Get(gin.BodyBytesKey); ok {
				if bytes, ok := cached.([]byte); ok {
					if json.Unmarshal(bytes, &raw) == nil {
						if value, present := raw["expires_at"]; present && string(value) == "null" {
							in.ClearExpiration = true
						}
					}
				}
			}
			out, err = h.service.UpdateCredential(ctx, a, w, p, id, k, in)
		case "credential.revoke":
			out, err = h.service.RevokeCredential(ctx, a, w, p, id, k, admin)
		case "credential.rotate":
			if err = h.service.RequireCredentialMutation(ctx, a, w, p, id, "service_account.credential.rotate"); err != nil {
				response.ErrorFrom(c, err)
				return
			}
			executeServiceAccountSecret(c, fmt.Sprintf("service-account:%d:%d:%d:rotate:%d", w, p, id, k), gin.H{"workspace_id": w, "project_id": p, "service_account_id": id, "credential_id": k}, func(execCtx context.Context) (*service.ServiceAccountSecret, error) {
				return h.service.RotateCredential(execCtx, a, w, p, id, k)
			})
			return
		}
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if list {
			response.Paginated(c, out, total, page, size)
		} else {
			response.Success(c, out)
		}
	}
}

// Capture the one-time secret outside the coordinator's result. Only safe
// metadata is serialized to its durable response cache, including replay.
func executeServiceAccountSecret(c *gin.Context, scope string, payload any, execute func(context.Context) (*service.ServiceAccountSecret, error)) {
	var secret *service.ServiceAccountSecret
	safeExecute := func(ctx context.Context) (any, error) {
		value, err := execute(ctx)
		if err != nil {
			return nil, err
		}
		secret = value
		return value.Credential, nil
	}
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		if c.GetHeader("Idempotency-Key") != "" {
			response.ErrorFrom(c, service.ErrIdempotencyStoreUnavail)
			return
		}
		_, err := safeExecute(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, secret)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{Scope: scope, ActorScope: fmt.Sprintf("user:%d", subject.UserID), Method: c.Request.Method, Route: c.FullPath(), IdempotencyKey: c.GetHeader("Idempotency-Key"), Payload: payload, TTL: service.DefaultWriteIdempotencyTTL()}, safeExecute)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
		metadata := map[string]string{}
		if bytes, e := json.Marshal(result.Data); e == nil {
			var credential service.ServiceAccountCredential
			if json.Unmarshal(bytes, &credential) == nil {
				metadata["credential_id"] = strconv.FormatInt(credential.ID, 10)
			}
		}
		response.ErrorFrom(c, service.ErrServiceAccountSecretConsumed.WithMetadata(metadata))
		return
	}
	response.Success(c, secret)
}
