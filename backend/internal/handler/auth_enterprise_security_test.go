package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type enterpriseCompletionUsers struct {
	service.UserRepository
	user *service.User
}

func (r *enterpriseCompletionUsers) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}

func (r *enterpriseCompletionUsers) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type enterpriseCompletionRepo struct {
	service.EnterpriseIdentityRepository
	consumes int
}

func (r *enterpriseCompletionRepo) PreviewLoginCompletion(context.Context, []byte, []byte, time.Time) (*service.OIDCLoginResult, int64, error) {
	return &service.OIDCLoginResult{Workspace: 7, ProviderID: 9, ReturnTo: "/workspaces/7/overview", Assurance: service.WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, ProviderRevision: 2, AuthenticatedAt: time.Now(), AuthMethod: "oidc"}}, 42, nil
}
func (r *enterpriseCompletionRepo) ConsumeLoginCompletion(ctx context.Context, a, b []byte, n time.Time) (*service.OIDCLoginResult, int64, error) {
	r.consumes++
	return r.PreviewLoginCompletion(ctx, a, b, n)
}

func TestEnterpriseSSOExchangePreservesLocalMFABeforeConsumingCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &enterpriseCompletionUsers{user: &service.User{ID: 42, Status: service.StatusActive, TotpEnabled: true}}
	repo := &enterpriseCompletionRepo{}
	h := &AuthHandler{cfg: &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}, EnterpriseSSO: config.EnterpriseSSOConfig{RedirectURL: "https://app.example/api/v1/auth/sso/callback"}}, authService: &service.AuthService{}, enterpriseIdentity: service.NewEnterpriseIdentityService(repo, nil, users, nil)}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/sso/exchange", strings.NewReader(`{}`))
	c.Request.Header.Set("Origin", "https://app.example")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.AddCookie(&http.Cookie{Name: enterpriseSSOCompletionCookie, Value: "opaque-completion"})
	c.Request.AddCookie(&http.Cookie{Name: enterpriseSSOCookieName, Value: "browser-binding"})
	h.EnterpriseSSOExchange(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"requires_2fa":true`)
	require.NotContains(t, recorder.Body.String(), "access_token")
	require.Zero(t, repo.consumes, "MFA challenge must leave the single-use completion pending")
	require.Empty(t, recorder.Header().Values("Set-Cookie"), "challenge preserves HttpOnly credentials")
}

type enterpriseHandlerWorkspaceRepo struct {
	service.WorkspaceRepository
	role string
}

func (r *enterpriseHandlerWorkspaceRepo) GetAccess(_ context.Context, actor, workspace, project int64) (*service.WorkspaceAccess, error) {
	return &service.WorkspaceAccess{Workspace: &service.Workspace{ID: workspace, Type: service.WorkspaceTypeOrganization, Status: "active"}, Member: &service.WorkspaceMember{UserID: actor, Role: r.role, Status: "active"}}, nil
}

type enterpriseHandlerIdentityRepo struct {
	service.EnterpriseIdentityRepository
	recoveries int
}

func (r *enterpriseHandlerIdentityRepo) GetPolicy(_ context.Context, workspace, actor int64) (*service.WorkspaceIdentityPolicy, error) {
	return &service.WorkspaceIdentityPolicy{WorkspaceID: workspace, RequireSSO: true}, nil
}
func (r *enterpriseHandlerIdentityRepo) BreakGlass(_ context.Context, workspace, actor int64, reason string) (*service.WorkspaceIdentityPolicy, error) {
	r.recoveries++
	return &service.WorkspaceIdentityPolicy{WorkspaceID: workspace, RequireSSO: false}, nil
}

type enterpriseHandlerPolicyRepo struct{ domain.PolicyRepository }

func TestEnterpriseSSOEnforcesEveryRegisteredTenantRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	workspaces := service.NewWorkspaceService(&enterpriseHandlerWorkspaceRepo{role: "owner"})
	identity := service.NewEnterpriseIdentityService(&enterpriseHandlerIdentityRepo{}, nil, nil, nil)
	handler := NewWorkspaceHandler(workspaces, nil)
	handler.SetServiceAccountService(nil)
	handler.SetPolicyHandler(NewPolicyHandler(workspaces, nil, enterpriseHandlerPolicyRepo{}, nil))
	handler.SetEnterpriseIdentityService(identity)
	handler.SetEnterpriseSCIMService(service.NewEnterpriseSCIMService(&scimHTTPRepo{}))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, PrincipalType: service.PrincipalHuman})
		c.Next()
	})
	handler.RegisterTenantRoutes(router.Group("/api/v1"))
	checked := 0
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/workspaces/:id") {
			continue
		}
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			path := route.Path
			for _, name := range []string{"id", "project_id", "member_id", "team_id", "grant_id", "invitation_id", "key_id", "webhook_id", "delivery_id", "domain_id", "provider_id", "service_account_id", "credential_id", "connector_id", "token_id", "group_id"} {
				path = strings.ReplaceAll(path, ":"+name, "7")
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(route.Method, path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Contains(t, recorder.Body.String(), "SSO_REQUIRED")
		})
		checked++
	}
	require.Greater(t, checked, 70, "covers Workspace, project, identity, Service Account and Policy route groups")
}

func TestEnterpriseSSORecoveryRequiresPasswordMFAConfirmationAndOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hash, err := bcrypt.GenerateFromPassword([]byte("recovery-password"), bcrypt.MinCost)
	require.NoError(t, err)
	cases := []struct {
		name, role, password, principal string
		mfa, confirmed                  bool
		want                            int
	}{
		{"owner", "owner", "recovery-password", service.PrincipalHuman, false, true, 200},
		{"wrong_password", "owner", "incorrect", service.PrincipalHuman, false, true, 401},
		{"admin", "admin", "recovery-password", service.PrincipalHuman, false, true, 403},
		{"mfa_required", "owner", "recovery-password", service.PrincipalHuman, true, true, 403},
		{"confirmation_required", "owner", "recovery-password", service.PrincipalHuman, false, false, 400},
		{"machine", "owner", "recovery-password", service.PrincipalServiceAccount, false, true, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := &enterpriseCompletionUsers{user: &service.User{ID: 42, Email: "owner@example.com", Status: service.StatusActive, PasswordHash: string(hash), TotpEnabled: tc.mfa}}
			repo := &enterpriseHandlerIdentityRepo{}
			access := service.NewWorkspaceAccessService(&enterpriseHandlerWorkspaceRepo{role: tc.role})
			handler := &AuthHandler{userService: service.NewUserService(users, nil, nil, nil), enterpriseIdentity: service.NewEnterpriseIdentityService(repo, access, users, nil)}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			confirmation := "false"
			if tc.confirmed {
				confirmation = "true"
			}
			body := `{"workspace_id":7,"password":"` + tc.password + `","reason":"provider unavailable for recovery","confirmed":` + confirmation + `}`
			c.Request = httptest.NewRequest("POST", "/api/v1/auth/sso/recover", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, PrincipalType: tc.principal})
			handler.EnterpriseSSORecover(c)
			require.Equal(t, tc.want, recorder.Code)
			if tc.want == 200 {
				require.Equal(t, 1, repo.recoveries)
			} else {
				require.Zero(t, repo.recoveries)
			}
		})
	}
}
