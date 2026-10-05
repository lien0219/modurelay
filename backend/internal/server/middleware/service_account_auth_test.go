//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServiceAccountAuthUsesMachinePrincipalAndPayer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id, project := int64(9), int64(2)
	payer := &service.User{ID: 3, Status: service.StatusActive, Role: service.RoleUser, Balance: 20, Concurrency: 4}
	key := &service.APIKey{ID: 5, ServiceAccountID: &id, ServiceAccountStatus: service.StatusActive, ProjectID: &project, Status: service.StatusActive, BillingPrincipal: payer, Tenant: &service.TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}}
	repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { k := *key; return &k, nil }}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
	for _, google := range []bool{false, true} {
		r := gin.New()
		if google {
			r.Use(APIKeyAuthGoogle(svc, cfg))
		} else {
			r.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
		}
		r.GET("/t", func(c *gin.Context) {
			authenticated, ok := GetAPIKeyFromContext(c)
			require.True(t, ok)
			require.Nil(t, authenticated.User)
			principal := service.ExecutionPrincipalFromContext(c.Request.Context())
			require.Equal(t, key.ExecutionPrincipal(), principal)
			subject, ok := GetAuthSubjectFromContext(c)
			require.True(t, ok)
			require.Zero(t, subject.UserID)
			require.Equal(t, payer.ID, subject.FundingUserID())
			require.Equal(t, payer.Concurrency, subject.Concurrency)
			c.Status(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer sk-machine")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
}

func TestServiceAccountAdmissionPreservesHumanConcurrency(t *testing.T) {
	key := &service.APIKey{UserID: 10, User: &service.User{ID: 10, Concurrency: 3}, BillingPrincipal: &service.User{ID: 20, Concurrency: 9}}
	subject := authSubjectForAPIKey(key)
	require.Equal(t, int64(10), subject.UserID)
	require.Equal(t, int64(20), subject.BillingUserID)
	require.Equal(t, int64(10), subject.FundingUserID())
	require.Equal(t, 3, subject.Concurrency)
}
