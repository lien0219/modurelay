package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	service *service.NotificationCenterService
}

func NewNotificationHandler(svc *service.NotificationCenterService) *NotificationHandler {
	return &NotificationHandler{service: svc}
}

func (h *NotificationHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	filter := service.NotificationListFilter{Page: page, PageSize: pageSize, Category: c.Query("category")}
	if raw := c.Query("unread"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.BadRequest(c, "Invalid unread filter")
			return
		}
		filter.Unread = &value
	}
	if id, err := optionalPositiveID(c.Query("workspace_id")); err != nil {
		response.BadRequest(c, "Invalid workspace_id")
		return
	} else if id != nil {
		filter.WorkspaceID = id
	}
	if id, err := optionalPositiveID(c.Query("project_id")); err != nil {
		response.BadRequest(c, "Invalid project_id")
		return
	} else if id != nil {
		filter.ProjectID = id
	}
	filter = filter.Normalized()
	items, total, err := h.service.List(c.Request.Context(), subject.UserID, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	workspaceID, err := optionalPositiveID(c.Query("workspace_id"))
	if err != nil {
		response.BadRequest(c, "Invalid workspace_id")
		return
	}
	projectID, err := optionalPositiveID(c.Query("project_id"))
	if err != nil {
		response.BadRequest(c, "Invalid project_id")
		return
	}
	count, err := h.service.UnreadCount(c.Request.Context(), subject.UserID, workspaceID, projectID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"unread_count": count, "count": count})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid notification ID")
		return
	}
	if err := h.service.MarkRead(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	workspaceID, err := optionalPositiveID(c.Query("workspace_id"))
	if err != nil {
		response.BadRequest(c, "Invalid workspace_id")
		return
	}
	projectID, err := optionalPositiveID(c.Query("project_id"))
	if err != nil {
		response.BadRequest(c, "Invalid project_id")
		return
	}
	count, err := h.service.MarkAllRead(c.Request.Context(), subject.UserID, workspaceID, projectID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"marked": count})
}

func optionalPositiveID(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, strconv.ErrSyntax
	}
	return &id, nil
}
