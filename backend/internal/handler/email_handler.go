package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// EmailHandler exposes the user-safe temporary inbox API and admin controls.
type EmailHandler struct {
	svc *service.EmailVerificationService
}

func NewEmailHandler(svc *service.EmailVerificationService) *EmailHandler {
	return &EmailHandler{svc: svc}
}
func (h *EmailHandler) Enabled(ctx context.Context) bool {
	return h != nil && h.svc != nil && h.svc.Enabled(ctx)
}

type emailQuoteRequest struct {
	ServiceCode string `form:"service" json:"service_code"`
	AddressType string `form:"address_type" json:"address_type"`
}

func noStore(c *gin.Context) { c.Header("Cache-Control", "private, no-store") }
func (h *EmailHandler) Services(c *gin.Context) {
	noStore(c)
	items, e := h.svc.ListServices(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, items)
}
func (h *EmailHandler) Quotes(c *gin.Context) {
	noStore(c)
	var req emailQuoteRequest
	if e := c.ShouldBindQuery(&req); e != nil {
		response.BadRequest(c, "invalid email quote request")
		return
	}
	items, e := h.svc.Quote(c.Request.Context(), strings.TrimSpace(req.ServiceCode), strings.TrimSpace(req.AddressType))
	if e != nil {
		if e == service.ErrEmailFeatureDisabled {
			response.ErrorWithDetails(c, http.StatusNotFound, "Email service is unavailable", "FEATURE_DISABLED", nil)
		} else {
			response.ErrorFrom(c, e)
		}
		return
	}
	response.Success(c, items)
}

type emailPurchaseRequest struct {
	ChannelCode   string   `json:"channel_code"`
	ServiceCode   string   `json:"service_code"`
	AddressType   string   `json:"address_type"`
	ExpectedPrice *float64 `json:"expected_price"`
	QuoteID       string   `json:"quote_id"`
}

