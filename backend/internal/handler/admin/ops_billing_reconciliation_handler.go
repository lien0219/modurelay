package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (h *OpsHandler) GetBillingReconciliation(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	result, err := h.opsService.GetBillingReconciliation(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// RecheckBillingOperation queues evidence reconciliation, never a deduction.
func (h *OpsHandler) RecheckBillingOperation(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid operation ID")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	var payload struct {
		Note string `json:"note" binding:"required,max=1000"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.Note) == "" {
		response.BadRequest(c, "A reconciliation note is required (maximum 1000 characters)")
		return
	}
	if err := h.opsService.RetryBillingOperation(c.Request.Context(), id, subject.UserID, strings.TrimSpace(payload.Note)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(c, http.StatusConflict, "Operation is no longer awaiting manual reconciliation")
		} else {
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, gin.H{"state": "queued"})
}

// AcknowledgeBillingRecordingGap retains the missed observation count and audit
// trail. It only acknowledges a reviewed gap; it does not repair accounting.
func (h *OpsHandler) AcknowledgeBillingRecordingGap(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	instanceID := strings.TrimSpace(c.Param("instanceId"))
	if instanceID == "" || len(instanceID) > 100 {
		response.BadRequest(c, "Invalid instance ID")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	var payload struct {
		Note string `json:"note" binding:"required,max=1000"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.Note) == "" {
		response.BadRequest(c, "A review note is required (maximum 1000 characters)")
		return
	}
	if err := h.opsService.AcknowledgeBillingRecordingGap(c.Request.Context(), instanceID, subject.UserID, strings.TrimSpace(payload.Note)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(c, http.StatusConflict, "Recording gap is no longer awaiting acknowledgment")
		} else {
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, gin.H{"state": "acknowledged"})
}
