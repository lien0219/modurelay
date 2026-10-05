package handler

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProjectKeyDTOCreateSecretIsOneTimeAndReadsAreMasked(t *testing.T) {
	projectID := int64(4)
	created := &service.APIKey{ID: 8, ProjectID: &projectID, Key: "sk-created-secret"}
	createdDTO, ok := projectKeyCreateDTO(created).(workspaceKeyDTO)
	require.True(t, ok)
	require.Equal(t, "sk-created-secret", createdDTO.Key)

	readDTO, ok := projectKeyDTO(created).(workspaceKeyDTO)
	require.True(t, ok)
	require.Equal(t, "****cret", readDTO.Key)
	// Mapping a read must not destroy the one-time value retained by the caller.
	require.Equal(t, "sk-created-secret", created.Key)
}

type deniedWorkspaceRepo struct {
	service.WorkspaceRepository
	access *service.WorkspaceAccess
}

func (r *deniedWorkspaceRepo) GetAccess(context.Context, int64, int64, int64) (*service.WorkspaceAccess, error) {
	if r.access != nil {
		return r.access, nil
	}
	return nil, service.ErrWorkspaceNotFound
}
func (r *deniedWorkspaceRepo) ListProjects(context.Context, int64, int64, pagination.PaginationParams) ([]service.Project, int64, error) {
	return nil, 0, service.ErrWorkspaceNotFound
}
func (r *deniedWorkspaceRepo) ListMembers(context.Context, int64, int64, pagination.PaginationParams) ([]service.WorkspaceMember, int64, error) {
	return nil, 0, service.ErrWorkspaceNotFound
}
func (r *deniedWorkspaceRepo) ListInvitations(context.Context, int64, int64, pagination.PaginationParams) ([]service.WorkspaceInvitation, int64, error) {
	return nil, 0, service.ErrWorkspaceNotFound
}
func (r *deniedWorkspaceRepo) ListAudit(context.Context, int64, int64, pagination.PaginationParams) ([]service.WorkspaceAudit, int64, error) {
	return nil, 0, service.ErrWorkspaceNotFound
}

type deniedProjectKeyRepo struct {
	service.APIKeyRepository
	service.ProjectKeyRepository
}

func TestWorkspaceHTTPForeignNumericScopesAreHidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &deniedWorkspaceRepo{}
	keys := service.NewAPIKeyService(&deniedProjectKeyRepo{}, nil, nil, nil, nil, nil, nil)
	keys.ConfigureWorkspaces(repo)
	h := NewWorkspaceHandler(service.NewWorkspaceService(repo), keys)
	for _, authenticated := range []bool{false, true} {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if authenticated {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2})
			}
		})
		h.RegisterTenantRoutes(r.Group("/api/v1"))
		cases := []struct{ method, path, body string }{
			{"GET", "/workspaces/1", ""}, {"PATCH", "/workspaces/1", `{"name":"Valid","slug":"valid"}`}, {"DELETE", "/workspaces/1", ""},
			{"GET", "/workspaces/1/members", ""}, {"PATCH", "/workspaces/1/members/8", `{"role":"viewer","status":"active"}`}, {"DELETE", "/workspaces/1/members/8", ""},
			{"GET", "/workspaces/1/invitations", ""}, {"POST", "/workspaces/1/invitations", `{"email":"new@example.com","role":"viewer"}`}, {"DELETE", "/workspaces/1/invitations/8", ""},
			{"GET", "/workspaces/1/projects", ""}, {"POST", "/workspaces/1/projects", `{"name":"Valid","slug":"valid"}`}, {"GET", "/workspaces/1/projects/4", ""}, {"PATCH", "/workspaces/1/projects/4", `{"name":"Valid","slug":"valid"}`}, {"DELETE", "/workspaces/1/projects/4", ""},
			{"GET", "/workspaces/1/projects/4/keys", ""}, {"POST", "/workspaces/1/projects/4/keys", `{"name":"Valid"}`}, {"GET", "/workspaces/1/projects/4/keys/8", ""}, {"PATCH", "/workspaces/1/projects/4/keys/8", `{"name":"Changed"}`}, {"DELETE", "/workspaces/1/projects/4/keys/8", ""}, {"GET", "/workspaces/1/projects/4/groups/available", ""}, {"GET", "/workspaces/1/audit", ""},
			{"GET", "/workspaces/1/usage", ""}, {"GET", "/workspaces/1/overview", ""}, {"GET", "/workspaces/1/budget", ""}, {"PUT", "/workspaces/1/budget", `{"amount":10,"enabled":true}`},
			{"GET", "/workspaces/1/projects/4/usage", ""}, {"GET", "/workspaces/1/projects/4/overview", ""}, {"GET", "/workspaces/1/projects/4/budget", ""}, {"PUT", "/workspaces/1/projects/4/budget", `{"amount":10,"enabled":true}`},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%t %s %s", authenticated, tc.method, tc.path), func(t *testing.T) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(tc.method, "/api/v1"+tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(w, req)
				want := 401
				if authenticated {
					want = 404
				}
				require.Equal(t, want, w.Code, w.Body.String())
			})
		}
	}
}

func TestWorkspaceFinOpsDefaultRangeIsLocalCalendarMonth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?timezone=Asia%2FShanghai&end=2026-10-05T00%3A00%3A00Z", nil)
	start, end, tz, err := finopsRange(ctx)
	require.NoError(t, err)
	require.Equal(t, "Asia/Shanghai", tz)
	require.True(t, start.Equal(time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)))
	require.True(t, end.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)))
}
func TestWorkspaceHTTPForbiddenAndConflictEnvelopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		role, status string
		want         int
	}{{"viewer", "active", 403}, {"owner", "suspended", 409}, {"owner", "archived", 409}} {
		repo := &deniedWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Name: "Valid", Slug: "valid", Status: tc.status}, Project: &service.Project{ID: 4, Status: "active"}, Member: &service.WorkspaceMember{Role: tc.role, Status: "active"}}}
		keys := service.NewAPIKeyService(&deniedProjectKeyRepo{}, nil, nil, nil, nil, nil, nil)
		keys.ConfigureWorkspaces(repo)
		h := NewWorkspaceHandler(service.NewWorkspaceService(repo), keys)
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2}) })
		h.RegisterTenantRoutes(r.Group("/api/v1"))
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/workspaces/1/projects/4/keys", strings.NewReader(`{"name":"Valid","workspace_id":999,"project_id":999}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, tc.want, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), `"reason":"WORKSPACE_`)
	}
}
