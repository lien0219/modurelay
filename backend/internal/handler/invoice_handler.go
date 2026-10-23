package handler

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type InvoiceHandler struct {
	service  *service.InvoiceService
	payments *service.PaymentService
	users    *service.UserService
}

func NewInvoiceHandler(s *service.InvoiceService, payments *service.PaymentService, users *service.UserService) *InvoiceHandler {
	return &InvoiceHandler{service: s, payments: payments, users: users}
}
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

type invoiceExportInput struct {
	IDs     []int64  `json:"ids"`
	Status  string   `json:"status"`
	Search  string   `json:"search"`
	Columns []string `json:"columns"`
}

func (h *InvoiceHandler) Export(c *gin.Context) {
	h.export(c, invoiceExportInput{Status: c.Query("status"), Search: c.Query("search")})
}

func (h *InvoiceHandler) ExportSelected(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	var input invoiceExportInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "导出参数格式无效")
		return
	}
	h.export(c, input)
}

func (h *InvoiceHandler) export(c *gin.Context, input invoiceExportInput) {
	columns, err := service.ValidateInvoiceExportColumns(input.Columns)
	if response.ErrorFrom(c, err) {
		return
	}
	items, err := h.service.Export(c.Request.Context(), service.InvoiceFilter{IDs: input.IDs, Status: input.Status, Search: input.Search})
	if response.ErrorFrom(c, err) {
		return
	}
	var output bytes.Buffer
	if response.ErrorFrom(c, service.WriteInvoiceCSVColumns(&output, items, columns)) {
		return
	}
	c.Header("Content-Disposition", `attachment; filename="invoice-applications.csv"`)
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", output.Bytes())
}

func (h *InvoiceHandler) UploadDocument(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	// Bound the multipart body before parsing, including a small allowance for headers.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.InvoiceDocumentMaxBytes+64*1024)
	if err := c.Request.ParseMultipartForm(service.InvoiceDocumentMaxBytes + 64*1024); err != nil {
		response.BadRequest(c, "请上传不超过 10 MiB 的 PDF 文件")
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择 PDF 电子发票")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, service.InvoiceDocumentMaxBytes+1))
	if err != nil {
		response.BadRequest(c, "无法读取 PDF 文件")
		return
	}
	inv, err := h.service.UploadDocument(c.Request.Context(), id, header.Filename, header.Header.Get("Content-Type"), content)
	if !response.ErrorFrom(c, err) {
		response.Success(c, inv)
	}
}

func (h *InvoiceHandler) Document(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	h.document(c, subject.UserID)
}

func (h *InvoiceHandler) AdminDocument(c *gin.Context) { h.document(c, 0) }

func (h *InvoiceHandler) document(c *gin.Context, userID int64) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	document, err := h.service.GetDocument(c.Request.Context(), id, userID)
	if response.ErrorFrom(c, err) {
		return
	}
	disposition := "inline"
	if c.Query("download") == "1" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": document.Filename}))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Data(http.StatusOK, "application/pdf", document.Content)
}

func (h *InvoiceHandler) AdminAccount(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	inv, err := h.service.Get(c.Request.Context(), id, 0)
	if response.ErrorFrom(c, err) {
		return
	}
	user, err := h.users.GetByID(c.Request.Context(), inv.UserID)
	if response.ErrorFrom(c, err) {
		return
	}
	page, size := response.ParsePagination(c)
	orders, total, err := h.payments.AdminListOrders(c.Request.Context(), inv.UserID, service.OrderListParams{Page: page, PageSize: size, OrderType: "balance"})
	if response.ErrorFrom(c, err) {
		return
	}
	pages := (total + size - 1) / size
	response.Success(c, gin.H{
		"user":   gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "balance": user.Balance},
		"orders": gin.H{"items": admin.SanitizePaymentOrdersForResponse(orders), "total": total, "page": page, "page_size": size, "pages": pages},
	})
}
