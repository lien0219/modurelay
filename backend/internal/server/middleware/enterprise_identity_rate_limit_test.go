package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseIdentityExternalCallsAreRateLimitedAndFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &PanelRateLimiter{limiter: &fakePanelAllower{}}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(ContextKeyUser), AuthSubject{UserID: 7}); c.Next() })
	router.Use(limiter.EnterpriseIdentity())
	handler := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.POST("/api/v1/workspaces/:id/domains/:domain_id/verify", handler)
	router.POST("/api/v1/workspaces/:id/identity-providers/:provider_id/test", handler)
	router.GET("/api/v1/workspaces/:id", handler)
	for i := 0; i < 11; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest("POST", "/api/v1/workspaces/9/domains/2/verify", nil))
		if i < 10 {
			require.Equal(t, http.StatusOK, recorder.Code)
		} else {
			require.Equal(t, http.StatusTooManyRequests, recorder.Code)
			require.NotEmpty(t, recorder.Header().Get("Retry-After"))
		}
	}
	limiter.limiter = &fakePanelAllower{err: errors.New("redis unavailable")}
	for _, path := range []string{"/api/v1/workspaces/9/domains/2/verify", "/api/v1/workspaces/9/identity-providers/3/test"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest("POST", path, nil))
		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/v1/workspaces/9", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}
