package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/url"
)

const enterpriseSSOCookieName = "modurelay_enterprise_sso"

type enterpriseSSOStartRequest struct {
	WorkspaceID int64  `json:"workspace_id" form:"workspace_id"`
	ProviderID  int64  `json:"provider_id" form:"provider_id"`
	ReturnTo    string `json:"return_to" form:"return_to"`
	Password    string `json:"password"`
	TOTPCode    string `json:"totp_code"`
}

func (h *AuthHandler) enterpriseSSORedirectURI(c *gin.Context) string {
	if h == nil || h.cfg == nil {
		return ""
	}
	return strings.TrimSpace(h.cfg.EnterpriseSSO.RedirectURL)
}

func (h *AuthHandler) EnterpriseSSOStart(c *gin.Context) {
	h.enterpriseSSOStart(c, false)
}

func (h *AuthHandler) EnterpriseSSOLinkStart(c *gin.Context) { h.enterpriseSSOStart(c, true) }

func (h *AuthHandler) enterpriseSSOStart(c *gin.Context, link bool) {
	if !h.enterpriseReady(c) {
		return
	}
	var req enterpriseSSOStartRequest
	if c.Request.Method == http.MethodPost {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request")
			return
		}
	} else {
		req.WorkspaceID, _ = strconv.ParseInt(c.Query("workspace_id"), 10, 64)
		req.ProviderID, _ = strconv.ParseInt(c.Query("provider_id"), 10, 64)
		req.ReturnTo = c.Query("return_to")
	}
	if req.WorkspaceID <= 0 || req.ProviderID <= 0 {
		response.ErrorFrom(c, service.ErrEnterpriseIdentityInvalid)
		return
	}
	if req.ReturnTo == "" {
		req.ReturnTo = "/workspaces/" + strconv.FormatInt(req.WorkspaceID, 10) + "/overview"
	}
	var result *service.EnterpriseSSOStartResult
	var err error
	if link {
		if req.Password != "" {
			if !h.enterpriseVerifyPasswordProof(c, req.Password, req.TOTPCode) {
				return
			}
		} else if !h.RequireEnterpriseRecentAuthentication(c) {
			return
		}
		subject, _ := middleware.GetAuthSubjectFromContext(c)
		result, err = h.enterpriseIdentity.StartEnterpriseSSO(c.Request.Context(), req.WorkspaceID, req.ProviderID, req.ReturnTo, h.enterpriseSSORedirectURI(c), &subject.UserID)
	} else {
		result, err = h.enterpriseIdentity.StartEnterpriseSSO(c.Request.Context(), req.WorkspaceID, req.ProviderID, req.ReturnTo, h.enterpriseSSORedirectURI(c), nil)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.enterpriseCookie(c, enterpriseSSOCompletionCookie, "", -1)
	if result.Protocol == "saml" {
		h.enterpriseSAMLBrowserCookie(c, result.BrowserCookie, int(time.Until(result.ExpiresAt).Seconds()))
	} else {
		h.enterpriseCookie(c, enterpriseSSOCookieName, result.BrowserCookie, int(time.Until(result.ExpiresAt).Seconds()))
	}
	if c.Request.Method == http.MethodGet {
		c.Redirect(http.StatusFound, result.AuthorizationURL)
		return
	}
	response.Success(c, gin.H{"authorization_url": result.AuthorizationURL, "redirect_url": result.AuthorizationURL, "method": "GET", "protocol": result.Protocol, "expires_at": result.ExpiresAt})
}

func (h *AuthHandler) EnterpriseSSOCallback(c *gin.Context) {
	if !h.enterpriseReady(c) {
		return
	}
	fail := func(code string) {
		h.enterpriseCookie(c, enterpriseSSOCookieName, "", -1)
		h.enterpriseCookie(c, enterpriseSSOCompletionCookie, "", -1)
		c.Redirect(http.StatusSeeOther, "/auth/sso/callback?error="+url.QueryEscape(code))
	}
	if c.Query("error") != "" {
		fail("OIDC_PROVIDER_ERROR")
		return
	}
	state, code := c.Query("state"), c.Query("code")
	if state == "" || code == "" || len(state) > 256 || len(code) > 8192 {
		fail("OIDC_STATE_INVALID")
		return
	}
	cookie, err := c.Request.Cookie(enterpriseSSOCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		fail("OIDC_STATE_INVALID")
		return
	}
	result, err := h.enterpriseIdentity.CompleteOIDC(c.Request.Context(), state, cookie.Value, code, h.enterpriseSSORedirectURI(c))
	if err != nil {
		if err == service.ErrOIDCAccountLinkRequired {
			fail("OIDC_ACCOUNT_LINK_REQUIRED")
		} else {
			fail("OIDC_AUTHENTICATION_FAILED")
		}
		return
	}
	if err := h.ensureBackendModeAllowsUser(c.Request.Context(), result.User); err != nil {
		fail("OIDC_AUTHENTICATION_FAILED")
		return
	}
	completion, err := h.enterpriseIdentity.CreateLoginCompletion(c.Request.Context(), result, cookie.Value)
	if err != nil {
		fail("OIDC_AUTHENTICATION_FAILED")
		return
	}
	h.enterpriseCookie(c, enterpriseSSOCompletionCookie, completion, 120)
	h.enterpriseCookie(c, enterpriseSSOCookieName, cookie.Value, 120)
	c.Redirect(http.StatusSeeOther, "/auth/sso/callback")
}
