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

func TestLoginSecuritySettingsPUTUsesStepUpMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Setting: adminhandler.NewSettingHandler(nil, nil, nil, nil, nil, nil, nil),
	}}
	stepUpCalled := false
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) {
		stepUpCalled = true
		servermiddleware.AbortWithError(c, http.StatusForbidden, "STEP_UP_REQUIRED", "two-factor verification required")
	})
	registerSettingsRoutes(router.Group("/api/v1/admin"), handlers, stepUp)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/login-security", nil)
	router.ServeHTTP(recorder, request)

	require.True(t, stepUpCalled)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
