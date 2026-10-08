package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type securityHandlerPolicyRepo struct {
	service.EnterpriseIdentityRepository
	policy      service.WorkspaceSecurityPolicy
	unavailable bool
}

func (r *securityHandlerPolicyRepo) GetPolicy(_ context.Context, w, a int64) (*service.WorkspaceSecurityPolicy, error) {
	if r.unavailable {
		return nil, service.ErrSecurityPolicyUnavailable
	}
	result := r.policy
	result.WorkspaceID = w
	return &result, nil
}

func TestWorkspaceSecurityEveryTenantRouteUsesCurrentSessionProof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	age := 900
	for _, tc := range []struct {
		name, reason string
		policy       service.WorkspaceSecurityPolicy
		auth         service.SessionAuthentication
		unavailable  bool
	}{
		{name: "enrolled_without_verification", reason: "MFA_REQUIRED", policy: service.WorkspaceSecurityPolicy{RequireMFA: true}, auth: service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now(), MFAEnrolled: true}},
		{name: "unenrolled", reason: "MFA_ENROLLMENT_REQUIRED", policy: service.WorkspaceSecurityPolicy{RequireMFA: true}, auth: service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now()}},
		{name: "expired_original_authentication", reason: "WORKSPACE_REAUTH_REQUIRED", policy: service.WorkspaceSecurityPolicy{SessionMaxAgeSeconds: &age}, auth: service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().Add(-time.Hour), MFASatisfied: true}},
		{name: "unavailable_fail_closed", reason: "SECURITY_POLICY_UNAVAILABLE", unavailable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspaces := service.NewWorkspaceService(&enterpriseHandlerWorkspaceRepo{role: "owner"})
			identity := service.NewEnterpriseIdentityService(&securityHandlerPolicyRepo{policy: tc.policy, unavailable: tc.unavailable}, nil, nil, nil)
			h := NewWorkspaceHandler(workspaces, nil)
			h.SetServiceAccountService(nil)
			h.SetPolicyHandler(NewPolicyHandler(workspaces, nil, enterpriseHandlerPolicyRepo{}, nil))
			h.SetEnterpriseIdentityService(identity)
			h.SetEnterpriseSCIMService(service.NewEnterpriseSCIMService(&scimHTTPRepo{}))
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, PrincipalType: service.PrincipalHuman, AuthMethod: tc.auth.AuthMethod, AuthenticatedAt: tc.auth.AuthenticatedAt, MFASatisfied: tc.auth.MFASatisfied})
				c.Request = c.Request.WithContext(service.WithSessionAuthentication(c.Request.Context(), tc.auth))
				c.Next()
			})
			h.RegisterTenantRoutes(router.Group("/api/v1"))
			checked := 0
			for _, route := range router.Routes() {
				if !strings.HasPrefix(route.Path, "/api/v1/workspaces/:id") {
					continue
				}
				path := route.Path
				for _, name := range []string{"id", "project_id", "member_id", "team_id", "grant_id", "invitation_id", "key_id", "webhook_id", "delivery_id", "domain_id", "provider_id", "service_account_id", "credential_id", "connector_id", "token_id", "group_id"} {
					path = strings.ReplaceAll(path, ":"+name, "7")
				}
				response := httptest.NewRecorder()
				request := httptest.NewRequest(route.Method, path, strings.NewReader(`{}`))
				request.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(response, request)
				want := http.StatusForbidden
				if tc.unavailable {
					want = http.StatusServiceUnavailable
				}
				require.Equal(t, want, response.Code, route.Method+" "+route.Path+" "+response.Body.String())
				require.Contains(t, response.Body.String(), tc.reason)
				checked++
			}
			require.Greater(t, checked, 70)
		})
	}
}
