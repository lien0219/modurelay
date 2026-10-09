package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *WorkspaceHandler) registerLifecycleRoutes(v1 *gin.RouterGroup) {
	routes := []struct{ method, path, action string }{
		{"GET", "/workspaces/:id/lifecycle", "read"},
		{"PUT", "/workspaces/:id/retention/:category", "retention"},
		{"POST", "/workspaces/:id/restore", "restore"},
		{"POST", "/workspaces/:id/projects/:project_id/restore", "project.restore"},
		{"GET", "/workspaces/:id/exports", "export.list"}, {"POST", "/workspaces/:id/exports", "export.create"},
		{"GET", "/workspaces/:id/exports/:export_id", "export.get"},
		{"POST", "/workspaces/:id/exports/:export_id/cancel", "export.cancel"},
		{"POST", "/workspaces/:id/exports/:export_id/download", "export.authorize"},
		{"POST", "/workspaces/:id/exports/:export_id/download/redeem", "export.download"},
		{"GET", "/workspaces/:id/deletion/preflight", "deletion.preflight"},
		{"POST", "/workspaces/:id/deletion/challenge", "deletion.challenge"},
		{"GET", "/workspaces/:id/deletion", "deletion.get"}, {"POST", "/workspaces/:id/deletion", "deletion.request"},
		{"POST", "/workspaces/:id/deletion/:job_id/cancel", "deletion.cancel"},
		{"POST", "/workspaces/:id/deletion/:job_id/retry", "deletion.retry"},
	}
	for _, route := range routes {
		v1.Handle(route.method, route.path, h.lifecycleHandle(route.action))
	}
}

// UUID job parameters are parsed separately from numeric entity identifiers;
// every route runs the same live tenant/security gate as other Workspace APIs.
func (h *WorkspaceHandler) lifecycleHandle(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			response.Unauthorized(c, "User not authenticated")
			return
		}
		if subject.PrincipalType != "" && subject.PrincipalType != service.PrincipalHuman {
			response.ErrorFrom(c, service.ErrWorkspaceForbidden)
			return
		}
		w, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || w <= 0 {
			response.ErrorFrom(c, service.ErrWorkspaceNotFound)
			return
		}
		if !checkEnterpriseWorkspaceAccess(c, h.workspaces, h.identity, subject, w) {
			return
		}
		strong := action == "restore" || action == "project.restore" || action == "deletion.challenge" || action == "deletion.request" || action == "deletion.cancel" || action == "deletion.retry"
		if strong {
			if h.identityRecentAuth == nil {
				response.ErrorFrom(c, service.ErrWorkspaceForbidden)
				return
			}
			if !h.identityRecentAuth(c) {
				return
			}
		}
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		a, ctx := subject.UserID, c.Request.Context()
		id := c.Param("export_id")
		if id == "" {
			id = c.Param("job_id")
		}
		if id != "" {
			parsed, e := uuid.Parse(id)
			if e != nil || parsed == uuid.Nil || parsed.String() != id {
				response.ErrorFrom(c, service.ErrWorkspaceNotFound)
				return
			}
		}
		page, size := response.ParsePagination(c)
		var out any
		var total int64
		list := false
		switch action {
		case "read":
			var policies []service.LifecycleRetentionPolicy
			policies, err = h.workspaces.LifecycleRetention(ctx, a, w)
			if err == nil {
				var job *service.LifecycleDeletionJob
				job, err = h.workspaces.GetLifecycleDeletion(ctx, a, w)
				out = gin.H{"retention": policies, "deletion": job, "capabilities": h.workspaces.LifecycleCapabilities()}
			}
		case "retention":
			var req struct {
				RetentionDays *int `json:"retention_days"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			if req.RetentionDays == nil {
				response.ErrorFrom(c, service.ErrWorkspaceInvalid)
				return
			}
			out, err = h.workspaces.UpdateLifecycleRetention(ctx, a, w, c.Param("category"), *req.RetentionDays)
		case "restore":
			err = h.workspaces.RestoreWorkspace(ctx, a, w)
		case "project.restore":
			var p int64
			p, err = strconv.ParseInt(c.Param("project_id"), 10, 64)
			if err == nil && p > 0 {
				err = h.workspaces.RestoreProject(ctx, a, w, p)
			} else {
				err = service.ErrWorkspaceNotFound
			}
		case "export.create":
			out, err = h.workspaces.CreateLifecycleExport(ctx, a, w)
		case "export.list":
			out, total, err = h.workspaces.ListLifecycleExports(ctx, a, w, page, size)
			list = true
		case "export.get":
			out, err = h.workspaces.GetLifecycleExport(ctx, a, w, id)
		case "export.cancel":
			err = h.workspaces.CancelLifecycleExport(ctx, a, w, id)
		case "export.authorize":
			out, err = h.workspaces.AuthorizeLifecycleDownload(ctx, a, w, id)
		case "export.download":
			var req struct {
				Token string `json:"token"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			var data []byte
			data, err = h.workspaces.DownloadLifecycleExport(ctx, a, w, id, req.Token)
			if err == nil {
				c.Header("Content-Disposition", `attachment; filename="workspace-`+strconv.FormatInt(w, 10)+`-`+id+`.zip"`)
				c.Header("X-Content-Type-Options", "nosniff")
				c.Data(http.StatusOK, "application/zip", data)
				return
			}
		case "deletion.preflight":
			out, err = h.workspaces.LifecyclePreflight(ctx, a, w)
		case "deletion.challenge":
			out, err = h.workspaces.LifecycleDeletionChallenge(ctx, a, w)
		case "deletion.request":
			var req struct {
				Name  string `json:"name"`
				Token string `json:"token"`
			}
			if !workspaceBind(c, &req) {
				return
			}
			out, err = h.workspaces.RequestLifecycleDeletion(ctx, a, w, req.Name, req.Token)
		case "deletion.get":
			out, err = h.workspaces.GetLifecycleDeletion(ctx, a, w)
		case "deletion.cancel":
			err = h.workspaces.CancelLifecycleDeletion(ctx, a, w, id)
		case "deletion.retry":
			out, err = h.workspaces.RetryLifecycleDeletion(ctx, a, w, id)
		default:
			err = service.ErrWorkspaceNotFound
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
