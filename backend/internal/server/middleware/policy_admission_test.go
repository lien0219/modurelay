package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type policyQuotaMiddlewareRepo struct {
	mu           sync.Mutex
	reserved     int
	requestUnits []int64
	finalized    []int64
	released     int
	reserveErr   error
}

func policyInt64ptr(v int64) *int64 { return &v }

func (r *policyQuotaMiddlewareRepo) Reserve(_ context.Context, req service.PolicyQuotaReservationRequest) (*service.PolicyQuotaReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reserveErr != nil {
		return nil, r.reserveErr
	}
	r.reserved++
	r.requestUnits = append(r.requestUnits, req.RequestUnits)
	return &service.PolicyQuotaReservation{ID: "middleware-quota", RequestID: req.RequestID, APIKeyID: req.APIKeyID, EstimatedTokens: req.EstimatedTokens, Status: service.PolicyQuotaReservationPending}, nil
}
func (r *policyQuotaMiddlewareRepo) Finalize(_ context.Context, _ string, tokens int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finalized = append(r.finalized, tokens)
	return nil
}
func (r *policyQuotaMiddlewareRepo) Release(_ context.Context, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.released++
	return nil
}

func policyTestRouter(t *testing.T, store *domain.MemoryPolicyStore, key *service.APIKey, platform string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Request = c.Request.WithContext(service.WithExecutionPrincipal(c.Request.Context(), key.ExecutionPrincipal()))
		if platform != "" {
			c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), platform))
		}
		c.Next()
	})
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store)))
	r.Use(GroupModelAllowlist())
	r.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.POST("/v1beta/models/:modelAction", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

func TestPolicyAdmissionRejectsWorkspaceModel(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{AllowedModels: []string{"gpt-6"}})
	require.NoError(t, err)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, "openai")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-4"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "model_not_found")
	require.Contains(t, w.Body.String(), "POLICY_MODEL_DENIED")
}

func TestPolicyAdmissionKeepsGroupRestrictionAsAnANDLayer(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	key := &service.APIKey{ID: 5, UserID: 7, Group: &service.Group{ID: 3, Platform: service.PlatformOpenAI, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6"}}}}
	r := policyTestRouter(t, store, key, service.PlatformOpenAI)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-4"}`)))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "model_not_found")
	require.NotContains(t, w.Body.String(), "POLICY_MODEL_DENIED")
}

func TestPolicyAdmissionChecksResolvedPlatform(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{AllowedPlatforms: []string{"openai"}})
	require.NoError(t, err)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, service.PlatformAnthropic)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "POLICY_PLATFORM_DENIED")
}

func TestPolicyAdmissionChecksForcedPlatformOnModelFreeRequests(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{AllowedPlatforms: []string{service.PlatformOpenAI}})
	require.NoError(t, err)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		ctx := context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, service.PlatformAntigravity)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store)))
	r.GET("/antigravity/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/antigravity/models", nil))

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "POLICY_PLATFORM_DENIED")
}

func TestPolicyAdmissionChecksEveryModelCandidate(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{AllowedModels: []string{"gpt-6"}})
	require.NoError(t, err)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, "openai")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6","Model":"claude-4"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "POLICY_MODEL_DENIED")
	require.Contains(t, w.Body.String(), "scope=workspace")
}

func TestPolicyAdmissionExplainsCredentialRestriction(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21, AllowedModels: []string{"gpt-6"}}}
	r := policyTestRouter(t, store, key, "openai")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-4"}`)))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "model_not_found")
	require.NotContains(t, w.Body.String(), "POLICY_MODEL_DENIED")
}

func TestPolicyAdmissionExplainsGroupRestriction(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	key := &service.APIKey{ID: 5, UserID: 7, Group: &service.Group{
		ID: 3, Platform: service.PlatformOpenAI,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}},
	}}
	r := policyTestRouter(t, store, key, service.PlatformOpenAI)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-4"}`)))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "model_not_found")
	require.NotContains(t, w.Body.String(), "POLICY_MODEL_DENIED")
}

func TestPolicyAdmissionLoadsGroupAndCredentialLayers(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeGroup, ScopeID: 3}, 0, domain.Policy{AllowedModels: []string{"gpt-6"}})
	require.NoError(t, err)
	_, err = store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeCredential, ScopeID: 5}, 0, domain.Policy{AllowedModels: []string{"claude-4"}})
	require.NoError(t, err)
	groupID := int64(3)
	key := &service.APIKey{ID: 5, UserID: 7, GroupID: &groupID, Group: &service.Group{ID: 3, Platform: service.PlatformOpenAI}, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, service.PlatformOpenAI)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "scope=credential")
}

func TestPolicyAdmissionDirectKeyNeverLoadsServiceAccountPolicy(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: 31}, 0, domain.Policy{
		AllowedModels: []string{},
	})
	require.NoError(t, err)
	_, err = store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{
		AllowedModels: []string{"gpt-6"},
	})
	require.NoError(t, err)

	// A direct key has no service-account identity. The same tenant may contain
	// a machine identity with a restrictive policy, but it must not affect this
	// user's request.
	key := &service.APIKey{ID: 51, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, service.PlatformOpenAI)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestPolicyAdmissionServiceAccountPolicyStillAppliesToMachineKey(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: 31}, 0, domain.Policy{
		AllowedModels: []string{},
	})
	require.NoError(t, err)
	serviceAccountID := int64(31)
	key := &service.APIKey{ID: 52, ServiceAccountID: &serviceAccountID, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, service.PlatformOpenAI)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "scope=service_account")
}

