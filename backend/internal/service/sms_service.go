package service

// SMS Verification domain service. This file deliberately keeps upstream
// provider details behind a narrow adapter interface and returns public
// channel DTOs only. Provider credentials are read from environment variables
// (SMS_<PROVIDER>_API_KEY); credential_ref may point to env:NAME but is never
// treated as a raw secret.

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
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrSMSFeatureDisabled     = errors.New("sms service is disabled")
	ErrSMSInsufficientBalance = errors.New("insufficient balance")
	ErrSMSProviderUnavailable = errors.New("selected channel is unavailable")
	ErrSMSPriceChanged        = errors.New("price changed; please confirm again")
	ErrSMSProviderUnknown     = errors.New("channel purchase is being reconciled")
	ErrSMSRefundPending       = errors.New("channel refund is being processed")
)

type SMSProviderCapabilities struct {
	Temporary         bool `json:"supports_temporary"`
	Rental            bool `json:"supports_rental"`
	Webhook           bool `json:"supports_webhook"`
	Polling           bool `json:"supports_polling"`
	Cancel            bool `json:"supports_cancel"`
	Refund            bool `json:"supports_refund"`
	RefundStatus      bool `json:"supports_refund_status"`
	Extend            bool `json:"supports_extend"`
	Resend            bool `json:"supports_resend"`
	Voice             bool `json:"supports_voice"`
	OperatorSelection bool `json:"supports_operator_selection"`
	ServiceSelection  bool `json:"supports_service_selection"`
}

