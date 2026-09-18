package service

// SMS Verification domain service. This file deliberately keeps upstream
// provider details behind a narrow adapter interface and returns public
// channel DTOs only. Provider credentials may be supplied through the
// deployment environment or entered by an administrator. Values entered in
// the admin UI are encrypted before they are stored; credential_ref may also
// point to env:NAME.

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrSMSFeatureDisabled           = errors.New("sms service is disabled")
	ErrSMSInsufficientBalance       = errors.New("insufficient balance")
	ErrSMSProviderUnavailable       = errors.New("selected channel is unavailable")
	ErrSMSPriceChanged              = errors.New("price changed; please confirm again")
	ErrSMSProviderUnknown           = errors.New("channel purchase is being reconciled")
	ErrSMSRefundPending             = errors.New("channel refund is being processed")
	ErrSMSProviderCredentialMissing = errors.New("sms provider credential is not configured")
	ErrSMSProviderTestUnsupported   = errors.New("provider does not support a safe connection test")
	ErrSMSProviderTestCooldown      = errors.New("provider connection test is cooling down")
	ErrSMSQuoteInvalid              = errors.New("sms quote is invalid")
	ErrSMSQuoteExpired              = errors.New("sms quote has expired")
	ErrSMSCancelTooEarly            = errors.New("order cannot be cancelled until the configured waiting period has elapsed")
	ErrSMSInsufficientStock         = errors.New("selected channel does not have enough stock for this batch")
	smsVerificationCodePattern      = regexp.MustCompile(`(?:^|[^0-9])([0-9]{4,8})(?:$|[^0-9])`)
)

var smsCatalogCache struct {
	sync.RWMutex
	services  []SMSSvcCatalogItem
	countries []SMSCountryCatalogItem
	expiresAt time.Time
}

type smsProviderServiceCacheEntry struct {
	items     []SMSSvcCatalogItem
	expiresAt time.Time
}

type smsProviderCountryCacheEntry struct {
	items     []SMSCountryCatalogItem
	expiresAt time.Time
}

var smsProviderServiceCache = struct {
	sync.RWMutex
	items map[string]smsProviderServiceCacheEntry
}{items: map[string]smsProviderServiceCacheEntry{}}

var smsProviderCountryCache = struct {
	sync.RWMutex
	items map[string]smsProviderCountryCacheEntry
}{items: map[string]smsProviderCountryCacheEntry{}}

const (
	smsProviderServiceCacheTTL = 2 * time.Minute
	smsProviderCountryCacheTTL = 45 * time.Second
)

func smsProviderCatalogCacheKey(providerCode, productType string, durationValue int, durationUnit string) string {
	return strings.ToLower(strings.TrimSpace(providerCode)) + "|" +
		strings.ToLower(strings.TrimSpace(productType)) + "|" +
		strconv.Itoa(durationValue) + "|" + strings.ToLower(strings.TrimSpace(durationUnit))
}

func smsProviderCountryCacheKey(providerCode, serviceCode, productType string, durationValue int, durationUnit string) string {
	return smsProviderCatalogCacheKey(providerCode, productType, durationValue, durationUnit) + "|" + strings.ToLower(strings.TrimSpace(serviceCode))
}

func cachedProviderServices(key string) ([]SMSSvcCatalogItem, bool) {
	smsProviderServiceCache.RLock()
	entry, ok := smsProviderServiceCache.items[key]
	smsProviderServiceCache.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return append([]SMSSvcCatalogItem(nil), entry.items...), true
}

func cacheProviderServices(key string, items []SMSSvcCatalogItem) {
	smsProviderServiceCache.Lock()
	smsProviderServiceCache.items[key] = smsProviderServiceCacheEntry{
		items:     append([]SMSSvcCatalogItem(nil), items...),
		expiresAt: time.Now().Add(smsProviderServiceCacheTTL),
	}
	smsProviderServiceCache.Unlock()
}

func cachedProviderCountries(key string) ([]SMSCountryCatalogItem, bool) {
	smsProviderCountryCache.RLock()
	entry, ok := smsProviderCountryCache.items[key]
	smsProviderCountryCache.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return append([]SMSCountryCatalogItem(nil), entry.items...), true
}

func cacheProviderCountries(key string, items []SMSCountryCatalogItem) {
	smsProviderCountryCache.Lock()
	smsProviderCountryCache.items[key] = smsProviderCountryCacheEntry{
		items:     append([]SMSCountryCatalogItem(nil), items...),
		expiresAt: time.Now().Add(smsProviderCountryCacheTTL),
	}
	smsProviderCountryCache.Unlock()
}

const (
	smsReconciliationPurchase = "purchase"
	smsReconciliationCancel   = "cancel"
	smsReconciliationRefund   = "refund"
	smsReconciliationExpire   = "expire"
)

type SMSProviderCapabilities struct {
	Temporary         bool `json:"supports_temporary"`
	Rental            bool `json:"supports_rental"`
	RentalCancel      bool `json:"supports_rental_cancel"`
	Webhook           bool `json:"supports_webhook"`
	Polling           bool `json:"supports_polling"`
	Cancel            bool `json:"supports_cancel"`
	Refund            bool `json:"supports_refund"`
	RefundStatus      bool `json:"supports_refund_status"`
	Finish            bool `json:"supports_finish"`
	Ban               bool `json:"supports_ban"`
	Extend            bool `json:"supports_extend"`
	Resend            bool `json:"supports_resend"`
	Voice             bool `json:"supports_voice"`
	VoiceSMS          bool `json:"supports_voice_sms"`
	VoiceCallerID     bool `json:"supports_voice_caller_id"`
	VoiceCall         bool `json:"supports_voice_call"`
	OperatorSelection bool `json:"supports_operator_selection"`
	ServiceSelection  bool `json:"supports_service_selection"`
	ConversionStats   bool `json:"supports_conversion_stats"`
}

// SMSPricingSettings is stored in the existing settings repository so admins
// can change pricing without a deployment. Provider cost is never configured here.
type SMSPricingSettings struct {
	CostMultiplier                float64            `json:"cost_multiplier"`
	FixedMarkup                   float64            `json:"fixed_markup"`
	UnknownGradeMultiplier        float64            `json:"unknown_grade_multiplier"`
	UnknownGradeFixedMarkup       float64            `json:"unknown_grade_fixed_markup"`
	TemporaryExpiryMinutes        int                `json:"temporary_expiry_minutes"`
	SelfServiceCancelAfterMinutes int                `json:"self_service_cancel_after_minutes"`
	GradeMultipliers              map[string]float64 `json:"grade_multipliers,omitempty"`
	GradeFixedMarkups             map[string]float64 `json:"grade_fixed_markups,omitempty"`
}

func defaultSMSPricingSettings() SMSPricingSettings {
	return SMSPricingSettings{CostMultiplier: 1.30, UnknownGradeMultiplier: 1, TemporaryExpiryMinutes: 10, SelfServiceCancelAfterMinutes: 1, GradeMultipliers: map[string]float64{}, GradeFixedMarkups: map[string]float64{}}
}
func (s SMSPricingSettings) Validate() error {
	if s.CostMultiplier <= 0 || s.CostMultiplier > 100 || s.FixedMarkup < 0 || s.UnknownGradeMultiplier <= 0 || s.UnknownGradeMultiplier > 100 || s.UnknownGradeFixedMarkup < 0 || s.TemporaryExpiryMinutes < 1 || s.TemporaryExpiryMinutes > 1440 || s.SelfServiceCancelAfterMinutes < 0 || s.SelfServiceCancelAfterMinutes > 1440 {
		return errors.New("invalid SMS pricing settings")
	}
	valid := map[string]bool{"S": true, "A": true, "B": true, "C": true, "D": true}
	for grade, value := range s.GradeMultipliers {
		if !valid[strings.ToUpper(grade)] || value <= 0 || value > 100 {
			return errors.New("invalid SMS grade multiplier")
		}
	}
	for grade, value := range s.GradeFixedMarkups {
		if !valid[strings.ToUpper(grade)] || value < 0 {
			return errors.New("invalid SMS grade fixed markup")
		}
	}
	return nil
}

func normalizeSMSPricingGradeMaps(settings *SMSPricingSettings) {
	if settings == nil {
		return
	}
	multipliers := make(map[string]float64, len(settings.GradeMultipliers))
	for grade, value := range settings.GradeMultipliers {
		multipliers[strings.ToUpper(strings.TrimSpace(grade))] = value
	}
	fixedMarkups := make(map[string]float64, len(settings.GradeFixedMarkups))
	for grade, value := range settings.GradeFixedMarkups {
		fixedMarkups[strings.ToUpper(strings.TrimSpace(grade))] = value
	}
	settings.GradeMultipliers = multipliers
	settings.GradeFixedMarkups = fixedMarkups
}

type SMSQuoteRequest struct {
	ProviderCode  string `json:"provider_code,omitempty"`
	ServiceCode   string `json:"service_code"`
	CountryCode   string `json:"country_code"`
	ProductType   string `json:"product_type"`
	OperatorCode  string `json:"operator_code,omitempty"`
	VoiceMode     int    `json:"voice_mode,omitempty"`
	DurationValue int    `json:"duration_value,omitempty"`
	DurationUnit  string `json:"duration_unit,omitempty"`
}
type SMSProviderQuote struct {
	Cost                     decimal.Decimal `json:"cost"`
	Currency                 string          `json:"currency"`
	Stock                    int             `json:"stock"`
	ExpiresAt                time.Time       `json:"expires_at"`
	EstimatedDeliverySeconds int             `json:"estimated_delivery_seconds"`
}
type SMSPurchaseRequest struct {
	ChannelCode       string  `json:"channel_code"`
	ServiceCode       string  `json:"service_code"`
	CountryCode       string  `json:"country_code"`
	ProductType       string  `json:"product_type"`
	OperatorCode      string  `json:"operator_code,omitempty"`
	VoiceMode         int     `json:"voice_mode,omitempty"`
	DurationValue     int     `json:"duration_value,omitempty"`
	DurationUnit      string  `json:"duration_unit,omitempty"`
	QuoteID           string  `json:"quote_id,omitempty"`
	ProviderCostLimit float64 `json:"-"`
}

// CloneQuote creates an independent consumable quote for a batch item. The
// provider snapshot is copied atomically; no provider call is repeated and the
// original quote remains available for the first item.
func (s *SMSService) CloneQuote(ctx context.Context, userID int64, quoteID string) (string, error) {
	cloneID := randomID()
	result, err := s.db.ExecContext(ctx, `INSERT INTO sms_quotes (id,user_id,channel_id,provider_id,service_id,country_id,product_type,provider_service_code,provider_country_code,operator_code,voice_mode,duration_value,duration_unit,provider_cost_snapshot,sale_price_snapshot,success_rate_snapshot,success_rate_source_snapshot,success_rate_grade_snapshot,success_rate_multiplier_snapshot,fixed_markup_snapshot,stock,estimated_delivery_seconds,expires_at) SELECT $1,user_id,channel_id,provider_id,service_id,country_id,product_type,provider_service_code,provider_country_code,operator_code,voice_mode,duration_value,duration_unit,provider_cost_snapshot,sale_price_snapshot,success_rate_snapshot,success_rate_source_snapshot,success_rate_grade_snapshot,success_rate_multiplier_snapshot,fixed_markup_snapshot,stock,estimated_delivery_seconds,expires_at FROM sms_quotes WHERE id=$2 AND user_id=$3 AND consumed_at IS NULL AND expires_at>NOW()`, cloneID, strings.TrimSpace(quoteID), userID)
	if err != nil {
		return "", err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return "", ErrSMSQuoteExpired
	}
	return cloneID, nil
}

// SMSMessage is the user-safe projection of a received provider message.
// Provider order IDs and raw provider payloads remain internal-only.
type SMSMessage struct {
	ID               int64     `json:"id"`
	MessageText      string    `json:"message_text"`
	VerificationCode string    `json:"verification_code,omitempty"`
	ReceivedAt       time.Time `json:"received_at"`
}
type SMSPurchaseResult struct {
	ProviderOrderID      string         `json:"provider_order_id"`
	PhoneNumber          string         `json:"phone_number"`
	ExpiresAt            *time.Time     `json:"expires_at,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	ProviderCost         float64        `json:"-"`
	ProviderOperatorCode string         `json:"-"`
}
type SMSStatusResult struct {
	Status               string         `json:"status"`
	PhoneNumber          string         `json:"phone_number"`
	Messages             []string       `json:"messages,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	ProviderCost         float64        `json:"-"`
	ProviderOperatorCode string         `json:"-"`
}

type SMSProvider interface {
	Code() string
	Capabilities(context.Context) SMSProviderCapabilities
	Quote(context.Context, SMSQuoteRequest) (*SMSProviderQuote, error)
	PurchaseTemporary(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error)
	GetTemporaryStatus(context.Context, string) (*SMSStatusResult, error)
	CancelTemporary(context.Context, string) error
	RequestTemporaryRefund(context.Context, string) error
	PurchaseRental(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error)
	GetRentalStatus(context.Context, string) (*SMSStatusResult, error)
	ExtendRental(context.Context, string, int, string) error
	CancelRental(context.Context, string) error
}

// SMSCatalogProvider is optional: providers with a catalog endpoint can expose
// live supported services/countries; providers without it continue using the
// administrator-maintained mapping tables.
type SMSCatalogProvider interface {
	Catalog(context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error)
}

type SMSServiceCatalogProvider interface {
	CatalogServices(context.Context, []SMSCountryCatalogItem) ([]SMSSvcCatalogItem, error)
}
type SMSServiceCountryProvider interface {
	CountriesForService(context.Context, string) ([]SMSCountryCatalogItem, error)
}
type SMSProductServiceCatalogProvider interface {
	CatalogServicesForProduct(context.Context, string, int, string) ([]SMSSvcCatalogItem, error)
}
type SMSProductServiceCountryProvider interface {
	CountriesForServiceProduct(context.Context, string, string, int, string) ([]SMSCountryCatalogItem, error)
}

type SMSOperatorOption struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Stock        int     `json:"stock,omitempty"`
	ProviderCost float64 `json:"-"`
	ProviderRate float64 `json:"provider_rate,omitempty"`
	Available    bool    `json:"available"`
}

type SMSOperatorProvider interface {
	Operators(context.Context, string, string, int) ([]SMSOperatorOption, error)
}
type SMSProductOperatorProvider interface {
	OperatorsForProduct(context.Context, string, string, string, int, int, string) ([]SMSOperatorOption, error)
}

type SMSOrderActionProvider interface {
	FinishTemporary(context.Context, string) error
	BanTemporary(context.Context, string) error
}
type SMSResendProvider interface {
	ResendTemporary(context.Context, string) error
}

type smsProviderHealthChecker interface {
	TestConnection(context.Context) error
}

type httpSMSProvider struct {
	code, baseURL, apiKey string
	client                *http.Client
	cap                   SMSProviderCapabilities
	credentialQueryParam  string
}

func (p *httpSMSProvider) Code() string                                         { return p.code }
func (p *httpSMSProvider) Capabilities(context.Context) SMSProviderCapabilities { return p.cap }

