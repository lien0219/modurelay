package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SMSHandler exposes both the user-safe SMS API and administrator operations.
// Admin routes are protected by the existing admin middleware at registration.
type SMSHandler struct{ svc *service.SMSService }

func NewSMSHandler(svc *service.SMSService) *SMSHandler { return &SMSHandler{svc: svc} }

func (h *SMSHandler) Enabled(ctx context.Context) bool {
	return h != nil && h.svc != nil && h.svc.Enabled(ctx)
}

type smsQuoteRequest struct {
	ServiceCode string `form:"service" json:"service_code"`
	CountryCode string `form:"country" json:"country_code"`
	ProductType string `form:"product_type" json:"product_type"`
}

func (h *SMSHandler) Quotes(c *gin.Context) {
	var req smsQuoteRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "invalid quote request")
		return
	}
	req.ServiceCode = strings.ToLower(strings.TrimSpace(req.ServiceCode))
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	if req.ProductType == "" {
		req.ProductType = "temporary"
	}
	quotes, err := h.svc.Quote(c.Request.Context(), service.SMSQuoteRequest{ServiceCode: req.ServiceCode, CountryCode: req.CountryCode, ProductType: req.ProductType})
	if err != nil {
		if err == service.ErrSMSFeatureDisabled {
			response.ErrorWithDetails(c, http.StatusNotFound, "SMS Verification is unavailable", "FEATURE_DISABLED", nil)
		} else {
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, quotes)
}

func (h *SMSHandler) Services(c *gin.Context) {
	items, err := h.svc.ListServices(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *SMSHandler) Countries(c *gin.Context) {
	items, err := h.svc.ListCountries(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

type smsPurchaseRequest struct {
	ChannelCode   string   `json:"channel_code"`
	ServiceCode   string   `json:"service_code"`
	CountryCode   string   `json:"country_code"`
	ProductType   string   `json:"product_type"`
	DurationValue int      `json:"duration_value"`
	DurationUnit  string   `json:"duration_unit"`
	ExpectedPrice *float64 `json:"expected_price"`
}

func (h *SMSHandler) Purchase(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req smsPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid purchase request")
		return
	}
	if req.ProductType == "" {
		req.ProductType = "temporary"
	}
	order, err := h.svc.Purchase(c.Request.Context(), subject.UserID, service.SMSPurchaseRequest{ChannelCode: strings.TrimSpace(req.ChannelCode), ServiceCode: strings.ToLower(strings.TrimSpace(req.ServiceCode)), CountryCode: strings.ToUpper(strings.TrimSpace(req.CountryCode)), ProductType: req.ProductType, DurationValue: req.DurationValue, DurationUnit: req.DurationUnit}, c.GetHeader("Idempotency-Key"), req.ExpectedPrice)
	if err != nil {
		switch err {
		case service.ErrSMSFeatureDisabled:
			response.ErrorWithDetails(c, http.StatusNotFound, "SMS Verification is unavailable", "FEATURE_DISABLED", nil)
		case service.ErrSMSInsufficientBalance:
			response.ErrorWithDetails(c, http.StatusPaymentRequired, "Insufficient balance", "INSUFFICIENT_BALANCE", nil)
		case service.ErrSMSPriceChanged:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote changed; please confirm again", "PRICE_CHANGED", nil)
		case service.ErrSMSProviderUnknown:
			response.ErrorWithDetails(c, http.StatusAccepted, "The channel purchase is being reconciled", "ORDER_RECONCILING", nil)
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, order)
}
func (h *SMSHandler) Orders(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if c.Query("page") != "" || c.Query("page_size") != "" || c.Query("keyword") != "" || c.Query("status") != "" {
		page, pageSize := response.ParsePagination(c)
		orders, err := h.svc.ListUserOrdersPage(c.Request.Context(), subject.UserID, page, pageSize, c.Query("keyword"), c.Query("status"))
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, orders)
		return
	}
	orders, err := h.svc.ListOrders(c.Request.Context(), subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, orders)
}
func (h *SMSHandler) Order(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	_ = h.svc.SyncOrderStatus(c.Request.Context(), subject.UserID, c.Param("id"))
	order, err := h.svc.GetOrderByPublicID(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, order)
}

func (h *SMSHandler) Webhook(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	secret := strings.TrimSpace(os.Getenv("SMS_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_WEBHOOK_SECRET"))
	supplied := strings.TrimSpace(c.GetHeader("X-SMS-Webhook-Secret"))
	if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(supplied)) != 1 {
		response.ErrorWithDetails(c, http.StatusUnauthorized, "Webhook signature is invalid", "WEBHOOK_UNAUTHORIZED", nil)
		return
	}
	var payload struct {
		OrderID  string   `json:"provider_order_id"`
		Status   string   `json:"status"`
		Phone    string   `json:"phone_number"`
		Messages []string `json:"messages"`
		Message  string   `json:"message"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		response.BadRequest(c, "invalid webhook payload")
		return
	}
	messages := append([]string(nil), payload.Messages...)
	if payload.Message != "" {
		messages = append(messages, payload.Message)
	}
	if err := h.svc.ProcessWebhook(c.Request.Context(), provider, service.SMSStatusResult{Status: payload.Status, PhoneNumber: payload.Phone, Messages: messages}, payload.OrderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"accepted": true})
}
func (h *SMSHandler) Cancel(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.CancelOrder(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrSMSProviderUnknown {
			response.ErrorWithDetails(c, http.StatusAccepted, "The cancellation is being reconciled", "ORDER_RECONCILING", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Cancellation could not be completed", "CANCEL_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "cancelled"})
}
func (h *SMSHandler) Refund(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.RequestRefund(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrSMSRefundPending {
			response.ErrorWithDetails(c, http.StatusAccepted, "The channel refund is being processed", "REFUND_PENDING", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, err.Error(), "REFUND_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "approved"})
}
func (h *SMSHandler) RefundStatus(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	order, err := h.svc.GetOrderByPublicID(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"status": order.RefundStatus, "reason": order.RefundReason})
}
func (h *SMSHandler) ExtendRental(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		DurationValue int    `json:"duration_value"`
		DurationUnit  string `json:"duration_unit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid rental extension request")
		return
	}
	order, err := h.svc.ExtendRental(c.Request.Context(), subject.UserID, c.Param("id"), req.DurationValue, req.DurationUnit, c.GetHeader("Idempotency-Key"))
	if err != nil {
		switch err {
		case service.ErrSMSFeatureDisabled:
			response.ErrorWithDetails(c, http.StatusNotFound, "SMS Verification is unavailable", "FEATURE_DISABLED", nil)
		case service.ErrSMSProviderUnknown:
			response.ErrorWithDetails(c, http.StatusAccepted, "The rental extension is being reconciled", "ORDER_RECONCILING", nil)
		default:
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Rental extension is unavailable for this channel", "RENTAL_EXTENSION_UNSUPPORTED", nil)
		}
		return
	}
	response.Success(c, order)
}

