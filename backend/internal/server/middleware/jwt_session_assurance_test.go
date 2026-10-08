//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestJWTCompleteSessionAuthenticationContext(t *testing.T) {
	for _, enterprise := range []bool{false, true} {
		t.Run(map[bool]string{false: "password", true: "saml"}[enterprise], func(t *testing.T) {
			user := &service.User{ID: 41, Email: "member@example.com", Role: service.RoleUser, Status: service.StatusActive, TotpEnabled: true}
			router, authSvc := newJWTTestEnv(map[int64]*service.User{41: user})
			original := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
			method := "password"
			ctx := context.Background()
			deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
			if enterprise {
				method = "saml"
				ctx = service.WithAuthenticationAssurance(ctx, service.WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, ProviderRevision: 3, AuthMethod: method, AuthenticatedAt: original.Add(-time.Hour), ValidUntil: deadline})
			}
			ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: method, AuthenticatedAt: original, MFASatisfied: true})
			token, err := authSvc.GenerateToken(ctx, user)
			require.NoError(t, err)
			called := false
			router.GET("/session-context", func(c *gin.Context) {
				called = true
				auth, ok := service.SessionAuthenticationFromContext(c.Request.Context())
				require.True(t, ok)
				require.Equal(t, original, auth.AuthenticatedAt)
				require.Equal(t, method, auth.AuthMethod)
				require.True(t, auth.MFASatisfied)
				require.True(t, auth.MFAEnrolled)
				if enterprise {
					assurance, ok := service.AuthenticationAssuranceFromContext(c.Request.Context())
					require.True(t, ok)
					require.Equal(t, original.Add(-time.Hour), assurance.AuthenticatedAt)
					require.Equal(t, deadline, assurance.ValidUntil)
				}
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/session-context", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.True(t, called)
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestOptionalJWTEnrollmentHintDoesNotGrantMFA(t *testing.T) {
	user := &service.User{ID: 41, Email: "member@example.com", Status: service.StatusActive, TotpEnabled: true}
	_, authSvc := newJWTTestEnv(map[int64]*service.User{41: user})
	users := service.NewUserService(&stubJWTUserRepo{users: map[int64]*service.User{41: user}}, nil, nil, nil)
	original := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	token, err := authSvc.GenerateToken(service.WithSessionAuthentication(context.Background(), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: original}), user)
	require.NoError(t, err)
	router := gin.New()
	router.Use(gin.HandlerFunc(NewOptionalJWTAuthMiddleware(authSvc, users, nil, nil)))
	router.GET("/optional", func(c *gin.Context) {
		auth, ok := service.SessionAuthenticationFromContext(c.Request.Context())
		require.True(t, ok)
		require.True(t, auth.MFAEnrolled)
		require.False(t, auth.MFASatisfied)
		require.Equal(t, original, auth.AuthenticatedAt)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/optional", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestAdminJWTCompleteSessionAuthenticationContext(t *testing.T) {
	user := &service.User{ID: 41, Email: "admin@example.com", Status: service.StatusActive, Role: service.RoleAdmin, TotpEnabled: true}
	_, authSvc := newJWTTestEnv(map[int64]*service.User{41: user})
	users := service.NewUserService(&stubJWTUserRepo{users: map[int64]*service.User{41: user}}, nil, nil, nil)
	original := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	token, err := authSvc.GenerateToken(service.WithSessionAuthentication(context.Background(), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: original, MFASatisfied: true}), user)
	require.NoError(t, err)
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(authSvc, users, nil, nil)))
	router.GET("/admin", func(c *gin.Context) {
		auth, ok := service.SessionAuthenticationFromContext(c.Request.Context())
		require.True(t, ok)
		require.True(t, auth.MFAEnrolled)
		require.True(t, auth.MFASatisfied)
		require.Equal(t, original, auth.AuthenticatedAt)
		subject, _ := GetAuthSubjectFromContext(c)
		require.Equal(t, service.PrincipalHuman, subject.PrincipalType)
		require.Equal(t, "password", subject.AuthMethod)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
