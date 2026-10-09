package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/common/expfmt"
)

// A finite process-wide budget bounds on-demand database observations without
// retaining per-actor, per-workspace or per-query state. Panel limiting remains
// the outer HTTP policy; this budget also covers guarded operation transactions.
var enterpriseAdminRequests = make(chan struct{}, 4)

const enterpriseAdminTimeout = 10 * time.Second

type enterpriseAdminPage struct {
	Items       []service.Workspace `json:"items"`
	Total       int64               `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
	Pages       int64               `json:"pages"`
	TotalCapped bool                `json:"total_capped"`
	ObservedAt  time.Time           `json:"observed_at"`
}

func (h *WorkspaceHandler) enterpriseAdminHandle(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			enterpriseAdminError(c, 401, "UNAUTHORIZED", "User not authenticated")
			return
		}
		role, _ := middleware.GetUserRoleFromContext(c)
		if role != service.RoleAdmin || subject.ServiceAccountID > 0 || (subject.PrincipalType != "" && subject.PrincipalType != service.PrincipalHuman) {
			enterpriseAdminFailure(c, service.ErrWorkspaceForbidden, false)
			return
		}
		mutation := action == "status" || action == "retry"
		if mutation && (c.GetString("auth_method") != "jwt" || c.GetHeader("x-api-key") != "") {
			enterpriseAdminFailure(c, service.ErrWorkspaceForbidden, true)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), enterpriseAdminTimeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		if mutation {
			// This is unconditional and independent of the optional default-off
			// StepUpGuard. Its trusted proof is rechecked by service and repository.
			if h.identityRecentAuth == nil {
				enterpriseAdminFailure(c, service.ErrWorkspaceForbidden, true)
				return
			}
			if !h.identityRecentAuth(c) {
				if !c.Writer.Written() {
					enterpriseAdminFailure(c, service.ErrRecentAuthenticationRequired, true)
				}
				return
			}
		}
		var workspaceID, webhookID, deliveryID int64
		for _, param := range []struct {
			name   string
			target *int64
		}{{"id", &workspaceID}, {"webhook_id", &webhookID}, {"delivery_id", &deliveryID}} {
			if raw := c.Param(param.name); raw != "" {
				id, err := enterpriseAdminPositive(raw)
				if err != nil {
					enterpriseAdminFailure(c, service.ErrWorkspaceInvalid, mutation)
					return
				}
				*param.target = id
			}
		}
		filter, err := enterpriseAdminQuery(c, action == "search")
		if err != nil {
			enterpriseAdminFailure(c, err, mutation)
			return
		}
		var input service.AdminOperationInput
		if mutation && !enterpriseAdminBind(c, &input) {
			return
		}
		if h.workspaces == nil {
			enterpriseAdminError(c, 503, "ADMIN_DIAGNOSTICS_UNAVAILABLE", "Administrator diagnostics are unavailable")
			return
		}
		select {
		case enterpriseAdminRequests <- struct{}{}:
			defer func() { <-enterpriseAdminRequests }()
		default:
			c.Header("Retry-After", "1")
			enterpriseAdminError(c, 429, "ADMIN_DIAGNOSTICS_BUSY", "Administrator diagnostics are busy")
			return
		}
		// The recent-auth guard can append trusted proof to the request context.
		ctx = c.Request.Context()
		var out any
		switch action {
		case "search":
			var page *service.AdminWorkspacePage
			page, err = h.workspaces.AdminSearchWorkspaces(ctx, subject.UserID, filter)
			if err == nil && page != nil {
				pages := page.TotalPages
				if pages < 1 {
					pages = 1
				}
				items := page.Items
				if items == nil {
					items = []service.Workspace{}
				}
				for i := range items {
					items[i].Permissions = []string{}
				}
				out = enterpriseAdminPage{items, page.Total, page.Page, page.PageSize, pages, page.TotalCapped, page.ObservedAt}
			}
		case "overview":
			out, err = h.workspaces.AdminDiagnosticsOverview(ctx, subject.UserID)
		case "diagnostics":
			out, err = h.workspaces.AdminWorkspaceDiagnostics(ctx, subject.UserID, workspaceID)
		case "inspect":
			var workspace *service.Workspace
			workspace, err = h.workspaces.AdminInspect(ctx, subject.UserID, workspaceID)
			if workspace != nil {
				workspace.Permissions = []string{}
			}
			out = workspace
		case "jobs", "health", "metrics":
			// Even cached/current-instance metrics require an independent live
			// repository authorization and bounded global observation each scrape.
			out, err = h.workspaces.AdminOperationsJobs(ctx, subject.UserID)
			if action == "metrics" && err == nil {
				enterpriseAdminMetrics(c)
				return
			}
		case "status", "retry":
			var receipt *service.AdminOperationReceipt
			if action == "status" {
				receipt, err = h.workspaces.AdminOperateWorkspace(ctx, subject.UserID, workspaceID, input)
			} else {
				receipt, err = h.workspaces.AdminRetryWebhook(ctx, subject.UserID, workspaceID, webhookID, deliveryID, input)
			}
			if receipt != nil {
				c.Header("X-Admin-Operation-Receipt-ID", receipt.ID)
			}
			if err == nil && receipt != nil {
				if action == "retry" {
					out = receipt
				} else {
					var workspace *service.Workspace
					workspace, err = h.workspaces.AdminInspect(ctx, subject.UserID, workspaceID)
					if err != nil || workspace == nil {
						// The transaction already committed (or replayed). Surface
						// its immutable identity even if the follow-up read fails.
						response.ErrorWithDetails(c, 502, "Administrator operation completed; result inspection is unavailable", "ADMIN_OPERATION_RESULT_UNAVAILABLE", map[string]string{"receipt_id": receipt.ID, "mutation_committed": "true"})
						return
					}
					workspace.Status, workspace.UpdatedAt = receipt.ResultStatus, receipt.ResultUpdatedAt
					workspace.Permissions = []string{}
					out = workspace
				}
			}
		}
		// Production scanWorkspace already normalizes no-rows errors. Keep a
		// defensive fallback for raw errors from compatible repository delegates
		// on these target reads; overview/jobs source errors remain unavailable.
		if (action == "inspect" || action == "diagnostics") && errors.Is(err, sql.ErrNoRows) {
			err = service.ErrWorkspaceNotFound
		}
		if err != nil {
			enterpriseAdminFailure(c, err, mutation)
			return
		}
		if out == nil {
			enterpriseAdminError(c, 500, "ADMIN_DIAGNOSTICS_UNAVAILABLE", "Administrator diagnostics are unavailable")
			return
		}
		response.Success(c, out)
	}
}

func enterpriseAdminPositive(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != raw {
		return 0, service.ErrWorkspaceInvalid
	}
	return id, nil
}

func enterpriseAdminQuery(c *gin.Context, search bool) (service.AdminWorkspaceFilter, error) {
	f := service.AdminWorkspaceFilter{}
	if len(c.Request.URL.RawQuery) > 4096 {
		return f, service.ErrWorkspaceInvalid
	}
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil || (!search && len(query) > 0) {
		// The frontend transport attaches its bounded IANA timezone metadata to
		// every GET. It is accepted and ignored; all diagnostic semantics remain
		// UTC/server sourced. No other unknown query is accepted.
		if err != nil || len(query) != 1 || len(query["timezone"]) != 1 {
			return f, service.ErrWorkspaceInvalid
		}
	}
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return f, service.ErrWorkspaceInvalid
		}
		value := values[0]
		switch key {
		case "timezone":
			if len(value) > 64 {
				return f, service.ErrWorkspaceInvalid
			}
			if _, e := time.LoadLocation(value); e != nil {
				return f, service.ErrWorkspaceInvalid
			}
		case "workspace_id", "owner_user_id", "billing_owner_user_id", "page", "page_size":
			id, e := enterpriseAdminPositive(value)
			if e != nil {
				return f, e
			}
			switch key {
			case "workspace_id":
				f.WorkspaceID = id
			case "owner_user_id":
				f.OwnerUserID = id
			case "billing_owner_user_id":
				f.BillingOwnerUserID = id
			case "page":
				if id > 10001 {
					return f, service.ErrWorkspaceInvalid
				}
				f.Page = int(id)
			case "page_size":
				if id > 100 {
					return f, service.ErrWorkspaceInvalid
				}
				f.PageSize = int(id)
			}
		case "name_prefix":
			f.NamePrefix = value
		case "slug_prefix":
			f.SlugPrefix = value
		case "status":
			f.Status = value
		case "type":
			f.Type = value
		case "created_from":
			f.CreatedFrom = value
		case "created_to":
			f.CreatedTo = value
		case "updated_from":
			f.UpdatedFrom = value
		case "updated_to":
			f.UpdatedTo = value
		case "sort":
			f.Sort = value
		case "direction":
			f.Direction = value
		default:
			return f, service.ErrWorkspaceInvalid
		}
	}
	return f, f.Validate()
}

func enterpriseAdminBind(c *gin.Context, input *service.AdminOperationInput) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			enterpriseAdminError(c, 413, "ADMIN_OPERATION_BODY_TOO_LARGE", "Administrator operation body exceeds the limit")
		} else {
			enterpriseAdminFailure(c, service.ErrWorkspaceInvalid, true)
		}
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if !utf8.Valid(raw) || decoder.Decode(input) != nil || decoder.Decode(new(any)) != io.EOF || input.Validate() != nil {
		enterpriseAdminFailure(c, service.ErrWorkspaceInvalid, true)
		return false
	}
	return true
}

func enterpriseAdminMetrics(c *gin.Context) {
	families, err := service.EnterpriseAdminMetricsGatherer().Gather()
	if err != nil {
		enterpriseAdminFailure(c, err, false)
		return
	}
	var buffer bytes.Buffer
	for _, family := range families {
		if _, err = expfmt.MetricFamilyToText(&buffer, family); err != nil {
			enterpriseAdminFailure(c, err, false)
			return
		}
	}
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", buffer.Bytes())
}

func enterpriseAdminError(c *gin.Context, status int, code, message string) {
	response.ErrorWithDetails(c, status, message, code, nil)
}

// Only fixed errors cross the HTTP boundary. Wrapped SQL/provider messages or
// arbitrary ApplicationError metadata cannot be copied into public responses.
func enterpriseAdminFailure(c *gin.Context, err error, mutation bool) {
	switch {
	case errors.Is(err, service.ErrWorkspaceForbidden):
		enterpriseAdminError(c, 403, "WORKSPACE_FORBIDDEN", "Workspace permission denied")
	case errors.Is(err, service.ErrWorkspaceNotFound):
		enterpriseAdminError(c, 404, "WORKSPACE_NOT_FOUND", "Workspace resource not found")
	case errors.Is(err, service.ErrWorkspaceInvalid):
		enterpriseAdminError(c, 400, "WORKSPACE_INVALID", "Invalid workspace input")
	case errors.Is(err, service.ErrWorkspaceConflict):
		enterpriseAdminError(c, 409, "WORKSPACE_CONFLICT", "Workspace state or precondition conflicts")
	case errors.Is(err, service.ErrAdminOperationIdempotencyConflict):
		enterpriseAdminError(c, 409, "ADMIN_OPERATION_IDEMPOTENCY_CONFLICT", "Administrator operation key conflicts with a different request")
	case errors.Is(err, service.ErrRecentAuthenticationRequired):
		enterpriseAdminError(c, 403, "RECENT_AUTH_REQUIRED", "Recent authentication is required for this operation")
	case errors.Is(err, service.ErrMFARequired):
		enterpriseAdminError(c, 403, "MFA_REQUIRED", "The current session must complete MFA")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		enterpriseAdminError(c, 504, "ADMIN_DIAGNOSTICS_TIMEOUT", "Administrator request timed out")
	default:
		if mutation {
			enterpriseAdminError(c, 500, "ADMIN_OPERATION_FAILED", "Administrator operation failed")
		} else {
			enterpriseAdminError(c, 500, "ADMIN_DIAGNOSTICS_UNAVAILABLE", "Administrator diagnostics are unavailable")
		}
	}
}