func providerAPIKey(code, credentialRef string, encryptor SecretEncryptor) string {
	envName := "SMS_" + strings.ToUpper(strings.ReplaceAll(code, "-", "_")) + "_API_KEY"
	ref := strings.TrimSpace(credentialRef)
	if strings.HasPrefix(ref, "env:") {
		if key := strings.TrimSpace(os.Getenv(strings.TrimPrefix(ref, "env:"))); key != "" {
			return key
		}
	}
	if strings.HasPrefix(ref, "enc:") && encryptor != nil {
		if value, err := encryptor.Decrypt(strings.TrimPrefix(ref, "enc:")); err == nil {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	if key := strings.TrimSpace(os.Getenv(envName)); key != "" {
		return key
	}
	return ""
}

func (p *httpSMSProvider) request(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	data, err := p.requestBytes(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 && json.Unmarshal(data, out) != nil {
		return fmt.Errorf("provider %s returned invalid response", p.code)
	}
	return nil
}

func (p *httpSMSProvider) requestBytes(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	publicRequest := strings.HasPrefix(strings.TrimLeft(path, "/"), "guest/")
	if strings.TrimSpace(p.apiKey) == "" && !publicRequest {
		return nil, fmt.Errorf("provider %s credential is not configured", p.code)
	}
	base := strings.TrimRight(p.baseURL, "/")
	target := base
	if strings.TrimSpace(path) != "" {
		target += "/" + strings.TrimLeft(path, "/")
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	if p.credentialQueryParam != "" {
		if query == nil {
			query = url.Values{}
		}
		query.Set(p.credentialQueryParam, p.apiKey)
		u.RawQuery = query.Encode()
	}
	var payload io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return nil, e
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.credentialQueryParam == "" && strings.TrimSpace(p.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider %s request: %w", p.code, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(data))
		if len(detail) > 512 {
			detail = detail[:512] + "..."
		}
		if detail != "" {
			return nil, fmt.Errorf("provider %s returned HTTP %d: %s", p.code, resp.StatusCode, detail)
		}
		return nil, fmt.Errorf("provider %s returned HTTP %d", p.code, resp.StatusCode)
	}
	return data, nil
}

type fiveSIMProvider struct{ *httpSMSProvider }

func (p *fiveSIMProvider) Capabilities(context.Context) SMSProviderCapabilities {
	// 5SIM numbers are short-lived activation numbers. Do not expose them as
	// platform rentals: the documented activation flow has no user-selected
	// rental term, renewal lifecycle, or rental cancellation contract.
	return SMSProviderCapabilities{Temporary: true, Rental: false, Polling: true, Cancel: true, Refund: true, Finish: true, Ban: true, Voice: true, VoiceSMS: true, VoiceCall: true, OperatorSelection: true, ServiceSelection: true}
}

func (p *fiveSIMProvider) Catalog(ctx context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	// 5SIM exposes a read-only country catalog without credentials. Its keys are
	// provider slugs (for example, "usa" and "england"), while the nested ISO
	// map gives us the public ISO2 value used by the platform allow-list.
	var payload map[string]struct {
		ISO    map[string]int `json:"iso"`
		NameEN string         `json:"text_en"`
	}
	if err := p.request(ctx, http.MethodGet, "guest/countries", nil, nil, &payload); err != nil {
		return nil, nil, err
	}
	countries := make([]SMSCountryCatalogItem, 0, len(payload))
	for slug, country := range payload {
		iso2Value := ""
		for iso2 := range country.ISO {
			iso2 = strings.ToUpper(strings.TrimSpace(iso2))
			if iso2 == "" {
				continue
			}
			iso2Value = iso2
			break
		}
		if iso2Value == "" {
			continue
		}
		countries = append(countries, SMSCountryCatalogItem{ISO2: iso2Value, ProviderCode: slug, NameEN: strings.TrimSpace(country.NameEN)})
		_ = slug
	}
	return nil, countries, nil
}

func (p *fiveSIMProvider) CatalogServices(ctx context.Context, _ []SMSCountryCatalogItem) ([]SMSSvcCatalogItem, error) {
	return p.CatalogServicesForProduct(ctx, "temporary", 0, "")
}

func (p *fiveSIMProvider) CatalogServicesForProduct(ctx context.Context, productType string, _ int, _ string) ([]SMSSvcCatalogItem, error) {
	if strings.EqualFold(strings.TrimSpace(productType), "rental") {
		return []SMSSvcCatalogItem{}, nil
	}
	var products map[string]struct {
		Category string  `json:"Category"`
		Qty      int     `json:"Qty"`
		Price    float64 `json:"Price"`
	}
	if err := p.request(ctx, http.MethodGet, "guest/products/any/any", nil, nil, &products); err != nil {
		return nil, err
	}
	wanted := "activation"
	out := make([]SMSSvcCatalogItem, 0, len(products))
	for code, product := range products {
		category := strings.ToLower(strings.TrimSpace(product.Category))
		if category != wanted {
			continue
		}
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		out = append(out, SMSSvcCatalogItem{Code: code, Name: fiveSIMDisplayName(code), Category: category, ProviderCode: code, Stock: product.Qty, ProviderCost: product.Price, Available: product.Qty > 0})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

type fiveSIMPricePoint struct {
	Cost  float64 `json:"cost"`
	Count int     `json:"count"`
	Rate  float64 `json:"rate"`
}

func fiveSIMDisplayName(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	names := map[string]string{
		"amazon":    "Amazon",
		"apple":     "Apple",
		"discord":   "Discord",
		"facebook":  "Facebook",
		"google":    "Google/YouTube",
		"instagram": "Instagram/Threads",
		"microsoft": "Microsoft",
		"openai":    "OpenAI/ChatGPT",
		"telegram":  "Telegram",
		"whatsapp":  "WhatsApp",
	}
	if name := names[code]; name != "" {
		return name
	}
	if code == "" {
		return ""
	}
	return code
}

func fiveSIMPriceCountries(prices map[string]map[string]map[string]fiveSIMPricePoint, serviceCode string) map[string]map[string]fiveSIMPricePoint {
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if byCountry, ok := prices[serviceCode]; ok {
		return byCountry
	}
	out := make(map[string]map[string]fiveSIMPricePoint)
	for country, products := range prices {
		if operators, ok := products[serviceCode]; ok {
			out[country] = operators
		}
	}
	return out
}

func fiveSIMPriceOperators(prices map[string]map[string]map[string]fiveSIMPricePoint, countryCode, serviceCode string) map[string]fiveSIMPricePoint {
	countryCode = strings.ToLower(strings.TrimSpace(countryCode))
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if byCountry, ok := prices[serviceCode]; ok {
		if operators, ok := byCountry[countryCode]; ok {
			return operators
		}
	}
	if products, ok := prices[countryCode]; ok {
		return products[serviceCode]
	}
	return nil
}

func (p *fiveSIMProvider) CountriesForService(ctx context.Context, serviceCode string) ([]SMSCountryCatalogItem, error) {
	return p.CountriesForServiceProduct(ctx, serviceCode, "temporary", 0, "")
}

func (p *fiveSIMProvider) CountriesForServiceProduct(ctx context.Context, serviceCode, productType string, _ int, _ string) ([]SMSCountryCatalogItem, error) {
	if strings.EqualFold(strings.TrimSpace(productType), "rental") {
		return []SMSCountryCatalogItem{}, nil
	}
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if serviceCode == "" {
		return nil, ErrSMSProviderUnavailable
	}
	var prices map[string]map[string]map[string]fiveSIMPricePoint
	if err := p.request(ctx, http.MethodGet, "guest/prices", url.Values{"product": {serviceCode}}, nil, &prices); err != nil {
		return nil, err
	}
	var countryMeta map[string]struct {
		ISO    map[string]int `json:"iso"`
		NameEN string         `json:"text_en"`
	}
	if err := p.request(ctx, http.MethodGet, "guest/countries", nil, nil, &countryMeta); err != nil {
		return nil, err
	}
	out := []SMSCountryCatalogItem{}
	for slug, operators := range fiveSIMPriceCountries(prices, serviceCode) {
		stock := 0
		minCost := 0.0
		for _, point := range operators {
			stock += point.Count
			if point.Count > 0 && point.Cost > 0 && (minCost == 0 || point.Cost < minCost) {
				minCost = point.Cost
			}
		}
		meta, ok := countryMeta[slug]
		if !ok {
			continue
		}
		iso2 := ""
		for iso := range meta.ISO {
			iso2 = strings.ToUpper(strings.TrimSpace(iso))
			break
		}
		if iso2 == "" {
			continue
		}
		out = append(out, SMSCountryCatalogItem{ISO2: iso2, ProviderCode: slug, NameEN: strings.TrimSpace(meta.NameEN), Stock: stock, ProviderCost: minCost, Available: stock > 0})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ISO2 < out[j].ISO2 })
	return out, nil
}

func (p *fiveSIMProvider) TestConnection(ctx context.Context) error {
	var out map[string]any
	if err := p.request(ctx, http.MethodGet, "user/profile", nil, nil, &out); err != nil {
		return err
	}
	if providerResponseHasError(out) {
		return errors.New("5SIM credential was rejected")
	}
	return nil
}

func (p *fiveSIMProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	if strings.EqualFold(strings.TrimSpace(req.ProductType), "rental") {
		return nil, ErrSMSProviderUnavailable
	}
	operator := strings.ToLower(strings.TrimSpace(req.OperatorCode))
	if operator == "" {
		operator = "any"
	}
	if req.VoiceMode == 1 || req.VoiceMode < 0 || req.VoiceMode > 2 {
		return nil, ErrSMSProviderUnavailable
	}
	country := strings.ToLower(strings.TrimSpace(req.CountryCode))
	serviceCode := strings.ToLower(strings.TrimSpace(req.ServiceCode))
	if country == "" || serviceCode == "" {
		return nil, ErrSMSProviderUnavailable
	}
	var products map[string]struct {
		Category string  `json:"Category"`
		Qty      int     `json:"Qty"`
		Price    float64 `json:"Price"`
	}
	if err := p.request(ctx, http.MethodGet, "guest/products/"+url.PathEscape(country)+"/"+url.PathEscape(operator), nil, nil, &products); err != nil {
		return nil, err
	}
	product, ok := products[serviceCode]
	if !ok || product.Price <= 0 || product.Qty <= 0 {
		return nil, ErrSMSProviderUnavailable
	}
	wanted := "activation"
	if category := strings.ToLower(strings.TrimSpace(product.Category)); category != "" && category != wanted {
		return nil, ErrSMSProviderUnavailable
	}
	return &SMSProviderQuote{Cost: decimal.NewFromFloat(product.Price), Currency: "USD", Stock: product.Qty, ExpiresAt: time.Now().Add(30 * time.Second), EstimatedDeliverySeconds: 90}, nil
}

func (p *fiveSIMProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return p.buy(ctx, "activation", req)
}

func (p *fiveSIMProvider) buy(ctx context.Context, category string, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	var out struct {
		ID       any       `json:"id"`
		Phone    string    `json:"phone"`
		Expires  time.Time `json:"expires"`
		Price    float64   `json:"price"`
		Operator string    `json:"operator"`
	}
	operator := strings.ToLower(strings.TrimSpace(req.OperatorCode))
	if operator == "" {
		operator = "any"
	}
	path := "user/buy/" + category + "/" + url.PathEscape(strings.ToLower(req.CountryCode)) + "/" + url.PathEscape(operator) + "/" + url.PathEscape(strings.ToLower(req.ServiceCode))
	query := url.Values{}
	if category == "activation" && req.VoiceMode == 2 {
		query.Set("voice", "1")
	}
	if operator == "any" && req.ProviderCostLimit > 0 {
		query.Set("maxPrice", strconv.FormatFloat(req.ProviderCostLimit, 'f', -1, 64))
	}
	if err := p.request(ctx, http.MethodGet, path, query, nil, &out); err != nil {
		return nil, err
	}
	id := fmt.Sprint(out.ID)
	if id == "" || id == "<nil>" {
		return nil, errors.New("5SIM returned no order id")
	}
	return &SMSPurchaseResult{ProviderOrderID: id, PhoneNumber: out.Phone, ExpiresAt: &out.Expires, ProviderCost: out.Price, ProviderOperatorCode: strings.ToLower(strings.TrimSpace(out.Operator))}, nil
}

func (p *fiveSIMProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	var out struct {
		Status   string  `json:"status"`
		Phone    string  `json:"phone"`
		Price    float64 `json:"price"`
		Operator string  `json:"operator"`
		SMS      []struct {
			Code string `json:"code"`
			Text string `json:"text"`
		} `json:"sms"`
	}
	if err := p.request(ctx, http.MethodGet, "user/check/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	msgs := make([]string, 0, len(out.SMS)*2)
	for _, m := range out.SMS {
		if strings.TrimSpace(m.Text) != "" {
			msgs = append(msgs, m.Text)
		}
		if strings.TrimSpace(m.Code) != "" {
			msgs = append(msgs, m.Code)
		}
	}
	return &SMSStatusResult{Status: strings.ToLower(out.Status), PhoneNumber: out.Phone, Messages: msgs, ProviderCost: out.Price, ProviderOperatorCode: strings.ToLower(strings.TrimSpace(out.Operator))}, nil
}
func (p *fiveSIMProvider) CancelTemporary(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodGet, "user/cancel/"+url.PathEscape(id), nil, nil, nil)
}
func (p *fiveSIMProvider) RequestTemporaryRefund(ctx context.Context, id string) error {
	// For 5SIM the cancellation endpoint is also the refund-producing operation.
	return p.CancelTemporary(ctx, id)
}
func (p *fiveSIMProvider) FinishTemporary(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodGet, "user/finish/"+url.PathEscape(id), nil, nil, nil)
}
func (p *fiveSIMProvider) BanTemporary(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodGet, "user/ban/"+url.PathEscape(id), nil, nil, nil)
}

func (p *fiveSIMProvider) Operators(ctx context.Context, countryCode, serviceCode string, voiceMode int) ([]SMSOperatorOption, error) {
	return p.OperatorsForProduct(ctx, countryCode, serviceCode, "temporary", voiceMode, 0, "")
}
func (p *fiveSIMProvider) OperatorsForProduct(ctx context.Context, countryCode, serviceCode, productType string, _ int, _ int, _ string) ([]SMSOperatorOption, error) {
	if strings.EqualFold(strings.TrimSpace(productType), "rental") {
		return []SMSOperatorOption{}, nil
	}
	countryCode = strings.ToLower(strings.TrimSpace(countryCode))
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if countryCode == "" || serviceCode == "" {
		return nil, ErrSMSProviderUnavailable
	}
	out := []SMSOperatorOption{}
	var aggregate map[string]struct {
		Category string  `json:"Category"`
		Qty      int     `json:"Qty"`
		Price    float64 `json:"Price"`
	}
	if err := p.request(ctx, http.MethodGet, "guest/products/"+url.PathEscape(countryCode)+"/any", nil, nil, &aggregate); err == nil {
		if item, ok := aggregate[serviceCode]; ok {
			out = append(out, SMSOperatorOption{Code: "any", Name: "Any / 自动选择", Stock: item.Qty, ProviderCost: item.Price, Available: item.Qty > 0})
		}
	}
	var prices map[string]map[string]map[string]fiveSIMPricePoint
	if err := p.request(ctx, http.MethodGet, "guest/prices", url.Values{"country": {countryCode}, "product": {serviceCode}}, nil, &prices); err != nil {
		return nil, err
	}
	for code, point := range fiveSIMPriceOperators(prices, countryCode, serviceCode) {
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" || code == "any" {
			continue
		}
		out = append(out, SMSOperatorOption{Code: code, Name: code, Stock: point.Count, ProviderCost: point.Cost, ProviderRate: point.Rate, Available: point.Count > 0})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Code == "any" {
			return true
		}
		if out[j].Code == "any" {
			return false
		}
		if out[i].ProviderCost == out[j].ProviderCost {
			return out[i].Name < out[j].Name
		}
		return out[i].ProviderCost < out[j].ProviderCost
	})
	return out, nil
}

func (p *fiveSIMProvider) PurchaseRental(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return nil, ErrSMSProviderUnavailable
}
func (p *fiveSIMProvider) GetRentalStatus(context.Context, string) (*SMSStatusResult, error) {
	return nil, ErrSMSProviderUnavailable
}
func (p *fiveSIMProvider) ExtendRental(context.Context, string, int, string) error {
	return ErrSMSProviderUnavailable
}
func (p *fiveSIMProvider) CancelRental(context.Context, string) error {
	return ErrSMSProviderUnavailable
}

type providerJSONResponse struct {
	ID        string         `json:"id"`
	OrderID   string         `json:"order_id"`
	Phone     string         `json:"phone_number"`
	Cost      float64        `json:"cost"`
	Price     float64        `json:"price"`
	Stock     int            `json:"stock"`
	Available int            `json:"available"`
	Currency  string         `json:"currency"`
	ETA       int            `json:"estimated_delivery_seconds"`
	Status    string         `json:"status"`
	Messages  []string       `json:"messages"`
	Message   string         `json:"message"`
	Expires   *time.Time     `json:"expires_at"`
	Metadata  map[string]any `json:"metadata"`
}

func jsonNumber(raw json.RawMessage) (float64, bool) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return 0, false
	}
	if strings.HasPrefix(value, `"`) {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return 0, false
		}
		value = strings.TrimSpace(text)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func providerResponseHasError(values map[string]any) bool {
	for key, value := range values {
		name := strings.ToLower(strings.TrimSpace(key))
		switch typed := value.(type) {
		case bool:
			if name == "success" && !typed {
				return true
			}
		case float64:
			if name == "success" && typed == 0 {
				return true
			}
		case string:
			text := strings.ToLower(strings.TrimSpace(typed))
			if name == "success" && (text == "false" || text == "0") {
				return true
			}
			if strings.Contains(name, "error") || name == "response" || name == "status" {
				for _, marker := range []string{"error", "invalid", "wrong_key", "unauthorized", "failed", "denied"} {
					if strings.Contains(text, marker) {
						return true
					}
				}
			}
		}
	}
	return false
}

// providerJSONAdapter is shared transport/decoding code. Each concrete
// provider below owns its endpoint contract and capability boundary.
type providerJSONAdapter struct{ *httpSMSProvider }

func (p *providerJSONAdapter) quoteJSON(ctx context.Context, path string, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	var out providerJSONResponse
	if err := p.request(ctx, http.MethodPost, path, nil, req, &out); err != nil {
		return nil, err
	}
	cost := out.Cost
	if cost == 0 {
		cost = out.Price
	}
	stock := out.Stock
	if stock == 0 {
		stock = out.Available
	}
	if cost <= 0 || stock <= 0 {
		return nil, ErrSMSProviderUnavailable
	}
	return &SMSProviderQuote{Cost: decimal.NewFromFloat(cost), Currency: out.Currency, Stock: stock, ExpiresAt: time.Now().Add(30 * time.Second), EstimatedDeliverySeconds: out.ETA}, nil
}
func (p *providerJSONAdapter) purchaseJSON(ctx context.Context, path string, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	var out providerJSONResponse
	if err := p.request(ctx, http.MethodPost, path, nil, req, &out); err != nil {
		return nil, err
	}
	id := out.ID
	if id == "" {
		id = out.OrderID
	}
	if id == "" {
		return nil, errors.New("provider returned no order id")
	}
	return &SMSPurchaseResult{ProviderOrderID: id, PhoneNumber: out.Phone, ExpiresAt: out.Expires, Metadata: out.Metadata}, nil
}
func (p *providerJSONAdapter) statusJSON(ctx context.Context, path, id string) (*SMSStatusResult, error) {
	var out providerJSONResponse
	if err := p.request(ctx, http.MethodGet, fmt.Sprintf(path, url.PathEscape(id)), nil, nil, &out); err != nil {
		return nil, err
	}
	msgs := append([]string(nil), out.Messages...)
	if out.Message != "" {
		msgs = append(msgs, out.Message)
	}
	return &SMSStatusResult{Status: strings.ToLower(out.Status), PhoneNumber: out.Phone, Messages: msgs, Metadata: out.Metadata}, nil
}

type smsPoolProvider struct{ *providerJSONAdapter }

func (p *smsPoolProvider) Catalog(context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	return nil, nil, errors.New("SMSPool catalog endpoint is not configured")
}

func (p *smsPoolProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{Temporary: true, Polling: true, Cancel: true, Refund: true, ServiceSelection: true}
}
func (p *smsPoolProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	var out struct {
		Price     json.RawMessage `json:"price"`
		Cost      json.RawMessage `json:"cost"`
		Stock     int             `json:"stock"`
		Available int             `json:"available"`
		Success   *bool           `json:"success"`
		Error     string          `json:"error"`
		Message   string          `json:"message"`
	}
	query := url.Values{"country": {req.CountryCode}, "service": {req.ServiceCode}}
	if err := p.request(ctx, http.MethodGet, "/request/price", query, nil, &out); err != nil {
		return nil, err
	}
	if out.Success != nil && !*out.Success || strings.TrimSpace(out.Error) != "" {
		return nil, ErrSMSProviderUnavailable
	}
	cost, ok := jsonNumber(out.Price)
	if !ok {
		cost, ok = jsonNumber(out.Cost)
	}
	stock := out.Stock
	if stock == 0 {
		stock = out.Available
	}
	if !ok || cost <= 0 {
		return nil, ErrSMSProviderUnavailable
	}
	if stock <= 0 {
		stock = 1
	}
	return &SMSProviderQuote{Cost: decimal.NewFromFloat(cost), Currency: "USD", Stock: stock, ExpiresAt: time.Now().Add(30 * time.Second), EstimatedDeliverySeconds: 90}, nil
}
func (p *smsPoolProvider) TestConnection(ctx context.Context) error {
	var out map[string]any
	if err := p.request(ctx, http.MethodGet, "/request/balance", nil, nil, &out); err != nil {
		return err
	}
	if providerResponseHasError(out) {
		return errors.New("SMSPool credential was rejected")
	}
	return nil
}
func (p *smsPoolProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return p.purchaseJSON(ctx, "/purchase/sms", req)
}
func (p *smsPoolProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	return p.statusJSON(ctx, "/sms/status/%s", id)
}
func (p *smsPoolProvider) CancelTemporary(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodPost, fmt.Sprintf("/sms/cancel/%s", url.PathEscape(id)), nil, nil, nil)
}
func (p *smsPoolProvider) RequestTemporaryRefund(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodPost, fmt.Sprintf("/sms/refund/%s", url.PathEscape(id)), nil, nil, nil)
}
func (p *smsPoolProvider) PurchaseRental(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return nil, errors.New("rental is not supported by SMSPool")
}
func (p *smsPoolProvider) GetRentalStatus(context.Context, string) (*SMSStatusResult, error) {
	return nil, errors.New("rental is not supported by SMSPool")
}
func (p *smsPoolProvider) ExtendRental(context.Context, string, int, string) error {
	return errors.New("rental is not supported by SMSPool")
}
func (p *smsPoolProvider) CancelRental(context.Context, string) error {
	return errors.New("rental is not supported by SMSPool")
}

type smsActivateProvider struct{ *httpSMSProvider }

