package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const enterpriseSSOCompletionCookie = "modurelay_enterprise_sso_completion"

func (h *AuthHandler) enterpriseSameOrigin(c *gin.Context) bool {
	expected, err := url.Parse(h.enterpriseSSORedirectURI(c))
	if err != nil || expected.Host == "" {
		return false
	}
	origin, err := url.Parse(c.GetHeader("Origin"))
	return err == nil && origin.Host == expected.Host && origin.Scheme == expected.Scheme && origin.User == nil && origin.RawQuery == "" && origin.Fragment == "" && origin.Path == "" && c.GetHeader("Sec-Fetch-Site") != "cross-site"
}

func (h *AuthHandler) enterpriseCookie(c *gin.Context, name, value string, maxAge int) {
	secure := strings.HasPrefix(h.enterpriseSSORedirectURI(c), "https://")
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/api/v1/auth/sso", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func (h *AuthHandler) enterpriseReady(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	if h == nil || h.enterpriseIdentity == nil || h.enterpriseSSORedirectURI(c) == "" {
		response.ErrorFrom(c, infraerrors.New(http.StatusServiceUnavailable, "ENTERPRISE_SSO_NOT_CONFIGURED", "enterprise SSO callback is not configured"))
		return false
	}
	if !h.cfg.Totp.EncryptionKeyConfigured {
		response.ErrorFrom(c, infraerrors.New(http.StatusServiceUnavailable, "ENTERPRISE_SSO_ENCRYPTION_NOT_CONFIGURED", "a persistent encryption key is required for enterprise SSO"))
		return false
	}
	return true
}

