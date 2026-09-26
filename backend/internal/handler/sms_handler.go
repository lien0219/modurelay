package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

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
	ProviderCode  string `form:"provider" json:"provider_code"`
	ServiceCode   string `form:"service" json:"service_code"`
	Services      string `form:"services" json:"services"`
	CountryCode   string `form:"country" json:"country_code"`
	ProductType   string `form:"product_type" json:"product_type"`
	OperatorCode  string `form:"operator" json:"operator_code"`
	VoiceMode     int    `form:"voice_mode" json:"voice_mode"`
	DurationValue int    `form:"duration_value" json:"duration_value"`
	DurationUnit  string `form:"duration_unit" json:"duration_unit"`
}

// isPublicSMSChannel keeps the user API boundary opaque. Provider codes are
// internal routing identifiers and must not be accepted from browser-visible
// catalog or quote endpoints. The public catalog currently exposes exactly
// the two production channels; adding another channel requires an explicit
// public projection rather than falling back to a provider code.
func isPublicSMSChannel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "channel_1", "channel_2":
		return true
	default:
		return false
	}
}

func rejectNonPublicSMSChannel(c *gin.Context, value string) bool {
	if isPublicSMSChannel(value) {
		return false
	}
	// Keep the response provider-neutral as well. In particular, do not echo
	// a manually supplied provider code such as "smspva" back to the browser.
	response.ErrorWithDetails(c, http.StatusNotFound, "SMS channel is unavailable", "CHANNEL_UNAVAILABLE", nil)
	return true
}

func (h *SMSHandler) Quotes(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req smsQuoteRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "invalid quote request")
		return
	}
	req.ProviderCode = strings.ToLower(strings.TrimSpace(req.ProviderCode))
	req.ServiceCode = strings.ToLower(strings.TrimSpace(req.ServiceCode))
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	if req.ProviderCode != "" && rejectNonPublicSMSChannel(c, req.ProviderCode) {
		return
	}
	if req.ProductType == "" {
		req.ProductType = "temporary"
	}
	internalProviderCode := ""
	if req.ProviderCode != "" {
		var resolveErr error
		internalProviderCode, resolveErr = h.svc.ResolvePublicChannelProvider(c.Request.Context(), req.ProviderCode)
		if resolveErr != nil {
			response.ErrorWithDetails(c, http.StatusServiceUnavailable, "The selected channel is temporarily unavailable", "PROVIDER_UNAVAILABLE", nil)
			return
		}
	}
	serviceCodes := []string{}
	for _, value := range strings.Split(req.Services, ",") {
		if value = strings.TrimSpace(value); value != "" {
			serviceCodes = append(serviceCodes, value)
		}
	}
	quotes, err := h.svc.Quote(c.Request.Context(), subject.UserID, service.SMSQuoteRequest{ProviderCode: internalProviderCode, ServiceCode: req.ServiceCode, ServiceCodes: serviceCodes, CountryCode: req.CountryCode, ProductType: req.ProductType, OperatorCode: strings.TrimSpace(req.OperatorCode), VoiceMode: req.VoiceMode, DurationValue: req.DurationValue, DurationUnit: strings.ToLower(strings.TrimSpace(req.DurationUnit))})
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
	items, _, err := h.svc.ProviderCatalog(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	for i := range items {
		items[i].ProviderCode = ""
		items[i].ProviderIconPath = ""
	}
	response.Success(c, items)
}
func (h *SMSHandler) Providers(c *gin.Context) {
	items, err := h.svc.ListPublicProviders(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *SMSHandler) RecentSuccesses(c *gin.Context) {
	feed, err := h.svc.RecentSuccesses(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, feed)
}

// Settings exposes the user-safe SMS purchase policy. Administrative pricing
// and markup fields remain available only through the admin pricing endpoint.
func (h *SMSHandler) Settings(c *gin.Context) {
	pricing, err := h.svc.GetPricingSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"batch_purchase_limit": pricing.BatchPurchaseLimit})
}

