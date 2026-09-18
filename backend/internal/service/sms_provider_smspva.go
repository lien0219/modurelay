package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// smsPVAProvider implements the current SMSpva activation API.
// Authentication is carried in the apikey header (not Bearer auth).
type smsPVAProvider struct{ *httpSMSProvider }

type smsPVAEnvelope struct {
	StatusCode int             `json:"statusCode"`
	Data       json.RawMessage `json:"data"`
	Error      json.RawMessage `json:"error"`
}

func (p *smsPVAProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{
		Temporary: true, Polling: true, Cancel: true, Refund: true,
		Resend: true, Voice: true, OperatorSelection: true, ServiceSelection: true,
	}
}

func (p *smsPVAProvider) requestJSON(ctx context.Context, method, path string, query url.Values, out any, accepted ...int) (int, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return 0, ErrSMSProviderCredentialMissing
	}
	base := strings.TrimRight(p.baseURL, "/")
	target := base + "/" + strings.TrimLeft(path, "/")
	u, err := url.Parse(target)
	if err != nil { return 0, err }
	if query != nil { u.RawQuery = query.Encode() }
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil { return 0, err }
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apikey", p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil { return 0, fmt.Errorf("provider smspva request: %w", err) }
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil { return resp.StatusCode, err }

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	for _, code := range accepted { if resp.StatusCode == code { ok = true; break } }
	if !ok {
		return resp.StatusCode, fmt.Errorf("provider smspva returned HTTP %d", resp.StatusCode)
	}
	if out != nil && len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp.StatusCode, fmt.Errorf("provider smspva returned invalid response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func (p *smsPVAProvider) Catalog(ctx context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/servicesprices", nil, &env); err != nil { return nil, nil, err }
	if env.StatusCode != 0 && env.StatusCode != 200 { return nil, nil, ErrSMSProviderUnavailable }
	var rows []struct {
		Service string          `json:"service"`
		Description string      `json:"serviceDescription"`
		Country string          `json:"country"`
		Price json.RawMessage   `json:"price"`
	}
	if err := json.Unmarshal(env.Data, &rows); err != nil { return nil, nil, err }
	servicesMap := map[string]SMSSvcCatalogItem{}
	countriesMap := map[string]SMSCountryCatalogItem{}
	for _, row := range rows {
		service := strings.ToLower(strings.TrimSpace(row.Service))
		country := strings.ToUpper(strings.TrimSpace(row.Country))
		if service != "" {
			name := strings.TrimSpace(row.Description)
			if name == "" { name = service }
			servicesMap[service] = SMSSvcCatalogItem{Code: service, Name: name, ProviderCode: service, Category: "activation", Available: true}
		}
		if len(country) == 2 {
			countriesMap[country] = SMSCountryCatalogItem{ISO2: country, ProviderCode: country, NameEN: country, Available: true}
		}
	}
	services := make([]SMSSvcCatalogItem, 0, len(servicesMap))
	for _, item := range servicesMap { services = append(services, item) }
	countries := make([]SMSCountryCatalogItem, 0, len(countriesMap))
	for _, item := range countriesMap { countries = append(countries, item) }
	return services, countries, nil
}

func (p *smsPVAProvider) CatalogServices(ctx context.Context, _ []SMSCountryCatalogItem) ([]SMSSvcCatalogItem, error) {
	services, _, err := p.Catalog(ctx)
	return services, err
}

func (p *smsPVAProvider) CountriesForService(ctx context.Context, serviceCode string) ([]SMSCountryCatalogItem, error) {
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/servicesprices", nil, &env); err != nil { return nil, err }
	var rows []struct {
		Service string        `json:"service"`
		Country string        `json:"country"`
		Price   json.RawMessage `json:"price"`
	}
	if err := json.Unmarshal(env.Data, &rows); err != nil { return nil, err }
	seen := map[string]bool{}
	out := make([]SMSCountryCatalogItem, 0)
	for _, row := range rows {
		if strings.ToLower(strings.TrimSpace(row.Service)) != serviceCode { continue }
		country := strings.ToUpper(strings.TrimSpace(row.Country))
		if len(country) != 2 || seen[country] { continue }
		seen[country] = true
		cost, _ := jsonNumber(row.Price)
		out = append(out, SMSCountryCatalogItem{ISO2: country, ProviderCode: country, NameEN: country, ProviderCost: cost, Available: true})
	}
	return out, nil
}

func (p *smsPVAProvider) TestConnection(ctx context.Context) error {
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/userinfo", nil, &env); err != nil { return err }
	if env.StatusCode != 0 && env.StatusCode != 200 { return errors.New("SMSPVA credential was rejected") }
	return nil
}

