//go:build unit

package middleware

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type tenantAdmissionFake struct{ err error }

func (f *tenantAdmissionFake) ResolveTenant(_ context.Context, k *service.APIKey) (*service.TenantContext, error) {
	if f.err != nil {
		return nil, f.err
	}
	p := int64(3)
	k.ProjectID = &p
	return &service.TenantContext{WorkspaceID: 2, ProjectID: 3, BillingPrincipalUserID: 7}, nil
}

type payerAdmissionFake struct{ tenantAdmissionFake }

func (f *payerAdmissionFake) ResolveTenant(_ context.Context, k *service.APIKey) (*service.TenantContext, error) {
	k.BillingPrincipal = &service.User{ID: 20, Status: service.StatusActive, Balance: 100}
	return &service.TenantContext{WorkspaceID: 2, ProjectID: 3, BillingPrincipalUserID: 20}, nil
}
func TestWorkspaceSubscriptionAdmissionUsesPayerAndKeepsActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		cfg := &config.Config{RunMode: config.RunModeStandard}
		groupID := int64(42)
		key := &service.APIKey{ID: 1, UserID: 10, Key: "subscription-key", GroupID: &groupID, Status: service.StatusActive, User: &service.User{ID: 10, Status: service.StatusActive, Concurrency: 3}, Group: &service.Group{ID: 42, Status: service.StatusActive, Hydrated: true, SubscriptionType: service.SubscriptionTypeSubscription}}
		repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { copyKey := *key; return &copyKey, nil }}
		svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
		svc.SetTenantResolver(&payerAdmissionFake{})
		subscriptionRepo := &stubUserSubscriptionRepo{getActive: func(_ context.Context, u, g int64) (*service.UserSubscription, error) {
			if u != 20 || g != 42 {
				return nil, service.ErrSubscriptionNotFound
			}
			now := time.Now()
			return &service.UserSubscription{DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now, ID: 9, UserID: 20, GroupID: 42, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}, nil
		}}
		subs := service.NewSubscriptionService(nil, subscriptionRepo, nil, nil, cfg)
		t.Cleanup(subs.Stop)
		r := gin.New()
		if google {
			r.Use(APIKeyAuthWithSubscriptionGoogle(svc, subs, cfg))
		} else {
			r.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, subs, cfg)))
		}
		r.GET("/t", func(c *gin.Context) {
			actor, ok := GetAuthSubjectFromContext(c)
			require.True(t, ok)
			require.Equal(t, int64(10), actor.UserID)
			require.Equal(t, 3, actor.Concurrency)
			c.Status(200)
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t", nil)
		req.Header.Set("Authorization", "Bearer subscription-key")
		r.ServeHTTP(w, req)
		require.Equal(t, 200, w.Code, w.Body.String())
	}
}
func TestWorkspaceLiveGatePrecedesSimpleAndBillingBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		for _, path := range []string{"/v1/usage", "/v1/responses"} {
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.APIKeyAuth.L1Size = 128
			repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) {
				return &service.APIKey{ID: 1, UserID: 7, Key: "tenant-test-key", Status: service.StatusActive, User: &service.User{ID: 7, Status: service.StatusActive}}, nil
			}}
			svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
			gate := &tenantAdmissionFake{}
			svc.SetTenantResolver(gate)
			r := gin.New()
			if google {
				r.Use(APIKeyAuthGoogle(svc, cfg))
			} else {
				r.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
			}
			r.GET(path, func(c *gin.Context) {
				k, _ := GetAPIKeyFromContext(c)
				require.Equal(t, int64(2), k.Tenant.WorkspaceID)
				c.Status(200)
			})
			request := func() int {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path+"?workspace_id=999&project_id=999", nil)
				req.Header.Set("Authorization", "Bearer tenant-test-key")
				req.Header.Set("X-Workspace-ID", "999")
				r.ServeHTTP(w, req)
				return w.Code
			}
			require.Equal(t, 200, request())
			for _, denial := range []error{service.ErrWorkspaceConflict, service.ErrWorkspaceForbidden, service.ErrAPIKeyNotFound, errors.New("database unavailable")} {
				gate.err = denial
				require.NotEqual(t, 200, request())
			}
		}
	}
}
