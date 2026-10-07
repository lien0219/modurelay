package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseSSOCallbackConfigurationNeverUsesHostOrSocialOIDC(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "https://attacker.example/api/v1/auth/sso/start", nil)
	h := &AuthHandler{cfg: &config.Config{OIDC: config.OIDCConnectConfig{RedirectURL: "https://social.example/callback"}}}
	require.Empty(t, h.enterpriseSSORedirectURI(c), "SSO requires separate operator configured callback")
	h.cfg.EnterpriseSSO.RedirectURL = "https://app.example/api/v1/auth/sso/callback"
	require.Equal(t, h.cfg.EnterpriseSSO.RedirectURL, h.enterpriseSSORedirectURI(c))
}

func TestEnterpriseSSOExchangeRejectsCrossOriginBeforeConsumingCookies(t *testing.T) {
	h := &AuthHandler{cfg: &config.Config{EnterpriseSSO: config.EnterpriseSSOConfig{RedirectURL: "https://app.example/api/v1/auth/sso/callback"}}}
	for _, origin := range []string{"https://evil.example", "null", "https://app.example.evil.example", ""} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/api/v1/auth/sso/exchange", nil)
		c.Request.Header.Set("Origin", origin)
		require.False(t, h.enterpriseSameOrigin(c), origin)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/sso/exchange", nil)
	c.Request.Header.Set("Origin", "https://app.example")
	require.True(t, h.enterpriseSameOrigin(c))
}
