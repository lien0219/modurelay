package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type lifecycleHandlerRepo struct {
	service.WorkspaceRepository
	service.WorkspaceLifecycleRepository
}

func (r *lifecycleHandlerRepo) GetAccess(_ context.Context, a, w, p int64) (*service.WorkspaceAccess, error) {
	if w != 1 {
		return nil, service.ErrWorkspaceNotFound
	}
	return &service.WorkspaceAccess{Workspace: &service.Workspace{ID: w, Status: "active", Type: "organization"}, Member: &service.WorkspaceMember{UserID: a, Status: "active", Role: "owner"}}, nil
}
func (r *lifecycleHandlerRepo) LifecycleRetention(context.Context, int64, int64) ([]service.LifecycleRetentionPolicy, error) {
	return []service.LifecycleRetentionPolicy{{Category: "financial", Protected: true}}, nil
}
func (r *lifecycleHandlerRepo) UpdateLifecycleRetention(ctx context.Context, a, w int64, _ string, _ int) ([]service.LifecycleRetentionPolicy, error) {
	return r.LifecycleRetention(ctx, a, w)
}
func (r *lifecycleHandlerRepo) GetLifecycleDeletion(context.Context, int64, int64) (*service.LifecycleDeletionJob, error) {
	return nil, nil
}

func TestLifecycleHandlerAuthScopeAndDisabledCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, method, body string
		auth                     bool
		want                     int
	}{
		{name: "unauthenticated", path: "/workspaces/1/lifecycle", method: "GET", want: 401},
		{name: "read", path: "/workspaces/1/lifecycle", method: "GET", auth: true, want: 200},
		{name: "foreign scope", path: "/workspaces/2/lifecycle", method: "GET", auth: true, want: 404},
		{name: "disabled export", path: "/workspaces/1/exports", method: "POST", auth: true, want: 409},
		{name: "malformed export", path: "/workspaces/1/exports/nope", method: "GET", auth: true, want: 404},
		{name: "strong guard missing", path: "/workspaces/1/deletion", method: "POST", body: `{"name":"Tenant","token":"fake"}`, auth: true, want: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			if tc.auth {
				engine.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1}) })
			}
			h := NewWorkspaceHandler(service.NewWorkspaceService(&lifecycleHandlerRepo{}), nil)
			h.registerLifecycleRoutes(engine.Group(""))
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			if tc.name == "read" {
				require.Contains(t, rec.Body.String(), `"protected":true`)
				require.Contains(t, rec.Body.String(), `"export_enabled":false`)
				require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}

}