func localizeSMSCountryCatalog(items []service.SMSCountryCatalogItem) {
	zh := display.Regions(language.SimplifiedChinese)
	en := display.Regions(language.English)
	for i := range items {
		tag := language.Make("und-" + strings.ToUpper(strings.TrimSpace(items[i].ISO2)))
		region, confidence := tag.Region()
		if confidence == language.No {
			continue
		}
		if strings.TrimSpace(items[i].NameZH) == "" {
			items[i].NameZH = zh.Name(region)
		}
		if strings.TrimSpace(items[i].NameEN) == "" {
			items[i].NameEN = en.Name(region)
		}
	}
}

func (h *SMSHandler) ProviderServices(c *gin.Context) {
	if rejectNonPublicSMSChannel(c, c.Param("provider")) {
		return
	}
	durationValue, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("duration_value", "0")))
	providerCode, err := h.svc.ResolvePublicChannelProvider(c.Request.Context(), c.Param("provider"))
	if err != nil {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "The selected channel is temporarily unavailable", "PROVIDER_UNAVAILABLE", nil)
		return
	}
	items, err := h.svc.ProviderServicesForProduct(c.Request.Context(), providerCode, c.DefaultQuery("product_type", "temporary"), durationValue, c.Query("duration_unit"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	for i := range items {
		items[i].ProviderCode = ""
		items[i].ProviderIconPath = ""
	}
	popular := map[string]int{"amazon": 1, "apple": 2, "discord": 3, "facebook": 4, "google": 5, "instagram": 6, "microsoft": 7, "openai": 8, "telegram": 9, "whatsapp": 10}
	sort.SliceStable(items, func(i, j int) bool {
		ri, iPopular := popular[strings.ToLower(items[i].Code)]
		rj, jPopular := popular[strings.ToLower(items[j].Code)]
		if iPopular != jPopular {
			return iPopular
		}
		if iPopular && ri != rj {
			return ri < rj
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	if c.Query("page") == "" && c.Query("page_size") == "" && c.Query("keyword") == "" {
		response.Success(c, items)
		return
	}

	page, size := response.ParsePagination(c)
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	filtered := make([]service.SMSSvcCatalogItem, 0, len(items))
	for _, item := range items {
		if keyword == "" || strings.Contains(strings.ToLower(item.Code), keyword) || strings.Contains(strings.ToLower(item.Name), keyword) {
			filtered = append(filtered, item)
		}
	}
	start := (page - 1) * size
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	pages := (len(filtered) + size - 1) / size
	response.Success(c, gin.H{
		"items":     filtered[start:end],
		"total":     len(filtered),
		"page":      page,
		"page_size": size,
		"pages":     pages,
		"has_more":  end < len(filtered),
	})
}

func (h *SMSHandler) ProviderOperators(c *gin.Context) {
	if rejectNonPublicSMSChannel(c, c.Param("provider")) {
		return
	}
	voiceMode, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("voice_mode", "0")))
	if err != nil || voiceMode < 0 || voiceMode > 2 {
		response.BadRequest(c, "invalid voice_mode")
		return
	}
	durationValue, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("duration_value", "0")))
	providerCode, err := h.svc.ResolvePublicChannelProvider(c.Request.Context(), c.Param("provider"))
	if err != nil {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "The selected channel is temporarily unavailable", "PROVIDER_UNAVAILABLE", nil)
		return
	}
	items, err := h.svc.ProviderOperatorsForProduct(c.Request.Context(), providerCode, c.Param("service"), c.Param("country"), c.DefaultQuery("product_type", "temporary"), voiceMode, durationValue, c.Query("duration_unit"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *SMSHandler) Countries(c *gin.Context) {
	_, items, err := h.svc.ProviderCatalog(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	localizeSMSCountryCatalog(items)
	for i := range items {
		items[i].ProviderCode = ""
	}
	response.Success(c, items)
}

func (h *SMSHandler) ServiceCountries(c *gin.Context) {
	if rejectNonPublicSMSChannel(c, c.Param("provider")) {
		return
	}
	serviceCode := strings.ToLower(strings.TrimSpace(c.Param("service")))
	durationValue, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("duration_value", "0")))
	internalProviderCode, err := h.svc.ResolvePublicChannelProvider(c.Request.Context(), c.Param("provider"))
	if err != nil {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "The selected channel is temporarily unavailable", "PROVIDER_UNAVAILABLE", nil)
		return
	}
	items, err := h.svc.CountriesForProviderServiceProduct(c.Request.Context(), internalProviderCode, serviceCode, c.DefaultQuery("product_type", "temporary"), durationValue, c.Query("duration_unit"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	for i := range items {
		items[i].ProviderCode = ""
	}
	localizeSMSCountryCatalog(items)
	for i := range items {
		items[i].ProviderCode = ""
	}
	providerCode := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	sortMode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("sort", "")))
	// channel_1 is the opaque public projection of the frozen 5SIM baseline;
	// preserve its established recommended ordering without exposing or
	// accepting the upstream provider code at the HTTP boundary.
	if sortMode == "" && providerCode == "channel_1" {
		sortMode = "recommended"
	}
	effectiveRate := func(item service.SMSCountryCatalogItem) float64 {
		if item.Platform30dSuccessRate != nil {
			return *item.Platform30dSuccessRate
		}
		return item.ConversionRate
	}
	switch sortMode {
	case "recommended":
		sort.SliceStable(items, func(i, j int) bool {
			left := effectiveRate(items[i])
			right := effectiveRate(items[j])
			if left != right {
				return left > right
			}
			if items[i].Platform30dSampleSize != items[j].Platform30dSampleSize {
				return items[i].Platform30dSampleSize > items[j].Platform30dSampleSize
			}
			if items[i].Stock != items[j].Stock {
				return items[i].Stock > items[j].Stock
			}
			return strings.ToLower(items[i].NameEN) < strings.ToLower(items[j].NameEN)
		})
	case "platform":
		sort.SliceStable(items, func(i, j int) bool {
			left, right := items[i].Platform30dSuccessRate, items[j].Platform30dSuccessRate
			if left == nil && right != nil {
				return false
			}
			if right == nil && left != nil {
				return true
			}
			if left != nil && right != nil && *left != *right {
				return *left > *right
			}
			if items[i].Platform30dSampleSize != items[j].Platform30dSampleSize {
				return items[i].Platform30dSampleSize > items[j].Platform30dSampleSize
			}
			return items[i].ConversionRate > items[j].ConversionRate
		})
	case "rate":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].ConversionRate != items[j].ConversionRate {
				return items[i].ConversionRate > items[j].ConversionRate
			}
			if items[i].Stock != items[j].Stock {
				return items[i].Stock > items[j].Stock
			}
			return strings.ToLower(items[i].NameEN) < strings.ToLower(items[j].NameEN)
		})
	case "price":
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].RecommendedStartingPrice
			if left <= 0 {
				left = items[i].StartingPrice
			}
			right := items[j].RecommendedStartingPrice
			if right <= 0 {
				right = items[j].StartingPrice
			}
			if left <= 0 && right > 0 {
				return false
			}
			if right <= 0 && left > 0 {
				return true
			}
			if left != right {
				return left < right
			}
			return items[i].ConversionRate > items[j].ConversionRate
		})
	case "stock":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Stock != items[j].Stock {
				return items[i].Stock > items[j].Stock
			}
			return items[i].ConversionRate > items[j].ConversionRate
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.TrimSpace(items[i].NameEN)
			right := strings.TrimSpace(items[j].NameEN)
			if left == "" {
				left = items[i].ISO2
			}
			if right == "" {
				right = items[j].ISO2
			}
			return strings.ToLower(left) < strings.ToLower(right)
		})
	}
	if c.Query("page") == "" && c.Query("page_size") == "" && c.Query("keyword") == "" {
		response.Success(c, items)
		return
	}

	page, size := response.ParsePagination(c)
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	filtered := make([]service.SMSCountryCatalogItem, 0, len(items))
	for _, item := range items {
		if keyword == "" ||
			strings.Contains(strings.ToLower(item.ISO2), keyword) ||
			strings.Contains(strings.ToLower(item.NameEN), keyword) ||
			strings.Contains(strings.ToLower(item.NameZH), keyword) {
			filtered = append(filtered, item)
		}
	}
	start := (page - 1) * size
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	pages := (len(filtered) + size - 1) / size
	response.Success(c, gin.H{
		"items":     filtered[start:end],
		"total":     len(filtered),
		"page":      page,
		"page_size": size,
		"pages":     pages,
		"has_more":  end < len(filtered),
	})
}