func (p *smsActivateProvider) Catalog(ctx context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	// SMS-Activate exposes a read-only service list. Country identifiers in its
	// pricing API are numeric and are therefore left to the admin mapping table;
	// returning services here still prevents presenting unsupported platforms.
	body, err := p.api(ctx, "getServicesList", url.Values{"action": {"getServicesList"}})
	if err != nil {
		return nil, nil, err
	}
	var payload struct {
		Services []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"services"`
	}
	if json.Unmarshal(body, &payload) != nil || len(payload.Services) == 0 {
		return nil, nil, errors.New("SMS-Activate returned no service catalog")
	}
	items := make([]SMSSvcCatalogItem, 0, len(payload.Services))
	for _, item := range payload.Services {
		if strings.TrimSpace(item.Code) == "" {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = item.Code
		}
		items = append(items, SMSSvcCatalogItem{Code: strings.ToLower(item.Code), Name: name})
	}
	if len(items) == 0 {
		return nil, nil, errors.New("SMS-Activate returned no usable service catalog")
	}
	return items, nil, nil
}

func (p *smsActivateProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{Temporary: true, Polling: true, Cancel: true, Refund: true, ServiceSelection: true}
}
func (p *smsActivateProvider) api(ctx context.Context, action string, params url.Values) ([]byte, error) {
	params.Set("api_key", p.apiKey)
	return p.requestBytes(ctx, http.MethodGet, "", params, nil)
}
func (p *smsActivateProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	b, err := p.api(ctx, "getPrices", url.Values{"action": {"getPrices"}, "country": {req.CountryCode}, "service": {req.ServiceCode}})
	if err != nil {
		return nil, err
	}
	var out map[string]map[string]struct {
		Cost  float64 `json:"cost"`
		Count int     `json:"count"`
	}
	if json.Unmarshal(b, &out) != nil {
		return nil, errors.New("SMS-Activate returned invalid prices")
	}
	for _, svc := range out {
		for _, v := range svc {
			return &SMSProviderQuote{Cost: decimal.NewFromFloat(v.Cost), Currency: "USD", Stock: v.Count, ExpiresAt: time.Now().Add(30 * time.Second), EstimatedDeliverySeconds: 90}, nil
		}
	}
	return nil, ErrSMSProviderUnavailable
}
func (p *smsActivateProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	b, err := p.api(ctx, "getNumberV2", url.Values{"action": {"getNumberV2"}, "country": {req.CountryCode}, "service": {req.ServiceCode}})
	if err != nil {
		return nil, err
	}
	var out struct {
		ActivationID int64  `json:"activationId"`
		Phone        string `json:"phoneNumber"`
	}
	if json.Unmarshal(b, &out) != nil || out.ActivationID == 0 {
		return nil, errors.New("SMS-Activate purchase failed")
	}
	return &SMSPurchaseResult{ProviderOrderID: strconv.FormatInt(out.ActivationID, 10), PhoneNumber: out.Phone, ExpiresAt: smsPtrTime(time.Now().Add(10 * time.Minute))}, nil
}
func (p *smsActivateProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	b, err := p.api(ctx, "getStatus", url.Values{"action": {"getStatus"}, "id": {id}})
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(b))
	if strings.HasPrefix(raw, "STATUS_OK:") {
		return &SMSStatusResult{Status: "completed", Messages: []string{strings.TrimPrefix(raw, "STATUS_OK:")}}, nil
	}
	switch raw {
	case "STATUS_WAIT_CODE":
		return &SMSStatusResult{Status: "waiting"}, nil
	case "STATUS_CANCEL", "STATUS_WAIT_RETRY":
		return &SMSStatusResult{Status: "cancelled"}, nil
	case "STATUS_FINISH":
		return &SMSStatusResult{Status: "completed"}, nil
	default:
		return &SMSStatusResult{Status: "unknown"}, nil
	}
}
func (p *smsActivateProvider) CancelTemporary(ctx context.Context, id string) error {
	_, err := p.api(ctx, "setStatus", url.Values{"action": {"setStatus"}, "status": {"8"}, "id": {id}})
	return err
}
func (p *smsActivateProvider) RequestTemporaryRefund(ctx context.Context, id string) error {
	return p.CancelTemporary(ctx, id)
}
func (p *smsActivateProvider) PurchaseRental(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return nil, errors.New("rental is not supported by SMS-Activate")
}
func (p *smsActivateProvider) GetRentalStatus(context.Context, string) (*SMSStatusResult, error) {
	return nil, errors.New("rental is not supported by SMS-Activate")
}
func (p *smsActivateProvider) ExtendRental(context.Context, string, int, string) error {
	return errors.New("rental is not supported by SMS-Activate")
}
func (p *smsActivateProvider) CancelRental(context.Context, string) error {
	return errors.New("rental is not supported by SMS-Activate")
}

type onlineSIMProvider struct{ *httpSMSProvider }

func (p *onlineSIMProvider) Catalog(context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	return nil, nil, errors.New("OnlineSIM catalog is not exposed by this adapter")
}

func (p *onlineSIMProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{Temporary: true, Rental: true, Polling: true, Cancel: true, ServiceSelection: true}
}
func (p *onlineSIMProvider) call(ctx context.Context, path string, query url.Values, out any) error {
	query.Set("apikey", p.apiKey)
	return p.request(ctx, http.MethodGet, path, query, nil, out)
}
func (p *onlineSIMProvider) TestConnection(ctx context.Context) error {
	var out map[string]any
	if err := p.call(ctx, "getBalance.php", url.Values{}, &out); err != nil {
		return err
	}
	if providerResponseHasError(out) {
		return errors.New("OnlineSIM credential was rejected")
	}
	return nil
}
func (p *onlineSIMProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	var out struct {
		Services map[string]struct {
			Cost  float64 `json:"price"`
			Count int     `json:"count"`
		} `json:"services"`
	}
	if err := p.call(ctx, "getPrices.php", url.Values{"country": {req.CountryCode}, "service": {req.ServiceCode}}, &out); err != nil {
		return nil, err
	}
	for _, v := range out.Services {
		return &SMSProviderQuote{Cost: decimal.NewFromFloat(v.Cost), Currency: "USD", Stock: v.Count, ExpiresAt: time.Now().Add(30 * time.Second), EstimatedDeliverySeconds: 120}, nil
	}
	return nil, ErrSMSProviderUnavailable
}
func (p *onlineSIMProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	var out struct {
		TZID  string `json:"tzid"`
		Phone string `json:"number"`
	}
	if err := p.call(ctx, "getNum.php", url.Values{"country": {req.CountryCode}, "service": {req.ServiceCode}}, &out); err != nil {
		return nil, err
	}
	if out.TZID == "" {
		return nil, errors.New("OnlineSIM purchase failed")
	}
	return &SMSPurchaseResult{ProviderOrderID: out.TZID, PhoneNumber: out.Phone}, nil
}
func (p *onlineSIMProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	var out struct {
		Response string `json:"response"`
		Message  string `json:"msg"`
		Code     string `json:"code"`
	}
	if err := p.call(ctx, "getState.php", url.Values{"tzid": {id}}, &out); err != nil {
		return nil, err
	}
	status := strings.ToLower(out.Response)
	if out.Code != "" {
		return &SMSStatusResult{Status: "completed", Messages: []string{out.Code, out.Message}}, nil
	}
	return &SMSStatusResult{Status: status, Messages: []string{out.Message}}, nil
}
func (p *onlineSIMProvider) CancelTemporary(ctx context.Context, id string) error {
	return p.call(ctx, "setOperationRevise.php", url.Values{"tzid": {id}, "status": {"revised"}}, nil)
}
func (p *onlineSIMProvider) RequestTemporaryRefund(context.Context, string) error {
	return errors.New("refund is not supported by OnlineSIM")
}
func (p *onlineSIMProvider) PurchaseRental(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	if req.DurationValue <= 0 || req.DurationUnit == "" {
		return nil, errors.New("rental duration is required")
	}
	return p.PurchaseTemporary(ctx, req)
}
func (p *onlineSIMProvider) GetRentalStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	return p.GetTemporaryStatus(ctx, id)
}
func (p *onlineSIMProvider) ExtendRental(context.Context, string, int, string) error {
	return errors.New("rental extension is not supported by OnlineSIM")
}
func (p *onlineSIMProvider) CancelRental(ctx context.Context, id string) error {
	return p.CancelTemporary(ctx, id)
}

type pingMeProvider struct{ *providerJSONAdapter }

func (p *pingMeProvider) Catalog(context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	return nil, nil, errors.New("PingMe catalog endpoint is not configured")
}

func (p *pingMeProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{Temporary: true, Rental: true, Polling: true, Cancel: true, Extend: true, ServiceSelection: true}
}
func (p *pingMeProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	return p.quoteJSON(ctx, "/v1/number/quote", req)
}
func (p *pingMeProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return p.purchaseJSON(ctx, "/v1/number/purchase", req)
}
func (p *pingMeProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	return p.statusJSON(ctx, "/v1/number/%s", id)
}
func (p *pingMeProvider) CancelTemporary(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodPost, fmt.Sprintf("/v1/number/%s/cancel", url.PathEscape(id)), nil, nil, nil)
}
func (p *pingMeProvider) RequestTemporaryRefund(context.Context, string) error {
	return errors.New("refund is not supported by PingMe")
}
func (p *pingMeProvider) PurchaseRental(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	if req.DurationValue <= 0 || req.DurationUnit == "" {
		return nil, errors.New("rental duration is required")
	}
	return p.purchaseJSON(ctx, "/v1/number/rental", req)
}
func (p *pingMeProvider) GetRentalStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	return p.statusJSON(ctx, "/v1/number/rental/%s", id)
}
func (p *pingMeProvider) ExtendRental(ctx context.Context, id string, value int, unit string) error {
	return p.request(ctx, http.MethodPost, fmt.Sprintf("/v1/number/rental/%s/extend", url.PathEscape(id)), nil, SMSPurchaseRequest{DurationValue: value, DurationUnit: unit}, nil)
}
func (p *pingMeProvider) CancelRental(ctx context.Context, id string) error {
	return p.CancelTemporary(ctx, id)
}

func smsPtrTime(v time.Time) *time.Time { return &v }

func providerFor(code, baseURL, apiKey string) SMSProvider {
	client := &http.Client{Timeout: 15 * time.Second}
	p := &httpSMSProvider{code: code, baseURL: baseURL, apiKey: apiKey, client: client, cap: SMSProviderCapabilities{Temporary: true, Polling: true, Cancel: true, Refund: true, ServiceSelection: true}}
	switch code {
	case "5sim":
		return &fiveSIMProvider{p}
	case "smspva":
		return &smsPVAProvider{httpSMSProvider: p}
	case "smspool":
		// Legacy/BETA provider kept for compatibility but not user-selectable.
		p.credentialQueryParam = "key"
		return &smsPoolProvider{providerJSONAdapter: &providerJSONAdapter{httpSMSProvider: p}}
	case "sms_activate":
		return &smsActivateProvider{httpSMSProvider: p}
	case "onlinesim":
		p.cap.Rental = true
		p.cap.Refund = false
		return &onlineSIMProvider{httpSMSProvider: p}
	case "pingme":
		p.cap.Rental = true
		p.cap.Refund = false
		return &pingMeProvider{providerJSONAdapter: &providerJSONAdapter{httpSMSProvider: p}}
	default:
		return nil
	}
}

type SMSService struct {
	db                   *sql.DB
	settings             *SettingService
	encryptor            SecretEncryptor
	catalogSyncMu        sync.Mutex
	catalogSyncLastCheck time.Time
}

var (
	smsProviderTestMu sync.Mutex
	smsProviderTests  = map[int64]time.Time{}
)

func NewSMSService(db *sql.DB, settings *SettingService, encryptor SecretEncryptor) *SMSService {
	return &SMSService{db: db, settings: settings, encryptor: encryptor}
}
func (s *SMSService) Enabled(ctx context.Context) bool {
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return false
	}
	v, err := s.settings.settingRepo.GetValue(ctx, SettingKeySMSServiceEnabled)
	return err == nil && strings.EqualFold(strings.TrimSpace(v), "true")
}
func (s *SMSService) SetEnabled(ctx context.Context, enabled bool) error {
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return errors.New("settings repository unavailable")
	}
	return s.settings.settingRepo.Set(ctx, SettingKeySMSServiceEnabled, strconv.FormatBool(enabled))
}

func (s *SMSService) GetPricingSettings(ctx context.Context) (SMSPricingSettings, error) {
	settings := defaultSMSPricingSettings()
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return settings, errors.New("settings repository unavailable")
	}
	raw, err := s.settings.settingRepo.GetValue(ctx, SettingKeySMSPricingSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return settings, nil
		}
		return settings, err
	}
	if strings.TrimSpace(raw) == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return defaultSMSPricingSettings(), errors.New("invalid SMS pricing settings")
	}
	if settings.CostMultiplier <= 0 {
		settings.CostMultiplier = 1
	}
	if settings.UnknownGradeMultiplier <= 0 {
		settings.UnknownGradeMultiplier = 1
	}
	if settings.TemporaryExpiryMinutes <= 0 {
		settings.TemporaryExpiryMinutes = 10
	}
	if settings.SelfServiceCancelAfterMinutes < 0 {
		settings.SelfServiceCancelAfterMinutes = 0
	}
	// Older settings rows predate the self-service cancellation policy. Use the
	// safe default for those rows while still allowing administrators to save 0
	// explicitly when immediate cancellation is supported by their channels.
	if settings.SelfServiceCancelAfterMinutes == 0 && !strings.Contains(raw, "self_service_cancel_after_minutes") {
		settings.SelfServiceCancelAfterMinutes = defaultSMSPricingSettings().SelfServiceCancelAfterMinutes
	}
	if settings.GradeMultipliers == nil {
		settings.GradeMultipliers = map[string]float64{}
	}
	if settings.GradeFixedMarkups == nil {
		settings.GradeFixedMarkups = map[string]float64{}
	}
	normalizeSMSPricingGradeMaps(&settings)
	if len(settings.GradeMultipliers) == 0 && s.db != nil {
		rows, queryErr := s.db.QueryContext(ctx, `SELECT grade,multiplier,fixed_markup FROM sms_success_rate_rules WHERE enabled`)
		if queryErr == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var grade string
				var multiplier, fixed float64
				if rows.Scan(&grade, &multiplier, &fixed) == nil {
					grade = strings.ToUpper(strings.TrimSpace(grade))
					settings.GradeMultipliers[grade] = multiplier
					settings.GradeFixedMarkups[grade] = fixed
				}
			}
		}
	}
	return settings, nil
}

func (s *SMSService) SetPricingSettings(ctx context.Context, settings SMSPricingSettings) error {
	normalizeSMSPricingGradeMaps(&settings)
	if err := settings.Validate(); err != nil {
		return err
	}
	if settings.GradeMultipliers == nil {
		settings.GradeMultipliers = map[string]float64{}
	}
	if settings.GradeFixedMarkups == nil {
		settings.GradeFixedMarkups = map[string]float64{}
	}
	b, _ := json.Marshal(settings)
	return s.settings.settingRepo.Set(ctx, SettingKeySMSPricingSettings, string(b))
}

type SMSPublicChannel struct {
	Code                     string                  `json:"channel_code"`
	PublicName               string                  `json:"public_name"`
	Role                     string                  `json:"channel_role"`
	SalePrice                float64                 `json:"sale_price"`
	Stock                    int                     `json:"stock"`
	SuccessRate              *float64                `json:"success_rate,omitempty"`
	SuccessRateGrade         string                  `json:"success_rate_grade,omitempty"`
	SuccessRateSource        string                  `json:"success_rate_source"`
	EstimatedDeliverySeconds int                     `json:"estimated_delivery_seconds"`
	Capabilities             SMSProviderCapabilities `json:"capabilities"`
	QuoteID                  string                  `json:"quote_id"`
	QuoteExpiresAt           time.Time               `json:"quote_expires_at"`
	ProviderCost             float64                 `json:"-"`
	GradeMultiplier          float64                 `json:"-"`
	GradeFixedMarkup         float64                 `json:"-"`
	ProviderServiceCode      string                  `json:"-"`
	ProviderCountryCode      string                  `json:"-"`
}
type SMSOrder struct {
	ID                string                  `json:"id"`
	ProductType       string                  `json:"product_type"`
	Status            string                  `json:"status"`
	ChannelCode       string                  `json:"channel_code"`
	ChannelName       string                  `json:"channel_name"`
	ServiceCode       string                  `json:"service_code"`
	CountryCode       string                  `json:"country_code"`
	PhoneNumber       string                  `json:"phone_number,omitempty"`
	OperatorCode      string                  `json:"operator_code,omitempty"`
	VoiceMode         int                     `json:"voice_mode,omitempty"`
	Price             float64                 `json:"price"`
	SuccessRate       *float64                `json:"success_rate,omitempty"`
	SuccessRateGrade  string                  `json:"success_rate_grade,omitempty"`
	SuccessRateSource string                  `json:"success_rate_source"`
	RefundStatus      string                  `json:"refund_status"`
	RefundReason      string                  `json:"refund_reason,omitempty"`
	Capabilities      SMSProviderCapabilities `json:"capabilities"`
	Messages          []SMSMessage            `json:"messages,omitempty"`
	ExpiresAt         *time.Time              `json:"expires_at,omitempty"`
	RemainingSeconds  int64                   `json:"remaining_seconds"`
	CreatedAt         time.Time               `json:"created_at"`
}

func setSMSOrderRemaining(order *SMSOrder) {
	if order == nil || order.ExpiresAt == nil {
		return
	}
	if order.Status != "active" && order.Status != "provider_unknown" && order.Status != "reconciling" && order.Status != "pending" {
		order.RemainingSeconds = 0
		return
	}
	remaining := int64(time.Until(*order.ExpiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	order.RemainingSeconds = remaining
}

type SMSOrderPage struct {
	Items    []SMSOrder `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Pages    int        `json:"pages"`
}

type SMSRecentSuccessItem struct {
	Username    string `json:"username"`
	CountryCode string `json:"country_code"`
	Phone       string `json:"phone"`
}

type SMSRecentSuccessFeed struct {
	Source           string                 `json:"source"`
	RealSuccessCount int64                  `json:"real_success_count"`
	Items            []SMSRecentSuccessItem `json:"items"`
}

func maskSMSFeedIdentity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "user***"
	}
	if at := strings.Index(value, "@"); at > 0 {
		value = value[:at]
	}
	runes := []rune(value)
	if len(runes) <= 2 {
		return string(runes[:1]) + "***"
	}
	return string(runes[:2]) + "***"
}

func maskSMSFeedPhone(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 7 {
		return "***"
	}
	return string(runes[:4]) + "****" + string(runes[len(runes)-3:])
}

func mockSMSRecentSuccesses() []SMSRecentSuccessItem {
	return []SMSRecentSuccessItem{
		{Username: "al***", CountryCode: "US", Phone: "+120****728"},
		{Username: "mi***", CountryCode: "GB", Phone: "+447****391"},
		{Username: "sa***", CountryCode: "DE", Phone: "+491****526"},
		{Username: "ke***", CountryCode: "CA", Phone: "+160****844"},
		{Username: "yu***", CountryCode: "JP", Phone: "+819****317"},
		{Username: "an***", CountryCode: "AR", Phone: "+549****682"},
		{Username: "ro***", CountryCode: "BR", Phone: "+551****405"},
		{Username: "le***", CountryCode: "FR", Phone: "+336****971"},
		{Username: "ch***", CountryCode: "AU", Phone: "+614****238"},
		{Username: "no***", CountryCode: "NL", Phone: "+316****114"},
		{Username: "ma***", CountryCode: "ES", Phone: "+346****559"},
		{Username: "ha***", CountryCode: "SG", Phone: "+658****620"},
	}
}

func (s *SMSService) RecentSuccesses(ctx context.Context) (*SMSRecentSuccessFeed, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sms_orders WHERE status='completed' AND phone_number<>''`).Scan(&total); err != nil {
		return nil, err
	}
	if total <= 50 {
		return &SMSRecentSuccessFeed{Source: "mock", RealSuccessCount: total, Items: mockSMSRecentSuccesses()}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(NULLIF(u.username,''),u.email),co.iso2,o.phone_number
		FROM sms_orders o
		JOIN users u ON u.id=o.user_id
		JOIN sms_countries co ON co.id=o.country_id
		WHERE o.status='completed' AND o.phone_number<>''
		ORDER BY o.updated_at DESC
		LIMIT 30`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]SMSRecentSuccessItem, 0, 30)
	for rows.Next() {
		var username, countryCode, phone string
		if err := rows.Scan(&username, &countryCode, &phone); err != nil {
			return nil, err
		}
		items = append(items, SMSRecentSuccessItem{
			Username:    maskSMSFeedIdentity(username),
			CountryCode: strings.ToUpper(strings.TrimSpace(countryCode)),
			Phone:       maskSMSFeedPhone(phone),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &SMSRecentSuccessFeed{Source: "mock", RealSuccessCount: total, Items: mockSMSRecentSuccesses()}, nil
	}
	return &SMSRecentSuccessFeed{Source: "real", RealSuccessCount: total, Items: items}, nil
}

type SMSSvcCatalogItem struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Icon          string  `json:"icon,omitempty"`
	Category      string  `json:"category,omitempty"`
	Description   string  `json:"description,omitempty"`
	ProviderCode  string  `json:"provider_code,omitempty"`
	Stock         int     `json:"stock,omitempty"`
	ProviderCost  float64 `json:"-"`
	StartingPrice float64 `json:"starting_price,omitempty"`
	Available     bool    `json:"available"`
}
type SMSPublicProvider struct {
	Code         string                  `json:"code"`
	Name         string                  `json:"name"`
	Beta         bool                    `json:"beta"`
	Selectable   bool                    `json:"selectable"`
	Capabilities SMSProviderCapabilities `json:"capabilities"`
}
type SMSCountryCatalogItem struct {
	ISO2           string  `json:"iso2"`
	ISO3           string  `json:"iso3,omitempty"`
	CallingCode    string  `json:"calling_code,omitempty"`
	NameZH         string  `json:"name_zh,omitempty"`
	NameEN         string  `json:"name_en,omitempty"`
	ProviderCode   string  `json:"provider_code,omitempty"`
	Stock          int     `json:"stock,omitempty"`
	ProviderCost   float64 `json:"-"`
	StartingPrice  float64 `json:"starting_price,omitempty"`
	ConversionRate float64 `json:"conversion_rate,omitempty"`
	Available      bool    `json:"available"`
}