func (h *AuthHandler) EnterpriseSSOExchange(c *gin.Context) {
	if !h.enterpriseReady(c) {
		return
	}
	if !h.enterpriseSameOrigin(c) {
		response.ErrorFrom(c, service.ErrOIDCStateSessionMismatch)
		return
	}
	completion, err := c.Request.Cookie(enterpriseSSOCompletionCookie)
	browser, browserErr := c.Request.Cookie(enterpriseSSOCookieName)
	if err != nil || browserErr != nil {
		response.ErrorFrom(c, service.ErrOIDCStateSessionMismatch)
		return
	}
	result, err := h.enterpriseIdentity.PreviewLoginCompletion(c.Request.Context(), completion.Value, browser.Value)
	if err != nil {
		response.ErrorFrom(c, service.ErrOIDCStateSessionMismatch)
		return
	}
	mfaSatisfied := false
	if result.User.TotpEnabled {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		var req struct {
			TOTPCode string `json:"totp_code"`
		}
		if c.Request.ContentLength != 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
		}
		if req.TOTPCode == "" {
			response.Success(c, gin.H{"requires_2fa": true})
			return
		}
		if len(req.TOTPCode) != 6 || h.totpService == nil {
			response.ErrorFrom(c, service.ErrTotpInvalidCode)
			return
		}
		if err := h.totpService.VerifyCode(c.Request.Context(), result.User.ID, req.TOTPCode); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		mfaSatisfied = true
	}
	result, err = h.enterpriseIdentity.ExchangeLoginCompletion(c.Request.Context(), completion.Value, browser.Value)
	h.enterpriseCookie(c, enterpriseSSOCompletionCookie, "", -1)
	h.enterpriseCookie(c, enterpriseSSOCookieName, "", -1)
	if err != nil {
		response.ErrorFrom(c, service.ErrOIDCStateSessionMismatch)
		return
	}
	if result.User.TotpEnabled && !mfaSatisfied {
		response.ErrorFrom(c, service.ErrTotpInvalidCode)
		return
	}
	if err := h.ensureBackendModeAllowsUser(c.Request.Context(), result.User); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	ctx := service.WithAuthenticationAssurance(c.Request.Context(), result.Assurance)
	ctx = service.WithSessionAuthentication(ctx, service.SessionAuthentication{AuthMethod: "oidc", AuthenticatedAt: result.Assurance.AuthenticatedAt, MFASatisfied: mfaSatisfied})
	pair, err := h.authService.GenerateTokenPair(ctx, result.User, "")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.authService.RecordSuccessfulLogin(ctx, result.User.ID)
	response.Success(c, gin.H{"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken, "expires_in": pair.ExpiresIn, "token_type": "Bearer", "user": dto.UserFromService(result.User), "workspace_id": result.Workspace, "provider_id": result.ProviderID, "return_to": result.ReturnTo})
}

func (h *AuthHandler) EnterpriseSSODiscover(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.enterpriseIdentity == nil {
		response.Success(c, gin.H{"enterprise_sso_available": false, "providers": []any{}})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Email) > 320 {
		response.BadRequest(c, "Invalid request")
		return
	}
	providers, err := h.enterpriseIdentity.DiscoverSSO(c.Request.Context(), req.Email)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if providers == nil {
		providers = []service.SSODiscoveryProvider{}
	}
	response.Success(c, gin.H{"enterprise_sso_available": len(providers) > 0, "providers": providers})
}

// This gate is independent of the optional global step-up setting. Original
// authentication time is preserved on refresh, and TOTP must be satisfied
// whenever the account supports it.
func (h *AuthHandler) RequireEnterpriseRecentAuthentication(c *gin.Context) bool {
	if h == nil || h.cfg == nil || !h.cfg.Totp.EncryptionKeyConfigured {
		response.ErrorFrom(c, infraerrors.New(http.StatusServiceUnavailable, "ENTERPRISE_SSO_ENCRYPTION_NOT_CONFIGURED", "a persistent encryption key is required for enterprise SSO"))
		return false
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || subject.PrincipalType != "" && subject.PrincipalType != service.PrincipalHuman || h.userService == nil {
		response.ErrorFrom(c, service.ErrWorkspaceForbidden)
		return false
	}
	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil || user == nil || !user.IsActive() {
		response.ErrorFrom(c, service.ErrUserNotActive)
		return false
	}
	auth := service.SessionAuthentication{AuthMethod: subject.AuthMethod, AuthenticatedAt: subject.AuthenticatedAt, MFASatisfied: subject.MFASatisfied}
	recent := auth.Recent(time.Now(), 10*time.Minute)
	if user.TotpEnabled {
		if recent && auth.MFASatisfied {
			return true
		}
		if h.totpService != nil && c.GetString(middleware.ContextKeySessionID) != "" {
			granted, err := h.totpService.HasStepUpGrant(c.Request.Context(), user.ID, middleware.StepUpSessionKey(c, user.ID))
			if err == nil && granted {
				return true
			}
		}
		response.ErrorFrom(c, infraerrors.Forbidden("STEP_UP_REQUIRED", "recent two-factor authentication is required"))
		return false
	}
	if !recent || (auth.AuthMethod != "password" && auth.AuthMethod != "oidc" && auth.AuthMethod != "passkey") {
		response.ErrorFrom(c, infraerrors.Forbidden("RECENT_AUTH_REQUIRED", "sign in again before changing enterprise identity settings"))
		return false
	}
	return true
}

func (h *AuthHandler) enterpriseVerifyPasswordProof(c *gin.Context, password, totpCode string) bool {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || subject.PrincipalType != "" && subject.PrincipalType != service.PrincipalHuman || h.userService == nil || len(password) > 72 {
		response.ErrorFrom(c, service.ErrWorkspaceForbidden)
		return false
	}
	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil || user == nil || !user.IsActive() {
		response.ErrorFrom(c, service.ErrUserNotActive)
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		response.ErrorFrom(c, service.ErrInvalidCredentials)
		return false
	}
	if user.TotpEnabled {
		if h.totpService == nil || len(totpCode) != 6 {
			response.ErrorFrom(c, infraerrors.Forbidden("STEP_UP_REQUIRED", "two-factor authentication is required"))
			return false
		}
		if err := h.totpService.VerifyCode(c.Request.Context(), user.ID, totpCode); err != nil {
			response.ErrorFrom(c, err)
			return false
		}
	}
	return true
}

func (h *AuthHandler) EnterpriseSSORecover(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h == nil || h.enterpriseIdentity == nil || h.userService == nil {
		response.ErrorFrom(c, service.ErrWorkspaceForbidden)
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 || subject.PrincipalType != "" && subject.PrincipalType != service.PrincipalHuman {
		response.ErrorFrom(c, service.ErrWorkspaceForbidden)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var req struct {
		WorkspaceID int64  `json:"workspace_id"`
		Password    string `json:"password"`
		TOTPCode    string `json:"totp_code"`
		Reason      string `json:"reason"`
		Confirm     bool   `json:"confirmed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm || req.WorkspaceID <= 0 || len(req.Password) == 0 || len(req.Password) > 72 || len(strings.TrimSpace(req.Reason)) < 10 || len(req.Reason) > 500 {
		response.ErrorFrom(c, service.ErrEnterpriseIdentityInvalid)
		return
	}
	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil || user == nil || !user.IsActive() {
		response.ErrorFrom(c, service.ErrUserNotActive)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		response.ErrorFrom(c, service.ErrInvalidCredentials)
		return
	}
	if user.TotpEnabled {
		if h.totpService == nil || len(req.TOTPCode) != 6 {
			response.ErrorFrom(c, infraerrors.Forbidden("STEP_UP_REQUIRED", "two-factor authentication is required for recovery"))
			return
		}
		if err := h.totpService.VerifyCode(c.Request.Context(), user.ID, req.TOTPCode); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	policy, err := h.enterpriseIdentity.BreakGlass(c.Request.Context(), user.ID, req.WorkspaceID, time.Now(), true, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}
