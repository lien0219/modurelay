package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type invoiceHandlerRepo struct {
	service.InvoiceRepository
	userID int64
	input  service.InvoiceInput
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
	h := NewInvoiceHandler(service.NewInvoiceService(repo, &invoiceHandlerSettings{}))
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
	h := NewInvoiceHandler(service.NewInvoiceService(repo, &invoiceHandlerSettings{}))
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
