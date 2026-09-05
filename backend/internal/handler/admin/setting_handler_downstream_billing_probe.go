package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type updateDownstreamBillingProbeSettingsRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// GetDownstreamBillingProbeSettings returns whether downstream API keys may
// query this deployment's effective billing multiplier.
func (h *SettingHandler) GetDownstreamBillingProbeSettings(c *gin.Context) {
	if h.settingService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Setting service unavailable")
		return
	}
	settings, err := h.settingService.GetDownstreamBillingProbeSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateDownstreamBillingProbeSettings updates the downstream disclosure gate.
func (h *SettingHandler) UpdateDownstreamBillingProbeSettings(c *gin.Context) {
	if h.settingService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Setting service unavailable")
		return
	}
	var req updateDownstreamBillingProbeSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	settings := &service.DownstreamBillingProbeSettings{Enabled: *req.Enabled}
	if err := h.settingService.SetDownstreamBillingProbeSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
