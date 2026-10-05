package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServiceAccountSecretIdempotencyNeverStoresOrReplaysSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newUserMemoryIdempotencyRepoStub()
	coordinator := service.NewIdempotencyCoordinator(repo, service.DefaultIdempotencyConfig())
	service.SetDefaultIdempotencyCoordinator(coordinator)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })
	calls := 0
	r := gin.New()
	r.POST("/secret", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		executeServiceAccountSecret(c, "safe-secret", map[string]any{"name": "bot"}, func(context.Context) (*service.ServiceAccountSecret, error) {
			calls++
			return &service.ServiceAccountSecret{Credential: &service.ServiceAccountCredential{ID: 42, Name: "bot"}, Secret: "sk-machine-secret-once"}, nil
		})
	})
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/secret", nil)
		req.Header.Set("Idempotency-Key", "unique-secret-key")
		r.ServeHTTP(w, req)
		return w
	}
	first := request()
	require.Equal(t, 200, first.Code)
	require.Contains(t, first.Body.String(), "sk-machine-secret-once")
	second := request()
	require.Equal(t, 409, second.Code)
	require.NotContains(t, second.Body.String(), "sk-machine-secret-once")
	require.Contains(t, second.Body.String(), "42")
	require.Equal(t, 1, calls)
	for _, record := range repo.data {
		bytes, e := json.Marshal(record)
		require.NoError(t, e)
		require.NotContains(t, string(bytes), "sk-machine-secret-once")
	}
}

func TestServiceAccountHTTPForeignScopesHiddenAndHumanAuthRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	denied := &deniedWorkspaceRepo{}
	svc := service.NewServiceAccountService(nil, service.NewWorkspaceAccessService(denied), nil, nil)
	h := NewServiceAccountHandler(svc)
	for _, auth := range []bool{false, true} {
		r := gin.New()
		if auth {
			r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2}) })
		}
		h.RegisterTenantRoutes(r.Group("/api/v1"))
		for _, path := range []string{"/workspaces/1/projects/4/service-accounts", "/workspaces/1/projects/4/service-accounts/8", "/workspaces/1/projects/4/service-accounts/8/credentials"} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1"+path, nil))
			if auth {
				require.Equal(t, 404, w.Code, path)
			} else {
				require.Equal(t, 401, w.Code, path)
			}
			require.False(t, strings.Contains(w.Body.String(), "sha256:"))
		}
	}
}