func (h *SMSHandler) AdminProviders(c *gin.Context) {
	items, err := h.svc.ListProviders(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *SMSHandler) AdminProviderUpdate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider id")
		return
	}
	var req struct {
		Enabled       bool   `json:"enabled"`
		BaseURL       string `json:"base_url"`
		CredentialRef string `json:"credential_ref"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid provider request")
		return
	}
	if err := h.svc.UpdateProvider(c.Request.Context(), id, req.Enabled, req.BaseURL, req.CredentialRef); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}
func (h *SMSHandler) AdminProviderTest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider id")
		return
	}
	result, err := h.svc.AdminTestProvider(c.Request.Context(), id)
	if err != nil {
		if err == service.ErrSMSProviderCredentialMissing {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Provider credential is not configured", "PROVIDER_CREDENTIAL_MISSING", nil)
			return
		}
		if err == service.ErrSMSProviderTestUnsupported {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "This provider does not support a safe connection test", "TEST_UNSUPPORTED", nil)
			return
		}
		if err == service.ErrSMSProviderTestCooldown {
			response.ErrorWithDetails(c, http.StatusTooManyRequests, "Provider test is cooling down", "TEST_COOLDOWN", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusBadGateway, "Provider health check failed", "PROVIDER_HEALTH_CHECK_FAILED", nil)
		return
	}
	response.Success(c, result)
}
func (h *SMSHandler) AdminChannels(c *gin.Context) {
	items, err := h.svc.ListChannelsAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *SMSHandler) AdminChannelUpdate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid channel id")
		return
	}
	var req struct {
		Enabled    bool   `json:"enabled"`
		Visible    bool   `json:"visible"`
		Healthy    bool   `json:"healthy"`
		ProviderID *int64 `json:"provider_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid channel request")
		return
	}
	if err := h.svc.UpdateChannel(c.Request.Context(), id, req.Enabled, req.Visible, req.Healthy, req.ProviderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}
func (h *SMSHandler) AdminStats(c *gin.Context) {
	stats, err := h.svc.AdminStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}
func (h *SMSHandler) AdminOrders(c *gin.Context) {
	orders, err := h.svc.ListOrders(c.Request.Context(), 0, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, orders)
}
func (h *SMSHandler) AdminToggle(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid feature flag request")
		return
	}
	if err := h.svc.SetEnabled(c.Request.Context(), req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": req.Enabled})
}
