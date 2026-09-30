package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const loginProtectionUnavailableAuditInterval = time.Minute

var nextLoginProtectionUnavailableAudit atomic.Int64

func (h *AuthHandler) SetLoginAbuseProtector(protector *service.LoginAbuseProtector) {
	if h != nil {
		h.loginAbuse = protector
	}
}

// LoginRequestRateLimit is the outer, account-agnostic login throttle.
// It runs before CAPTCHA to absorb scripted floods, while account/password
// counters are only touched after CAPTCHA succeeds.
func (h *AuthHandler) LoginRequestRateLimit(c *gin.Context) {
	if h == nil || h.loginAbuse == nil {
		respondLoginProtectionUnavailable(c)
		c.Abort()
		return
	}
	decision, err := h.loginAbuse.CheckRequestRate(c.Request.Context(), middleware2.SecurityClientIP(c))
	if err != nil {
		respondLoginProtectionUnavailable(c)
		return
	}
	if decision.Blocked {
		if decision.Audit {
			middleware2.SetAuditAction(c, "security.login.throttled")
			middleware2.SetAuditExtra(c, map[string]any{
				"limit_scope":         decision.Scope,
				"retry_after_seconds": retryAfterSeconds(decision.RetryAfter),
				"error_code":          "LOGIN_RATE_LIMITED",
			})
		} else {
			// Keep one representative event per Redis request window.
			middleware2.SkipAudit(c)
		}
		respondLoginRateLimited(c, decision.RetryAfter)
		return
	}
	c.Next()
}

func (h *AuthHandler) preflightLoginSecurity(c *gin.Context, email string) bool {
	if h == nil || h.loginAbuse == nil {
		// Production routing always injects the protector. Keeping direct
		// handler invocations backward compatible avoids breaking isolated
		// unit tests and recovery tooling that do not construct the full router.
		return true
	}
	decision, err := h.loginAbuse.CheckCredentialsAllowed(
		c.Request.Context(),
		email,
		middleware2.SecurityClientIP(c),
	)
	if err != nil {
		respondLoginProtectionUnavailable(c)
		return false
	}
	if !decision.Blocked {
		return true
	}
	// The threshold-crossing password failure is already retained as a
	// security event. Suppress repeated requests during the cooldown to avoid
	// turning an attack into an append-only audit-log write DoS.
	middleware2.SkipAudit(c)
	respondLoginRateLimited(c, decision.RetryAfter)
	return false
}

func (h *AuthHandler) recordLoginFailure(c *gin.Context, email string, loginErr error) bool {
	if h == nil || h.loginAbuse == nil || !service.IsPasswordMismatch(loginErr) {
		return false
	}
	outcome, err := h.loginAbuse.RecordPasswordFailure(
		c.Request.Context(),
		email,
		middleware2.SecurityClientIP(c),
	)
	if err != nil {
		respondLoginProtectionUnavailable(c)
		return true
	}
	if outcome.AccountBlocked || outcome.AccountIPBlocked {
		middleware2.SetAuditAction(c, "security.login.bruteforce_detected")
	} else {
		middleware2.SetAuditAction(c, "security.login.password_rejected")
	}
	middleware2.SetAuditExtra(c, map[string]any{
		"account_ip_failures": outcome.AccountIPCount,
		"account_failures":    outcome.AccountCount,
		"account_ip_blocked":  outcome.AccountIPBlocked,
		"account_blocked":     outcome.AccountBlocked,
	})
	return false
}

func (h *AuthHandler) recordLoginSuccess(c *gin.Context, user *service.User, email string) {
	if user != nil {
		middleware2.SetAuditActor(c, user.ID, user.Email)
	}
	if h == nil || h.loginAbuse == nil {
		return
	}
	if err := h.loginAbuse.RecordSuccess(c.Request.Context(), email, middleware2.SecurityClientIP(c)); err != nil {
		slog.Warn("failed to clear login abuse source counter", "error", err)
	}
}

func (h *AuthHandler) enforceAdminPasswordMFA(c *gin.Context, user *service.User) bool {
	if h == nil || user == nil || !user.IsAdmin() || h.loginAbuse == nil {
		return true
	}
	settings, err := h.loginAbuse.Settings(c.Request.Context())
	if err != nil {
		respondLoginProtectionUnavailable(c)
		return false
	}
	if !settings.AdminMFARequired {
		return true
	}
	// Password sign-in for administrators must enter the existing TOTP flow.
	// Administrators that prefer Passkey can use the dedicated Passkey login
	// path, which is already phishing-resistant and does not traverse here.
	if h.settingSvc != nil && h.settingSvc.IsTotpEnabled(c.Request.Context()) && user.TotpEnabled && h.totpService != nil {
		return true
	}

	middleware2.SetAuditAction(c, "security.admin_mfa_required")
	middleware2.SetAuditActor(c, user.ID, user.Email)
	middleware2.SetAuditExtra(c, map[string]any{"error_code": "ADMIN_MFA_REQUIRED"})
	response.ErrorFrom(c, service.ErrAdminMFARequired)
	return false
}

func respondLoginProtectionUnavailable(c *gin.Context) {
	now := time.Now().UnixNano()
	for {
		next := nextLoginProtectionUnavailableAudit.Load()
		if next > now {
			middleware2.SkipAudit(c)
			break
		}
		if nextLoginProtectionUnavailableAudit.CompareAndSwap(next, now+int64(loginProtectionUnavailableAuditInterval)) {
			middleware2.SetAuditAction(c, "security.login.protection_unavailable")
			middleware2.SetAuditExtra(c, map[string]any{"error_code": "LOGIN_PROTECTION_UNAVAILABLE"})
			break
		}
	}
	c.Abort()
	response.Error(c, http.StatusServiceUnavailable, "Login protection is temporarily unavailable")
}

func respondLoginRateLimited(c *gin.Context, retryAfter time.Duration) {
	seconds := retryAfterSeconds(retryAfter)
	if seconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
	}
	c.Abort()
	response.ErrorWithDetails(
		c,
		http.StatusTooManyRequests,
		"rate limit exceeded: too many login attempts, please try again later",
		"LOGIN_RATE_LIMITED",
		map[string]string{"retry_after_seconds": strconv.FormatInt(seconds, 10)},
	)
}

func retryAfterSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	seconds := int64(d / time.Second)
	if d%time.Second != 0 {
		seconds++
	}
	return seconds
}