func (s *SMSService) ListServices(ctx context.Context) ([]SMSSvcCatalogItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT s.code,s.name,s.icon,s.category,s.description FROM sms_services s JOIN sms_provider_service_mappings m ON m.service_id=s.id AND m.enabled AND (m.temporary_supported OR m.rental_supported) JOIN sms_channels c ON c.provider_id=m.provider_id AND c.enabled AND c.visible AND c.healthy JOIN sms_providers p ON p.id=m.provider_id AND p.enabled WHERE s.enabled ORDER BY s.code`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSSvcCatalogItem{}
	for rows.Next() {
		var item SMSSvcCatalogItem
		if err := rows.Scan(&item.Code, &item.Name, &item.Icon, &item.Category, &item.Description); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SMSService) ListPublicProviders(ctx context.Context) ([]SMSPublicProvider, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.code,p.name,p.base_url,p.enabled,p.health_status,p.credential_ref,p.capabilities,EXISTS(SELECT 1 FROM sms_channels c WHERE c.provider_id=p.id AND c.enabled AND c.visible AND c.healthy) FROM sms_providers p ORDER BY CASE p.code WHEN '5sim' THEN 1 WHEN 'smspva' THEN 2 ELSE 100 END,p.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSPublicProvider{}
	for rows.Next() {
		var item SMSPublicProvider
		var enabled, channelReady bool
		var baseURL, healthStatus, credentialRef string
		var raw []byte
		if err := rows.Scan(&item.Code, &item.Name, &baseURL, &enabled, &healthStatus, &credentialRef, &raw, &channelReady); err != nil {
			return nil, err
		}
		item.Beta = item.Code != "5sim" && item.Code != "smspva"
		credentialReady := providerAPIKey(item.Code, credentialRef, s.encryptor) != ""
		item.Selectable = enabled && !item.Beta && strings.EqualFold(healthStatus, "healthy") && credentialReady && channelReady
		item.Capabilities = resolveSMSCapabilities(item.Code, baseURL, raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SMSService) ProviderServices(ctx context.Context, providerCode string) ([]SMSSvcCatalogItem, error) {
	return s.ProviderServicesForProduct(ctx, providerCode, "temporary", 0, "")
}

func (s *SMSService) decorateServiceStartingPrices(ctx context.Context, items []SMSSvcCatalogItem) []SMSSvcCatalogItem {
	pricing, err := s.GetPricingSettings(ctx)
	if err != nil {
		return items
	}
	for i := range items {
		if items[i].ProviderCost <= 0 {
			continue
		}
		price := decimal.NewFromFloat(items[i].ProviderCost).
			Mul(decimal.NewFromFloat(pricing.CostMultiplier)).
			Mul(decimal.NewFromFloat(pricing.UnknownGradeMultiplier)).
			Add(decimal.NewFromFloat(pricing.UnknownGradeFixedMarkup + pricing.FixedMarkup))
		items[i].StartingPrice = quantize(price)
	}
	return items
}

func (s *SMSService) decorateCountryStartingPrices(ctx context.Context, items []SMSCountryCatalogItem) []SMSCountryCatalogItem {
	pricing, err := s.GetPricingSettings(ctx)
	if err != nil {
		return items
	}
	for i := range items {
		if items[i].ProviderCost <= 0 {
			continue
		}
		price := decimal.NewFromFloat(items[i].ProviderCost).
			Mul(decimal.NewFromFloat(pricing.CostMultiplier)).
			Mul(decimal.NewFromFloat(pricing.UnknownGradeMultiplier)).
			Add(decimal.NewFromFloat(pricing.UnknownGradeFixedMarkup + pricing.FixedMarkup))
		items[i].StartingPrice = quantize(price)
	}
	return items
}

func (s *SMSService) ProviderServicesForProduct(ctx context.Context, providerCode, productType string, durationValue int, durationUnit string) ([]SMSSvcCatalogItem, error) {
	providerCode = strings.ToLower(strings.TrimSpace(providerCode))
	productType = strings.ToLower(strings.TrimSpace(productType))
	if productType == "" {
		productType = "temporary"
	}
	if providerCode == "5sim" && productType == "rental" {
		return []SMSSvcCatalogItem{}, nil
	}
	if providerCode == "5sim" || providerCode == "smspva" {
		cacheKey := smsProviderCatalogCacheKey(providerCode, productType, durationValue, durationUnit)
		if items, ok := cachedProviderServices(cacheKey); ok {
			return s.decorateServiceStartingPrices(ctx, items), nil
		}
		var base, credential string
		if err := s.db.QueryRowContext(ctx, `SELECT base_url,credential_ref FROM sms_providers WHERE code=$1 AND enabled`, providerCode).Scan(&base, &credential); err != nil {
			return nil, err
		}
		provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
		if productCatalog, ok := provider.(SMSProductServiceCatalogProvider); ok {
			items, err := productCatalog.CatalogServicesForProduct(ctx, productType, durationValue, durationUnit)
			if err == nil && len(items) > 0 {
				persisted, persistErr := s.persistProviderServices(ctx, providerCode, items)
				if persistErr != nil {
					return nil, persistErr
				}
				cacheProviderServices(cacheKey, persisted)
				return s.decorateServiceStartingPrices(ctx, persisted), nil
			}
		}
	}
	rows, snapshotErr := s.db.QueryContext(ctx, `SELECT c.provider_service_code,c.provider_service_name,c.category FROM sms_provider_catalog_services c JOIN sms_providers p ON p.id=c.provider_id WHERE p.code=$1 AND p.enabled AND c.enabled AND (($2='rental' AND lower(c.category)='rental') OR ($2<>'rental' AND lower(c.category)<>'rental')) ORDER BY c.provider_service_code`, providerCode, productType)
	if snapshotErr == nil {
		defer func() { _ = rows.Close() }()
		items := make([]SMSSvcCatalogItem, 0)
		for rows.Next() {
			var item SMSSvcCatalogItem
			if scanErr := rows.Scan(&item.Code, &item.Name, &item.Category); scanErr != nil {
				return nil, scanErr
			}
			item.ProviderCode = item.Code
			items = append(items, item)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		if len(items) > 0 {
			return s.decorateServiceStartingPrices(ctx, items), nil
		}
	}
	var base, credential string
	if err := s.db.QueryRowContext(ctx, `SELECT base_url,credential_ref FROM sms_providers WHERE code=$1 AND enabled`, providerCode).Scan(&base, &credential); err != nil {
		return nil, err
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	if productCatalog, ok := provider.(SMSProductServiceCatalogProvider); ok {
		items, err := productCatalog.CatalogServicesForProduct(ctx, productType, durationValue, durationUnit)
		if err == nil && len(items) > 0 {
			persisted, persistErr := s.persistProviderServices(ctx, providerCode, items)
			if persistErr != nil {
				return nil, persistErr
			}
			return s.decorateServiceStartingPrices(ctx, persisted), nil
		}
		if err != nil && productType == "rental" {
			return nil, err
		}
	}
	if catalog, ok := provider.(SMSCatalogProvider); ok {
		_, countries, err := catalog.Catalog(ctx)
		if err != nil {
			return nil, err
		}
		if serviceCatalog, ok := provider.(SMSServiceCatalogProvider); ok {
			items, err := serviceCatalog.CatalogServices(ctx, countries)
			if err != nil {
				return nil, err
			}
			persisted, persistErr := s.persistProviderServices(ctx, providerCode, items)
			if persistErr != nil {
				return nil, persistErr
			}
			return s.decorateServiceStartingPrices(ctx, persisted), nil
		}
	}
	return s.ListServices(ctx)
}

func (s *SMSService) persistProviderServices(ctx context.Context, providerCode string, items []SMSSvcCatalogItem) ([]SMSSvcCatalogItem, error) {
	var providerID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM sms_providers WHERE code=$1`, providerCode).Scan(&providerID); err != nil {
		return nil, err
	}
	for _, item := range items {
		item.Code = strings.ToLower(strings.TrimSpace(item.Code))
		if item.Code == "" {
			continue
		}
		providerServiceCode := strings.TrimSpace(item.ProviderCode)
		if providerServiceCode == "" {
			providerServiceCode = item.Code
		}
		if item.Name == "" {
			item.Name = item.Code
		}
		_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_provider_catalog_services(provider_id,provider_service_code,provider_service_name,category,enabled,observed_at) VALUES($1,$2,$3,$4,TRUE,NOW()) ON CONFLICT(provider_id,provider_service_code) DO UPDATE SET provider_service_name=EXCLUDED.provider_service_name,category=EXCLUDED.category,enabled=TRUE,observed_at=NOW()`, providerID, providerServiceCode, item.Name, item.Category)
		var serviceID int64
		if s.db.QueryRowContext(ctx, `INSERT INTO sms_services(code,name,category,enabled) VALUES ($1,$2,$3,TRUE) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,category=EXCLUDED.category,enabled=TRUE RETURNING id`, item.Code, item.Name, item.Category).Scan(&serviceID) == nil && providerCode != "5sim" && providerCode != "smspva" {
			_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_provider_service_mappings(provider_id,service_id,provider_service_code,provider_service_name,temporary_supported,rental_supported,enabled) VALUES ($1,$2,$3,$4,TRUE,FALSE,TRUE) ON CONFLICT (provider_id,service_id) DO UPDATE SET provider_service_code=EXCLUDED.provider_service_code,provider_service_name=EXCLUDED.provider_service_name,enabled=TRUE`, providerID, serviceID, providerServiceCode, item.Name)
		}
	}
	return items, nil
}

func (s *SMSService) ProviderCatalog(ctx context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	smsCatalogCache.RLock()
	if time.Now().Before(smsCatalogCache.expiresAt) {
		services := append([]SMSSvcCatalogItem(nil), smsCatalogCache.services...)
		countries := append([]SMSCountryCatalogItem(nil), smsCatalogCache.countries...)
		smsCatalogCache.RUnlock()
		return services, countries, nil
	}
	smsCatalogCache.RUnlock()
	// A live provider catalog is the source of provider capability. Internal
	// mappings remain a fail-closed purchase allow-list for providers without a
	// catalog endpoint, while providers with a live catalog can expose all
	// observed provider codes.
	fallbackServices, serviceErr := s.ListServices(ctx)
	fallbackCountries, countryErr := s.ListCountries(ctx)
	if serviceErr != nil {
		return nil, nil, serviceErr
	}
	if countryErr != nil {
		return nil, nil, countryErr
	}
	allowedServices := make(map[string]SMSSvcCatalogItem, len(fallbackServices))
	for _, item := range fallbackServices {
		item.Code = strings.ToLower(strings.TrimSpace(item.Code))
		allowedServices[item.Code] = item
	}
	allowedCountries := make(map[string]SMSCountryCatalogItem, len(fallbackCountries))
	for _, item := range fallbackCountries {
		item.ISO2 = strings.ToUpper(strings.TrimSpace(item.ISO2))
		allowedCountries[item.ISO2] = item
	}
	rows, err := s.db.QueryContext(ctx, `SELECT code,base_url,credential_ref FROM sms_providers WHERE enabled ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	// Administrator mappings remain visible as the fallback for providers that
	// do not expose a catalog endpoint.
	services := make(map[string]SMSSvcCatalogItem, len(fallbackServices))
	for _, item := range fallbackServices {
		code := strings.ToLower(strings.TrimSpace(item.Code))
		if code != "" {
			item.Code = code
			services[code] = item
		}
	}
	countries := make(map[string]SMSCountryCatalogItem, len(fallbackCountries))
	for _, item := range fallbackCountries {
		iso2 := strings.ToUpper(strings.TrimSpace(item.ISO2))
		if iso2 != "" {
			item.ISO2 = iso2
			countries[iso2] = item
		}
	}
	for rows.Next() {
		var code, base, credential string
		if err := rows.Scan(&code, &base, &credential); err != nil {
			return nil, nil, err
		}
		provider := providerFor(code, base, providerAPIKey(code, credential, s.encryptor))
		if catalog, ok := provider.(SMSCatalogProvider); ok {
			items, regions, catalogErr := catalog.Catalog(ctx)
			if catalogErr != nil {
				continue
			}
			if serviceCatalog, ok := provider.(SMSServiceCatalogProvider); ok {
				if dynamicItems, err := serviceCatalog.CatalogServices(ctx, regions); err == nil {
					items = append(items, dynamicItems...)
				}
			}
			for _, item := range items {
				code := strings.ToLower(strings.TrimSpace(item.Code))
				if item.ProviderCode != "" || allowedServices[code].Code != "" {
					item.Code = code
					services[code] = item
				}
			}
			for _, item := range regions {
				iso2 := strings.ToUpper(strings.TrimSpace(item.ISO2))
				if item.ProviderCode != "" || allowedCountries[iso2].ISO2 != "" {
					item.ISO2 = iso2
					countries[iso2] = item
				}
			}
		}
	}
	serviceList := make([]SMSSvcCatalogItem, 0, len(services))
	for _, item := range services {
		serviceList = append(serviceList, item)
	}
	countryList := make([]SMSCountryCatalogItem, 0, len(countries))
	for _, item := range countries {
		countryList = append(countryList, item)
	}
	sort.Slice(serviceList, func(i, j int) bool { return serviceList[i].Code < serviceList[j].Code })
	sort.Slice(countryList, func(i, j int) bool { return countryList[i].ISO2 < countryList[j].ISO2 })
	smsCatalogCache.Lock()
	smsCatalogCache.services = append([]SMSSvcCatalogItem(nil), serviceList...)
	smsCatalogCache.countries = append([]SMSCountryCatalogItem(nil), countryList...)
	smsCatalogCache.expiresAt = time.Now().Add(5 * time.Minute)
	smsCatalogCache.Unlock()
	return serviceList, countryList, nil
}

func (s *SMSService) CountriesForService(ctx context.Context, serviceCode string) ([]SMSCountryCatalogItem, error) {
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if serviceCode == "" {
		return s.ListCountries(ctx)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT code,base_url,credential_ref FROM sms_providers WHERE enabled ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var code, base, credential string
		if err := rows.Scan(&code, &base, &credential); err != nil {
			return nil, err
		}
		provider := providerFor(code, base, providerAPIKey(code, credential, s.encryptor))
		if p, ok := provider.(SMSServiceCountryProvider); ok {
			if out, e := p.CountriesForService(ctx, serviceCode); e == nil && len(out) > 0 {
				var providerID int64
				if s.db.QueryRowContext(ctx, `SELECT id FROM sms_providers WHERE code=$1`, code).Scan(&providerID) == nil {
					for _, country := range out {
						var countryID int64
						if s.db.QueryRowContext(ctx, `INSERT INTO sms_countries(iso2,name_en,enabled) VALUES ($1,$2,TRUE) ON CONFLICT (iso2) DO UPDATE SET name_en=COALESCE(NULLIF(EXCLUDED.name_en,''),sms_countries.name_en) RETURNING id`, country.ISO2, country.NameEN).Scan(&countryID) == nil {
							_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_provider_country_mappings(provider_id,country_id,provider_country_id,provider_country_code) VALUES ($1,$2,$3,$4) ON CONFLICT (provider_id,country_id) DO UPDATE SET provider_country_id=EXCLUDED.provider_country_id,provider_country_code=EXCLUDED.provider_country_code`, providerID, countryID, country.ProviderCode, country.ProviderCode)
						}
					}
				}
				return out, nil
			}
		}
	}
	return []SMSCountryCatalogItem{}, nil
}
func (s *SMSService) CountriesForProviderService(ctx context.Context, providerCode, serviceCode string) ([]SMSCountryCatalogItem, error) {
	return s.CountriesForProviderServiceProduct(ctx, providerCode, serviceCode, "temporary", 0, "")
}

