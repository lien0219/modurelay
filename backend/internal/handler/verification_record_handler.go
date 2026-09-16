package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type VerificationRecordHandler struct {
	service *service.VerificationRecordService
}

func NewVerificationRecordHandler(recordService *service.VerificationRecordService) *VerificationRecordHandler {
	return &VerificationRecordHandler{service: recordService}
}

func (h *VerificationRecordHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.list(c, subject.UserID, false)
}

func (h *VerificationRecordHandler) AdminList(c *gin.Context) {
	h.list(c, 0, true)
}

func (h *VerificationRecordHandler) Options(c *gin.Context) {
	h.options(c)
}

func (h *VerificationRecordHandler) AdminOptions(c *gin.Context) {
	h.options(c)
}

func (h *VerificationRecordHandler) options(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 50 {
		pageSize = 50
	}
	result, err := h.service.ListOptions(c.Request.Context(), service.VerificationRecordOptionListOptions{
		Kind: c.Query("kind"), Type: c.Query("type"), Query: c.Query("query"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidVerificationRecordFilter) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *VerificationRecordHandler) list(c *gin.Context, userID int64, admin bool) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	createdFrom, err := parseVerificationRecordTime(c.Query("created_from"), false)
	if err != nil {
		response.BadRequest(c, "invalid created_from")
		return
	}
	createdTo, err := parseVerificationRecordTime(c.Query("created_to"), true)
	if err != nil {
		response.BadRequest(c, "invalid created_to")
		return
	}
	result, err := h.service.List(c.Request.Context(), service.VerificationRecordListOptions{
		Page:        page,
		PageSize:    pageSize,
		UserID:      userID,
		Type:        c.Query("type"),
		Outcome:     c.Query("outcome"),
		ServiceCode: c.Query("platform"),
		Region:      c.Query("country"),
		Keyword:     c.Query("keyword"),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}, admin)
	if err != nil {
		if errors.Is(err, service.ErrInvalidVerificationRecordFilter) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	analytics, err := h.service.Analytics(c.Request.Context(), service.VerificationRecordListOptions{
		UserID:      userID,
		Type:        c.Query("type"),
		Outcome:     c.Query("outcome"),
		ServiceCode: c.Query("platform"),
		Region:      c.Query("country"),
		Keyword:     c.Query("keyword"),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	}, admin)
	if err != nil {
		if errors.Is(err, service.ErrInvalidVerificationRecordFilter) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	result.Analytics = analytics
	response.Success(c, result)
}

func parseVerificationRecordTime(value string, endExclusive bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	if endExclusive {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return &parsed, nil
}
