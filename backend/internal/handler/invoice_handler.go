package handler

import (
	"bytes"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type InvoiceHandler struct{ service *service.InvoiceService }

func NewInvoiceHandler(s *service.InvoiceService) *InvoiceHandler { return &InvoiceHandler{service: s} }
func (h *InvoiceHandler) SetOnUpdateCallback(callback func()) {
	h.service.SetOnUpdateCallback(callback)
}

func invoiceID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的申请编号")
		return 0, false
	}
	return id, true
}

func (h *InvoiceHandler) Eligibility(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	value, err := h.service.Eligibility(c.Request.Context(), subject.UserID)
	if !response.ErrorFrom(c, err) {
		response.Success(c, value)
	}
}

func (h *InvoiceHandler) Create(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var input service.InvoiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "申请格式无效，金额必须为整数分")
		return
	}
	value, err := h.service.Create(c.Request.Context(), subject.UserID, input)
	if !response.ErrorFrom(c, err) {
		response.Created(c, value)
	}
}

func (h *InvoiceHandler) List(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	page, size := response.ParsePagination(c)
	items, total, err := h.service.List(c.Request.Context(), service.InvoiceFilter{UserID: subject.UserID, Page: page, PageSize: size})
	if !response.ErrorFrom(c, err) {
		response.Paginated(c, items, total, page, size)
	}
}

func (h *InvoiceHandler) Get(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	value, err := h.service.Get(c.Request.Context(), id, subject.UserID)
	if !response.ErrorFrom(c, err) {
		response.Success(c, value)
	}
}

func (h *InvoiceHandler) Cancel(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	value, err := h.service.Cancel(c.Request.Context(), id, subject.UserID)
	if !response.ErrorFrom(c, err) {
		response.Success(c, value)
	}
}

func (h *InvoiceHandler) AdminList(c *gin.Context) {
	page, size := response.ParsePagination(c)
	items, total, err := h.service.List(c.Request.Context(), service.InvoiceFilter{Page: page, PageSize: size, Status: c.Query("status"), Search: c.Query("search")})
	if !response.ErrorFrom(c, err) {
		response.Paginated(c, items, total, page, size)
	}
}

func (h *InvoiceHandler) AdminGet(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	value, err := h.service.Get(c.Request.Context(), id, 0)
	if !response.ErrorFrom(c, err) {
		response.Success(c, value)
	}
}

func (h *InvoiceHandler) AdminUpdate(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	var input service.InvoiceUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "审核信息格式无效")
		return
	}
	value, err := h.service.Update(c.Request.Context(), id, input)
	if !response.ErrorFrom(c, err) {
		response.Success(c, value)
	}
}

func (h *InvoiceHandler) GetConfig(c *gin.Context) {
	enabled, err := h.service.Enabled(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"enabled": enabled})
	}
}

func (h *InvoiceHandler) UpdateConfig(c *gin.Context) {
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Enabled == nil {
		response.BadRequest(c, "请设置是否启用发票申请")
		return
	}
	if !response.ErrorFrom(c, h.service.SetEnabled(c.Request.Context(), *input.Enabled)) {
		response.Success(c, gin.H{"enabled": *input.Enabled})
	}
}

func (h *InvoiceHandler) Export(c *gin.Context) {
	items, err := h.service.Export(c.Request.Context(), service.InvoiceFilter{Status: c.Query("status"), Search: c.Query("search")})
	if response.ErrorFrom(c, err) {
		return
	}
	var output bytes.Buffer
	if response.ErrorFrom(c, service.WriteInvoiceCSV(&output, items)) {
		return
	}
	c.Header("Content-Disposition", `attachment; filename="invoice-applications.csv"`)
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "text/csv; charset=utf-8", output.Bytes())
}
