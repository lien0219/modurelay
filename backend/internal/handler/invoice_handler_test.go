package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type invoiceHandlerRepo struct {
	service.InvoiceRepository
	userID       int64
	input        service.InvoiceInput
	document     *service.InvoiceDocument
	exportFilter service.InvoiceFilter
}

func (r *invoiceHandlerRepo) Create(_ context.Context, userID int64, input service.InvoiceInput) (*service.Invoice, error) {
	r.userID = userID
	r.input = input
	return &service.Invoice{ID: 1, UserID: userID, InvoiceInput: input}, nil
}
func (r *invoiceHandlerRepo) Get(_ context.Context, _ int64, userID int64) (*service.Invoice, error) {
	r.userID = userID
	return nil, service.ErrInvoiceNotFound
}
func (r *invoiceHandlerRepo) Update(_ context.Context, _ int64, userID int64, _ service.InvoiceUpdate) (*service.Invoice, error) {
	r.userID = userID
	return nil, service.ErrInvoiceNotFound
}

type invoiceHandlerSettings struct{ service.SettingRepository }

func (*invoiceHandlerSettings) GetValue(context.Context, string) (string, error) { return "true", nil }

func TestInvoiceHandlerUsesAuthenticatedOwnerAndRejectsFractionalCents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &invoiceHandlerRepo{}
	h := NewInvoiceHandler(service.NewInvoiceService(repo, &invoiceHandlerSettings{}), nil, nil)
	for _, tc := range []struct {
		name, body    string
		authenticated bool
		status        int
	}{
		{"unauthenticated", `{}`, false, http.StatusUnauthorized},
		{"forged owner ignored", `{"amount_cents":30000,"invoice_type":"vat_normal","title_type":"personal","title":"测试","email":"finance@example.test","user_id":999}`, true, http.StatusCreated},
		{"fractional cents", `{"amount_cents":30000.5}`, true, http.StatusBadRequest},
		{"below minimum", `{"amount_cents":29999,"invoice_type":"vat_normal","title_type":"personal","title":"测试","email":"finance@example.test"}`, true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/invoices", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			if tc.authenticated {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			}
			h.Create(c)
			require.Equal(t, tc.status, recorder.Code)
			if tc.status == http.StatusCreated {
				require.Equal(t, int64(42), repo.userID)
				require.Equal(t, int64(30000), repo.input.AmountCents)
			}
		})
	}
}

func TestInvoiceHandlerScopesDetailsAndCancellationToOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &invoiceHandlerRepo{}
	h := NewInvoiceHandler(service.NewInvoiceService(repo, &invoiceHandlerSettings{}), nil, nil)
	for _, action := range []gin.HandlerFunc{h.Get, h.Cancel} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/invoices/9?user_id=999", nil)
		c.Params = gin.Params{{Key: "id", Value: "9"}}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		action(c)
		require.Equal(t, int64(42), repo.userID)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	}
}

func (r *invoiceHandlerRepo) GetDocument(_ context.Context, _ int64, userID int64) (*service.InvoiceDocument, error) {
	r.userID = userID
	if r.document == nil {
		return nil, service.ErrInvoiceDocumentNotFound
	}
	return r.document, nil
}
func (r *invoiceHandlerRepo) SaveDocument(_ context.Context, id int64, document *service.InvoiceDocument) (*service.Invoice, error) {
	r.document = document
	return &service.Invoice{ID: id, Document: &document.InvoiceDocumentMetadata}, nil
}
func (r *invoiceHandlerRepo) Export(_ context.Context, filter service.InvoiceFilter) ([]*service.Invoice, error) {
	r.exportFilter = filter
	return []*service.Invoice{{ID: 1, UserName: "昵称", InvoiceInput: service.InvoiceInput{AmountCents: 30000}}}, nil
}

func TestInvoiceDocumentHandlerScopesOwnerAndControlsDisposition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &invoiceHandlerRepo{document: &service.InvoiceDocument{InvoiceDocumentMetadata: service.InvoiceDocumentMetadata{Filename: "电子发票.pdf"}, Content: []byte("%PDF-1.4\ncontent\n%%EOF")}}
	h := NewInvoiceHandler(service.NewInvoiceService(repo, nil), nil, nil)
	for _, tc := range []struct {
		query          string
		authenticated  bool
		expectedStatus int
		disposition    string
	}{
		{"", false, http.StatusUnauthorized, ""},
		{"?user_id=999", true, http.StatusOK, "inline"},
		{"?download=1", true, http.StatusOK, "attachment"},
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/invoices/1/document"+tc.query, nil)
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		if tc.authenticated {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		h.Document(c)
		require.Equal(t, tc.expectedStatus, recorder.Code)
		if tc.expectedStatus == http.StatusOK {
			require.Equal(t, int64(42), repo.userID)
			require.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
			require.True(t, strings.HasPrefix(recorder.Header().Get("Content-Disposition"), tc.disposition+";"))
			require.Equal(t, repo.document.Content, recorder.Body.Bytes())
		}
	}
	repo.document = nil
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/invoices/1/document", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	h.AdminDocument(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Zero(t, repo.userID)
}

func TestInvoicePDFUploadHandlerRejectsSpoofedFilesAndAcceptsPDF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, content, contentType string
		status                     int
	}{
		{"valid", "%PDF-1.4\ncontent\n%%EOF", "application/pdf", http.StatusOK},
		{"spoofed", "<html>invalid invoice</html>", "application/pdf", http.StatusBadRequest},
		{"wrong mime", "%PDF-1.4\ncontent\n%%EOF", "image/jpeg", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="invoice.pdf"`}, "Content-Type": {tc.contentType}})
			require.NoError(t, err)
			_, err = part.Write([]byte(tc.content))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/invoices/1/document", &body)
			c.Request.Header.Set("Content-Type", writer.FormDataContentType())
			c.Params = gin.Params{{Key: "id", Value: "1"}}
			repo := &invoiceHandlerRepo{}
			h := NewInvoiceHandler(service.NewInvoiceService(repo, nil), nil, nil)
			h.UploadDocument(c)
			require.Equal(t, tc.status, recorder.Code)
			if tc.status == http.StatusOK {
				require.Equal(t, tc.content, string(repo.document.Content))
			} else {
				require.Nil(t, repo.document)
			}
		})
	}
}

func TestInvoiceSelectedExportValidatesColumnsAndReturnsOnlyRequestedColumns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"ids":[1],"columns":["user_name","amount"]}`, http.StatusOK},
		{`{"ids":[1],"columns":["password_hash"]}`, http.StatusBadRequest},
		{`{"ids":[1,1],"columns":["amount"]}`, http.StatusBadRequest},
		{`{"ids":[-1]}`, http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/invoices/export", strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		repo := &invoiceHandlerRepo{}
		h := NewInvoiceHandler(service.NewInvoiceService(repo, nil), nil, nil)
		h.ExportSelected(c)
		require.Equal(t, tc.status, recorder.Code, tc.body)
		if tc.status == http.StatusOK {
			require.Equal(t, []int64{1}, repo.exportFilter.IDs)
			rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(recorder.Body.String(), "\ufeff"))).ReadAll()
			require.NoError(t, err)
			require.Equal(t, []string{"昵称", "金额（人民币元）"}, rows[0])
			require.Equal(t, []string{"昵称", "300.00"}, rows[1])
		}
	}
}