type smsPurchaseRequest struct {
	ChannelCode   string   `json:"channel_code"`
	ServiceCode   string   `json:"service_code"`
	ServiceCodes  []string `json:"service_codes"`
	CountryCode   string   `json:"country_code"`
	ProductType   string   `json:"product_type"`
	OperatorCode  string   `json:"operator_code"`
	VoiceMode     int      `json:"voice_mode"`
	DurationValue int      `json:"duration_value"`
	DurationUnit  string   `json:"duration_unit"`
	QuoteID       string   `json:"quote_id"`
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
	if strings.TrimSpace(req.QuoteID) == "" {
		response.BadRequest(c, "quote_id is required; request a fresh quote before purchasing")
		return
	}
	if req.ProductType == "" {
		req.ProductType = "temporary"
	}
	order, err := h.svc.Purchase(c.Request.Context(), subject.UserID, service.SMSPurchaseRequest{ChannelCode: strings.TrimSpace(req.ChannelCode), ServiceCode: strings.ToLower(strings.TrimSpace(req.ServiceCode)), ServiceCodes: req.ServiceCodes, CountryCode: strings.ToUpper(strings.TrimSpace(req.CountryCode)), ProductType: req.ProductType, OperatorCode: strings.TrimSpace(req.OperatorCode), VoiceMode: req.VoiceMode, DurationValue: req.DurationValue, DurationUnit: req.DurationUnit, QuoteID: strings.TrimSpace(req.QuoteID)}, c.GetHeader("Idempotency-Key"), req.ExpectedPrice)
	if err != nil {
		switch err {
		case service.ErrSMSFeatureDisabled:
			response.ErrorWithDetails(c, http.StatusNotFound, "SMS Verification is unavailable", "FEATURE_DISABLED", nil)
		case service.ErrSMSInsufficientBalance:
			response.ErrorWithDetails(c, http.StatusPaymentRequired, "Insufficient balance", "INSUFFICIENT_BALANCE", nil)
		case service.ErrSMSInsufficientStock:
			response.ErrorWithDetails(c, http.StatusConflict, "The selected channel does not have enough stock", "INSUFFICIENT_STOCK", nil)
		case service.ErrSMSProviderUnavailable:
			response.ErrorWithDetails(c, http.StatusServiceUnavailable, "The selected channel is temporarily unavailable", "PROVIDER_UNAVAILABLE", nil)
		case service.ErrSMSPriceChanged:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote changed; please confirm again", "PRICE_CHANGED", nil)
		case service.ErrSMSQuoteInvalid, service.ErrSMSQuoteExpired:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote is no longer valid; please request a new quote", "QUOTE_EXPIRED", nil)
		case service.ErrSMSProviderUnknown:
			response.ErrorWithDetails(c, http.StatusAccepted, "The channel purchase is being reconciled", "ORDER_RECONCILING", nil)
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, order)
}

func (h *SMSHandler) PurchaseBatch(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		Items []smsPurchaseRequest `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.BadRequest(c, "items are required")
		return
	}
	items := make([]service.SMSPurchaseRequest, 0, len(req.Items))
	prices := make([]*float64, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, service.SMSPurchaseRequest{ChannelCode: strings.TrimSpace(item.ChannelCode), ServiceCode: strings.ToLower(strings.TrimSpace(item.ServiceCode)), ServiceCodes: item.ServiceCodes, CountryCode: strings.ToUpper(strings.TrimSpace(item.CountryCode)), ProductType: item.ProductType, OperatorCode: strings.TrimSpace(item.OperatorCode), VoiceMode: item.VoiceMode, DurationValue: item.DurationValue, DurationUnit: item.DurationUnit, QuoteID: strings.TrimSpace(item.QuoteID)})
		prices = append(prices, item.ExpectedPrice)
	}
	orders, err := h.svc.PurchaseBatch(c.Request.Context(), subject.UserID, items, c.GetHeader("Idempotency-Key"), prices)
	if err != nil && len(orders) == 0 {
		var batchLimitErr service.SMSBatchPurchaseLimitError
		if errors.As(err, &batchLimitErr) {
			response.ErrorWithDetails(c, http.StatusBadRequest, batchLimitErr.Error(), "BATCH_PURCHASE_LIMIT_EXCEEDED", map[string]string{"max": strconv.Itoa(batchLimitErr.Limit)})
			return
		}
		switch err {
		case service.ErrSMSInsufficientBalance:
			response.ErrorWithDetails(c, http.StatusPaymentRequired, "Insufficient balance", "INSUFFICIENT_BALANCE", nil)
		case service.ErrSMSInsufficientStock:
			response.ErrorWithDetails(c, http.StatusConflict, "The selected channel does not have enough stock", "INSUFFICIENT_STOCK", nil)
		case service.ErrSMSProviderUnavailable:
			response.ErrorWithDetails(c, http.StatusServiceUnavailable, "The selected channel is temporarily unavailable", "PROVIDER_UNAVAILABLE", nil)
		case service.ErrSMSPriceChanged:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote changed; please confirm again", "PRICE_CHANGED", nil)
		case service.ErrSMSQuoteInvalid, service.ErrSMSQuoteExpired:
			response.ErrorWithDetails(c, http.StatusConflict, "The quote is no longer valid; please request a new quote", "QUOTE_EXPIRED", nil)
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, gin.H{"items": orders, "partial_error": func() string {
		if err != nil {
			return err.Error()
		}
		return ""
	}()})
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
	// SMSPVA has no documented webhook endpoint, signature scheme, or retry
	// contract. Reject it before reading a deployment secret or payload so an
	// accidentally configured generic secret cannot turn polling-only channel 2
	// into a false webhook capability. Keep the legacy 5SIM path unchanged.
	if provider == "smspva" {
		response.ErrorWithDetails(c, http.StatusNotImplemented, "This provider does not support webhooks", "WEBHOOK_UNSUPPORTED", nil)
		return
	}
	secret := strings.TrimSpace(os.Getenv("SMS_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_WEBHOOK_SECRET"))
	supplied := strings.TrimSpace(c.GetHeader("X-SMS-Webhook-Secret"))
	if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(supplied)) != 1 {
		response.ErrorWithDetails(c, http.StatusUnauthorized, "Webhook signature is invalid", "WEBHOOK_UNAUTHORIZED", nil)
		return
	}
	var payload struct {
		OrderID  string   `json:"provider_order_id"`
		ID       any      `json:"id"`
		Status   string   `json:"status"`
		Phone    string   `json:"phone_number"`
		PhoneAlt string   `json:"phone"`
		Code     string   `json:"code"`
		Messages []string `json:"messages"`
		Message  string   `json:"message"`
		SMS      []struct {
			Text   string `json:"text"`
			Code   string `json:"code"`
			Sender string `json:"sender"`
		} `json:"sms"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		response.BadRequest(c, "invalid webhook payload")
		return
	}
	orderID := strings.TrimSpace(payload.OrderID)
	if orderID == "" {
		switch value := payload.ID.(type) {
		case string:
			orderID = strings.TrimSpace(value)
		case float64:
			if value == float64(int64(value)) {
				orderID = strconv.FormatInt(int64(value), 10)
			}
		}
	}
	if orderID == "" {
		response.BadRequest(c, "provider_order_id is required")
		return
	}
	messages := append([]string(nil), payload.Messages...)
	metadataMessages := make([]map[string]any, 0, len(messages)+len(payload.SMS)+1)
	for range messages {
		metadataMessages = append(metadataMessages, map[string]any{})
	}
	if payload.Message != "" {
		messages = append(messages, payload.Message)
		metadataMessages = append(metadataMessages, map[string]any{})
	}
	for _, item := range payload.SMS {
		text := strings.TrimSpace(item.Text)
		code := strings.TrimSpace(item.Code)
		if text == "" {
			text = code
		}
		if text == "" {
			continue
		}
		messages = append(messages, text)
		metadataMessages = append(metadataMessages, map[string]any{
			"verification_code": code,
			"sender":            strings.TrimSpace(item.Sender),
		})
	}
	if code := strings.TrimSpace(payload.Code); code != "" {
		if len(messages) == 0 {
			messages = append(messages, code)
			metadataMessages = append(metadataMessages, map[string]any{"verification_code": code})
		} else if len(metadataMessages) > 0 {
			metadataMessages[0]["verification_code"] = code
		}
	}
	phone := strings.TrimSpace(payload.Phone)
	if phone == "" {
		phone = strings.TrimSpace(payload.PhoneAlt)
	}
	metadata := map[string]any{}
	if len(metadataMessages) > 0 {
		metadata["messages"] = metadataMessages
	}
	if err := h.svc.ProcessWebhook(c.Request.Context(), provider, service.SMSStatusResult{Status: payload.Status, PhoneNumber: phone, Messages: messages, Metadata: metadata}, orderID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"accepted": true})
}
func (h *SMSHandler) Resend(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.ResendOrder(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, err.Error(), "RESEND_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "active"})
}

func (h *SMSHandler) Finish(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.FinishOrder(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrSMSProviderUnknown {
			response.ErrorWithDetails(c, http.StatusAccepted, "The finish action is being reconciled", "ORDER_RECONCILING", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, err.Error(), "FINISH_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "completed"})
}
func (h *SMSHandler) Ban(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.BanOrder(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrSMSProviderUnknown {
			response.ErrorWithDetails(c, http.StatusAccepted, "The ban action is being reconciled", "ORDER_RECONCILING", nil)
			return
		}
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, err.Error(), "BAN_REJECTED", nil)
		return
	}
	response.Success(c, gin.H{"status": "reconciling"})
}

func (h *SMSHandler) Cancel(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.svc.CancelOrder(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
		if err == service.ErrSMSCancelTooEarly {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Please wait until the configured cancellation window before cancelling this order", "CANCEL_TOO_EARLY", nil)
			return
		}
		if err == service.ErrSMSOrderExpired {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "The order validity period has ended; automatic cancellation/refund is being handled", "CANCEL_TOO_LATE", nil)
			return
		}
		if err == service.ErrSMSProviderUnknown || err == service.ErrSMSRefundPending {
			response.ErrorWithDetails(c, http.StatusAccepted, "The cancellation/refund is being reconciled", "ORDER_RECONCILING", nil)
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
		if err == service.ErrSMSCancelTooEarly {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Please wait until the configured cancellation window before requesting a refund", "CANCEL_TOO_EARLY", nil)
			return
		}
		if err == service.ErrSMSOrderExpired {
			response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "The order validity period has ended; automatic cancellation/refund is being handled", "CANCEL_TOO_LATE", nil)
			return
		}
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
func (h *SMSHandler) RentalServiceOptions(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	rentDays, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("rent_days", "7")))
	if err != nil || rentDays <= 0 {
		response.BadRequest(c, "invalid rent_days")
		return
	}
	items, err := h.svc.RentalServiceOptions(c.Request.Context(), subject.UserID, c.Param("id"), rentDays)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *SMSHandler) RentalServiceQuote(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		ServiceCode string `json:"service_code"`
		RentDays    int    `json:"rent_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ServiceCode) == "" || req.RentDays <= 0 {
		response.BadRequest(c, "invalid rental service quote request")
		return
	}
	quote, err := h.svc.CreateRentalServiceQuote(c.Request.Context(), subject.UserID, c.Param("id"), req.ServiceCode, req.RentDays)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}

func (h *SMSHandler) AddRentalService(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		QuoteID       string   `json:"quote_id"`
		ExpectedPrice *float64 `json:"expected_price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.QuoteID) == "" {
		response.BadRequest(c, "invalid rental service purchase request")
		return
	}
	order, err := h.svc.AddRentalService(c.Request.Context(), subject.UserID, c.Param("id"), req.QuoteID, c.GetHeader("Idempotency-Key"), req.ExpectedPrice)
	if err != nil {
		switch err {
		case service.ErrSMSPriceChanged:
			response.ErrorWithDetails(c, http.StatusConflict, "Price changed; refresh the quote and try again", "PRICE_CHANGED", nil)
		case service.ErrSMSInsufficientBalance:
			response.ErrorWithDetails(c, http.StatusPaymentRequired, "Insufficient balance", "INSUFFICIENT_BALANCE", nil)
		case service.ErrSMSProviderUnknown:
			response.ErrorWithDetails(c, http.StatusAccepted, "The provider result is being reconciled", "ORDER_RECONCILING", nil)
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, order)
}

func (h *SMSHandler) RentalRestoreQuote(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	quote, err := h.svc.CreateRentalRestoreQuote(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}

func (h *SMSHandler) RestoreRental(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		QuoteID       string   `json:"quote_id"`
		ExpectedPrice *float64 `json:"expected_price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.QuoteID) == "" {
		response.BadRequest(c, "invalid rental restore request")
		return
	}
	order, err := h.svc.RestoreRentalOrder(c.Request.Context(), subject.UserID, c.Param("id"), req.QuoteID, c.GetHeader("Idempotency-Key"), req.ExpectedPrice)
	if err != nil {
		switch err {
		case service.ErrSMSPriceChanged:
			response.ErrorWithDetails(c, http.StatusConflict, "Price changed; refresh the quote and try again", "PRICE_CHANGED", nil)
		case service.ErrSMSInsufficientBalance:
			response.ErrorWithDetails(c, http.StatusPaymentRequired, "Insufficient balance", "INSUFFICIENT_BALANCE", nil)
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, order)
}

func (h *SMSHandler) ServiceIcon(c *gin.Context) {
	// User-facing icon URLs are provider-neutral. The service resolves the
	// catalog's trusted upstream internally; no provider identifier belongs in
	// the browser-visible request path.
	data, contentType, err := h.svc.ServiceIcon(c.Request.Context(), "", c.Param("service"))
	if err != nil {
		// Service icons are optional decoration. Missing/stale upstream artwork
		// must fall back quietly instead of surfacing dozens of expected 404s in
		// the browser console during catalog rendering.
		c.Header("Cache-Control", "public, max-age=300")
		c.Status(http.StatusNoContent)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Header("Cache-Control", "public, max-age=43200, stale-while-revalidate=86400")
	c.Data(http.StatusOK, contentType, data)
}

func (h *SMSHandler) RentalConstraints(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	constraints, err := h.svc.RentalConstraints(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorWithDetails(c, http.StatusUnprocessableEntity, "Rental extension is unavailable for this order", "RENTAL_EXTENSION_UNSUPPORTED", nil)
		return
	}
	response.Success(c, constraints)
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

func (h *SMSHandler) AdminCatalogSync(c *gin.Context) {
	provider := strings.TrimSpace(c.Param("provider"))
	if err := h.svc.SyncProviderCatalog(c.Request.Context(), provider); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"synced": true})
}

func (h *SMSHandler) AdminCatalogSyncStatus(c *gin.Context) {
	status, err := h.svc.CatalogSyncStatus(c.Request.Context(), c.Param("provider"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}
func (h *SMSHandler) AdminProviderMappings(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider id")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, err := h.svc.ListProviderMappings(c.Request.Context(), id, strings.ToLower(strings.TrimSpace(c.Query("kind"))), page, pageSize, c.Query("keyword"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *SMSHandler) AdminProviderServiceMappingUpdate(c *gin.Context) {
	providerID, providerErr := strconv.ParseInt(c.Param("id"), 10, 64)
	serviceID, serviceErr := strconv.ParseInt(c.Param("service_id"), 10, 64)
	if providerErr != nil || serviceErr != nil {
		response.BadRequest(c, "invalid mapping id")
		return
	}
	var req struct {
		ProviderCode       string `json:"provider_code"`
		ProviderName       string `json:"provider_name"`
		TemporarySupported bool   `json:"temporary_supported"`
		RentalSupported    bool   `json:"rental_supported"`
		Enabled            bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid provider service mapping")
		return
	}
	if err := h.svc.UpsertProviderServiceMapping(c.Request.Context(), providerID, serviceID, req.ProviderCode, req.ProviderName, req.TemporarySupported, req.RentalSupported, req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}
func (h *SMSHandler) AdminProviderCountryMappingUpdate(c *gin.Context) {
	providerID, providerErr := strconv.ParseInt(c.Param("id"), 10, 64)
	countryID, countryErr := strconv.ParseInt(c.Param("country_id"), 10, 64)
	if providerErr != nil || countryErr != nil {
		response.BadRequest(c, "invalid mapping id")
		return
	}
	var req struct {
		ProviderCountryID   string `json:"provider_country_id"`
		ProviderCountryCode string `json:"provider_country_code"`
		Enabled             bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid provider country mapping")
		return
	}
	if err := h.svc.UpsertProviderCountryMapping(c.Request.Context(), providerID, countryID, req.ProviderCountryID, req.ProviderCountryCode, req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
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

func (h *SMSHandler) AdminPricing(c *gin.Context) {
	settings, err := h.svc.GetPricingSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

type smsPricingSettingsUpdateRequest struct {
	CostMultiplier                float64            `json:"cost_multiplier"`
	FixedMarkup                   float64            `json:"fixed_markup"`
	UnknownGradeMultiplier        float64            `json:"unknown_grade_multiplier"`
	UnknownGradeFixedMarkup       float64            `json:"unknown_grade_fixed_markup"`
	TemporaryExpiryMinutes        int                `json:"temporary_expiry_minutes"`
	SelfServiceCancelAfterMinutes int                `json:"self_service_cancel_after_minutes"`
	BatchPurchaseLimit            *int               `json:"batch_purchase_limit"`
	GradeMultipliers              map[string]float64 `json:"grade_multipliers"`
	GradeFixedMarkups             map[string]float64 `json:"grade_fixed_markups"`
}

func (h *SMSHandler) AdminPricingUpdate(c *gin.Context) {
	var req smsPricingSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid SMS pricing settings")
		return
	}
	batchPurchaseLimit := 0
	if req.BatchPurchaseLimit == nil {
		current, err := h.svc.GetPricingSettings(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		batchPurchaseLimit = current.BatchPurchaseLimit
	} else {
		batchPurchaseLimit = *req.BatchPurchaseLimit
	}
	settings := service.SMSPricingSettings{
		CostMultiplier:                req.CostMultiplier,
		FixedMarkup:                   req.FixedMarkup,
		UnknownGradeMultiplier:        req.UnknownGradeMultiplier,
		UnknownGradeFixedMarkup:       req.UnknownGradeFixedMarkup,
		TemporaryExpiryMinutes:        req.TemporaryExpiryMinutes,
		SelfServiceCancelAfterMinutes: req.SelfServiceCancelAfterMinutes,
		BatchPurchaseLimit:            batchPurchaseLimit,
		GradeMultipliers:              req.GradeMultipliers,
		GradeFixedMarkups:             req.GradeFixedMarkups,
	}
	if err := h.svc.SetPricingSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if saved, err := h.svc.GetPricingSettings(c.Request.Context()); err == nil {
		response.Success(c, saved)
		return
	}
	response.Success(c, settings)
}