func (s *SMSService) CountriesForProviderServiceProduct(ctx context.Context, providerCode, serviceCode, productType string, durationValue int, durationUnit string) ([]SMSCountryCatalogItem, error) {
	providerCode = strings.ToLower(strings.TrimSpace(providerCode))
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	productType = strings.ToLower(strings.TrimSpace(productType))
	if productType == "" {
		productType = "temporary"
	}
	if providerCode == "5sim" && productType == "rental" {
		return []SMSCountryCatalogItem{}, nil
	}
	cacheKey := smsProviderCountryCacheKey(providerCode, serviceCode, productType, durationValue, durationUnit)
	if providerCode == "5sim" || providerCode == "smspva" {
		if items, ok := cachedProviderCountries(cacheKey); ok {
			return s.decorateCountryStartingPrices(ctx, items), nil
		}
	}
	var base, credential string
	if err := s.db.QueryRowContext(ctx, `SELECT base_url,credential_ref FROM sms_providers WHERE code=$1 AND enabled`, providerCode).Scan(&base, &credential); err != nil {
		return nil, err
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	if p, ok := provider.(SMSProductServiceCountryProvider); ok {
		if items, err := p.CountriesForServiceProduct(ctx, serviceCode, productType, durationValue, durationUnit); err == nil {
			if providerCode == "5sim" || providerCode == "smspva" {
				cacheProviderCountries(cacheKey, items)
			}
			return s.decorateCountryStartingPrices(ctx, items), nil
		} else if productType == "rental" {
			return nil, err
		}
	}
	if p, ok := provider.(SMSServiceCountryProvider); ok {
		if items, err := p.CountriesForService(ctx, serviceCode); err == nil {
			// This live service-specific result is authoritative. A provider-wide
			// country snapshot cannot tell whether a particular app is available
			// in a country, so it must not override this list.
			if providerCode == "5sim" || providerCode == "smspva" {
				cacheProviderCountries(cacheKey, items)
			}
			return s.decorateCountryStartingPrices(ctx, items), nil
		}
	}

	// Fallback for legacy/BETA providers whose adapter cannot return a
	// service-specific country list.
	rows, snapshotErr := s.db.QueryContext(ctx, `SELECT c.iso2,c.provider_country_id,c.provider_country_code,c.name_zh,c.name_en FROM sms_provider_catalog_countries c JOIN sms_providers p ON p.id=c.provider_id WHERE p.code=$1 AND p.enabled AND c.enabled ORDER BY c.name_en`, providerCode)
	if snapshotErr != nil {
		return nil, snapshotErr
	}
	defer func() { _ = rows.Close() }()
	items := make([]SMSCountryCatalogItem, 0)
	for rows.Next() {
		var item SMSCountryCatalogItem
		var providerCountryID, providerCountryCode string
		if scanErr := rows.Scan(&item.ISO2, &providerCountryID, &providerCountryCode, &item.NameZH, &item.NameEN); scanErr != nil {
			return nil, scanErr
		}
		item.ProviderCode = providerCountryID
		if item.ProviderCode == "" {
			item.ProviderCode = providerCountryCode
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *SMSService) ListCountries(ctx context.Context) ([]SMSCountryCatalogItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT co.iso2,co.iso3,co.calling_code,co.name_zh,co.name_en FROM sms_countries co JOIN sms_provider_country_mappings m ON m.country_id=co.id JOIN sms_channels c ON c.provider_id=m.provider_id AND c.enabled AND c.visible AND c.healthy JOIN sms_providers p ON p.id=m.provider_id AND p.enabled WHERE co.enabled ORDER BY co.iso2`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSCountryCatalogItem{}
	for rows.Next() {
		var item SMSCountryCatalogItem
		if err := rows.Scan(&item.ISO2, &item.ISO3, &item.CallingCode, &item.NameZH, &item.NameEN); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SMSService) ensureProviderCatalogSelection(ctx context.Context, providerCode, serviceCode, countryCode, productType string, durationValue int, durationUnit string) error {
	providerCode = strings.ToLower(strings.TrimSpace(providerCode))
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	productType = strings.ToLower(strings.TrimSpace(productType))
	if providerCode == "5sim" && productType == "rental" {
		return ErrSMSProviderUnavailable
	}
	if providerCode == "" || serviceCode == "" || countryCode == "" {
		return nil
	}

	var providerID int64
	var rawCapabilities []byte
	var providerBaseURL, providerCredential string
	if err := s.db.QueryRowContext(ctx, `SELECT id,capabilities,base_url,credential_ref FROM sms_providers WHERE code=$1 AND enabled`, providerCode).Scan(&providerID, &rawCapabilities, &providerBaseURL, &providerCredential); err != nil {
		return err
	}
	capabilities := decodeCapabilities(rawCapabilities)
	rentalSupported := productType == "rental" && capabilities.Rental

	var providerServiceCode, providerServiceName, category string
	err := s.db.QueryRowContext(ctx, `SELECT provider_service_code,provider_service_name,category FROM sms_provider_catalog_services WHERE provider_id=$1 AND lower(provider_service_code)=lower($2) AND enabled ORDER BY observed_at DESC LIMIT 1`, providerID, serviceCode).Scan(&providerServiceCode, &providerServiceName, &category)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if providerServiceCode == "" {
		providerServiceCode = serviceCode
	}
	if providerServiceName == "" {
		providerServiceName = serviceCode
	}
	if category == "" {
		category = "other"
	}

	var serviceID int64
	if err := s.db.QueryRowContext(ctx, `INSERT INTO sms_services(code,name,category,enabled) VALUES($1,$2,$3,TRUE)
		ON CONFLICT(code) DO UPDATE SET name=CASE WHEN sms_services.name='' THEN EXCLUDED.name ELSE sms_services.name END, enabled=TRUE
		RETURNING id`, serviceCode, providerServiceName, category).Scan(&serviceID); err != nil {
		return err
	}
	if providerCode != "5sim" && providerCode != "smspva" {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO sms_provider_service_mappings(provider_id,service_id,provider_service_code,provider_service_name,temporary_supported,rental_supported,enabled)
			VALUES($1,$2,$3,$4,TRUE,$5,TRUE)
			ON CONFLICT(provider_id,service_id) DO UPDATE SET provider_service_code=EXCLUDED.provider_service_code,provider_service_name=EXCLUDED.provider_service_name,temporary_supported=TRUE,rental_supported=EXCLUDED.rental_supported,enabled=TRUE`,
			providerID, serviceID, providerServiceCode, providerServiceName, rentalSupported); err != nil {
			return err
		}
	}

	var providerCountryID, providerCountryCode, nameZH, nameEN string
	if providerCode == "5sim" || providerCode == "smspva" {
		provider := providerFor(providerCode, providerBaseURL, providerAPIKey(providerCode, providerCredential, s.encryptor))
		var liveCountries []SMSCountryCatalogItem
		var liveErr error
		if countryProvider, ok := provider.(SMSProductServiceCountryProvider); ok {
			liveCountries, liveErr = countryProvider.CountriesForServiceProduct(ctx, serviceCode, productType, durationValue, durationUnit)
		} else if countryProvider, ok := provider.(SMSServiceCountryProvider); ok {
			liveCountries, liveErr = countryProvider.CountriesForService(ctx, serviceCode)
		} else {
			liveErr = ErrSMSProviderUnavailable
		}
		if liveErr != nil {
			return liveErr
		}
		{
			for _, live := range liveCountries {
				if !strings.EqualFold(live.ISO2, countryCode) {
					continue
				}
				providerCountryID = strings.TrimSpace(live.ProviderCode)
				providerCountryCode = providerCountryID
				nameZH, nameEN = live.NameZH, live.NameEN
				if providerCountryID == "" {
					providerCountryID = countryCode
					providerCountryCode = countryCode
				}
				_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_provider_catalog_countries(provider_id,provider_country_id,provider_country_code,iso2,name_zh,name_en,enabled,observed_at) VALUES($1,$2,$3,$4,$5,$6,TRUE,NOW()) ON CONFLICT(provider_id,provider_country_id) DO UPDATE SET provider_country_code=EXCLUDED.provider_country_code,iso2=EXCLUDED.iso2,name_zh=EXCLUDED.name_zh,name_en=EXCLUDED.name_en,enabled=TRUE,observed_at=NOW()`, providerID, providerCountryID, providerCountryCode, countryCode, nameZH, nameEN)
				break
			}
			if providerCountryID == "" {
				return ErrSMSProviderUnavailable
			}
		}
	}
	if providerCountryID == "" {
		err = s.db.QueryRowContext(ctx, `SELECT provider_country_id,provider_country_code,name_zh,name_en FROM sms_provider_catalog_countries WHERE provider_id=$1 AND upper(iso2)=upper($2) AND enabled ORDER BY observed_at DESC LIMIT 1`, providerID, countryCode).Scan(&providerCountryID, &providerCountryCode, &nameZH, &nameEN)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	if providerCountryID == "" {
		providerCountryID = countryCode
	}
	if providerCountryCode == "" {
		providerCountryCode = countryCode
	}
	if nameEN == "" {
		nameEN = countryCode
	}

	var countryID int64
	if err := s.db.QueryRowContext(ctx, `INSERT INTO sms_countries(iso2,name_zh,name_en,enabled) VALUES($1,$2,$3,TRUE)
		ON CONFLICT(iso2) DO UPDATE SET name_zh=COALESCE(NULLIF(sms_countries.name_zh,''),EXCLUDED.name_zh),name_en=COALESCE(NULLIF(sms_countries.name_en,''),EXCLUDED.name_en),enabled=TRUE
		RETURNING id`, countryCode, nameZH, nameEN).Scan(&countryID); err != nil {
		return err
	}
	if providerCode != "5sim" && providerCode != "smspva" {
		_, err = s.db.ExecContext(ctx, `INSERT INTO sms_provider_country_mappings(provider_id,country_id,provider_country_id,provider_country_code)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(provider_id,country_id) DO UPDATE SET provider_country_id=EXCLUDED.provider_country_id,provider_country_code=EXCLUDED.provider_country_code`,
			providerID, countryID, providerCountryID, providerCountryCode)
		return err
	}
	return nil
}

func (s *SMSService) ProviderOperators(ctx context.Context, providerCode, serviceCode, countryCode string, voiceMode int) ([]SMSOperatorOption, error) {
	return s.ProviderOperatorsForProduct(ctx, providerCode, serviceCode, countryCode, "temporary", voiceMode, 0, "")
}

func (s *SMSService) ProviderOperatorsForProduct(ctx context.Context, providerCode, serviceCode, countryCode, productType string, voiceMode, durationValue int, durationUnit string) ([]SMSOperatorOption, error) {
	providerCode = strings.ToLower(strings.TrimSpace(providerCode))
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	productType = strings.ToLower(strings.TrimSpace(productType))
	if providerCode == "5sim" && productType == "rental" {
		return []SMSOperatorOption{}, nil
	}
	if voiceMode < 0 || voiceMode > 2 {
		return nil, errors.New("invalid voice mode")
	}
	if providerCode == "" || serviceCode == "" || countryCode == "" {
		return []SMSOperatorOption{}, nil
	}
	var base, credential string
	if err := s.db.QueryRowContext(ctx, `SELECT base_url,credential_ref FROM sms_providers WHERE code=$1 AND enabled`, providerCode).Scan(&base, &credential); err != nil {
		return nil, err
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	providerCountry := countryCode
	if countries, err := s.CountriesForProviderServiceProduct(ctx, providerCode, serviceCode, productType, durationValue, durationUnit); err == nil {
		for _, item := range countries {
			if strings.EqualFold(item.ISO2, countryCode) && strings.TrimSpace(item.ProviderCode) != "" {
				providerCountry = item.ProviderCode
				break
			}
		}
	}
	if p, ok := provider.(SMSProductOperatorProvider); ok {
		return p.OperatorsForProduct(ctx, providerCountry, serviceCode, productType, voiceMode, durationValue, durationUnit)
	}
	p, ok := provider.(SMSOperatorProvider)
	if !ok {
		return []SMSOperatorOption{}, nil
	}
	return p.Operators(ctx, providerCountry, serviceCode, voiceMode)
}

func (s *SMSService) Quote(ctx context.Context, userID int64, req SMSQuoteRequest) ([]SMSPublicChannel, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	req.ProviderCode = strings.ToLower(strings.TrimSpace(req.ProviderCode))
	req.ServiceCode = strings.ToLower(strings.TrimSpace(req.ServiceCode))
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	req.OperatorCode = strings.TrimSpace(req.OperatorCode)
	req.ProductType = strings.ToLower(strings.TrimSpace(req.ProductType))
	req.DurationUnit = strings.ToLower(strings.TrimSpace(req.DurationUnit))
	if req.ProviderCode != "" && req.ProviderCode != "5sim" && req.ProviderCode != "smspva" {
		return nil, ErrSMSProviderUnavailable
	}
	if req.OperatorCode == "" {
		req.OperatorCode = "any"
	}
	if req.VoiceMode < 0 || req.VoiceMode > 2 {
		return nil, errors.New("invalid voice mode")
	}
	if req.ProductType != "temporary" && req.ProductType != "rental" {
		return nil, errors.New("invalid product type")
	}
	if req.ProviderCode != "" {
		if err := s.ensureProviderCatalogSelection(ctx, req.ProviderCode, req.ServiceCode, req.CountryCode, req.ProductType, req.DurationValue, req.DurationUnit); err != nil {
			return nil, err
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,p.id,sv.id,co.id,c.code,c.public_name,c.role,p.code,p.base_url,p.credential_ref,
		COALESCE(NULLIF(cs.provider_service_code,''),NULLIF(psm.provider_service_code,''),sv.code),
		COALESCE(NULLIF(cc.provider_country_id,''),NULLIF(cc.provider_country_code,''),NULLIF(pcm.provider_country_id,''),NULLIF(pcm.provider_country_code,''),co.iso2),
		CASE WHEN p.code='smspva' THEN TRUE ELSE COALESCE((p.capabilities->>'supports_rental')::boolean,(p.capabilities->>'rental')::boolean,false) END
		FROM sms_channels c
		JOIN sms_providers p ON p.id=c.provider_id
		JOIN sms_services sv ON sv.code=$1 AND sv.enabled
		JOIN sms_countries co ON co.iso2=$2 AND co.enabled
		LEFT JOIN sms_provider_catalog_services cs ON cs.provider_id=p.id AND lower(cs.provider_service_code)=lower(sv.code) AND cs.enabled
		LEFT JOIN sms_provider_catalog_countries cc ON cc.provider_id=p.id AND upper(cc.iso2)=upper(co.iso2) AND cc.enabled
		LEFT JOIN sms_provider_service_mappings psm ON psm.provider_id=p.id AND psm.service_id=sv.id AND psm.enabled
		LEFT JOIN sms_provider_country_mappings pcm ON pcm.provider_id=p.id AND pcm.country_id=co.id
		WHERE c.enabled AND c.visible AND c.healthy AND p.enabled
		  AND ($4='' OR p.code=$4)
		  AND (
		    p.code IN ('5sim','smspva')
		    OR (
		      psm.provider_service_code IS NOT NULL
		      AND COALESCE(NULLIF(pcm.provider_country_id,''),NULLIF(pcm.provider_country_code,'')) IS NOT NULL
		      AND (($3='temporary' AND psm.temporary_supported) OR ($3='rental' AND psm.rental_supported))
		    )
		  )`, req.ServiceCode, req.CountryCode, req.ProductType, req.ProviderCode)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSPublicChannel{}
	for rows.Next() {
		var channelID, providerID, serviceID, countryID int64
		var code, name, role, pc, base, cred, providerServiceCode, providerCountryCode string
		var rental bool
		if err := rows.Scan(&channelID, &providerID, &serviceID, &countryID, &code, &name, &role, &pc, &base, &cred, &providerServiceCode, &providerCountryCode, &rental); err != nil {
			return nil, err
		}
		if req.ProductType == "rental" && !rental {
			continue
		}
		key := providerAPIKey(pc, cred, s.encryptor)
		p := providerFor(pc, base, key)
		if p == nil {
			continue
		}
		capabilities := p.Capabilities(ctx)
		if req.ProductType == "temporary" && !capabilities.Temporary {
			continue
		}
		if req.ProductType == "rental" && !capabilities.Rental {
			continue
		}
		providerReq := req
		providerReq.ServiceCode = providerServiceCode
		providerReq.CountryCode = providerCountryCode
		providerReq.DurationValue = req.DurationValue
		providerReq.DurationUnit = req.DurationUnit
		q, err := p.Quote(ctx, providerReq)
		if err != nil || q == nil || q.Stock <= 0 {
			continue
		}
		grade, rate := s.successGrade(ctx, code, req.ServiceCode, strings.ToUpper(req.CountryCode))
		pricing, pricingErr := s.GetPricingSettings(ctx)
		if pricingErr != nil {
			return nil, pricingErr
		}
		multiplier, fixed := s.gradePricing(ctx, grade, pricing)
		price := q.Cost.Mul(decimal.NewFromFloat(pricing.CostMultiplier)).Mul(decimal.NewFromFloat(multiplier)).Add(decimal.NewFromFloat(fixed))
		id := randomID()
		expiresAt := q.ExpiresAt
		if expiresAt.IsZero() || !expiresAt.After(time.Now()) {
			expiresAt = time.Now().Add(30 * time.Second)
		}
		providerCost := quantize(q.Cost)
		salePrice := quantize(price)
		if _, err := s.db.ExecContext(ctx, `INSERT INTO sms_quotes (id,user_id,channel_id,provider_id,service_id,country_id,product_type,provider_service_code,provider_country_code,operator_code,voice_mode,duration_value,duration_unit,provider_cost_snapshot,sale_price_snapshot,success_rate_snapshot,success_rate_source_snapshot,success_rate_grade_snapshot,success_rate_multiplier_snapshot,fixed_markup_snapshot,stock,estimated_delivery_seconds,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`, id, userID, channelID, providerID, serviceID, countryID, req.ProductType, providerServiceCode, providerCountryCode, req.OperatorCode, req.VoiceMode, req.DurationValue, req.DurationUnit, providerCost, salePrice, rate, rateSource(rate), grade, multiplier, fixed, q.Stock, q.EstimatedDeliverySeconds, expiresAt); err != nil {
			return nil, err
		}
		out = append(out, SMSPublicChannel{Code: code, PublicName: name, Role: role, SalePrice: salePrice, Stock: q.Stock, SuccessRate: rate, SuccessRateGrade: grade, SuccessRateSource: rateSource(rate), EstimatedDeliverySeconds: q.EstimatedDeliverySeconds, Capabilities: capabilities, QuoteID: id, QuoteExpiresAt: expiresAt, ProviderCost: providerCost, GradeMultiplier: multiplier, GradeFixedMarkup: fixed, ProviderServiceCode: providerServiceCode, ProviderCountryCode: providerCountryCode})
	}
	return out, rows.Err()
}

func quantize(v decimal.Decimal) float64 { x, _ := v.Round(8).Float64(); return x }
func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b)
}
func rateSource(v *float64) string {
	if v == nil {
		return "unavailable"
	}
	return "platform"
}
func (s *SMSService) successGrade(ctx context.Context, channel, service, country string) (string, *float64) {
	var rate sql.NullFloat64
	_ = s.db.QueryRowContext(ctx, `SELECT CASE WHEN COUNT(*) FILTER (WHERE status='completed')+COUNT(*) FILTER (WHERE status IN ('failed','expired')) >= 20 THEN COUNT(*) FILTER (WHERE status='completed')::float / NULLIF(COUNT(*) FILTER (WHERE status IN ('completed','failed','expired')),0) ELSE NULL END FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id WHERE c.code=$1 AND sv.code=$2 AND co.iso2=$3`, channel, service, country).Scan(&rate)
	if !rate.Valid {
		return "", nil
	}
	v := rate.Float64
	switch {
	case v >= .95:
		return "S", &v
	case v >= .90:
		return "A", &v
	case v >= .80:
		return "B", &v
	case v >= .60:
		return "C", &v
	default:
		return "D", &v
	}
}
func (s *SMSService) gradePricing(ctx context.Context, grade string, pricing SMSPricingSettings) (float64, float64) {
	var multiplier, fixed sql.NullFloat64
	if grade == "" {
		return pricing.UnknownGradeMultiplier, pricing.UnknownGradeFixedMarkup + pricing.FixedMarkup
	}
	if value, ok := pricing.GradeMultipliers[strings.ToUpper(grade)]; ok {
		return value, pricing.GradeFixedMarkups[strings.ToUpper(grade)] + pricing.FixedMarkup
	}
	if err := s.db.QueryRowContext(ctx, `SELECT multiplier,fixed_markup FROM sms_success_rate_rules WHERE grade=$1 AND enabled`, grade).Scan(&multiplier, &fixed); err != nil {
		return 1, pricing.FixedMarkup
	}
	if !multiplier.Valid {
		multiplier.Float64 = 1
	}
	if !fixed.Valid {
		fixed.Float64 = 0
	}
	return multiplier.Float64, fixed.Float64 + pricing.FixedMarkup
}

type smsQuoteRecord struct {
	ChannelID             int64
	ProviderID            int64
	ServiceID             int64
	CountryID             int64
	ChannelCode           string
	ChannelName           string
	ChannelRole           string
	ProviderCode          string
	ProviderBaseURL       string
	ProviderCredential    string
	ServiceCode           string
	CountryCode           string
	ProductType           string
	ProviderServiceCode   string
	ProviderCountryCode   string
	OperatorCode          string
	VoiceMode             int
	DurationValue         int
	DurationUnit          string
	ProviderCost          float64
	SalePrice             float64
	SuccessRate           sql.NullFloat64
	SuccessRateSource     string
	SuccessRateGrade      string
	SuccessRateMultiplier float64
	FixedMarkup           float64
	Stock                 int
	ExpiresAt             time.Time
}

func (s *SMSService) loadQuote(ctx context.Context, userID int64, quoteID string) (*smsQuoteRecord, error) {
	var quote smsQuoteRecord
	err := s.db.QueryRowContext(ctx, `SELECT q.channel_id,q.provider_id,q.service_id,q.country_id,c.code,c.public_name,c.role,p.code,p.base_url,p.credential_ref,sv.code,co.iso2,q.product_type,q.provider_service_code,q.provider_country_code,q.operator_code,q.voice_mode,q.duration_value,q.duration_unit,q.provider_cost_snapshot,q.sale_price_snapshot,q.success_rate_snapshot,q.success_rate_source_snapshot,q.success_rate_grade_snapshot,q.success_rate_multiplier_snapshot,q.fixed_markup_snapshot,q.stock,q.expires_at FROM sms_quotes q JOIN sms_channels c ON c.id=q.channel_id JOIN sms_providers p ON p.id=q.provider_id JOIN sms_services sv ON sv.id=q.service_id JOIN sms_countries co ON co.id=q.country_id WHERE q.id=$1 AND q.user_id=$2 AND q.consumed_at IS NULL`, strings.TrimSpace(quoteID), userID).Scan(&quote.ChannelID, &quote.ProviderID, &quote.ServiceID, &quote.CountryID, &quote.ChannelCode, &quote.ChannelName, &quote.ChannelRole, &quote.ProviderCode, &quote.ProviderBaseURL, &quote.ProviderCredential, &quote.ServiceCode, &quote.CountryCode, &quote.ProductType, &quote.ProviderServiceCode, &quote.ProviderCountryCode, &quote.OperatorCode, &quote.VoiceMode, &quote.DurationValue, &quote.DurationUnit, &quote.ProviderCost, &quote.SalePrice, &quote.SuccessRate, &quote.SuccessRateSource, &quote.SuccessRateGrade, &quote.SuccessRateMultiplier, &quote.FixedMarkup, &quote.Stock, &quote.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSMSQuoteInvalid
	}
	if err != nil {
		return nil, err
	}
	if !quote.ExpiresAt.After(time.Now()) {
		return nil, ErrSMSQuoteExpired
	}
	return &quote, nil
}

func (s *SMSService) Purchase(ctx context.Context, userID int64, req SMSPurchaseRequest, idempotencyKey string, expectedPrice *float64) (*SMSOrder, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, errors.New("Idempotency-Key is required")
	}
	if len(idempotencyKey) > 128 {
		return nil, errors.New("Idempotency-Key is too long")
	}
	req.ServiceCode = strings.ToLower(strings.TrimSpace(req.ServiceCode))
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	req.ProductType = strings.ToLower(strings.TrimSpace(req.ProductType))
	req.QuoteID = strings.TrimSpace(req.QuoteID)
	var existingID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM sms_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&existingID)
	if err == nil {
		return s.GetOrder(ctx, userID, existingID)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	if req.QuoteID == "" {
		return nil, ErrSMSQuoteInvalid
	}
	quote, quoteErr := s.loadQuote(ctx, userID, req.QuoteID)
	if quoteErr != nil {
		return nil, quoteErr
	}
	if quote.ProviderCode == "5sim" && quote.ProductType == "rental" {
		return nil, ErrSMSProviderUnavailable
	}
	if quote.ProviderCode != "5sim" && quote.ProviderCode != "smspva" {
		return nil, ErrSMSProviderUnavailable
	}
	if quote.ChannelCode != req.ChannelCode && strings.TrimSpace(req.ChannelCode) != "" {
		return nil, ErrSMSQuoteInvalid
	}
	if quote.ServiceCode != req.ServiceCode || quote.CountryCode != req.CountryCode || quote.ProductType != req.ProductType {
		return nil, ErrSMSQuoteInvalid
	}
	requestedOperator := strings.TrimSpace(req.OperatorCode)
	if requestedOperator == "" {
		requestedOperator = "any"
	}
	if quote.OperatorCode != requestedOperator || quote.VoiceMode != req.VoiceMode {
		return nil, ErrSMSQuoteInvalid
	}
	if quote.ProductType == "rental" && (quote.DurationValue != req.DurationValue || !strings.EqualFold(strings.TrimSpace(quote.DurationUnit), strings.TrimSpace(req.DurationUnit))) {
		return nil, ErrSMSQuoteInvalid
	}
	selected := &SMSPublicChannel{Code: quote.ChannelCode, PublicName: quote.ChannelName, Role: quote.ChannelRole, SalePrice: quote.SalePrice, SuccessRate: func() *float64 {
		if quote.SuccessRate.Valid {
			return &quote.SuccessRate.Float64
		}
		return nil
	}(), SuccessRateGrade: quote.SuccessRateGrade, SuccessRateSource: quote.SuccessRateSource, ProviderCost: quote.ProviderCost, GradeMultiplier: quote.SuccessRateMultiplier, GradeFixedMarkup: quote.FixedMarkup, ProviderServiceCode: quote.ProviderServiceCode, ProviderCountryCode: quote.ProviderCountryCode}
	if expectedPrice != nil && math.Abs(*expectedPrice-selected.SalePrice) > 0.00000001 {
		return nil, ErrSMSPriceChanged
	}
	var channelID, providerID, serviceID, countryID int64
	var base, providerCode, credential string
	err = s.db.QueryRowContext(ctx, `SELECT c.id,p.id,sv.id,co.id,p.code,p.base_url,p.credential_ref FROM sms_channels c JOIN sms_providers p ON p.id=c.provider_id JOIN sms_services sv ON sv.id=$1 JOIN sms_countries co ON co.id=$2 WHERE c.id=$3 AND p.id=$4 AND c.enabled AND c.healthy AND p.enabled`, quote.ServiceID, quote.CountryID, quote.ChannelID, quote.ProviderID).Scan(&channelID, &providerID, &serviceID, &countryID, &providerCode, &base, &credential)
	if err != nil {
		return nil, ErrSMSProviderUnavailable
	}
	key := providerAPIKey(providerCode, credential, s.encryptor)
	provider := providerFor(providerCode, base, key)
	if provider == nil {
		return nil, ErrSMSProviderUnavailable
	}
	// A quote is only a short-lived user confirmation snapshot. Immediately
	// before reserving balance, re-check the provider's current cost and stock so
	// a stale upstream price can never be purchased at the old sale price.
	providerServiceCode := quote.ProviderServiceCode
	if strings.TrimSpace(providerServiceCode) == "" {
		providerServiceCode = req.ServiceCode
	}
	providerCountryCode := quote.ProviderCountryCode
	if strings.TrimSpace(providerCountryCode) == "" {
		providerCountryCode = req.CountryCode
	}
	liveQuote, liveErr := provider.Quote(ctx, SMSQuoteRequest{
		ProviderCode:  providerCode,
		ServiceCode:   providerServiceCode,
		CountryCode:   providerCountryCode,
		ProductType:   req.ProductType,
		OperatorCode:  quote.OperatorCode,
		VoiceMode:     quote.VoiceMode,
		DurationValue: req.DurationValue,
		DurationUnit:  req.DurationUnit,
	})
	if liveErr != nil {
		return nil, sanitizeProviderError(liveErr)
	}
	if liveQuote == nil || liveQuote.Stock <= 0 {
		return nil, ErrSMSInsufficientStock
	}
	liveCost, _ := liveQuote.Cost.Float64()
	if math.Abs(liveCost-quote.ProviderCost) > 0.00000001 {
		return nil, ErrSMSPriceChanged
	}
	orderID, err := s.reserveSMSPurchase(ctx, userID, channelID, providerID, serviceID, countryID, req, selected, idempotencyKey)
	if err != nil {
		if lookupErr := s.db.QueryRowContext(ctx, `SELECT id FROM sms_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&orderID); lookupErr == nil {
			return s.GetOrder(ctx, userID, orderID)
		}
		return nil, err
	}
	purchaseReq := req
	purchaseReq.QuoteID = ""
	purchaseReq.OperatorCode = quote.OperatorCode
	purchaseReq.VoiceMode = quote.VoiceMode
	purchaseReq.ProviderCostLimit = quote.ProviderCost
	if selected.ProviderServiceCode != "" {
		purchaseReq.ServiceCode = selected.ProviderServiceCode
	}
	if selected.ProviderCountryCode != "" {
		purchaseReq.CountryCode = selected.ProviderCountryCode
	}
	var purchased *SMSPurchaseResult
	if req.ProductType == "rental" {
		purchased, err = provider.PurchaseRental(ctx, purchaseReq)
	} else {
		purchased, err = provider.PurchaseTemporary(ctx, purchaseReq)
	}
	if err != nil {
		if isSMSProviderTimeout(err) {
			// A timeout does not prove that the provider did not allocate a number.
			// Keep the hold in frozen_balance until reconciliation determines the result.
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4`, smsReconciliationPurchase, int(smsVerificationUnknownTimeout.Seconds()), "provider timeout; reconciliation required", orderID)
			return nil, ErrSMSProviderUnknown
		}
		if settleErr := s.failSMSPurchase(ctx, orderID, userID, sanitizeProviderError(err).Error()); settleErr != nil {
			return nil, settleErr
		}
		return nil, sanitizeProviderError(err)
	}
	if purchased == nil || strings.TrimSpace(purchased.ProviderOrderID) == "" {
		err = errors.New("provider returned no order id")
		if settleErr := s.failSMSPurchase(ctx, orderID, userID, sanitizeProviderError(err).Error()); settleErr != nil {
			return nil, settleErr
		}
		return nil, sanitizeProviderError(err)
	}
	pricing, _ := s.GetPricingSettings(ctx)
	expiresAt := smsOrderExpiresAt(time.Now(), req.ProductType, req.DurationValue, req.DurationUnit, purchased.ExpiresAt, time.Duration(pricing.TemporaryExpiryMinutes)*time.Minute)
	if err = s.activateSMSOrder(ctx, orderID, userID, purchased.ProviderOrderID, purchased.PhoneNumber, expiresAt, purchased.ProviderCost, purchased.ProviderOperatorCode); err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, userID, orderID)
}