func TestPolicyAdmissionProjectsLegacyGroupAndCredentialLayers(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	groupID := int64(3)
	key := &service.APIKey{
		ID:      5,
		UserID:  7,
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelAllowlist: service.GroupModelAllowlist{
				Enabled: true,
				Models:  []string{"gpt-*"},
			},
		},
		Tenant: &service.TenantContext{
			WorkspaceID: 11,
			ProjectID:   21,
			AllowedModels: []string{
				"gpt-6",
			},
		},
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Next()
	})
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store)))
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		policy, ok := service.EffectivePolicyFromContext(c.Request.Context())
		require.True(t, ok)
		require.NotNil(t, policy.Layers.Group)
		require.Equal(t, domain.PolicyScopeGroup, policy.Layers.Group.Scope)
		require.Equal(t, []string{"gpt-*"}, policy.Layers.Group.AllowedModels)
		require.NotNil(t, policy.Layers.Credential)
		require.Equal(t, domain.PolicyScopeCredential, policy.Layers.Credential.Scope)
		require.Equal(t, []string{"gpt-6"}, policy.Layers.Credential.AllowedModels)
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestResolvePolicyDoesNotProjectLegacyGroupRPMIntoGlobalPolicy(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	workspaceRPM := int64(100)
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{RPMLimit: &workspaceRPM})
	require.NoError(t, err)

	key := &service.APIKey{
		ID:     5,
		Group:  &service.Group{ID: 3, RPMLimit: 50},
		Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21},
	}

	policy, legacy, err := resolvePolicyForAPIKey(context.Background(), domain.NewEffectivePolicyResolver(store), key)
	require.NoError(t, err)
	require.Nil(t, policy.Layers.Group, "legacy group RPM must stay on the per-user legacy counter")
	require.False(t, legacy.group)
	require.Equal(t, int64(100), policy.RPMLimitValue())
}

func TestPolicyAdmissionPreservesCodeForGoogleProtocol(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{AllowedModels: []string{}})
	require.NoError(t, err)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := policyTestRouter(t, store, key, service.PlatformGemini)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", strings.NewReader(`{"contents":[]}`)))
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "POLICY_MODEL_DENIED")
}

func TestPolicyAdmissionReservesAndFinalizesPolicyQuota(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{DailyRequestLimit: policyInt64ptr(2), DailyTokenLimit: policyInt64ptr(100)})
	require.NoError(t, err)
	repo := &policyQuotaMiddlewareRepo{}
	quota := service.NewPolicyQuotaService(repo)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Next()
	})
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store), quota))
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		h := service.PolicyQuotaReservationFromContext(c.Request.Context())
		require.NotNil(t, h)
		require.NoError(t, h.Finalize(c.Request.Context(), 7))
		c.Status(http.StatusNoContent)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6","max_tokens":4}`)))
	require.Equal(t, http.StatusNoContent, w.Code)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.reserved)
	require.Equal(t, []int64{7}, repo.finalized)
	require.Zero(t, repo.released)
}

func TestPolicyAdmissionQuotaRejectsWithRetryAfter(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{DailyRequestLimit: policyInt64ptr(1)})
	require.NoError(t, err)
	quotaRepo := &policyQuotaMiddlewareRepo{}
	quota := service.NewPolicyQuotaService(quotaRepo)
	// Use a repository that reports a durable exhaustion decision.
	quotaRepo.reserveErr = fmt.Errorf("%w: workspace", service.ErrPolicyQuotaExceeded)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store), quota))
	r.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestPolicyAdmissionChargesBatchLogicalItemsAsRequestUnits(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{DailyRequestLimit: policyInt64ptr(5)})
	require.NoError(t, err)
	repo := &policyQuotaMiddlewareRepo{}
	quota := service.NewPolicyQuotaService(repo)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store), quota))
	r.POST("/v1/images/batches", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/images/batches", strings.NewReader(`{"model":"gpt-image-1","items":[{"prompt":"one"},{"prompt":"two"},{"prompt":"three"}]}`)))
	require.Equal(t, http.StatusNoContent, w.Code)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, []int64{3}, repo.requestUnits)
}

func TestPolicyAdmissionChargesOrdinaryRequestAsOneUnit(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{DailyRequestLimit: policyInt64ptr(5)})
	require.NoError(t, err)
	repo := &policyQuotaMiddlewareRepo{}
	quota := service.NewPolicyQuotaService(repo)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store), quota))
	r.POST("/v1/chat/completions", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-6"}`)))
	require.Equal(t, http.StatusNoContent, w.Code)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, []int64{1}, repo.requestUnits)
}

func TestPolicyAdmissionLeavesDurableAsyncReservationForDetachedWorker(t *testing.T) {
	store := domain.NewMemoryPolicyStore()
	_, err := store.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 11}, 0, domain.Policy{DailyRequestLimit: policyInt64ptr(5)})
	require.NoError(t, err)
	repo := &policyQuotaMiddlewareRepo{}
	quota := service.NewPolicyQuotaService(repo)
	key := &service.APIKey{ID: 5, UserID: 7, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	r.Use(PolicyAdmissionMiddleware(domain.NewEffectivePolicyResolver(store), quota))
	r.POST("/v1/images/generations/async", func(c *gin.Context) {
		handle := service.PolicyQuotaReservationFromContext(c.Request.Context())
		require.NotNil(t, handle)
		handle.Preserve()
		c.Status(http.StatusAccepted)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1"}`)))
	require.Equal(t, http.StatusAccepted, w.Code)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.reserved)
	require.Zero(t, repo.released)
}
