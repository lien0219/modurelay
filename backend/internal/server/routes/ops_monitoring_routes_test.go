package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Exercise the real route registration: none of the new read or notification
// mutation endpoints may bypass the enclosing administrator middleware.
func TestMonitoringRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Ops: adminhandler.NewOpsHandler(nil)}}
	auth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		status := http.StatusUnauthorized
		if c.GetHeader("Authorization") != "" {
			status = http.StatusForbidden
		}
		c.AbortWithStatus(status)
	})
	audit := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, auth, audit, stepUp, nil, nil)
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "/monitoring/capabilities"},
		{http.MethodGet, "/monitoring/query"},
		{http.MethodGet, "/monitoring/events"},
		{http.MethodGet, "/monitoring/billing"},
		{http.MethodPost, "/monitoring/billing/operations/1/recheck"},
		{http.MethodPost, "/monitoring/billing/recording-gaps/instance-a/acknowledge"},
		{http.MethodGet, "/alert-deliveries"},
		{http.MethodPost, "/alert-deliveries/1/retry"},
		{http.MethodPost, "/alert-webhooks/test"},
	} {
		for _, token := range []string{"", "Bearer ordinary-user"} {
			t.Run(endpoint.method+endpoint.path+token, func(t *testing.T) {
				request := httptest.NewRequest(endpoint.method, "/api/v1/admin/ops"+endpoint.path, nil)
				request.Header.Set("Authorization", token)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				want := http.StatusUnauthorized
				if token != "" {
					want = http.StatusForbidden
				}
				require.Equal(t, want, recorder.Code)
			})
		}
	}
}
