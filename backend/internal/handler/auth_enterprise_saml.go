package handler

import (
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const enterpriseSAMLBrowserCookieName = "modurelay_enterprise_saml"
const enterpriseSAMLFormMaxBytes = 768 << 10

// SAML returns via a cross-site POST. Keep this binding cookie separate from
// the Lax completion cookies and restrict it to the ACS endpoint.
func (h *AuthHandler) enterpriseSAMLBrowserCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: enterpriseSAMLBrowserCookieName, Value: value, Path: "/api/v1/auth/sso/saml/acs", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode})
}

func (h *AuthHandler) EnterpriseSAMLMetadata(c *gin.Context) {
	if !h.enterpriseReady(c) {
		return
	}
	publicID := c.Param("public_id")
	if len(publicID) != 43 {
		response.ErrorFrom(c, service.ErrWorkspaceNotFound)
		return
	}
	metadata, err := h.enterpriseIdentity.SAMLMetadata(c.Request.Context(), publicID, h.enterpriseSSORedirectURI(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Data(http.StatusOK, "application/samlmetadata+xml; charset=utf-8", metadata)
}

func (h *AuthHandler) EnterpriseSAMLACS(c *gin.Context) {
	if !h.enterpriseReady(c) {
		return
	}
	fail := func(code string) {
		h.enterpriseSAMLBrowserCookie(c, "", -1)
		h.enterpriseCookie(c, enterpriseSSOCookieName, "", -1)
		h.enterpriseCookie(c, enterpriseSSOCompletionCookie, "", -1)
		c.Redirect(http.StatusSeeOther, "/auth/sso/callback?error="+url.QueryEscape(code))
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" || c.Request.ContentLength > enterpriseSAMLFormMaxBytes || !strings.HasPrefix(h.enterpriseSSORedirectURI(c), "https://") {
		fail("SAML_RESPONSE_INVALID")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, enterpriseSAMLFormMaxBytes)
	if c.Request.ParseForm() != nil {
		fail("SAML_RESPONSE_INVALID")
		return
	}
	form := c.Request.PostForm
	if len(form["SAMLResponse"]) != 1 || len(form["RelayState"]) != 1 || len(form["SAMLResponse"][0]) > 700000 || len(form["RelayState"][0]) != 43 {
		fail("SAML_RESPONSE_INVALID")
		return
	}
	cookie, err := c.Request.Cookie(enterpriseSAMLBrowserCookieName)
	if err != nil || len(cookie.Value) != 43 {
		fail("SAML_RESPONSE_INVALID")
		return
	}
	result, err := h.enterpriseIdentity.CompleteSAML(c.Request.Context(), form.Get("RelayState"), cookie.Value, form.Get("SAMLResponse"), h.enterpriseSSORedirectURI(c))
	if err != nil {
		if err == service.ErrOIDCAccountLinkRequired {
			fail("OIDC_ACCOUNT_LINK_REQUIRED")
		} else {
			fail("SAML_AUTHENTICATION_FAILED")
		}
		return
	}
	if err = h.ensureBackendModeAllowsUser(c.Request.Context(), result.User); err != nil {
		fail("SAML_AUTHENTICATION_FAILED")
		return
	}
	completion, err := h.enterpriseIdentity.CreateLoginCompletion(c.Request.Context(), result, cookie.Value)
	if err != nil {
		fail("SAML_AUTHENTICATION_FAILED")
		return
	}
	h.enterpriseSAMLBrowserCookie(c, "", -1)
	h.enterpriseCookie(c, enterpriseSSOCompletionCookie, completion, 120)
	h.enterpriseCookie(c, enterpriseSSOCookieName, cookie.Value, 120)
	c.Redirect(http.StatusSeeOther, "/auth/sso/callback")
}
