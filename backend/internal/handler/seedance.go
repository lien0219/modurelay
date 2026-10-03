package handler

import (
	"context"
	"mime"
	"net/http"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SeedanceTasks exposes Ark's native asynchronous video task protocol.
func (h *OpenAIGatewayHandler) SeedanceTasks(c *gin.Context) {
	if c.Request.Method == http.MethodPost && c.GetHeader("Content-Type") != "" {
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" {
			h.errorResponse(c, http.StatusUnsupportedMediaType, "invalid_request_error", "Seedance requires application/json")
			return
		}
	}
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key.Group == nil || (key.Group.Platform != service.PlatformSeedance && key.Group.Platform != service.PlatformOpenAI && key.Group.Platform != service.PlatformComposite) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "Seedance requires a Seedance, legacy OpenAI, or composite group")
		return
	}
	if c.Request.Method == http.MethodPost && key.Group.Platform == service.PlatformComposite {
		resolvedPlatform, resolved := service.ResolvedTargetPlatformFromContext(c.Request.Context())
		if !resolved || resolvedPlatform != service.PlatformSeedance {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Composite Seedance tasks require a Seedance route")
			return
		}
	}
	endpoint := service.SeedanceEndpointCreate
	setActualUpstreamEndpoint(c, EndpointSeedanceTasks)
	taskID := ""
	if c.Request.Method != http.MethodPost {
		taskID = service.SeedanceTaskKey(c.Param("task_id"))
		endpoint = service.SeedanceEndpointStatus
		if c.Request.Method == http.MethodDelete {
			endpoint = service.SeedanceEndpointDelete
		}
	}
	h.handleGrokMedia(c, endpoint, taskID)
}

// New Seedance groups use configured video pricing; legacy OpenAI Seedance
// capabilities retain token billing when Ark reports completion tokens.
func prepareSeedanceCompletionBilling(ctx context.Context, h *OpenAIGatewayHandler, key *service.APIKey, subject middleware.AuthSubject, taskID string, result *service.OpenAIForwardResult) *service.OpenAIForwardResult {
	if result == nil || result.VideoCount <= 0 {
		return nil
	}
	pending, err := h.gatewayService.LoadGrokVideoPendingBilling(ctx, taskID, subject.UserID, key.ID)
	if err != nil || pending == nil {
		return nil
	}
	model := firstNonEmptyString(pending.BillingModel, pending.Model)
	resolution := firstNonEmptyString(result.VideoResolution, pending.VideoResolution)
	newPlatform := pending.QuotaPlatform == service.PlatformSeedance
	if newPlatform && !h.gatewayService.HasVideoPricingForRequest(ctx, key, model, resolution) {
		return nil
	}
	if !newPlatform && result.Usage.OutputTokens <= 0 && !h.gatewayService.HasVideoPricingForRequest(ctx, key, model, resolution) {
		return nil
	}
	claimed, err := h.gatewayService.ClaimGrokVideoBilling(ctx, taskID, subject.UserID, key.ID)
	if err != nil || !claimed {
		return nil
	}
	merged := *result
	merged.Model = pending.Model
	merged.BillingModel = model
	merged.UpstreamModel = firstNonEmptyString(pending.UpstreamModel, result.UpstreamModel)
	merged.ForceTokenBilling = !newPlatform && result.Usage.OutputTokens > 0
	merged.VideoResolution = resolution
	if merged.VideoDurationSeconds <= 0 {
		merged.VideoDurationSeconds = pending.VideoDurationSeconds
	}
	if merged.VideoCount <= 0 {
		merged.VideoCount = 1
	}
	merged.RequestID = service.StableGrokVideoBillingRequestID(taskID)
	merged.ResponseID = taskID
	merged.Duration = service.GrokVideoE2EDuration(pending.CreatedAt, time.Now())
	return &merged
}