type SMSQuoteRequest struct {
	ServiceCode string `json:"service_code"`
	CountryCode string `json:"country_code"`
	ProductType string `json:"product_type"`
}
type SMSProviderQuote struct {
	Cost                     decimal.Decimal `json:"cost"`
	Currency                 string          `json:"currency"`
	Stock                    int             `json:"stock"`
	ExpiresAt                time.Time       `json:"expires_at"`
	EstimatedDeliverySeconds int             `json:"estimated_delivery_seconds"`
}
type SMSPurchaseRequest struct {
	ChannelCode   string `json:"channel_code"`
	ServiceCode   string `json:"service_code"`
	CountryCode   string `json:"country_code"`
	ProductType   string `json:"product_type"`
	DurationValue int    `json:"duration_value,omitempty"`
	DurationUnit  string `json:"duration_unit,omitempty"`
}
type SMSPurchaseResult struct {
	ProviderOrderID string         `json:"provider_order_id"`
	PhoneNumber     string         `json:"phone_number"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}
type SMSStatusResult struct {
	Status      string         `json:"status"`
	PhoneNumber string         `json:"phone_number"`
	Messages    []string       `json:"messages,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
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

type httpSMSProvider struct {
	code, baseURL, apiKey string
	client                *http.Client
	cap                   SMSProviderCapabilities
}

func (p *httpSMSProvider) Code() string                                         { return p.code }
func (p *httpSMSProvider) Capabilities(context.Context) SMSProviderCapabilities { return p.cap }

func providerAPIKey(code, credentialRef string) string {
	envName := "SMS_" + strings.ToUpper(strings.ReplaceAll(code, "-", "_")) + "_API_KEY"
	if key := strings.TrimSpace(os.Getenv(envName)); key != "" {
		return key
	}
	ref := strings.TrimSpace(credentialRef)
	if strings.HasPrefix(ref, "env:") {
		return strings.TrimSpace(os.Getenv(strings.TrimPrefix(ref, "env:")))
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
	if strings.TrimSpace(p.apiKey) == "" {
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
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider %s request: %w", p.code, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider %s returned HTTP %d", p.code, resp.StatusCode)
	}
	return data, nil
}

type fiveSIMProvider struct{ *httpSMSProvider }

func (p *fiveSIMProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	var payload map[string]map[string]map[string]struct {
		Cost  float64 `json:"cost"`
		Count int     `json:"count"`
	}
	err := p.request(ctx, http.MethodGet, "guest/products/"+url.PathEscape(strings.ToLower(req.CountryCode))+"/any/"+url.PathEscape(req.ServiceCode), nil, nil, &payload)
	if err != nil {
		return nil, err
	}
	for _, services := range payload {
		for _, item := range services {
			for _, v := range item {
				return &SMSProviderQuote{Cost: decimal.NewFromFloat(v.Cost), Currency: "USD", Stock: v.Count, ExpiresAt: time.Now().Add(30 * time.Second), EstimatedDeliverySeconds: 90}, nil
			}
		}
	}
	return nil, ErrSMSProviderUnavailable
}
func (p *fiveSIMProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	var out struct {
		ID      any       `json:"id"`
		Phone   string    `json:"phone"`
		Expires time.Time `json:"expires"`
	}
	err := p.request(ctx, http.MethodGet, "user/buy/activation/"+url.PathEscape(strings.ToLower(req.CountryCode))+"/any/"+url.PathEscape(req.ServiceCode), nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &SMSPurchaseResult{ProviderOrderID: fmt.Sprint(out.ID), PhoneNumber: out.Phone, ExpiresAt: &out.Expires}, nil
}
func (p *fiveSIMProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	var out struct {
		Status string `json:"status"`
		Phone  string `json:"phone"`
		SMS    []struct {
			Code string `json:"code"`
			Text string `json:"text"`
		} `json:"sms"`
	}
	err := p.request(ctx, http.MethodGet, "user/check/"+url.PathEscape(id), nil, nil, &out)
	if err != nil {
		return nil, err
	}
	msgs := make([]string, 0, len(out.SMS))
	for _, m := range out.SMS {
		msgs = append(msgs, m.Text)
		if m.Code != "" {
			msgs = append(msgs, m.Code)
		}
	}
	return &SMSStatusResult{Status: strings.ToLower(out.Status), PhoneNumber: out.Phone, Messages: msgs}, nil
}
func (p *fiveSIMProvider) CancelTemporary(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodGet, "user/cancel/"+url.PathEscape(id), nil, nil, nil)
}
func (p *fiveSIMProvider) RequestTemporaryRefund(ctx context.Context, id string) error {
	return p.request(ctx, http.MethodGet, "user/cancel/"+url.PathEscape(id), nil, nil, nil)
}
func (p *fiveSIMProvider) PurchaseRental(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return nil, errors.New("rental is not supported by this provider")
}
func (p *fiveSIMProvider) GetRentalStatus(context.Context, string) (*SMSStatusResult, error) {
	return nil, errors.New("rental is not supported by this provider")
}
func (p *fiveSIMProvider) ExtendRental(context.Context, string, int, string) error {
	return errors.New("rental is not supported by this provider")
}
func (p *fiveSIMProvider) CancelRental(context.Context, string) error {
	return errors.New("rental is not supported by this provider")
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

func (p *smsPoolProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{Temporary: true, Polling: true, Cancel: true, Refund: true, ServiceSelection: true}
}
func (p *smsPoolProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	return p.quoteJSON(ctx, "/purchase/sms", req)
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

func (p *onlineSIMProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{Temporary: true, Rental: true, Polling: true, Cancel: true, ServiceSelection: true}
}
func (p *onlineSIMProvider) call(ctx context.Context, path string, query url.Values, out any) error {
	query.Set("apikey", p.apiKey)
	return p.request(ctx, http.MethodGet, path, query, nil, out)
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
	case "smspool":
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
	db       *sql.DB
	settings *SettingService
}

func NewSMSService(db *sql.DB, settings *SettingService) *SMSService {
	return &SMSService{db: db, settings: settings}
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
}
type SMSOrder struct {
	ID                string     `json:"id"`
	ProductType       string     `json:"product_type"`
	Status            string     `json:"status"`
	ChannelCode       string     `json:"channel_code"`
	ChannelName       string     `json:"channel_name"`
	ServiceCode       string     `json:"service_code"`
	CountryCode       string     `json:"country_code"`
	PhoneNumber       string     `json:"phone_number,omitempty"`
	Price             float64    `json:"price"`
	SuccessRate       *float64   `json:"success_rate,omitempty"`
	SuccessRateGrade  string     `json:"success_rate_grade,omitempty"`
	SuccessRateSource string     `json:"success_rate_source"`
	RefundStatus      string     `json:"refund_status"`
	RefundReason      string     `json:"refund_reason,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}
type SMSOrderPage struct {
	Items    []SMSOrder `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Pages    int        `json:"pages"`
}

type SMSSvcCatalogItem struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}
type SMSCountryCatalogItem struct {
	ISO2        string `json:"iso2"`
	ISO3        string `json:"iso3,omitempty"`
	CallingCode string `json:"calling_code,omitempty"`
	NameZH      string `json:"name_zh,omitempty"`
	NameEN      string `json:"name_en,omitempty"`
}

func (s *SMSService) ListServices(ctx context.Context) ([]SMSSvcCatalogItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code,name,icon,category,description FROM sms_services WHERE enabled ORDER BY sort_order,code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
func (s *SMSService) ListCountries(ctx context.Context) ([]SMSCountryCatalogItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT iso2,iso3,calling_code,name_zh,name_en FROM sms_countries WHERE enabled ORDER BY sort_order,iso2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

func (s *SMSService) Quote(ctx context.Context, req SMSQuoteRequest) ([]SMSPublicChannel, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	if req.ProductType != "temporary" && req.ProductType != "rental" {
		return nil, errors.New("invalid product type")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.code,c.public_name,c.role,p.code,p.base_url,p.credential_ref,COALESCE((p.capabilities->>'supports_rental')::boolean,(p.capabilities->>'rental')::boolean,false) FROM sms_channels c JOIN sms_providers p ON p.id=c.provider_id JOIN sms_services sv ON sv.code=$1 AND sv.enabled JOIN sms_countries co ON co.iso2=$2 AND co.enabled WHERE c.enabled AND c.visible AND c.healthy AND p.enabled`, req.ServiceCode, strings.ToUpper(req.CountryCode))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SMSPublicChannel{}
	for rows.Next() {
		var code, name, role, pc, base, cred string
		var rental bool
		if err := rows.Scan(&code, &name, &role, &pc, &base, &cred, &rental); err != nil {
			return nil, err
		}
		if req.ProductType == "rental" && !rental {
			continue
		}
		key := providerAPIKey(pc, cred)
		p := providerFor(pc, base, key)
		if p == nil {
			continue
		}
		if req.ProductType == "rental" && !p.Capabilities(ctx).Rental {
			continue
		}
		q, err := p.Quote(ctx, req)
		if err != nil || q == nil || q.Stock <= 0 {
			continue
		}
		grade, rate := s.successGrade(ctx, code, req.ServiceCode, strings.ToUpper(req.CountryCode))
		multiplier, fixed := s.gradePricing(ctx, grade)
		price := q.Cost.Mul(decimal.NewFromFloat(1.30)).Mul(decimal.NewFromFloat(multiplier)).Add(decimal.NewFromFloat(fixed))
		id := randomID()
		out = append(out, SMSPublicChannel{Code: code, PublicName: name, Role: role, SalePrice: quantize(price), Stock: q.Stock, SuccessRate: rate, SuccessRateGrade: grade, SuccessRateSource: rateSource(rate), EstimatedDeliverySeconds: q.EstimatedDeliverySeconds, Capabilities: p.Capabilities(ctx), QuoteID: id, QuoteExpiresAt: q.ExpiresAt, ProviderCost: quantize(q.Cost), GradeMultiplier: multiplier, GradeFixedMarkup: fixed})
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
func (s *SMSService) gradePricing(ctx context.Context, grade string) (float64, float64) {
	var multiplier, fixed sql.NullFloat64
	if grade == "" {
		return 1, 0
	}
	if err := s.db.QueryRowContext(ctx, `SELECT multiplier,fixed_markup FROM sms_success_rate_rules WHERE grade=$1 AND enabled`, grade).Scan(&multiplier, &fixed); err != nil {
		return 1, 0
	}
	if !multiplier.Valid {
		multiplier.Float64 = 1
	}
	if !fixed.Valid {
		fixed.Float64 = 0
	}
	return multiplier.Float64, fixed.Float64
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
	quotes, err := s.Quote(ctx, SMSQuoteRequest{ServiceCode: req.ServiceCode, CountryCode: req.CountryCode, ProductType: req.ProductType})
	if err != nil {
		return nil, err
	}
	var selected *SMSPublicChannel
	for i := range quotes {
		if req.ChannelCode == "" || quotes[i].Code == req.ChannelCode {
			selected = &quotes[i]
			break
		}
	}
	if selected == nil {
		return nil, ErrSMSProviderUnavailable
	}
	if expectedPrice != nil && math.Abs(*expectedPrice-selected.SalePrice) > 0.00000001 {
		return nil, ErrSMSPriceChanged
	}
	var existing SMSOrder
	var existingExpires sql.NullTime
	var existingRate sql.NullFloat64
	err = s.db.QueryRowContext(ctx, `SELECT o.public_id::text,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id WHERE o.user_id=$1 AND o.idempotency_key=$2`, userID, idempotencyKey).Scan(&existing.ID, &existing.ProductType, &existing.Status, &existing.ChannelCode, &existing.ChannelName, &existing.ServiceCode, &existing.CountryCode, &existing.PhoneNumber, &existing.Price, &existingRate, &existing.SuccessRateGrade, &existing.SuccessRateSource, &existing.RefundStatus, &existing.RefundReason, &existingExpires, &existing.CreatedAt)
	if err == nil {
		if existingRate.Valid {
			existing.SuccessRate = &existingRate.Float64
		}
		if existingExpires.Valid {
			existing.ExpiresAt = &existingExpires.Time
		}
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	var channelID, providerID, serviceID, countryID int64
	var base, providerCode, credential string
	err = s.db.QueryRowContext(ctx, `SELECT c.id,p.id,sv.id,co.id,p.code,p.base_url,p.credential_ref FROM sms_channels c JOIN sms_providers p ON p.id=c.provider_id JOIN sms_services sv ON sv.code=$1 JOIN sms_countries co ON co.iso2=$2 WHERE c.code=$3 AND c.enabled AND c.healthy AND p.enabled`, req.ServiceCode, strings.ToUpper(req.CountryCode), selected.Code).Scan(&channelID, &providerID, &serviceID, &countryID, &providerCode, &base, &credential)
	if err != nil {
		return nil, ErrSMSProviderUnavailable
	}
	key := providerAPIKey(providerCode, credential)
	provider := providerFor(providerCode, base, key)
	if provider == nil {
		return nil, ErrSMSProviderUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	var orderID int64
	var price = selected.SalePrice
	err = tx.QueryRowContext(ctx, `INSERT INTO sms_orders (user_id,channel_id,provider_id,service_id,country_id,product_type,status,provider_cost_snapshot,sale_price_snapshot,success_rate_snapshot,success_rate_source_snapshot,success_rate_grade_snapshot,success_rate_multiplier_snapshot,idempotency_key) VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8,$9,$10,$11,$12,$13) RETURNING id`, userID, channelID, providerID, serviceID, countryID, req.ProductType, selected.ProviderCost, price, selected.SuccessRate, selected.SuccessRateSource, selected.SuccessRateGrade, selected.GradeMultiplier, idempotencyKey).Scan(&orderID)
	if err != nil {
		_ = tx.Rollback()
		if lookupErr := s.db.QueryRowContext(ctx, `SELECT id FROM sms_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&orderID); lookupErr == nil {
			return s.GetOrder(ctx, userID, orderID)
		}
		return nil, err
	}
	res, execErr := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL AND balance >= $1`, price, userID)
	if execErr != nil {
		_ = tx.Rollback()
		return nil, execErr
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		_ = tx.Rollback()
		return nil, ErrSMSInsufficientBalance
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	purchaseReq := req
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
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',last_provider_error=$1,updated_at=NOW() WHERE id=$2`, "provider timeout; reconciliation required", orderID)
			return nil, ErrSMSProviderUnknown
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='failed',last_provider_error=$1,updated_at=NOW() WHERE id=$2`, sanitizeProviderError(err), orderID)
		_, _ = s.db.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID)
		return nil, sanitizeProviderError(err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='active',provider_order_id=$1,phone_number=$2,expires_at=$3,updated_at=NOW() WHERE id=$4`, purchased.ProviderOrderID, purchased.PhoneNumber, purchased.ExpiresAt, orderID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID)
	if err != nil {
		return nil, err
	}
	return s.GetOrder(ctx, userID, orderID)
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
	return fmt.Errorf("渠道暂时不可用，请稍后重试")
}
func (s *SMSService) GetOrder(ctx context.Context, userID, orderID int64) (*SMSOrder, error) {
	var o SMSOrder
	var rate sql.NullFloat64
	var exp sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT o.public_id::text,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id WHERE o.user_id=$1 AND o.id=$2`, userID, orderID).Scan(&o.ID, &o.ProductType, &o.Status, &o.ChannelCode, &o.ChannelName, &o.ServiceCode, &o.CountryCode, &o.PhoneNumber, &o.Price, &rate, &o.SuccessRateGrade, &o.SuccessRateSource, &o.RefundStatus, &o.RefundReason, &exp, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	if rate.Valid {
		o.SuccessRate = &rate.Float64
	}
	if exp.Valid {
		o.ExpiresAt = &exp.Time
	}
	return &o, nil
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
	case "cancel", "cancelled", "canceled", "expired":
		return strings.ToLower(strings.TrimSpace(status))
	case "failed", "error":
		return "failed"
	case "pending", "waiting", "processing", "active", "status_wait_code":
		return "active"
	default:
		return "provider_unknown"
	}
}

// SyncOrderStatus performs one bounded provider poll and records newly received
// messages. It is called by the order endpoint and can also be used by a worker.
func (s *SMSService) SyncOrderStatus(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var providerOrder, providerCode, base, credential, productType, status string
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.product_type,o.status FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &productType, &status); err != nil {
		return err
	}
	if status != "active" || providerOrder == "" {
		return nil
	}
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential))
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
		return err
	}
	if result == nil {
		return nil
	}
	newStatus := normalizeSMSStatus(result.Status)
	if _, err = s.db.ExecContext(ctx, `UPDATE sms_orders SET status=$1,phone_number=COALESCE(NULLIF($2,''),phone_number),updated_at=NOW() WHERE id=$3 AND status='active'`, newStatus, result.PhoneNumber, id); err != nil {
		return err
	}
	for _, message := range result.Messages {
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}
		_, _ = s.db.ExecContext(ctx, `INSERT INTO sms_messages(order_id,message_text,verification_code) SELECT $1,$2,$3 WHERE NOT EXISTS (SELECT 1 FROM sms_messages WHERE order_id=$1 AND message_text=$2)`, id, message, extractSMSCode(message))
	}
	return nil
}

func extractSMSCode(message string) string {
	fields := strings.Fields(message)
	for _, field := range fields {
		clean := strings.Trim(field, ".,:;()[]{}")
		if len(clean) >= 4 && len(clean) <= 8 {
			allDigits := true
			for _, r := range clean {
				if r < '0' || r > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return clean
			}
		}
	}
	return ""
}

func (s *SMSService) ProcessWebhook(ctx context.Context, providerCode string, payload SMSStatusResult, providerOrderID string) error {
	var id int64
	var current string
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.status FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE p.code=$1 AND o.provider_order_id=$2`, strings.ToLower(strings.TrimSpace(providerCode)), strings.TrimSpace(providerOrderID)).Scan(&id, &current); err != nil {
		return err
	}
	newStatus := normalizeSMSStatus(payload.Status)
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
	return nil
}