// PurchaseBatch executes multiple independently quoted purchases while keeping
// each item idempotent. A provider failure is returned with the successfully
// created orders so callers can present partial results and retry only failed
// items with new idempotency keys.
func (s *SMSService) PurchaseBatch(ctx context.Context, userID int64, items []SMSPurchaseRequest, idempotencyKey string, expectedPrices []*float64) ([]*SMSOrder, error) {
	if len(items) == 0 || len(items) > 50 {
		return nil, errors.New("batch size must be between 1 and 50")
	}
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || len(key) > 125 {
		return nil, errors.New("Idempotency-Key is required")
	}
	ordersByIndex := make(map[int]*SMSOrder, len(items))
	type pendingItem struct {
		index    int
		request  SMSPurchaseRequest
		expected *float64
		key      string
	}
	pending := make([]pendingItem, 0, len(items))
	// Recover already committed items before touching quotes. This makes a
	// retried batch idempotent even when the original response was lost.
	for i, item := range items {
		itemKey := fmt.Sprintf("%s-%d", key, i)
		var orderID int64
		err := s.db.QueryRowContext(ctx, `SELECT id FROM sms_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, itemKey).Scan(&orderID)
		if err == nil {
			order, getErr := s.GetOrder(ctx, userID, orderID)
			if getErr != nil {
				return nil, getErr
			}
			ordersByIndex[i] = order
			continue
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		expected := (*float64)(nil)
		if i < len(expectedPrices) {
			expected = expectedPrices[i]
		}
		pending = append(pending, pendingItem{index: i, request: item, expected: expected, key: itemKey})
	}
	if len(pending) == 0 {
		return batchOrdersInInputOrder(ordersByIndex, len(items)), nil
	}
	var firstErr error
	// Clone additional quotes before the first pending item consumes the
	// original quote. Existing items are skipped, so retries never consume a
	// fresh quote merely to rediscover an idempotent result.
	for i := range pending {
		if pending[i].index == 0 {
			continue
		}
		if cloneID, cloneErr := s.CloneQuote(ctx, userID, pending[i].request.QuoteID); cloneErr == nil {
			pending[i].request.QuoteID = cloneID
		} else if firstErr == nil {
			firstErr = cloneErr
		}
	}
	// Preflight the complete sale total before allocating any provider number.
	// This prevents a batch from consuming the first quotes and then failing
	// halfway through because the user's balance is insufficient.
	var total float64
	stockBySignature := make(map[string]int)
	requestedBySignature := make(map[string]int)
	for i := range pending {
		item := &pending[i]
		if strings.TrimSpace(item.request.QuoteID) == "" {
			continue
		}
		quote, quoteErr := s.loadQuote(ctx, userID, item.request.QuoteID)
		if quoteErr != nil {
			if firstErr == nil {
				firstErr = quoteErr
			}
			continue
		}
		if item.expected != nil && math.Abs(*item.expected-quote.SalePrice) > 0.00000001 {
			if firstErr == nil {
				firstErr = ErrSMSPriceChanged
			}
			continue
		}
		signature := fmt.Sprintf("%d:%d:%d:%d:%s:%s:%s:%s:%d:%d:%s", quote.ChannelID, quote.ProviderID, quote.ServiceID, quote.CountryID, quote.ProductType, quote.ProviderServiceCode, quote.ProviderCountryCode, quote.OperatorCode, quote.VoiceMode, quote.DurationValue, quote.DurationUnit)
		requestedBySignature[signature]++
		stockBySignature[signature] = quote.Stock
		total += quote.SalePrice
	}
	for signature, requested := range requestedBySignature {
		if stock := stockBySignature[signature]; stock > 0 && requested > stock {
			if firstErr == nil {
				firstErr = ErrSMSInsufficientStock
			}
		}
	}
	if firstErr != nil {
		return batchOrdersInInputOrder(ordersByIndex, len(items)), firstErr
	}
	var balance float64
	if err := s.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&balance); err != nil {
		return batchOrdersInInputOrder(ordersByIndex, len(items)), err
	}
	if balance+0.00000001 < total {
		return batchOrdersInInputOrder(ordersByIndex, len(items)), ErrSMSInsufficientBalance
	}
	for _, item := range pending {
		order, err := s.Purchase(ctx, userID, item.request, item.key, item.expected)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ordersByIndex[item.index] = order
	}
	return batchOrdersInInputOrder(ordersByIndex, len(items)), firstErr
}

func batchOrdersInInputOrder(orders map[int]*SMSOrder, count int) []*SMSOrder {
	out := make([]*SMSOrder, 0, len(orders))
	for i := 0; i < count; i++ {
		if order, ok := orders[i]; ok {
			out = append(out, order)
		}
	}
	return out
}

func (s *SMSService) reserveSMSPurchase(ctx context.Context, userID, channelID, providerID, serviceID, countryID int64, req SMSPurchaseRequest, selected *SMSPublicChannel, idempotencyKey string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var orderID int64
	price := selected.SalePrice
	err = tx.QueryRowContext(ctx, `INSERT INTO sms_orders (user_id,channel_id,provider_id,service_id,country_id,product_type,status,operator_code,voice_mode,provider_cost_snapshot,sale_price_snapshot,success_rate_snapshot,success_rate_source_snapshot,success_rate_grade_snapshot,success_rate_multiplier_snapshot,idempotency_key,reserved_amount,settlement_status,reconciliation_action,reconcile_after) VALUES ($1,$2,$3,$4,$5,$6,'reconciling',$7,$8,$9,$10,$11,$12,$13,$14,$15,$10,'held',$16,NOW()+($17 * INTERVAL '1 second')) RETURNING id`, userID, channelID, providerID, serviceID, countryID, req.ProductType, requestedSMSOperator(req.OperatorCode), req.VoiceMode, selected.ProviderCost, price, selected.SuccessRate, selected.SuccessRateSource, selected.SuccessRateGrade, selected.GradeMultiplier, idempotencyKey, smsReconciliationPurchase, int(smsVerificationUnknownTimeout.Seconds())).Scan(&orderID)
	if err != nil {
		return 0, err
	}
	quoteResult, err := tx.ExecContext(ctx, `UPDATE sms_quotes SET consumed_at=NOW(),consumed_order_id=$1 WHERE id=$2 AND user_id=$3 AND consumed_at IS NULL AND expires_at>NOW()`, orderID, req.QuoteID, userID)
	if err != nil {
		return 0, err
	}
	if consumed, _ := quoteResult.RowsAffected(); consumed != 1 {
		return 0, ErrSMSQuoteExpired
	}
	res, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL AND balance >= $1`, price, userID)
	if err != nil {
		return 0, err
	}
	if affected, _ := res.RowsAffected(); affected != 1 {
		return 0, ErrSMSInsufficientBalance
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return orderID, nil
}

func requestedSMSOperator(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "any"
	}
	return value
}

func smsOrderExpiresAt(now time.Time, productType string, durationValue int, durationUnit string, providerExpiry *time.Time, temporaryExpiries ...time.Duration) *time.Time {
	if productType == "rental" {
		if providerExpiry != nil && !providerExpiry.IsZero() && providerExpiry.After(now) {
			return providerExpiry
		}
		if duration, ok := rentalDuration(durationValue, durationUnit); ok {
			return smsPtrTime(now.Add(duration))
		}
		return smsPtrTime(now.Add(24 * time.Hour))
	}
	ttl := 10 * time.Minute
	if len(temporaryExpiries) > 0 {
		ttl = temporaryExpiries[0]
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	platformExpiry := now.Add(ttl)
	if providerExpiry != nil && !providerExpiry.IsZero() && providerExpiry.After(now) && providerExpiry.Before(platformExpiry) {
		return providerExpiry
	}
	return &platformExpiry
}

func (s *SMSService) failSMSPurchase(ctx context.Context, orderID, userID int64, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var amount float64
	if err = tx.QueryRowContext(ctx, `UPDATE sms_orders SET status='failed',last_provider_error=$1,released_amount=reserved_amount,settlement_status='released',reconciliation_action='',reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$2 AND settlement_status='held' RETURNING reserved_amount`, reason, orderID).Scan(&amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, amount, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SMSService) returnSMSBalance(ctx context.Context, orderID, userID int64, held bool, terminalStatus, refundStatus, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	settlementPredicate := "settlement_status='captured'"
	amountColumn := "refunded_amount"
	settlementStatus := "refunded"
	if held {
		settlementPredicate = "settlement_status='held'"
		amountColumn = "released_amount"
		settlementStatus = "released"
	}
	query := `UPDATE sms_orders SET status=$1,refund_status=$2,refund_reason=$3,` + amountColumn + `=reserved_amount,settlement_status=$4,reconciliation_action='',reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$5 AND ` + settlementPredicate + ` RETURNING reserved_amount`
	var amount float64
	if err = tx.QueryRowContext(ctx, query, terminalStatus, refundStatus, reason, settlementStatus, orderID).Scan(&amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if held {
		_, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, amount, userID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2`, amount, userID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SMSService) activateSMSOrder(ctx context.Context, orderID, userID int64, providerOrderID, phoneNumber string, expiresAt *time.Time, providerCost float64, providerOperatorCode string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var amount float64
	if err = tx.QueryRowContext(ctx, `UPDATE sms_orders SET status='active',provider_order_id=$1,phone_number=$2,expires_at=$3,provider_cost_snapshot=CASE WHEN $4>0 THEN $4 ELSE provider_cost_snapshot END,operator_code=COALESCE(NULLIF($5,''),operator_code),captured_amount=reserved_amount,settlement_status='captured',reconciliation_action='',reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$6 AND settlement_status='held' RETURNING reserved_amount`, providerOrderID, phoneNumber, expiresAt, providerCost, strings.TrimSpace(providerOperatorCode), orderID).Scan(&amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("sms order settlement is no longer pending")
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, amount, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func isSMSProviderTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func sanitizeProviderError(err error) error {
	if err == nil {
		return nil
	}
	return apperrors.ServiceUnavailable("PROVIDER_UNAVAILABLE", "渠道暂时不可用，请稍后重试").WithCause(err)
}
func (s *SMSService) GetOrder(ctx context.Context, userID, orderID int64) (*SMSOrder, error) {
	var o SMSOrder
	var rate sql.NullFloat64
	var exp sql.NullTime
	var providerCode, providerBaseURL string
	var capabilities []byte
	err := s.db.QueryRowContext(ctx, `SELECT o.public_id::text,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.operator_code,o.voice_mode,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at,p.code,p.base_url,p.capabilities FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_providers p ON p.id=o.provider_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id WHERE o.user_id=$1 AND o.id=$2`, userID, orderID).Scan(&o.ID, &o.ProductType, &o.Status, &o.ChannelCode, &o.ChannelName, &o.ServiceCode, &o.CountryCode, &o.PhoneNumber, &o.OperatorCode, &o.VoiceMode, &o.Price, &rate, &o.SuccessRateGrade, &o.SuccessRateSource, &o.RefundStatus, &o.RefundReason, &exp, &o.CreatedAt, &providerCode, &providerBaseURL, &capabilities)
	if err != nil {
		return nil, err
	}
	o.Capabilities = resolveSMSCapabilities(providerCode, providerBaseURL, capabilities)
	if rate.Valid {
		o.SuccessRate = &rate.Float64
	}
	if exp.Valid {
		o.ExpiresAt = &exp.Time
	}
	messages, err := s.listSMSMessages(ctx, orderID)
	if err != nil {
		return nil, err
	}
	o.Messages = messages
	setSMSOrderRemaining(&o)
	return &o, nil
}

func (s *SMSService) listSMSMessages(ctx context.Context, orderID int64) ([]SMSMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,message_text,verification_code,received_at FROM sms_messages WHERE order_id=$1 ORDER BY received_at ASC,id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]SMSMessage, 0)
	for rows.Next() {
		var item SMSMessage
		if err := rows.Scan(&item.ID, &item.MessageText, &item.VerificationCode, &item.ReceivedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
func (s *SMSService) GetOrderByPublicID(ctx context.Context, userID int64, publicID string) (*SMSOrder, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM sms_orders WHERE user_id=$1 AND public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id); err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, userID, id)
}

func normalizeSMSStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "completed", "finish", "finished", "received", "ok":
		return "completed"
	case "cancel", "cancelled", "canceled":
		return "cancelled"
	case "expired", "timeout":
		return "expired"
	case "failed", "error":
		return "failed"
	case "pending", "waiting", "processing", "active", "status_wait_code":
		return "active"
	default:
		return "provider_unknown"
	}
}

// A provider may report a terminal/success state before its message payload is
// available. The user-facing order must remain active until a verification code
// is actually persisted, otherwise a transient provider state would suppress
// polling and incorrectly capture the order as completed.
func smsStatusFromProvider(result *SMSStatusResult) string {
	if result == nil {
		return "provider_unknown"
	}
	for _, message := range result.Messages {
		if strings.TrimSpace(extractSMSCode(message)) != "" {
			return "completed"
		}
	}
	status := normalizeSMSStatus(result.Status)
	if status == "completed" {
		return "active"
	}
	return status
}

// SyncOrderStatus performs one bounded provider poll and records newly received
// messages. It is called by the order endpoint and can also be used by a worker.
func (s *SMSService) SyncOrderStatus(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var providerOrder, providerCode, base, credential, productType, status string
	var expiresAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.product_type,o.status,o.expires_at FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &productType, &status, &expiresAt); err != nil {
		return err
	}
	if (status != "active" && status != "provider_unknown" && status != "reconciling") || providerOrder == "" {
		return nil
	}
	if expiresAt.Valid && !expiresAt.Time.After(time.Now()) && (status == "active" || status == "provider_unknown") {
		return s.expireSMSOrder(ctx, id, userID, productType, providerOrder, providerCode, base, credential)
	}
	return s.pollSMSOrder(ctx, id, providerOrder, providerCode, base, credential, productType)
}

func (s *SMSService) pollSMSOrder(ctx context.Context, id int64, providerOrder, providerCode, base, credential, productType string) error {
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	if p == nil {
		return ErrSMSProviderUnavailable
	}
	var result *SMSStatusResult
	var err error
	if productType == "rental" {
		result, err = p.GetRentalStatus(ctx, providerOrder)
	} else {
		result, err = p.GetTemporaryStatus(ctx, providerOrder)
	}
	if err != nil {
		s.deferSMSReconciliation(ctx, id, sanitizeProviderError(err).Error(), strings.TrimSpace(providerAPIKey(providerCode, credential, s.encryptor)) == "")
		return err
	}
	if result == nil {
		return nil
	}
	if err := s.updateSMSProviderActuals(ctx, id, result.ProviderCost, result.ProviderOperatorCode); err != nil {
		return err
	}
	newStatus := smsStatusFromProvider(result)
	if productType == "rental" && newStatus == "completed" {
		newStatus = "active"
	}
	if productType == "temporary" && newStatus == "completed" {
		if action, ok := p.(SMSOrderActionProvider); ok && p.Capabilities(ctx).Finish {
			_ = action.FinishTemporary(ctx, providerOrder)
		}
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE sms_orders SET status=$1,phone_number=COALESCE(NULLIF($2,''),phone_number),reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$3 AND status IN ('active','provider_unknown','reconciling')`, newStatus, result.PhoneNumber, id); err != nil {
		return err
	}
	for _, message := range result.Messages {
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}
		_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_messages(order_id,message_text,verification_code) SELECT $1,$2,$3 WHERE NOT EXISTS (SELECT 1 FROM sms_messages WHERE order_id=$1 AND message_text=$2)`, id, message, extractSMSCode(message))
	}
	return s.convergeSMSProviderStatus(ctx, id, newStatus)
}

func (s *SMSService) updateSMSProviderActuals(ctx context.Context, id int64, providerCost float64, providerOperatorCode string) error {
	providerOperatorCode = strings.ToLower(strings.TrimSpace(providerOperatorCode))
	if providerCost <= 0 && providerOperatorCode == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sms_orders SET provider_cost_snapshot=CASE WHEN $1>0 THEN $1 ELSE provider_cost_snapshot END,operator_code=COALESCE(NULLIF($2,''),operator_code),updated_at=NOW() WHERE id=$3`, providerCost, providerOperatorCode, id)
	return err
}

func (s *SMSService) convergeSMSProviderStatus(ctx context.Context, id int64, status string) error {
	var userID int64
	var settlementStatus, action, providerRefundStatus, providerCode string
	if err := s.db.QueryRowContext(ctx, `SELECT o.user_id,o.settlement_status,o.reconciliation_action,o.provider_refund_status,p.code FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.id=$1`, id).Scan(&userID, &settlementStatus, &action, &providerRefundStatus, &providerCode); err != nil {
		return err
	}
	switch status {
	case "completed":
		if settlementStatus == "held" {
			return s.captureSMSSettlement(ctx, id, userID)
		}
	case "failed", "cancelled", "canceled", "expired":
		if settlementStatus == "held" {
			return s.releaseSMSHold(ctx, id, userID, status, "provider ended the order before allocation was confirmed")
		}
		if settlementStatus == "captured" && providerCode == "5sim" && (status == "cancelled" || status == "canceled" || status == "expired") {
			s.markProviderRefund(ctx, id, "succeeded", "5SIM confirmed cancellation/timeout; provider refund is automatic")
			return s.refundSMSCapture(ctx, id, userID, normalizedSMSTerminalStatus(status, action), "5SIM confirmed cancellation/timeout and automatic refund")
		}
		if settlementStatus == "captured" && providerRefundStatus == "succeeded" {
			return s.refundSMSCapture(ctx, id, userID, normalizedSMSTerminalStatus(status, action), "provider confirmed the order ended without service")
		}
		if settlementStatus == "captured" && providerRefundStatus == "not_requested" {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,updated_at=NOW() WHERE id=$4`, smsReconciliationRefund, int(smsVerificationPollInterval.Seconds()), "provider refund confirmation is required before platform refund", id)
		}
	}
	return nil
}

func normalizedSMSTerminalStatus(providerStatus, action string) string {
	if action == smsReconciliationCancel {
		return "cancelled"
	}
	if action == smsReconciliationRefund || action == smsReconciliationExpire {
		return "refunded"
	}
	status := strings.ToLower(strings.TrimSpace(providerStatus))
	if status == "canceled" {
		return "cancelled"
	}
	return status
}

func (s *SMSService) releaseSMSHold(ctx context.Context, id, userID int64, terminalStatus, reason string) error {
	return s.returnSMSBalance(ctx, id, userID, true, terminalStatus, "not_requested", reason)
}

func (s *SMSService) refundSMSCapture(ctx context.Context, id, userID int64, terminalStatus, reason string) error {
	return s.returnSMSBalance(ctx, id, userID, false, terminalStatus, "approved", reason)
}

func (s *SMSService) markSMSClosed(ctx context.Context, id int64, status, refundStatus, reason string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sms_orders SET status=$1,refund_status=$2,refund_reason=$3,reconciliation_action='',reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown','reconciling')`, status, refundStatus, reason, id)
	return err
}

func (s *SMSService) markProviderRefund(ctx context.Context, id int64, status, reason string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET provider_refund_status=$1,refund_reason=CASE WHEN $2 <> '' THEN $2 ELSE refund_reason END,updated_at=NOW() WHERE id=$3`, status, reason, id)
}

func (s *SMSService) deferSMSReconciliation(ctx context.Context, id int64, reason string, credentialMissing bool) {
	baseSeconds := 30
	if credentialMissing {
		baseSeconds = 300
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET reconciliation_attempts=reconciliation_attempts+1,reconcile_after=NOW()+(LEAST($1 * POWER(2,LEAST(reconciliation_attempts,6)),1800) * INTERVAL '1 second'),last_provider_error=$2,updated_at=NOW() WHERE id=$3`, baseSeconds, reason, id)
}

func extractSMSCode(message string) string {
	match := smsVerificationCodePattern.FindStringSubmatch(message)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func (s *SMSService) ProcessWebhook(ctx context.Context, providerCode string, payload SMSStatusResult, providerOrderID string) error {
	providerCode = strings.ToLower(strings.TrimSpace(providerCode))
	providerOrderID = strings.TrimSpace(providerOrderID)
	if providerCode == "" || providerOrderID == "" {
		return errors.New("provider and provider order id are required")
	}
	var id int64
	var orderUserID int64
	var current string
	var productType, baseURL, credential string
	var expiresAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.user_id,o.status,o.product_type,p.base_url,p.credential_ref,o.expires_at FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE p.code=$1 AND o.provider_order_id=$2`, providerCode, providerOrderID).Scan(&id, &orderUserID, &current, &productType, &baseURL, &credential, &expiresAt); err != nil {
		return err
	}
	if expiresAt.Valid && !expiresAt.Time.After(time.Now()) && (current == "active" || current == "provider_unknown" || current == "reconciling") {
		return s.expireSMSOrder(ctx, id, orderUserID, productType, providerOrderID, providerCode, baseURL, credential)
	}
	newStatus := smsStatusFromProvider(&payload)
	if current == "completed" || current == "refunded" || current == "cancelled" || current == "failed" || current == "expired" {
		newStatus = current
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sms_orders SET status=$1,phone_number=COALESCE(NULLIF($2,''),phone_number),updated_at=NOW() WHERE id=$3`, newStatus, payload.PhoneNumber, id); err != nil {
		return err
	}
	for _, message := range payload.Messages {
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}
		_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_messages(order_id,message_text,verification_code) SELECT $1,$2,$3 WHERE NOT EXISTS (SELECT 1 FROM sms_messages WHERE order_id=$1 AND message_text=$2)`, id, message, extractSMSCode(message))
	}
	return s.convergeSMSProviderStatus(ctx, id, newStatus)
}

func (s *SMSService) ResendOrder(ctx context.Context, userID int64, publicID string) error {
	var providerOrder, providerCode, base, credential, status, productType string
	if err := s.db.QueryRowContext(ctx, `SELECT o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&providerOrder, &providerCode, &base, &credential, &status, &productType); err != nil {
		return err
	}
	if status != "active" || productType != "temporary" || providerOrder == "" {
		return errors.New("order cannot request another SMS")
	}
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	resender, ok := p.(SMSResendProvider)
	if !ok || !p.Capabilities(ctx).Resend {
		return errors.New("provider does not support another SMS")
	}
	if err := resender.ResendTemporary(ctx, providerOrder); err != nil {
		return sanitizeProviderError(err)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sms_messages WHERE order_id=(SELECT id FROM sms_orders WHERE user_id=$1 AND public_id=$2::uuid)`, userID, strings.TrimSpace(publicID))
	return nil
}

func (s *SMSService) FinishOrder(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var providerOrder, providerCode, base, credential, status, productType string
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &status, &productType); err != nil {
		return err
	}
	if status != "active" || productType != "temporary" || providerOrder == "" {
		return errors.New("order cannot be finished")
	}
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	action, ok := p.(SMSOrderActionProvider)
	if !ok || !p.Capabilities(ctx).Finish {
		return errors.New("provider does not support finish")
	}
	if err := action.FinishTemporary(ctx, providerOrder); err != nil {
		if isSMSProviderTimeout(err) {
			s.deferSMSReconciliation(ctx, id, "provider finish timeout", false)
			return ErrSMSProviderUnknown
		}
		return errors.New("channel refused finish")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sms_orders SET status='completed',updated_at=NOW() WHERE id=$1 AND status='active'`, id); err != nil {
		return err
	}
	return s.captureSMSSettlement(ctx, id, userID)
}

func (s *SMSService) BanOrder(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var providerOrder, providerCode, base, credential, status, productType string
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &status, &productType); err != nil {
		return err
	}
	if status != "active" || productType != "temporary" || providerOrder == "" {
		return errors.New("order cannot be banned")
	}
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	action, ok := p.(SMSOrderActionProvider)
	if !ok || !p.Capabilities(ctx).Ban {
		return errors.New("provider does not support ban")
	}
	if err := action.BanTemporary(ctx, providerOrder); err != nil {
		if isSMSProviderTimeout(err) {
			s.deferSMSReconciliation(ctx, id, "provider ban timeout", false)
			return ErrSMSProviderUnknown
		}
		return errors.New("channel refused ban")
	}
	s.markProviderRefund(ctx, id, "pending", "provider ban accepted; refund confirmation pending")
	_, err := s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),updated_at=NOW() WHERE id=$3`, smsReconciliationRefund, int(smsVerificationPollInterval.Seconds()), id)
	return err
}

func (s *SMSService) reconcileTemporaryRefundState(ctx context.Context, p SMSProvider, id, userID int64, providerOrder, terminalStatus string) (bool, error) {
	result, err := p.GetTemporaryStatus(ctx, providerOrder)
	if err != nil || result == nil {
		return false, nil
	}
	if err := s.updateSMSProviderActuals(ctx, id, result.ProviderCost, result.ProviderOperatorCode); err != nil {
		return true, err
	}
	state := smsStatusFromProvider(result)
	switch state {
	case "cancelled", "expired":
		s.markProviderRefund(ctx, id, "succeeded", "provider status confirms cancellation/timeout refund")
		return true, s.refundSMSCapture(ctx, id, userID, terminalStatus, "provider status confirms cancellation/timeout refund")
	case "completed":
		s.markProviderRefund(ctx, id, "rejected", "verification SMS was already received; cancellation/refund is no longer available")
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET refund_status='rejected',refund_reason=$1,updated_at=NOW() WHERE id=$2`, "已收到验证码，供应商不再允许取消退款", id)
		return true, errors.New("verification SMS has already been received; cancellation/refund is no longer available")
	}
	return false, nil
}