func (p *smsPVAProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	country := strings.ToUpper(strings.TrimSpace(req.CountryCode))
	service := strings.ToLower(strings.TrimSpace(req.ServiceCode))
	if country == "" || service == "" { return nil, ErrSMSProviderUnavailable }

	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/serviceprice/"+url.PathEscape(country)+"/"+url.PathEscape(service), url.Values{"voice":{"0"}}, &env); err != nil { return nil, err }
	var data struct {
		Price json.RawMessage `json:"price"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil { return nil, err }
	price, ok := jsonNumber(data.Price)
	if !ok || price <= 0 { return nil, ErrSMSProviderUnavailable }

	stock := 1
	var counts smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/countnumbers/"+url.PathEscape(country), nil, &counts); err == nil {
		var operators []struct {
			Services []struct {
				Service string `json:"service"`
				Total int      `json:"total"`
			} `json:"services"`
		}
		if json.Unmarshal(counts.Data, &operators) == nil {
			total := 0
			for _, op := range operators {
				for _, svc := range op.Services {
					if strings.EqualFold(svc.Service, service) { total += svc.Total }
				}
			}
			if total > 0 { stock = total }
		}
	}
	return &SMSProviderQuote{Cost: decimal.NewFromFloat(price), Currency: "USD", Stock: stock, ExpiresAt: time.Now().Add(30*time.Second), EstimatedDeliverySeconds: 90}, nil
}

func (p *smsPVAProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	var env smsPVAEnvelope
	path := "activation/number/" + url.PathEscape(strings.ToUpper(req.CountryCode)) + "/" + url.PathEscape(strings.ToLower(req.ServiceCode))
	if _, err := p.requestJSON(ctx, http.MethodGet, path, url.Values{"voice":{"0"}}, &env); err != nil { return nil, err }
	var data struct {
		OrderID json.RawMessage `json:"orderId"`
		PhoneNumber json.RawMessage `json:"phoneNumber"`
		OrderExpireIn int `json:"orderExpireIn"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil { return nil, err }
	orderID := strings.Trim(string(data.OrderID), "\"")
	phone := strings.Trim(string(data.PhoneNumber), "\"")
	if orderID == "" || orderID == "null" { return nil, errors.New("SMSPVA returned no order id") }
	expires := time.Now().Add(time.Duration(data.OrderExpireIn)*time.Second)
	return &SMSPurchaseResult{ProviderOrderID: orderID, PhoneNumber: phone, ExpiresAt: &expires}, nil
}

func (p *smsPVAProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/orders", nil, &env); err != nil { return nil, err }
	var payload struct {
		Orders []struct {
			OrderID json.RawMessage `json:"orderId"`
			PhoneNumber json.RawMessage `json:"phoneNumber"`
			Status string `json:"status"`
			SMS *struct {
				Code string `json:"code"`
				FullText string `json:"fullText"`
			} `json:"sms"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil { return nil, err }
	for _, order := range payload.Orders {
		orderID := strings.Trim(string(order.OrderID), "\"")
		if orderID != strings.TrimSpace(id) { continue }
		status := "active"
		switch strings.ToUpper(order.Status) {
		case "SMS_READY": status = "completed"
		case "PENDING_SMS", "PENDING_PAYMENT": status = "active"
		case "CANCELLED", "CLOSED": status = "cancelled"
		}
		msgs := []string{}
		if order.SMS != nil {
			if order.SMS.FullText != "" { msgs = append(msgs, order.SMS.FullText) }
			if order.SMS.Code != "" { msgs = append(msgs, order.SMS.Code) }
		}
		return &SMSStatusResult{Status: status, PhoneNumber: strings.Trim(string(order.PhoneNumber), "\""), Messages: msgs}, nil
	}
	// A missing active order may already be closed by the provider.
	return &SMSStatusResult{Status: "expired"}, nil
}

func (p *smsPVAProvider) CancelTemporary(ctx context.Context, id string) error {
	var env smsPVAEnvelope
	_, err := p.requestJSON(ctx, http.MethodPut, "activation/cancelorder/"+url.PathEscape(strings.TrimSpace(id)), nil, &env)
	return err
}

func (p *smsPVAProvider) RequestTemporaryRefund(ctx context.Context, id string) error {
	return p.CancelTemporary(ctx, id)
}

func (p *smsPVAProvider) PurchaseRental(context.Context, SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	return nil, errors.New("SMSPVA rental is not enabled in this purchase flow yet")
}
func (p *smsPVAProvider) GetRentalStatus(context.Context, string) (*SMSStatusResult, error) {
	return nil, errors.New("SMSPVA rental is not enabled in this purchase flow yet")
}
func (p *smsPVAProvider) ExtendRental(context.Context, string, int, string) error {
	return errors.New("SMSPVA rental is not enabled in this purchase flow yet")
}
func (p *smsPVAProvider) CancelRental(context.Context, string) error {
	return errors.New("SMSPVA rental is not enabled in this purchase flow yet")
}

func rawString(v json.RawMessage) string {
	s := strings.Trim(string(v), "\"")
	if s == "null" { return "" }
	return s
}

func rawInt64(v json.RawMessage) int64 {
	n, _ := strconv.ParseInt(rawString(v), 10, 64)
	return n
}
