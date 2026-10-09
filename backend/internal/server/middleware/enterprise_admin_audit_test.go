package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseAdminDiagnosticReadsAuditMetadataWithoutResponseOrQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &auditCaptureRepository{}
	audit := service.NewAuditLogService(repository, nil)
	audit.Start()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
		c.Set(string(ContextKeyUserRole), "admin")
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
	for _, path := range []string{"/workspaces", "/workspaces/:id", "/workspaces/diagnostics/overview", "/workspaces/:id/diagnostics", "/operations/jobs", "/operations/health", "/operations/metrics"} {
		router.GET("/api/v1/admin"+path, func(c *gin.Context) { c.JSON(200, gin.H{"secret": "audit-response-canary"}) })
	}
	for _, path := range []string{"/workspaces", "/workspaces/12", "/workspaces/diagnostics/overview", "/workspaces/12/diagnostics", "/operations/jobs", "/operations/health", "/operations/metrics"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin"+path+"?secret=audit-query-canary", nil))
		require.Equal(t, 200, w.Code)
	}
	audit.Stop()
	repository.mu.Lock()
	defer repository.mu.Unlock()
	require.Len(t, repository.logs, 7)
	for _, entry := range repository.logs {
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "audit-response-canary")
		require.NotContains(t, string(encoded), "audit-query-canary")
		require.Empty(t, entry.RequestBody)
		require.EqualValues(t, 7, *entry.ActorUserID)
		require.Equal(t, 200, entry.StatusCode)
		require.NotEmpty(t, entry.Action)
		require.GreaterOrEqual(t, entry.LatencyMs, int64(0))
	}
}

func TestEnterpriseAdminMutationAuditOmitsRejectedUnknownFieldsAndCredentialReasons(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &auditCaptureRepository{}
	audit := service.NewAuditLogService(repository, nil)
	audit.Start()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
		c.Set(string(ContextKeyUserRole), "admin")
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
	// Rejection occurs after middleware; even unparsed/unknown credential-like
	// strings and private operator reasons must never enter the HTTP audit layer.
	reject := func(c *gin.Context) { c.JSON(400, gin.H{"reason": "WORKSPACE_INVALID"}) }
	router.PATCH("/api/v1/admin/workspaces/:id/status", reject)
	router.POST("/api/v1/admin/workspaces/:id/webhooks/:webhook_id/deliveries/:delivery_id/retry", reject)
	for _, tc := range []struct{ method, path, body string }{
		{"PATCH", "/workspaces/12/status", `{"reason":"Bearer audit-reason-canary","unexpected":"audit-unknown-canary","proof":"audit-proof-canary"}`},
		{"POST", "/workspaces/12/webhooks/23/deliveries/34/retry", `{"reason":"password=audit-reason-canary","payload":"audit-unknown-canary","proof":"audit-proof-canary"}`},
	} {
		request := httptest.NewRequest(tc.method, "/api/v1/admin"+tc.path, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		require.Equal(t, 400, w.Code)
	}
	audit.Stop()
	repository.mu.Lock()
	defer repository.mu.Unlock()
	require.Len(t, repository.logs, 2)
	for _, entry := range repository.logs {
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "canary")
		require.Equal(t, 400, entry.StatusCode)
		require.EqualValues(t, 7, *entry.ActorUserID)
		require.NotEmpty(t, entry.Path)
		require.NotEmpty(t, entry.Action)
	}
}
