package admin

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func validateAdminMFAUser(user *service.User) error {
	if user == nil || !user.IsAdmin() || !user.TotpEnabled {
		return fmt.Errorf("configure TOTP for the current administrator before requiring admin MFA")
	}
	return nil
}

func rejectAdminMFAConfiguration(c *gin.Context, reason, message string) {
	response.ErrorWithDetails(c, http.StatusBadRequest, message, reason, nil)
}

// GetLoginSecuritySettings returns distributed password-login protection settings.
func (h *SettingHandler) GetLoginSecuritySettings(c *gin.Context) {
	settings, err := h.settingService.GetLoginSecuritySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateLoginSecuritySettings updates distributed password-login protection settings.
// Enabling mandatory admin MFA is fail-safe: the acting admin must already have
// TOTP configured, so saving this switch cannot lock the administrator out.
func (h *SettingHandler) UpdateLoginSecuritySettings(c *gin.Context) {
	var settings service.LoginSecuritySettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if settings.AdminMFARequired {
		if h.settingService == nil || !h.settingService.IsTotpEnabled(c.Request.Context()) {
			rejectAdminMFAConfiguration(c, "ADMIN_MFA_TOTP_FEATURE_REQUIRED", "Enable system TOTP before enabling mandatory administrator MFA")
			return
		}
		if h.userService == nil {
			rejectAdminMFAConfiguration(c, "ADMIN_MFA_TOTP_REQUIRED", "Configure TOTP for the current administrator before enabling mandatory administrator MFA")
			return
		}
		subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			response.Unauthorized(c, "Administrator session is required")
			return
		}
		user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if err := validateAdminMFAUser(user); err != nil {
			rejectAdminMFAConfiguration(c, "ADMIN_MFA_TOTP_REQUIRED", "Configure TOTP for the current administrator before enabling mandatory administrator MFA")
			return
		}
	}

	if err := h.settingService.SetLoginSecuritySettings(c.Request.Context(), &settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	updated, err := h.settingService.GetLoginSecuritySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}
