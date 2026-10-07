package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSCIMHumanMutationsRequireRecentHumanAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, recent := range []bool{false, true} {
		h := NewWorkspaceHandler(nil, nil)
		repo := &scimHTTPRepo{}
		h.SetEnterpriseSCIMService(service.NewEnterpriseSCIMService(repo))
		if recent {
			h.SetIdentityRecentAuthentication(func(c *gin.Context) bool { c.JSON(403, gin.H{"reason": "RECENT_AUTH_REQUIRED"}); return false })
		}
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, PrincipalType: service.PrincipalHuman})
		})
		h.RegisterTenantRoutes(r.Group("/api/v1"))
		for _, route := range []struct{ method, path string }{{"POST", "/scim-connectors"}, {"PATCH", "/scim-connectors/2"}, {"POST", "/scim-connectors/2/disable"}, {"POST", "/scim-connectors/2/tokens"}, {"DELETE", "/scim-connectors/2/tokens/3"}, {"PUT", "/scim-connectors/2/groups/" + strings.Repeat("g", 43) + "/team"}} {
			w := httptest.NewRecorder()
			request := httptest.NewRequest(route.method, "/api/v1/workspaces/1"+route.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, request)
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), service.SCIMErrorSchema)
			require.Empty(t, repo.outcomes, "human control rejection is not an external provisioning attempt")
		}
	}
}
func TestSCIMControlErrorsKeepPlatformEnvelopeAndMachinePrincipalsAreRejected(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	workspaceSCIMError(c, service.NewSCIMError(412, "", "resource version changed"))
	require.Equal(t, 412, w.Code)
	require.Contains(t, w.Body.String(), "SCIM_REVISION_CONFLICT")
	require.NotContains(t, w.Body.String(), service.SCIMErrorSchema)
	h := NewWorkspaceHandler(nil, nil)
	h.SetEnterpriseSCIMService(service.NewEnterpriseSCIMService(&scimHTTPRepo{}))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, PrincipalType: service.PrincipalServiceAccount})
	})
	h.RegisterTenantRoutes(r.Group("/api/v1"))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/workspaces/1/scim-connectors", nil))
	require.Equal(t, 401, w.Code)
}
