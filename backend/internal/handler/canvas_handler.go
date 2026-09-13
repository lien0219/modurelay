package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CanvasHandler struct{ service *service.CanvasService }

func NewCanvasHandler(s *service.CanvasService) *CanvasHandler { return &CanvasHandler{service: s} }

func (h *CanvasHandler) subject(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return 0, false
	}
	return subject.UserID, true
}
func canvasID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid canvas ID")
		return 0, false
	}
	return id, true
}
func revisionHeader(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.GetHeader("If-Match"))
	raw = strings.Trim(raw, "\"")
	if raw == "" {
		response.ErrorWithDetails(c, http.StatusPreconditionRequired, "If-Match revision is required", "CANVAS_REVISION_REQUIRED", nil)
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		response.BadRequest(c, "Invalid If-Match revision")
		return 0, false
	}
	return value, true
}
func (h *CanvasHandler) List(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	out, err := h.service.ListProjects(c, uid)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
func (h *CanvasHandler) Create(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if c.Request.Body != nil {
		_ = json.NewDecoder(c.Request.Body).Decode(&in)
	}
	out, err := h.service.CreateProject(c, uid, in.Title)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, out)
}
func (h *CanvasHandler) Get(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	out, err := h.service.GetProject(c, uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
func (h *CanvasHandler) Save(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	rev, ok := revisionHeader(c)
	if !ok {
		return
	}
	var in struct {
		Title    string          `json:"title"`
		Document json.RawMessage `json:"document"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid canvas document")
		return
	}
	out, err := h.service.SaveProject(c, uid, id, rev, in.Title, in.Document)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("ETag", strconv.Itoa(out.Revision))
	response.Success(c, out)
}
func (h *CanvasHandler) Delete(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteProject(c, uid, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *CanvasHandler) Revisions(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	out, err := h.service.ListRevisions(c, uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}
func (h *CanvasHandler) Checkpoint(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	out, err := h.service.CreateCheckpoint(c, uid, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, out)
}
func (h *CanvasHandler) Restore(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	rid, err := strconv.ParseInt(c.Param("revision_id"), 10, 64)
	if err != nil || rid <= 0 {
		response.BadRequest(c, "Invalid revision ID")
		return
	}
	rev, ok := revisionHeader(c)
	if !ok {
		return
	}
	out, e := h.service.RestoreRevision(c, uid, id, rid, int64(rev))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	c.Header("ETag", strconv.Itoa(out.Revision))
	response.Success(c, out)
}
func (h *CanvasHandler) Upload(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	defer file.Close()
	out, e := h.service.UploadAsset(c, uid, id, c.GetHeader("Idempotency-Key"), service.CanvasAssetUpload{FileName: header.Filename, ContentType: header.Header.Get("Content-Type"), Body: file, Size: header.Size})
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Created(c, out)
}
func (h *CanvasHandler) Promote(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	id, ok := canvasID(c)
	if !ok {
		return
	}
	var in struct {
		TaskID     string `json:"task_id"`
		ImageIndex int    `json:"image_index"`
	}
	if e := c.ShouldBindJSON(&in); e != nil || in.TaskID == "" {
		response.BadRequest(c, "task_id is required")
		return
	}
	out, e := h.service.PromoteTaskAsset(c, uid, id, in.TaskID, in.ImageIndex, c.GetHeader("Idempotency-Key"))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Created(c, out)
}
func (h *CanvasHandler) AssetURL(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	aid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || aid <= 0 {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	url, e := h.service.AssetAccessURL(c, uid, aid)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, gin.H{"url": url})
}
func (h *CanvasHandler) DeleteAsset(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	aid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || aid <= 0 {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	if e := h.service.DeleteAsset(c, uid, aid); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CanvasHandler) FetchModels(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	var in service.CanvasModelListRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid model provider configuration")
		return
	}
	out, err := h.service.FetchProviderModels(c, uid, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *CanvasHandler) ProxyProvider(c *gin.Context) {
	uid, ok := h.subject(c)
	if !ok {
		return
	}
	result, err := h.service.ProxyProviderRequest(
		c,
		uid,
		c.Request,
		c.GetHeader("X-Canvas-Upstream-URL"),
		c.GetHeader("X-Canvas-Provider-Authorization"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = result.Response.Body.Close() }()

	copyCanvasProviderResponseHeaders(c.Writer.Header(), result.Response.Header)
	c.Status(result.Response.StatusCode)
	streamCanvasProviderResponse(c, result.Response.Body, result.MaxBytes)
}

func copyCanvasProviderResponseHeaders(destination, source http.Header) {
	for _, key := range []string{
		"Cache-Control",
		"Content-Disposition",
		"Content-Length",
		"Content-Type",
		"ETag",
		"Last-Modified",
		"OpenAI-Processing-Ms",
		"OpenAI-Request-ID",
		"Retry-After",
		"X-Request-ID",
	} {
		for _, value := range source.Values(key) {
			destination.Add(key, value)
		}
	}
}

func streamCanvasProviderResponse(c *gin.Context, body io.Reader, maxBytes int64) {
	if body == nil || maxBytes <= 0 {
		return
	}
	limited := &io.LimitedReader{R: body, N: maxBytes + 1}
	buffer := make([]byte, 32*1024)
	written := int64(0)
	flushChunks := strings.HasPrefix(strings.ToLower(c.Writer.Header().Get("Content-Type")), "text/event-stream")
	for {
		count, readErr := limited.Read(buffer)
		if count > 0 {
			remaining := maxBytes - written
			if remaining <= 0 {
				return
			}
			if int64(count) > remaining {
				count = int(remaining)
			}
			if _, writeErr := c.Writer.Write(buffer[:count]); writeErr != nil {
				return
			}
			written += int64(count)
			if flushChunks {
				c.Writer.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}
