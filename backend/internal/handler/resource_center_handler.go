package handler

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ResourceCenterHandler struct {
	service *service.ResourceCenterService
}

func NewResourceCenterHandler(svc *service.ResourceCenterService) *ResourceCenterHandler {
	return &ResourceCenterHandler{service: svc}
}

// SetOnUpdateCallback forwards public resource-center setting updates to the
// server's shared settings cache invalidation callback.
func (h *ResourceCenterHandler) SetOnUpdateCallback(callback func()) {
	if h == nil || h.service == nil {
		return
	}
	h.service.SetOnUpdateCallback(callback)
}

type resourcePostRequest struct {
	CategoryID int64  `json:"category_id" binding:"required"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}
type resourceCommentRequest struct {
	ParentID *int64 `json:"parent_id"`
	Content  string `json:"content"`
}
type resourceConfigRequest struct {
	Enabled     bool     `json:"enabled"`
	ForbidURLs  bool     `json:"forbid_urls"`
	BannedWords []string `json:"banned_words"`
}
type resourceBatchDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,max=100,dive,gt=0"`
}

func currentSubject(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return 0, false
	}
	return subject.UserID, true
}
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid resource ID")
		return 0, false
	}
	return id, true
}

func resourcePageQuery(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	return page, pageSize
}

func resourceDateRangeQuery(c *gin.Context) (*time.Time, *time.Time, bool) {
	parse := func(key string) (*time.Time, error) {
		raw := c.Query(key)
		if raw == "" {
			return nil, nil
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, err
		}
		return &value, nil
	}
	start, err := parse("start_at")
	if err != nil {
		response.BadRequest(c, "Invalid start date")
		return nil, nil, false
	}
	end, err := parse("end_at")
	if err != nil || (start != nil && end != nil && !start.Before(*end)) {
		response.BadRequest(c, "Invalid end date")
		return nil, nil, false
	}
	return start, end, true
}

func (h *ResourceCenterHandler) Config(c *gin.Context) {
	cfg, err := h.service.PublicConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
func (h *ResourceCenterHandler) Categories(c *gin.Context) {
	cats, err := h.service.Categories(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cats)
}
func (h *ResourceCenterHandler) ListPosts(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	categoryID, _ := strconv.ParseInt(c.Query("category_id"), 10, 64)
	page, pageSize := resourcePageQuery(c)
	posts, err := h.service.ListPostsPage(c.Request.Context(), uid, categoryID, c.Query("q"), c.Query("sort"), false, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, posts)
}
func (h *ResourceCenterHandler) ListComments(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	page, pageSize := resourcePageQuery(c)
	comments, err := h.service.ListCommentsPage(c.Request.Context(), uid, postID, page, pageSize, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, comments)
}
func (h *ResourceCenterHandler) GetPost(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	post, err := h.service.GetPost(c.Request.Context(), uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, post)
}
func (h *ResourceCenterHandler) CreatePost(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	var req resourcePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid post payload")
		return
	}
	post, err := h.service.CreatePost(c.Request.Context(), uid, req.CategoryID, req.Title, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, post)
}
func (h *ResourceCenterHandler) CreateComment(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req resourceCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid comment payload")
		return
	}
	comment, err := h.service.CreateComment(c.Request.Context(), uid, postID, req.ParentID, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, comment)
}
func (h *ResourceCenterHandler) DeleteOwnPost(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteOwnPost(c.Request.Context(), uid, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}
func (h *ResourceCenterHandler) DeleteOwnComment(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteOwnComment(c.Request.Context(), uid, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}
func (h *ResourceCenterHandler) TogglePostLike(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	liked, err := h.service.TogglePostLike(c.Request.Context(), uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"liked": liked})
}
func (h *ResourceCenterHandler) ToggleCommentLike(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	liked, err := h.service.ToggleCommentLike(c.Request.Context(), uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"liked": liked})
}
func (h *ResourceCenterHandler) Notifications(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	items, err := h.service.Notifications(c.Request.Context(), uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *ResourceCenterHandler) MarkNotificationRead(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.MarkNotificationRead(c.Request.Context(), uid, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}
func (h *ResourceCenterHandler) MarkAllNotificationsRead(c *gin.Context) {
	uid, ok := currentSubject(c)
	if !ok {
		return
	}
	if err := h.service.MarkAllNotificationsRead(c.Request.Context(), uid); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *ResourceCenterHandler) AdminConfig(c *gin.Context) {
	cfg, err := h.service.Config(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
func (h *ResourceCenterHandler) UpdateAdminConfig(c *gin.Context) {
	var req resourceConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid resource center settings")
		return
	}
	if err := h.service.UpdateConfig(c.Request.Context(), service.ResourceCenterConfig{Enabled: req.Enabled, ForbidURLs: req.ForbidURLs, BannedWords: req.BannedWords}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.AdminConfig(c)
}
func (h *ResourceCenterHandler) AdminListPosts(c *gin.Context) {
	page, pageSize := resourcePageQuery(c)
	start, end, ok := resourceDateRangeQuery(c)
	if !ok {
		return
	}
	posts, err := h.service.ListAdminPostsPage(c.Request.Context(), c.Query("q"), c.Query("sort"), start, end, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, posts)
}
func (h *ResourceCenterHandler) AdminListPostComments(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	page, pageSize := resourcePageQuery(c)
	comments, err := h.service.ListCommentsPage(c.Request.Context(), 0, postID, page, pageSize, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, comments)
}
func (h *ResourceCenterHandler) AdminListComments(c *gin.Context) {
	page, pageSize := resourcePageQuery(c)
	start, end, ok := resourceDateRangeQuery(c)
	if !ok {
		return
	}
	comments, err := h.service.ListAdminCommentsPage(c.Request.Context(), c.Query("q"), start, end, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, comments)
}
func (h *ResourceCenterHandler) AdminGetPost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	post, err := h.service.GetPostForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, post)
}
func (h *ResourceCenterHandler) AdminDeletePost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeletePost(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}
func (h *ResourceCenterHandler) AdminDeleteComment(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteComment(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *ResourceCenterHandler) AdminBatchDeletePosts(c *gin.Context) {
	var req resourceBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Select between 1 and 100 posts")
		return
	}
	deleted, err := h.service.BatchDeletePosts(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

func (h *ResourceCenterHandler) AdminBatchDeleteComments(c *gin.Context) {
	var req resourceBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Select between 1 and 100 comments")
		return
	}
	deleted, err := h.service.BatchDeleteComments(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}

// Keep these references visible to static analysis when the handler is used by optional deployments.
