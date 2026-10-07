package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func samlHandlerFixture() *AuthHandler {
	return &AuthHandler{cfg: &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}, EnterpriseSSO: config.EnterpriseSSOConfig{RedirectURL: "https://app.example/api/v1/auth/sso/callback"}}, enterpriseIdentity: service.NewEnterpriseIdentityService(nil, nil, nil, nil)}
}

func TestSAMLBrowserCookieSupportsCrossSitePOSTAndRemainsHTTPOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/start", nil)
	h := samlHandlerFixture()
	h.enterpriseSAMLBrowserCookie(c, strings.Repeat("a", 43), 600)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, http.SameSiteNoneMode, cookies[0].SameSite)
	require.True(t, cookies[0].Secure)
	require.True(t, cookies[0].HttpOnly)
	require.Equal(t, "/api/v1/auth/sso/saml/acs", cookies[0].Path)
}

func TestSAMLACSRejectsUnsolicitedOversizedDuplicateAndWrongMediaForms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, body := range map[string]string{
		"unsolicited":    "SAMLResponse=unsigned&RelayState=" + strings.Repeat("a", 43),
		"oversized":      "SAMLResponse=" + strings.Repeat("x", 1<<20),
		"duplicate":      "SAMLResponse=one&SAMLResponse=two&RelayState=" + strings.Repeat("a", 43),
		"relay too long": "SAMLResponse=one&RelayState=" + strings.Repeat("a", 200),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/saml/acs", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			samlHandlerFixture().EnterpriseSAMLACS(c)
			c.Writer.WriteHeaderNow()
			require.Equal(t, http.StatusSeeOther, recorder.Code)
			location, err := url.Parse(recorder.Header().Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "/auth/sso/callback", location.Path)
			require.NotEmpty(t, location.Query().Get("error"))
			require.NotContains(t, recorder.Body.String(), "unsigned")
		})
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/saml/acs", strings.NewReader(`{"SAMLResponse":"x"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	samlHandlerFixture().EnterpriseSAMLACS(c)
	c.Writer.WriteHeaderNow()
	require.Equal(t, http.StatusSeeOther, recorder.Code)
}

func TestSAMLRecentAuthenticationAcceptedAndStaleRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &enterpriseCompletionUsers{user: &service.User{ID: 42, Status: service.StatusActive}}
	h := samlHandlerFixture()
	h.userService = service.NewUserService(users, nil, nil, nil)
	for _, age := range []time.Duration{time.Minute, 20 * time.Minute} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, PrincipalType: service.PrincipalHuman, AuthMethod: "saml", AuthenticatedAt: time.Now().Add(-age)})
		require.Equal(t, age < 10*time.Minute, h.RequireEnterpriseRecentAuthentication(c))
	}
}