func (s *SMSService) CancelOrder(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var providerOrder, providerCode, base, credential, status, productType string
	var createdAt time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type,o.created_at FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &status, &productType, &createdAt); err != nil {
		return err
	}
	if status != "active" {
		return errors.New("order cannot be cancelled")
	}
	if productType == "temporary" {
		pricing, pricingErr := s.GetPricingSettings(ctx)
		if pricingErr != nil {
			return pricingErr
		}
		if wait := time.Duration(pricing.SelfServiceCancelAfterMinutes) * time.Minute; wait > 0 && time.Now().Before(createdAt.Add(wait)) {
			return ErrSMSCancelTooEarly
		}
	}
	key := providerAPIKey(providerCode, credential, s.encryptor)
	p := providerFor(providerCode, base, key)
	if p == nil {
		return ErrSMSProviderUnavailable
	}
	capabilities := p.Capabilities(ctx)
	if productType == "rental" && !capabilities.RentalCancel {
		return errors.New("provider does not support rental cancellation")
	}
	if productType != "rental" && !capabilities.Cancel {
		return errors.New("provider does not support cancellation")
	}
	if productType == "rental" {
		if err := p.CancelRental(ctx, providerOrder); err != nil {
			if isSMSProviderTimeout(err) {
				_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status='active'`, smsReconciliationCancel, int(smsVerificationPollInterval.Seconds()), "provider rental cancellation timeout; reconciliation required", id)
				return ErrSMSProviderUnknown
			}
			return errors.New("channel refused rental cancellation")
		}
		return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider rental cancellation confirmed")
	}
	// For refund-capable temporary products, RequestTemporaryRefund is the
	// single provider mutation. On 5SIM and SMSPVA this maps to their cancel
	// operation, avoiding a dangerous double-cancel while keeping the explicit
	// refund endpoint correct as well.
	if capabilities.Refund {
		if err := p.RequestTemporaryRefund(ctx, providerOrder); err != nil {
			if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, p, id, userID, providerOrder, "cancelled"); handled {
				return reconcileErr
			}
			s.markProviderRefund(ctx, id, "pending", "provider refund is being confirmed")
			if isSMSProviderTimeout(err) {
				_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status='active'`, smsReconciliationRefund, int(smsVerificationPollInterval.Seconds()), "provider cancellation/refund is being confirmed", id)
				return ErrSMSRefundPending
			}
			s.markProviderRefund(ctx, id, "rejected", "provider refused cancellation/refund")
			return errors.New("channel refused cancellation/refund")
		}
		s.markProviderRefund(ctx, id, "succeeded", "")
		return s.refundSMSCapture(ctx, id, userID, "cancelled", "provider cancellation and refund confirmed")
	}
	if err := p.CancelTemporary(ctx, providerOrder); err != nil {
		if isSMSProviderTimeout(err) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='not_requested',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status='active'`, smsReconciliationCancel, int(smsVerificationPollInterval.Seconds()), "provider cancellation timeout; reconciliation required", id)
			return ErrSMSProviderUnknown
		}
		return errors.New("channel refused cancellation")
	}
	return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider cancellation confirmed; refund unsupported")
}
func (s *SMSService) ListOrders(ctx context.Context, userID int64, admin bool) ([]SMSOrder, error) {
	q := `SELECT o.id,o.public_id::text,o.user_id,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.operator_code,o.voice_mode,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at,p.code,p.base_url,p.capabilities FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_providers p ON p.id=o.provider_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id`
	args := []any{}
	if !admin {
		q += ` WHERE o.user_id=$1`
		args = append(args, userID)
	}
	q += ` ORDER BY o.created_at DESC LIMIT 100`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSOrder{}
	for rows.Next() {
		var o SMSOrder
		var internalID, owner int64
		var rate sql.NullFloat64
		var exp sql.NullTime
		var providerCode, providerBaseURL string
		var capabilities []byte
		if err := rows.Scan(&internalID, &o.ID, &owner, &o.ProductType, &o.Status, &o.ChannelCode, &o.ChannelName, &o.ServiceCode, &o.CountryCode, &o.PhoneNumber, &o.OperatorCode, &o.VoiceMode, &o.Price, &rate, &o.SuccessRateGrade, &o.SuccessRateSource, &o.RefundStatus, &o.RefundReason, &exp, &o.CreatedAt, &providerCode, &providerBaseURL, &capabilities); err != nil {
			return nil, err
		}
		o.Capabilities = resolveSMSCapabilities(providerCode, providerBaseURL, capabilities)
		if rate.Valid {
			o.SuccessRate = &rate.Float64
		}
		if exp.Valid {
			o.ExpiresAt = &exp.Time
		}
		o.Messages, err = s.listSMSMessages(ctx, internalID)
		if err != nil {
			return nil, err
		}
		setSMSOrderRemaining(&o)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *SMSService) ListUserOrdersPage(ctx context.Context, userID int64, page, pageSize int, keyword, status string) (*SMSOrderPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	keyword = strings.TrimSpace(keyword)
	status = strings.TrimSpace(status)
	where := ` WHERE o.user_id=$1`
	args := []any{userID}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(` AND (o.public_id::text ILIKE $%d OR o.phone_number ILIKE $%d OR sv.code ILIKE $%d OR co.iso2 ILIKE $%d)`, len(args), len(args), len(args), len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(` AND o.status=$%d`, len(args))
	}
	from := ` FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_providers p ON p.id=o.provider_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, pageSize)
	limitPlaceholder := fmt.Sprintf("$%d", len(listArgs))
	listArgs = append(listArgs, (page-1)*pageSize)
	offsetPlaceholder := fmt.Sprintf("$%d", len(listArgs))
	query := `SELECT o.id,o.public_id::text,o.user_id,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.operator_code,o.voice_mode,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at,p.code,p.base_url,p.capabilities` + from + where + ` ORDER BY o.created_at DESC LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]SMSOrder, 0, pageSize)
	for rows.Next() {
		var order SMSOrder
		var internalID, owner int64
		var rate sql.NullFloat64
		var expiresAt sql.NullTime
		var providerCode, providerBaseURL string
		var capabilities []byte
		if err := rows.Scan(&internalID, &order.ID, &owner, &order.ProductType, &order.Status, &order.ChannelCode, &order.ChannelName, &order.ServiceCode, &order.CountryCode, &order.PhoneNumber, &order.OperatorCode, &order.VoiceMode, &order.Price, &rate, &order.SuccessRateGrade, &order.SuccessRateSource, &order.RefundStatus, &order.RefundReason, &expiresAt, &order.CreatedAt, &providerCode, &providerBaseURL, &capabilities); err != nil {
			return nil, err
		}
		order.Capabilities = resolveSMSCapabilities(providerCode, providerBaseURL, capabilities)
		if rate.Valid {
			order.SuccessRate = &rate.Float64
		}
		if expiresAt.Valid {
			order.ExpiresAt = &expiresAt.Time
		}
		order.Messages, err = s.listSMSMessages(ctx, internalID)
		if err != nil {
			return nil, err
		}
		setSMSOrderRemaining(&order)
		items = append(items, order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &SMSOrderPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}
func (s *SMSService) RequestRefund(ctx context.Context, userID int64, orderPublicID string) error {
	var id int64
	var providerOrder, providerCode, base, cred, status, productType string
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type,o.created_at FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, orderPublicID).Scan(&id, &providerOrder, &providerCode, &base, &cred, &status, &productType, &createdAt)
	if err != nil {
		return err
	}
	if status != "active" {
		return errors.New("order cannot be refunded")
	}
	if productType == "rental" {
		return errors.New("rental refunds are unavailable for this channel")
	}
	pricing, pricingErr := s.GetPricingSettings(ctx)
	if pricingErr != nil {
		return pricingErr
	}
	if wait := time.Duration(pricing.SelfServiceCancelAfterMinutes) * time.Minute; wait > 0 && time.Now().Before(createdAt.Add(wait)) {
		return ErrSMSCancelTooEarly
	}
	key := providerAPIKey(providerCode, cred, s.encryptor)
	p := providerFor(providerCode, base, key)
	if p == nil {
		return ErrSMSProviderUnavailable
	}
	if !p.Capabilities(ctx).Refund {
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET refund_status='rejected',provider_refund_status='rejected',refund_reason=$1,updated_at=NOW() WHERE id=$2`, "provider does not support refunds; administrator review is required", id)
		return errors.New("provider does not support refunds; administrator review is required")
	}
	if err := p.RequestTemporaryRefund(ctx, providerOrder); err != nil {
		if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, p, id, userID, providerOrder, "refunded"); handled {
			return reconcileErr
		}
		if isSMSProviderTimeout(err) {
			s.markProviderRefund(ctx, id, "pending", "provider refund is being confirmed")
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,updated_at=NOW() WHERE id=$4 AND refund_status IN ('not_requested','rejected')`, smsReconciliationRefund, int(smsVerificationPollInterval.Seconds()), "渠道退款处理中", id)
			return ErrSMSRefundPending
		}
		s.markProviderRefund(ctx, id, "rejected", "provider refused refund")
		reason := "渠道方拒绝退款"
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET refund_status='rejected',refund_reason=$1,updated_at=NOW() WHERE id=$2`, reason, id)
		return errors.New(reason)
	}
	s.markProviderRefund(ctx, id, "succeeded", "")
	return s.refundSMSCapture(ctx, id, userID, "refunded", "provider refund confirmed")
}