func (s *SMSService) CancelOrder(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var providerOrder, providerCode, base, credential, status, productType string
	var price float64
	if err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type,o.sale_price_snapshot FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &providerOrder, &providerCode, &base, &credential, &status, &productType, &price); err != nil {
		return err
	}
	if status != "active" {
		return errors.New("order cannot be cancelled")
	}
	key := providerAPIKey(providerCode, credential)
	p := providerFor(providerCode, base, key)
	if p == nil {
		return ErrSMSProviderUnavailable
	}
	var cancelErr error
	if productType == "rental" {
		cancelErr = p.CancelRental(ctx, providerOrder)
	} else {
		cancelErr = p.CancelTemporary(ctx, providerOrder)
	}
	if cancelErr != nil {
		if isSMSProviderTimeout(cancelErr) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',last_provider_error=$1,updated_at=NOW() WHERE id=$2 AND status='active'`, "provider cancellation timeout; reconciliation required", id)
			return ErrSMSProviderUnknown
		}
		return errors.New("channel refused cancellation")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var changed int
	if err = tx.QueryRowContext(ctx, `UPDATE sms_orders SET status='cancelled',refund_status='approved',updated_at=NOW() WHERE id=$1 AND refund_status='not_requested' RETURNING id`, id).Scan(&changed); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SMSService) ListOrders(ctx context.Context, userID int64, admin bool) ([]SMSOrder, error) {
	q := `SELECT o.id,o.public_id::text,o.user_id,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id`
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
	defer rows.Close()
	out := []SMSOrder{}
	for rows.Next() {
		var o SMSOrder
		var internalID, owner int64
		var rate sql.NullFloat64
		var exp sql.NullTime
		if err := rows.Scan(&internalID, &o.ID, &owner, &o.ProductType, &o.Status, &o.ChannelCode, &o.ChannelName, &o.ServiceCode, &o.CountryCode, &o.PhoneNumber, &o.Price, &rate, &o.SuccessRateGrade, &o.SuccessRateSource, &o.RefundStatus, &o.RefundReason, &exp, &o.CreatedAt); err != nil {
			return nil, err
		}
		if rate.Valid {
			o.SuccessRate = &rate.Float64
		}
		if exp.Valid {
			o.ExpiresAt = &exp.Time
		}
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
	from := ` FROM sms_orders o JOIN sms_channels c ON c.id=o.channel_id JOIN sms_services sv ON sv.id=o.service_id JOIN sms_countries co ON co.id=o.country_id`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, pageSize)
	limitPlaceholder := fmt.Sprintf("$%d", len(listArgs))
	listArgs = append(listArgs, (page-1)*pageSize)
	offsetPlaceholder := fmt.Sprintf("$%d", len(listArgs))
	query := `SELECT o.id,o.public_id::text,o.user_id,o.product_type,o.status,c.code,c.public_name,sv.code,co.iso2,o.phone_number,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.success_rate_source_snapshot,o.refund_status,o.refund_reason,o.expires_at,o.created_at` + from + where + ` ORDER BY o.created_at DESC LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SMSOrder, 0, pageSize)
	for rows.Next() {
		var order SMSOrder
		var internalID, owner int64
		var rate sql.NullFloat64
		var expiresAt sql.NullTime
		if err := rows.Scan(&internalID, &order.ID, &owner, &order.ProductType, &order.Status, &order.ChannelCode, &order.ChannelName, &order.ServiceCode, &order.CountryCode, &order.PhoneNumber, &order.Price, &rate, &order.SuccessRateGrade, &order.SuccessRateSource, &order.RefundStatus, &order.RefundReason, &expiresAt, &order.CreatedAt); err != nil {
			return nil, err
		}
		if rate.Valid {
			order.SuccessRate = &rate.Float64
		}
		if expiresAt.Valid {
			order.ExpiresAt = &expiresAt.Time
		}
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
	var price float64
	err := s.db.QueryRowContext(ctx, `SELECT o.id,o.provider_order_id,p.code,p.base_url,p.credential_ref,o.status,o.product_type,o.sale_price_snapshot FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, orderPublicID).Scan(&id, &providerOrder, &providerCode, &base, &cred, &status, &productType, &price)
	if err != nil {
		return err
	}
	if status != "active" {
		return errors.New("order cannot be refunded")
	}
	if productType == "rental" {
		return errors.New("rental refunds are unavailable for this channel")
	}
	key := providerAPIKey(providerCode, cred)
	p := providerFor(providerCode, base, key)
	if p == nil {
		return ErrSMSProviderUnavailable
	}
	if err := p.RequestTemporaryRefund(ctx, providerOrder); err != nil {
		if isSMSProviderTimeout(err) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET refund_status='pending',refund_reason=$1,updated_at=NOW() WHERE id=$2 AND refund_status='not_requested'`, "渠道退款处理中", id)
			return ErrSMSRefundPending
		}
		reason := "渠道方拒绝退款"
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET refund_status='rejected',refund_reason=$1,updated_at=NOW() WHERE id=$2`, reason, id)
		return errors.New(reason)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var changed int
	if err = tx.QueryRowContext(ctx, `UPDATE sms_orders SET refund_status='approved',status='refunded',updated_at=NOW() WHERE id=$1 AND refund_status='not_requested' RETURNING id`, id).Scan(&changed); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
		return err
	}
	return tx.Commit()
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
	p := providerFor(providerCode, base, providerAPIKey(providerCode, credential))
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