func (h *EmailHandler) Purchase(c *gin.Context) {
	noStore(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req emailPurchaseRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, "invalid email purchase request")
		return
	}
	order, e := h.svc.Purchase(c.Request.Context(), subject.UserID, service.EmailPurchaseRequest{ChannelCode: strings.TrimSpace(req.ChannelCode), ServiceCode: strings.ToLower(strings.TrimSpace(req.ServiceCode)), AddressType: strings.ToLower(strings.TrimSpace(req.AddressType)), ExpectedPrice: req.ExpectedPrice, QuoteID: strings.TrimSpace(req.QuoteID)}, c.GetHeader("Idempotency-Key"))
	if e != nil {
		switch e {
		case service.ErrEmailFeatureDisabled:
			response.ErrorWithDetails(c, http.StatusNotFound, "Email service is unavailable", "FEATURE_DISABLED", nil)
		case service.ErrEmailInsufficientBalance:
			response.ErrorWithDetails(c, http.StatusPaymentRequired, "Insufficient balance", "INSUFFICIENT_BALANCE", nil)
		case service.ErrEmailPriceChanged:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote changed; please confirm again", "PRICE_CHANGED", nil)
		case service.ErrEmailQuoteExpired:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote expired; please request a new quote", "QUOTE_EXPIRED", nil)
		case service.ErrEmailQuoteInvalid:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote is invalid; please request a new quote", "QUOTE_INVALID", nil)
		case service.ErrEmailProviderUnknown:
			response.ErrorWithDetails(c, http.StatusAccepted, "The email channel is being reconciled", "ORDER_RECONCILING", nil)
		default:
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Email generation failed", "EMAIL_GENERATION_FAILED", nil)
		}
		return
	}
	response.Success(c, order)
}
func (h *EmailHandler) Orders(c *gin.Context) {
	noStore(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if c.Query("page") != "" || c.Query("page_size") != "" || c.Query("keyword") != "" || c.Query("status") != "" {
		page, pageSize := response.ParsePagination(c)
		items, e := h.svc.ListUserOrdersPage(c.Request.Context(), subject.UserID, page, pageSize, c.Query("keyword"), c.Query("status"))
		if e != nil {
			response.ErrorFrom(c, e)
			return
		}
		response.Success(c, items)
		return
	}
	items, e := h.svc.ListOrders(c.Request.Context(), subject.UserID)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, items)
}
func (h *EmailHandler) Order(c *gin.Context) {
	noStore(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	order, e := h.svc.GetOrder(c.Request.Context(), subject.UserID, strings.TrimSpace(c.Param("id")))
	if e != nil {
		response.ErrorWithDetails(c, http.StatusNotFound, "Email order not found", "NOT_FOUND", nil)
		return
	}
	response.Success(c, order)
}

func (h *EmailHandler) Cancel(c *gin.Context) {
	noStore(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.CancelOrder(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrEmailNotFound {
			response.ErrorWithDetails(c, http.StatusNotFound, "Email order not found", "NOT_FOUND", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Email order cannot be cancelled", "CANCEL_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "cancelled"})
}

func (h *EmailHandler) Refund(c *gin.Context) {
	noStore(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.RequestRefund(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrEmailNotFound {
			response.ErrorWithDetails(c, http.StatusNotFound, "Email order not found", "NOT_FOUND", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Email order is not eligible for refund", "REFUND_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "approved"})
}

func (h *EmailHandler) RefundStatus(c *gin.Context) {
	noStore(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	order, err := h.svc.GetOrder(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorWithDetails(c, http.StatusNotFound, "Email order not found", "NOT_FOUND", nil)
		return
	}
	response.Success(c, gin.H{"status": order.RefundStatus, "reason": order.RefundReason})
}

func (h *EmailHandler) AdminProviders(c *gin.Context) {
	items, e := h.svc.AdminProviders(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, items)
}
func (h *EmailHandler) AdminProviderUpdate(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.BadRequest(c, "invalid provider id")
		return
	}
	var req struct {
		Enabled       bool           `json:"enabled"`
		BaseURL       string         `json:"base_url"`
		CredentialRef string         `json:"credential_ref"`
		Billing       map[string]any `json:"billing"`
	}
	if e = c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, "invalid provider request")
		return
	}
	if e = h.svc.AdminUpdateProviderConfig(c.Request.Context(), id, req.Enabled, req.BaseURL, req.CredentialRef, req.Billing); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, gin.H{"updated": true})
}
func (h *EmailHandler) AdminProviderTest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider id")
		return
	}
	result, err := h.svc.AdminTestProvider(c.Request.Context(), id)
	if err != nil {
		if err == service.ErrEmailProviderTestCooldown {
			response.ErrorWithDetails(c, http.StatusTooManyRequests, "Provider test is cooling down", "TEST_COOLDOWN", nil)
			return
		}
		if err == service.ErrEmailProviderCredentialMissing || strings.Contains(err.Error(), "credential is not configured") {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Provider credential is not configured", "PROVIDER_CREDENTIAL_MISSING", nil)
			return
		}
		var providerErr *service.EmailProviderHTTPError
		if errors.As(err, &providerErr) {
			response.ErrorWithDetails(c, http.StatusBadGateway, "Provider rejected the health check", "PROVIDER_HEALTH_CHECK_REJECTED", map[string]string{"provider_status": strconv.Itoa(providerErr.StatusCode)})
			return
		}
		response.ErrorWithDetails(c, http.StatusBadGateway, "Provider health check failed", "PROVIDER_HEALTH_CHECK_FAILED", nil)
		return
	}
	response.Success(c, result)
}
func (h *EmailHandler) AdminChannels(c *gin.Context) {
	items, e := h.svc.AdminChannels(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, items)
}
func (h *EmailHandler) AdminChannelUpdate(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		response.BadRequest(c, "invalid channel id")
		return
	}
	var req struct {
		Enabled                     bool     `json:"enabled"`
		Visible                     bool     `json:"visible"`
		Healthy                     bool     `json:"healthy"`
		SalePrice                   float64  `json:"sale_price"`
		RefundPolicy                string   `json:"refund_policy"`
		CapturePolicy               string   `json:"capture_policy"`
		BaseMarkup                  *float64 `json:"base_markup"`
		FixedMarkup                 *float64 `json:"fixed_markup"`
		MinimumProfit               *float64 `json:"minimum_profit"`
		OrderTTLSeconds             *int     `json:"order_ttl_seconds"`
		MaxProviderRequestsPerOrder *int     `json:"max_provider_requests_per_order"`
		PollingBackoff              []int    `json:"polling_backoff"`
	}
	if e = c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, "invalid channel request")
		return
	}
	if e = h.svc.AdminUpdateChannel(c.Request.Context(), id, req.Enabled, req.Visible, req.Healthy, req.SalePrice, req.RefundPolicy, req.CapturePolicy, req.OrderTTLSeconds, req.MaxProviderRequestsPerOrder, req.PollingBackoff); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	if e = h.svc.AdminUpdateChannelPricing(c.Request.Context(), id, req.BaseMarkup, req.FixedMarkup, req.MinimumProfit); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, gin.H{"updated": true})
}
func (h *EmailHandler) AdminStats(c *gin.Context) {
	items, e := h.svc.AdminStats(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, items)
}
func (h *EmailHandler) AdminOrders(c *gin.Context) {
	items, e := h.svc.AdminOrders(c.Request.Context())
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, items)
}
func (h *EmailHandler) AdminToggle(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, "invalid feature flag request")
		return
	}
	if e := h.svc.SetEnabled(c.Request.Context(), req.Enabled); e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, gin.H{"enabled": req.Enabled})
}