func rentalDuration(value int, unit string) (time.Duration, bool) {
	if value <= 0 {
		return 0, false
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "minute":
		return time.Duration(value) * time.Minute, true
	case "hour":
		return time.Duration(value) * time.Hour, true
	case "day":
		return time.Duration(value) * 24 * time.Hour, true
	case "week":
		return time.Duration(value) * 7 * 24 * time.Hour, true
	case "month":
		return time.Duration(value) * 30 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func (s *SMSService) ExtendRental(ctx context.Context, userID int64, publicID string, value int, unit, idempotencyKey string) (*SMSOrder, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	delta, ok := rentalDuration(value, unit)
	if !ok {
		return nil, errors.New("invalid rental duration")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		return nil, errors.New("Idempotency-Key is required")
	}
	var id int64
	var providerOrder, providerCode, base, credential, productType, status string
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.product_type,o.status FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &productType, &status); err != nil {
		return nil, err
	}
	if productType != "rental" || status != "active" {
		return nil, errors.New("rental cannot be extended")
	}
	var already bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sms_order_events WHERE order_id=$1 AND event_type='rental_extend' AND idempotency_key=$2)`, id, idempotencyKey).Scan(&already); err != nil {
		return nil, err
	}
	if already {
		return s.GetOrderByPublicID(ctx, userID, publicID)
	}
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	if p == nil || !p.Capabilities(ctx).Extend {
		return nil, errors.New("rental extension is unavailable for this channel")
	}
	if err := p.ExtendRental(ctx, providerOrder, value, unit); err != nil {
		if isSMSProviderTimeout(err) {
			return nil, ErrSMSProviderUnknown
		}
		return nil, errors.New("channel refused rental extension")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO sms_order_events(order_id,event_type,idempotency_key,payload) VALUES ($1,'rental_extend',$2,$3) ON CONFLICT (order_id,event_type,idempotency_key) DO NOTHING`, id, idempotencyKey, fmt.Sprintf(`{"duration_value":%d,"duration_unit":%q}`, value, strings.ToLower(unit)))
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return s.GetOrderByPublicID(ctx, userID, publicID)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE sms_orders SET expires_at=COALESCE(expires_at,NOW())+($1 * INTERVAL '1 second'),updated_at=NOW() WHERE id=$2`, delta.Seconds(), id)
	if err != nil {
		return nil, err
	}
	return s.GetOrderByPublicID(ctx, userID, publicID)
}

type SMSProviderAdmin struct {
	ID                   int64                   `json:"id"`
	Code                 string                  `json:"code"`
	Name                 string                  `json:"name"`
	BaseURL              string                  `json:"base_url"`
	HealthStatus         string                  `json:"health_status"`
	Enabled              bool                    `json:"enabled"`
	CredentialConfigured bool                    `json:"credential_configured"`
	CredentialRef        string                  `json:"credential_ref,omitempty"`
	Capabilities         SMSProviderCapabilities `json:"capabilities"`
}
type SMSChannelAdmin struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	PublicName   string `json:"public_name"`
	Role         string `json:"role"`
	ProviderCode string `json:"provider_code"`
	ProviderID   int64  `json:"provider_id"`
	Enabled      bool   `json:"enabled"`
	Visible      bool   `json:"visible"`
	Healthy      bool   `json:"healthy"`
	SortOrder    int    `json:"sort_order"`
}
type SMSProviderMappingAdmin struct {
	Kind               string `json:"kind"`
	TargetID           int64  `json:"target_id"`
	InternalCode       string `json:"internal_code"`
	InternalName       string `json:"internal_name"`
	ProviderCode       string `json:"provider_code"`
	ProviderName       string `json:"provider_name,omitempty"`
	TemporarySupported bool   `json:"temporary_supported,omitempty"`
	RentalSupported    bool   `json:"rental_supported,omitempty"`
	Enabled            bool   `json:"enabled"`
}
type SMSProviderMappingPage struct {
	Items    []SMSProviderMappingAdmin `json:"items"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Pages    int                       `json:"pages"`
}

func decodeCapabilities(raw []byte) SMSProviderCapabilities {
	var values map[string]bool
	_ = json.Unmarshal(raw, &values)
	return SMSProviderCapabilities{
		Temporary:         values["supports_temporary"] || values["temporary"],
		Rental:            values["supports_rental"] || values["rental"],
		RentalCancel:      values["supports_rental_cancel"] || values["rental_cancel"],
		Webhook:           values["supports_webhook"] || values["webhook"],
		Polling:           values["supports_polling"] || values["polling"],
		Cancel:            values["supports_cancel"] || values["cancel"],
		Refund:            values["supports_refund"] || values["refund"],
		RefundStatus:      values["supports_refund_status"] || values["refund_status"],
		Finish:            values["supports_finish"] || values["finish"],
		Ban:               values["supports_ban"] || values["ban"],
		Extend:            values["supports_extend"] || values["extend"],
		Resend:            values["supports_resend"] || values["resend"],
		Voice:             values["supports_voice"] || values["voice"],
		VoiceSMS:          values["supports_voice_sms"] || values["voice_sms"],
		VoiceCallerID:     values["supports_voice_caller_id"] || values["voice_caller_id"],
		VoiceCall:         values["supports_voice_call"] || values["voice_call"],
		OperatorSelection: values["supports_operator_selection"] || values["operator_selection"],
		ServiceSelection:  values["supports_service_selection"] || values["service_selection"],
		ConversionStats:   values["supports_conversion_stats"] || values["conversion_stats"],
	}
}

func resolveSMSCapabilities(providerCode, baseURL string, raw []byte) SMSProviderCapabilities {
	if provider := providerFor(strings.ToLower(strings.TrimSpace(providerCode)), baseURL, ""); provider != nil {
		return provider.Capabilities(context.Background())
	}
	return decodeCapabilities(raw)
}

func (s *SMSService) ListProviders(ctx context.Context) ([]SMSProviderAdmin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,code,name,base_url,enabled,health_status,credential_ref,capabilities FROM sms_providers ORDER BY CASE code WHEN '5sim' THEN 1 WHEN 'smspva' THEN 2 ELSE 100 END,id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSProviderAdmin{}
	for rows.Next() {
		var x SMSProviderAdmin
		var cred string
		var raw []byte
		if err := rows.Scan(&x.ID, &x.Code, &x.Name, &x.BaseURL, &x.Enabled, &x.HealthStatus, &cred, &raw); err != nil {
			return nil, err
		}
		x.CredentialConfigured = providerAPIKey(x.Code, cred, s.encryptor) != ""
		x.Capabilities = resolveSMSCapabilities(x.Code, x.BaseURL, raw)
		out = append(out, x)
	}
	return out, rows.Err()
}

// AdminTestProvider performs a bounded, read-only provider authentication
// check. It never allocates a phone number or creates a provider order.
func (s *SMSService) AdminTestProvider(ctx context.Context, id int64) (map[string]any, error) {
	var code, base, credential string
	if err := s.db.QueryRowContext(ctx, `SELECT code,base_url,credential_ref FROM sms_providers WHERE id=$1`, id).Scan(&code, &base, &credential); err != nil {
		return nil, err
	}
	key := providerAPIKey(code, credential, s.encryptor)
	if key == "" {
		return nil, ErrSMSProviderCredentialMissing
	}
	provider := providerFor(strings.ToLower(strings.TrimSpace(code)), base, key)
	checker, ok := provider.(smsProviderHealthChecker)
	if !ok {
		return nil, ErrSMSProviderTestUnsupported
	}
	smsProviderTestMu.Lock()
	if previous, exists := smsProviderTests[id]; exists && time.Since(previous) < time.Minute {
		smsProviderTestMu.Unlock()
		return nil, ErrSMSProviderTestCooldown
	}
	smsProviderTests[id] = time.Now()
	smsProviderTestMu.Unlock()

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	started := time.Now()
	err := checker.TestConnection(testCtx)
	latency := time.Since(started)
	health := "healthy"
	if err != nil {
		health = "unavailable"
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE sms_providers SET health_status=$1,updated_at=NOW() WHERE id=$2`, health, id)
	result := map[string]any{"healthy": err == nil, "health_status": health, "latency_ms": latency.Milliseconds()}
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *SMSService) UpdateProvider(ctx context.Context, id int64, enabled bool, baseURL, credentialRef string) error {
	var code, existingCredential string
	if err := s.db.QueryRowContext(ctx, `SELECT code,credential_ref FROM sms_providers WHERE id=$1`, id).Scan(&code, &existingCredential); err != nil {
		return err
	}
	code = strings.ToLower(strings.TrimSpace(code))
	if enabled && code != "5sim" && code != "smspva" {
		return errors.New("provider is BETA and cannot be enabled")
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("provider base URL must be an http or https URL")
		}
	}
	credentialRef = strings.TrimSpace(credentialRef)
	if strings.HasPrefix(credentialRef, "env:") {
		name := strings.TrimPrefix(credentialRef, "env:")
		validName, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, name)
		if !validName {
			return errors.New("invalid provider credential environment reference")
		}
	}
	if credentialRef == "" && providerAPIKey(code, existingCredential, s.encryptor) == "" {
		return errors.New("provider credential is required")
	}
	if credentialRef != "" && !strings.HasPrefix(credentialRef, "env:") && !strings.HasPrefix(credentialRef, "enc:") {
		if s.encryptor == nil {
			return errors.New("credential encryption is unavailable")
		}
		encrypted, err := s.encryptor.Encrypt(credentialRef)
		if err != nil {
			return errors.New("credential encryption failed")
		}
		credentialRef = "enc:" + encrypted
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sms_providers SET enabled=$1,base_url=COALESCE(NULLIF($2,''),base_url),credential_ref=COALESCE(NULLIF($3,''),credential_ref),updated_at=NOW() WHERE id=$4`, enabled, baseURL, credentialRef, id)
	return err
}
func (s *SMSService) ListChannelsAdmin(ctx context.Context) ([]SMSChannelAdmin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.code,c.public_name,c.role,p.id,p.code,c.enabled,c.visible,c.healthy,c.sort_order FROM sms_channels c JOIN sms_providers p ON p.id=c.provider_id ORDER BY c.sort_order,c.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []SMSChannelAdmin{}
	for rows.Next() {
		var x SMSChannelAdmin
		if err := rows.Scan(&x.ID, &x.Code, &x.PublicName, &x.Role, &x.ProviderID, &x.ProviderCode, &x.Enabled, &x.Visible, &x.Healthy, &x.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *SMSService) UpdateChannel(ctx context.Context, id int64, enabled, visible, healthy bool, providerID *int64) error {
	var channelCode string
	if err := s.db.QueryRowContext(ctx, `SELECT code FROM sms_channels WHERE id=$1`, id).Scan(&channelCode); err != nil {
		return err
	}
	var targetProviderCode string
	if providerID != nil {
		if err := s.db.QueryRowContext(ctx, `SELECT code FROM sms_providers WHERE id=$1`, *providerID).Scan(&targetProviderCode); err != nil {
			return errors.New("provider not found")
		}
	} else {
		if err := s.db.QueryRowContext(ctx, `SELECT p.code FROM sms_channels c JOIN sms_providers p ON p.id=c.provider_id WHERE c.id=$1`, id).Scan(&targetProviderCode); err != nil {
			return err
		}
	}
	targetProviderCode = strings.ToLower(strings.TrimSpace(targetProviderCode))
	expectedProvider := map[string]string{"channel_1": "5sim", "channel_2": "smspva"}[strings.ToLower(strings.TrimSpace(channelCode))]
	if expectedProvider != "" && targetProviderCode != expectedProvider {
		return errors.New("production channel provider assignment is fixed")
	}
	if (enabled || visible || healthy) && targetProviderCode != "5sim" && targetProviderCode != "smspva" {
		return errors.New("BETA provider channels cannot be enabled")
	}
	if providerID != nil {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sms_providers WHERE id=$1)`, *providerID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("provider does not exist")
		}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sms_channels SET enabled=$1,visible=$2,healthy=$3,provider_id=COALESCE($4,provider_id),updated_at=NOW() WHERE id=$5`, enabled, visible, healthy, providerID, id)
	return err
}

func (s *SMSService) ListProviderMappings(ctx context.Context, providerID int64, kind string, page, pageSize int, keyword string) (*SMSProviderMappingPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	keyword = strings.TrimSpace(keyword)
	like := "%" + keyword + "%"
	var total int64
	items := []SMSProviderMappingAdmin{}
	offset := (page - 1) * pageSize
	switch kind {
	case "service":
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sms_services WHERE ($1='' OR code ILIKE $2 OR name ILIKE $2)`, keyword, like).Scan(&total); err != nil {
			return nil, err
		}
		rows, err := s.db.QueryContext(ctx, `SELECT s.id,s.code,s.name,COALESCE(m.provider_service_code,''),COALESCE(m.provider_service_name,''),COALESCE(m.temporary_supported,true),COALESCE(m.rental_supported,false),COALESCE(m.enabled,false) FROM sms_services s LEFT JOIN sms_provider_service_mappings m ON m.provider_id=$1 AND m.service_id=s.id WHERE ($2='' OR s.code ILIKE $3 OR s.name ILIKE $3) ORDER BY s.sort_order,s.id LIMIT $4 OFFSET $5`, providerID, keyword, like, pageSize, offset)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			item := SMSProviderMappingAdmin{Kind: kind}
			if err := rows.Scan(&item.TargetID, &item.InternalCode, &item.InternalName, &item.ProviderCode, &item.ProviderName, &item.TemporarySupported, &item.RentalSupported, &item.Enabled); err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	case "country":
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sms_countries WHERE ($1='' OR iso2 ILIKE $2 OR name_zh ILIKE $2 OR name_en ILIKE $2)`, keyword, like).Scan(&total); err != nil {
			return nil, err
		}
		rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.iso2,COALESCE(NULLIF(c.name_zh,''),c.name_en,c.iso2),COALESCE(NULLIF(m.provider_country_id,''),m.provider_country_code,''),COALESCE(m.provider_country_code,''),(m.id IS NOT NULL) FROM sms_countries c LEFT JOIN sms_provider_country_mappings m ON m.provider_id=$1 AND m.country_id=c.id WHERE ($2='' OR c.iso2 ILIKE $3 OR c.name_zh ILIKE $3 OR c.name_en ILIKE $3) ORDER BY c.sort_order,c.id LIMIT $4 OFFSET $5`, providerID, keyword, like, pageSize, offset)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			item := SMSProviderMappingAdmin{Kind: kind}
			if err := rows.Scan(&item.TargetID, &item.InternalCode, &item.InternalName, &item.ProviderCode, &item.ProviderName, &item.Enabled); err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("invalid provider mapping kind")
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &SMSProviderMappingPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (s *SMSService) UpsertProviderServiceMapping(ctx context.Context, providerID, serviceID int64, providerCode, providerName string, temporarySupported, rentalSupported, enabled bool) error {
	providerCode = strings.TrimSpace(providerCode)
	if enabled && providerCode == "" {
		return errors.New("provider service code is required when the mapping is enabled")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sms_provider_service_mappings(provider_id,service_id,provider_service_code,provider_service_name,temporary_supported,rental_supported,enabled) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (provider_id,service_id) DO UPDATE SET provider_service_code=EXCLUDED.provider_service_code,provider_service_name=EXCLUDED.provider_service_name,temporary_supported=EXCLUDED.temporary_supported,rental_supported=EXCLUDED.rental_supported,enabled=EXCLUDED.enabled`, providerID, serviceID, providerCode, strings.TrimSpace(providerName), temporarySupported, rentalSupported, enabled)
	return err
}

func (s *SMSService) UpsertProviderCountryMapping(ctx context.Context, providerID, countryID int64, providerCountryID, providerCountryCode string, enabled bool) error {
	providerCountryID = strings.TrimSpace(providerCountryID)
	providerCountryCode = strings.TrimSpace(providerCountryCode)
	if enabled && providerCountryID == "" && providerCountryCode == "" {
		return errors.New("provider country identifier is required when the mapping is enabled")
	}
	if !enabled {
		_, err := s.db.ExecContext(ctx, `DELETE FROM sms_provider_country_mappings WHERE provider_id=$1 AND country_id=$2`, providerID, countryID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sms_provider_country_mappings(provider_id,country_id,provider_country_id,provider_country_code) VALUES ($1,$2,$3,$4) ON CONFLICT (provider_id,country_id) DO UPDATE SET provider_country_id=EXCLUDED.provider_country_id,provider_country_code=EXCLUDED.provider_country_code`, providerID, countryID, providerCountryID, providerCountryCode)
	return err
}
func (s *SMSService) AdminStats(ctx context.Context) (map[string]any, error) {
	var orders, refunded, completed int64
	var amount float64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(sale_price_snapshot),0),COUNT(*) FILTER (WHERE status='refunded') FROM sms_orders`).Scan(&orders, &amount, &refunded); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sms_orders WHERE status='completed'`).Scan(&completed); err != nil {
		return nil, err
	}
	return map[string]any{"orders": orders, "revenue": amount, "refunded": refunded, "completed": completed, "feature_enabled": s.Enabled(ctx)}, nil
}
