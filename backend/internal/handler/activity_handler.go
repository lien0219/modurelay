package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ActivityHandler struct {
	service *service.ActivityService
}

func NewActivityHandler(activityService *service.ActivityService) *ActivityHandler {
	return &ActivityHandler{service: activityService}
}

// SetOnUpdateCallback forwards activity-setting updates to the server's
// shared settings cache invalidation callback.
func (h *ActivityHandler) SetOnUpdateCallback(callback func()) {
	if h == nil || h.service == nil {
		return
	}
	h.service.SetOnUpdateCallback(callback)
}

type activityActionRequest struct {
	RequestID string `json:"request_id" binding:"required"`
}

type activityCenterSettingsRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

type activityUpdateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sort_order"`
}

func (h *ActivityHandler) List(c *gin.Context) {
	userID, ok := currentSubject(c)
	if !ok {
		return
	}
	items, err := h.service.ListUserActivities(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *ActivityHandler) Detail(c *gin.Context) {
	userID, ok := currentSubject(c)
	if !ok {
		return
	}
	activity, err := h.service.GetUserActivity(c.Request.Context(), userID, strings.TrimSpace(c.Param("slug")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, activity)
}

func (h *ActivityHandler) Draw(c *gin.Context) {
	userID, ok := currentSubject(c)
	if !ok {
		return
	}
	var req activityActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid lottery request")
		return
	}
	result, err := h.service.DrawLottery(c.Request.Context(), userID, c.Param("slug"), strings.TrimSpace(req.RequestID))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ActivityHandler) Claim(c *gin.Context) {
	userID, ok := currentSubject(c)
	if !ok {
		return
	}
	var req activityActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid benefit request")
		return
	}
	result, err := h.service.ClaimBenefit(c.Request.Context(), userID, c.Param("slug"), strings.TrimSpace(req.RequestID))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ActivityHandler) Rewards(c *gin.Context) {
	userID, ok := currentSubject(c)
	if !ok {
		return
	}
	items, err := h.service.ListRewards(c.Request.Context(), userID, 20)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *ActivityHandler) AdminView(c *gin.Context) {
	view, err := h.service.AdminView(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *ActivityHandler) UpdateAdminSettings(c *gin.Context) {
	var req activityCenterSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "Invalid activity center settings")
		return
	}
	if err := h.service.SetEnabled(c.Request.Context(), *req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.AdminView(c)
}

func (h *ActivityHandler) UpdateActivity(c *gin.Context) {
	var req activityUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid activity settings")
		return
	}
	activity, err := h.service.UpdateActivity(c.Request.Context(), c.Param("slug"), service.UpdateActivityInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Enabled:     req.Enabled,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, activity)
}

func (h *ActivityHandler) PublishLotteryConfig(c *gin.Context) {
	adminID, ok := currentSubject(c)
	if !ok {
		return
	}
	var req service.LotteryConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid lottery configuration")
		return
	}
	req.CreatedBy = adminID
	activity, err := h.service.PublishLotteryConfig(c.Request.Context(), c.Param("slug"), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, activity)
}

func (h *ActivityHandler) PublishBenefitConfig(c *gin.Context) {
	adminID, ok := currentSubject(c)
	if !ok {
		return
	}
	var req service.BenefitConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid benefit configuration")
		return
	}
	req.CreatedBy = adminID
	activity, err := h.service.PublishBenefitConfig(c.Request.Context(), c.Param("slug"), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, activity)
}