func decodeCapabilities(raw []byte) SMSProviderCapabilities {
	var values map[string]bool
	_ = json.Unmarshal(raw, &values)
	return SMSProviderCapabilities{
		Temporary:         values["supports_temporary"] || values["temporary"],
		Rental:            values["supports_rental"] || values["rental"],
		Webhook:           values["supports_webhook"] || values["webhook"],
		Polling:           values["supports_polling"] || values["polling"],
		Cancel:            values["supports_cancel"] || values["cancel"],
		Refund:            values["supports_refund"] || values["refund"],
		RefundStatus:      values["supports_refund_status"] || values["refund_status"],
		Extend:            values["supports_extend"] || values["extend"],
		Resend:            values["supports_resend"] || values["resend"],
		Voice:             values["supports_voice"] || values["voice"],
		OperatorSelection: values["supports_operator_selection"] || values["operator_selection"],
		ServiceSelection:  values["supports_service_selection"] || values["service_selection"],
	}
}

func (s *SMSService) ListProviders(ctx context.Context) ([]SMSProviderAdmin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,code,name,base_url,enabled,health_status,credential_ref,capabilities FROM sms_providers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SMSProviderAdmin{}
	for rows.Next() {
		var x SMSProviderAdmin
		var cred string
		var raw []byte
		if err := rows.Scan(&x.ID, &x.Code, &x.Name, &x.BaseURL, &x.Enabled, &x.HealthStatus, &cred, &raw); err != nil {
			return nil, err
		}
		x.CredentialConfigured = providerAPIKey(x.Code, cred) != ""
		x.CredentialRef = cred
		x.Capabilities = decodeCapabilities(raw)
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *SMSService) UpdateProvider(ctx context.Context, id int64, enabled bool, baseURL, credentialRef string) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("provider base URL must be an http or https URL")
		}
	}
	credentialRef = strings.TrimSpace(credentialRef)
	if credentialRef != "" && !strings.HasPrefix(credentialRef, "env:") {
		return errors.New("credential_ref must use env:NAME")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sms_providers SET enabled=$1,base_url=COALESCE(NULLIF($2,''),base_url),credential_ref=COALESCE(NULLIF($3,''),credential_ref),updated_at=NOW() WHERE id=$4`, enabled, baseURL, credentialRef, id)
	return err
}
func (s *SMSService) ListChannelsAdmin(ctx context.Context) ([]SMSChannelAdmin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.code,c.public_name,c.role,p.id,p.code,c.enabled,c.visible,c.healthy,c.sort_order FROM sms_channels c JOIN sms_providers p ON p.id=c.provider_id ORDER BY c.sort_order,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
