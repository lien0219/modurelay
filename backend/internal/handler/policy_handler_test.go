package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type policyHandlerWorkspaceRepo struct {
	service.WorkspaceRepository
	access *service.WorkspaceAccess
}

func (r *policyHandlerWorkspaceRepo) GetAccess(context.Context, int64, int64, int64) (*service.WorkspaceAccess, error) {
	return r.access, nil
}
func (r *policyHandlerWorkspaceRepo) ListProjects(context.Context, int64, int64, pagination.PaginationParams) ([]service.Project, int64, error) {
	return nil, 0, service.ErrWorkspaceNotFound
}

type policyHandlerServiceAccountRepo struct {
	service.ServiceAccountRepository
}

func (r *policyHandlerServiceAccountRepo) Get(context.Context, int64, int64, int64, int64) (*service.ServiceAccount, error) {
	return &service.ServiceAccount{ID: 9, WorkspaceID: 1, ProjectID: 2, Status: service.StatusActive}, nil
}

type policyHandlerActorRepo struct {
	domain.PolicyRepository
	actorID int64
}

func (r *policyHandlerActorRepo) UpdatePolicy(ctx context.Context, ref domain.PolicyRef, revision int64, value domain.Policy) (*domain.Policy, error) {
	r.actorID = service.PolicyActorID(ctx)
	return r.PolicyRepository.UpdatePolicy(ctx, ref, revision, value)
}

func TestPolicyHTTPReadUsesCentralPolicyPermissionAndPreservesNull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	models := []string{}
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 1}, 0, domain.Policy{AllowedModels: models})
	require.NoError(t, err)
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "viewer", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/workspaces/1/policy", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	var envelope struct {
		Data domain.Policy `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data.AllowedModels)
	require.Empty(t, envelope.Data.AllowedModels)
}

func TestPolicyHTTPUpdateRequiresPolicyPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "developer", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(`{"expected_revision":0,"allowed_models":[]}`)))
	require.Equal(t, 403, w.Code, w.Body.String())
}

func TestPolicyHTTPUpdateReturnsRevisionConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 1}, 0, domain.Policy{})
	require.NoError(t, err)
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "owner", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(`{"expected_revision":9,"allowed_models":null}`)))
	require.Equal(t, 409, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "POLICY_CONFLICT")
}

func TestPolicyHTTPUpdateAttributesAuthenticatedActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &policyHandlerActorRepo{PolicyRepository: domain.NewMemoryPolicyStore()}
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "owner", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(`{"expected_revision":0,"allowed_models":[]}`)))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.EqualValues(t, 4, store.actorID)
}

func TestPolicyHTTPUpdateUsesStableInvalidReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "owner", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(`{"expected_revision":-1}`)))
	require.Equal(t, 400, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"reason":"POLICY_INVALID"`)
}

func TestPolicyHTTPUpdateRejectsOversizedBodyAfterValidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "owner", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	body := `{"expected_revision":0}` + strings.Repeat(" ", 1<<20)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(body)))
	require.Equal(t, 400, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"reason":"POLICY_INVALID"`)
	policy, err := store.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 1})
	require.NoError(t, err)
	require.Nil(t, policy)
}

func TestPolicyHTTPPatchRetainsNullVersusEmptyAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "owner", Status: "active"}}}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), nil, store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(`{"expected_revision":0,"allowed_models":[],"allowed_platforms":null}`)))
	require.Equal(t, 200, w.Code, w.Body.String())
	var envelope struct {
		Data domain.Policy `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Data.AllowedModels)
	require.Empty(t, envelope.Data.AllowedModels)
	require.Nil(t, envelope.Data.AllowedPlatforms)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/policy", strings.NewReader(`{"expected_revision":1,"allowed_models":null}`)))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Nil(t, envelope.Data.AllowedModels)
	require.Nil(t, envelope.Data.AllowedPlatforms)
}

func TestPolicyHTTPServiceAccountRouteRejectsCrossTenantTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{Workspace: &service.Workspace{ID: 1, Status: "active"}, Project: &service.Project{ID: 2, WorkspaceID: 1, Status: "active"}, Member: &service.WorkspaceMember{Role: "developer", Status: "active"}}}
	sa := &policyHandlerServiceAccountRepo{}
	h := NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), service.NewServiceAccountService(sa, service.NewWorkspaceAccessService(workspaceRepo), nil, nil), store, domain.NewEffectivePolicyResolver(store))
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/workspaces/999/projects/2/service-accounts/9/policy", nil))
	require.Equal(t, 404, w.Code, w.Body.String())
}

func TestPolicyHTTPServiceAccountViewerGrantCannotUpdatePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := domain.NewMemoryPolicyStore()
	workspaceRepo := &policyHandlerWorkspaceRepo{access: &service.WorkspaceAccess{
		Workspace:          &service.Workspace{ID: 1, Status: "active", ProjectAccessMode: service.ProjectAccessModeAssigned},
		Project:            &service.Project{ID: 2, WorkspaceID: 1, Status: "active"},
		Member:             &service.WorkspaceMember{Role: "developer", Status: "active"},
		ProjectRole:        service.ProjectAccessRoleViewer,
		ProjectPermissions: service.ProjectRolePermissions(service.ProjectAccessRoleViewer),
	}}
	sa := service.NewServiceAccountService(&policyHandlerServiceAccountRepo{}, service.NewWorkspaceAccessService(workspaceRepo), nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 4}) })
	NewPolicyHandler(service.NewWorkspaceService(workspaceRepo), sa, store, nil).RegisterTenantRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/v1/workspaces/1/projects/2/service-accounts/9/policy", strings.NewReader(`{"expected_revision":0,"allowed_models":[]}`)))
	require.Equal(t, 403, w.Code, w.Body.String())
	policy, err := store.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: 9})
	require.NoError(t, err)
	require.Nil(t, policy, "denied policy writes must have no side effects")
}
