package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SMSRentalProviderOrder is the provider-side active rental projection used for
// eligibility checks. Field names intentionally mirror SMSPVA's documented
// orders response where the unit of canprolongmax is provider-defined.
type SMSRentalProviderOrder struct {
	ID              string
	ServiceCode     string
	ServiceName     string
	State           string
	PhoneNumber     string
	CountryCode     string
	HasNewSMS       bool
	Until           int64
	CanProlong      bool
	CanProlongMax   int
	CanProlongUntil int64
	LastOnline      int64
}

type SMSRentalConstraints struct {
	CanExtend       bool       `json:"can_extend"`
	CanProlongMax   int        `json:"can_prolong_max,omitempty"`
	CurrentUntil    *time.Time `json:"current_until,omitempty"`
	CanProlongUntil *time.Time `json:"can_prolong_until,omitempty"`
	LastOnline      *time.Time `json:"last_online,omitempty"`
}

type SMSRentalHistoryItem struct {
	ProviderOrderID string `json:"provider_order_id"`
	ServiceCode     string `json:"service_code"`
	PhoneNumber     string `json:"phone_number"`
	CountryCode     string `json:"country_code"`
	HaveSMS         bool   `json:"have_sms"`
	CanRestore      bool   `json:"can_restore"`
	Days            int    `json:"days"`
	Begin           int64  `json:"begin"`
	End             int64  `json:"end"`
	Closed          int64  `json:"closed"`
}

type SMSRentalRestoreQuote struct {
	ProviderOrderID string  `json:"provider_order_id"`
	CountryCode     string  `json:"country_code"`
	ServiceCode     string  `json:"service_code"`
	ServiceName     string  `json:"service_name"`
	PhoneNumber     string  `json:"phone_number"`
	ProviderCost    float64 `json:"provider_cost"`
	OutDays         int     `json:"out_days"`
	ProlongTo       int     `json:"prolong_to"`
}

type SMSRentalOrderInspector interface {
	RentalOrder(context.Context, string) (*SMSRentalProviderOrder, error)
}

type SMSRentalAdvancedProvider interface {
	PurchaseRentalMulti(context.Context, SMSPurchaseRequest, []string) (*SMSPurchaseResult, error)
	AddRentalService(context.Context, string, string, string, int) (*SMSPurchaseResult, error)
	RentalHistory(context.Context, int, int) ([]SMSRentalHistoryItem, error)
	PrecalcRentalRestore(context.Context, string) (*SMSRentalRestoreQuote, error)
	RestoreRental(context.Context, string) (string, error)
}

func rawInt64(v json.RawMessage) int64 {
	text := strings.Trim(strings.TrimSpace(string(v)), "\"")
	if text == "" || text == "null" {
		return 0
	}
	value, _ := strconv.ParseInt(text, 10, 64)
	return value
}

func rawBool(v json.RawMessage) bool {
	text := strings.ToLower(strings.Trim(strings.TrimSpace(string(v)), "\""))
	return text == "true" || text == "1"
}

func (p *smsPVAProvider) RentalOrder(ctx context.Context, id string) (*SMSRentalProviderOrder, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("rental order id is required")
	}
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, url.Values{"method": {"orders"}}, &env); err != nil {
		return nil, err
	}
	var rows []struct {
		ID              json.RawMessage `json:"id"`
		ServiceCode     string          `json:"scode"`
		ServiceName     string          `json:"sname"`
		State           json.RawMessage `json:"state"`
		PhoneNumber     string          `json:"pnumber"`
		CountryCode     string          `json:"cname"`
		HasNewSMS       json.RawMessage `json:"hasnewsms"`
		Until           json.RawMessage `json:"until"`
		CanProlong      json.RawMessage `json:"canprolong"`
		CanProlongMax   json.RawMessage `json:"canprolongmax"`
		CanProlongUntil json.RawMessage `json:"canprolonguntil"`
		LastOnline      json.RawMessage `json:"lastonline"`
	}
	if err := json.Unmarshal(env.Data, &rows); err != nil {
		return nil, err
	}
	var found *SMSRentalProviderOrder
	for _, row := range rows {
		rowID := rawString(row.ID)
		if rowID != id {
			continue
		}
		if found != nil {
			return nil, errors.New("SMSPVA returned duplicate rental order ids")
		}
		found = &SMSRentalProviderOrder{
			ID:              rowID,
			ServiceCode:     strings.ToLower(strings.TrimSpace(row.ServiceCode)),
			ServiceName:     strings.TrimSpace(row.ServiceName),
			State:           rawString(row.State),
			PhoneNumber:     strings.TrimSpace(row.PhoneNumber),
			CountryCode:     strings.ToUpper(strings.TrimSpace(row.CountryCode)),
			HasNewSMS:       rawBool(row.HasNewSMS),
			Until:           rawInt64(row.Until),
			CanProlong:      rawBool(row.CanProlong),
			CanProlongMax:   int(rawInt64(row.CanProlongMax)),
			CanProlongUntil: rawInt64(row.CanProlongUntil),
			LastOnline:      rawInt64(row.LastOnline),
		}
	}
	if found == nil {
		return nil, errors.New("SMSPVA rental order was not found in active orders")
	}
	return found, nil
}

func (p *smsPVAProvider) PurchaseRentalMulti(ctx context.Context, req SMSPurchaseRequest, services []string) (*SMSPurchaseResult, error) {
	dtype, dcount, _, err := smsPVARentalPeriod(req.DurationValue, req.DurationUnit)
	if err != nil {
		return nil, err
	}
	clean := make([]string, 0, len(services))
	seen := map[string]struct{}{}
	for _, service := range services {
		service = strings.ToLower(strings.TrimSpace(service))
		if service == "" {
			continue
		}
		if _, exists := seen[service]; exists {
			continue
		}
		seen[service] = struct{}{}
		clean = append(clean, service)
	}
	if len(clean) < 2 {
		return nil, errors.New("SMSPVA multi-service rental requires at least two services")
	}
	q := url.Values{
		"method":   {"create_multi"},
		"dtype":    {dtype},
		"dcount":   {strconv.Itoa(dcount)},
		"country":  {strings.ToUpper(strings.TrimSpace(req.CountryCode))},
		"services": {strings.Join(clean, ",")},
	}
	if op := strings.TrimSpace(req.OperatorCode); op != "" && !strings.EqualFold(op, "any") {
		q.Set("provider", op)
	}
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, q, &env); err != nil {
		return nil, err
	}
	var data struct {
		ID    json.RawMessage `json:"id"`
		Phone string          `json:"pnumber"`
		Until int64           `json:"until"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	id := rawString(data.ID)
	if id == "" {
		return nil, errors.New("SMSPVA multi-service rental returned no order id")
	}
	var expires *time.Time
	if data.Until > 0 {
		t := time.Unix(data.Until, 0)
		expires = &t
	}
	var activation smsPVARentalEnvelope
	_ = p.rentalRequestJSON(ctx, url.Values{"method": {"activate"}, "id": {id}}, &activation)
	return &SMSPurchaseResult{ProviderOrderID: id, PhoneNumber: strings.TrimSpace(data.Phone), ExpiresAt: expires}, nil
}

func (p *smsPVAProvider) AddRentalService(ctx context.Context, id, phone, service string, rentDays int) (*SMSPurchaseResult, error) {
	id = strings.TrimSpace(id)
	phone = strings.TrimPrefix(strings.TrimSpace(phone), "+")
	service = strings.ToLower(strings.TrimSpace(service))
	if id == "" || phone == "" || service == "" {
		return nil, errors.New("SMSPVA add-service requires order id, phone and service")
	}
	q := url.Values{
		"method":  {"add_service_to_order"},
		"id":      {id},
		"pnumber": {phone},
		"service": {service},
	}
	if rentDays > 0 {
		q.Set("rent_days", strconv.Itoa(rentDays))
	}
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, q, &env); err != nil {
		return nil, err
	}
	var data struct {
		ID      json.RawMessage `json:"id"`
		Phone   string          `json:"pnumber"`
		Service string          `json:"service"`
		Until   int64           `json:"until"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	resultID := rawString(data.ID)
	if resultID == "" {
		return nil, errors.New("SMSPVA add-service returned no order id")
	}
	var expires *time.Time
	if data.Until > 0 {
		t := time.Unix(data.Until, 0)
		expires = &t
	}
	return &SMSPurchaseResult{ProviderOrderID: resultID, PhoneNumber: strings.TrimSpace(data.Phone), ExpiresAt: expires}, nil
}

func (p *smsPVAProvider) RentalHistory(ctx context.Context, skip, take int) ([]SMSRentalHistoryItem, error) {
	if skip < 0 {
		skip = 0
	}
	if take <= 0 || take > 100 {
		take = 20
	}
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, url.Values{
		"method": {"get_rent_history"},
		"skip":   {strconv.Itoa(skip)},
		"take":   {strconv.Itoa(take)},
	}, &env); err != nil {
		return nil, err
	}
	var rows []struct {
		OrderID    json.RawMessage `json:"orderId"`
		Resource   string          `json:"resourceCode"`
		Number     string          `json:"number"`
		HaveSMS    bool            `json:"haveSms"`
		CanRestore bool            `json:"isAvailForRestore"`
		Days       int             `json:"days"`
		Country    string          `json:"country"`
		Begin      int64           `json:"begin"`
		End        int64           `json:"end"`
		Closed     int64           `json:"closed"`
	}
	if err := json.Unmarshal(env.Data, &rows); err != nil {
		return nil, err
	}
	out := make([]SMSRentalHistoryItem, 0, len(rows))
	for _, row := range rows {
		id := rawString(row.OrderID)
		if id == "" {
			continue
		}
		out = append(out, SMSRentalHistoryItem{
			ProviderOrderID: id,
			ServiceCode:     strings.ToLower(strings.TrimSpace(row.Resource)),
			PhoneNumber:     strings.TrimSpace(row.Number),
			CountryCode:     strings.ToUpper(strings.TrimSpace(row.Country)),
			HaveSMS:         row.HaveSMS,
			CanRestore:      row.CanRestore,
			Days:            row.Days,
			Begin:           row.Begin,
			End:             row.End,
			Closed:          row.Closed,
		})
	}
	return out, nil
}

func (p *smsPVAProvider) PrecalcRentalRestore(ctx context.Context, id string) (*SMSRentalRestoreQuote, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("rental history order id is required")
	}
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, url.Values{"method": {"restore_user_precalc"}, "id": {id}}, &env); err != nil {
		return nil, err
	}
	var data struct {
		Country    string          `json:"ccode"`
		Service    string          `json:"scode"`
		Price      json.RawMessage `json:"price"`
		ServiceName string         `json:"sname"`
		Phone      string          `json:"pnumber"`
		OutDays    int             `json:"outdays"`
		OrderID    json.RawMessage `json:"orderid"`
		ProlongTo  int             `json:"prolongTo"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	price, ok := jsonNumber(data.Price)
	if !ok || price <= 0 {
		return nil, errors.New("SMSPVA restore precalculation returned no valid price")
	}
	orderID := rawString(data.OrderID)
	if orderID == "" {
		orderID = id
	}
	return &SMSRentalRestoreQuote{
		ProviderOrderID: orderID,
		CountryCode:     strings.ToUpper(strings.TrimSpace(data.Country)),
		ServiceCode:     strings.ToLower(strings.TrimSpace(data.Service)),
		ServiceName:     strings.TrimSpace(data.ServiceName),
		PhoneNumber:     strings.TrimSpace(data.Phone),
		ProviderCost:    price,
		OutDays:         data.OutDays,
		ProlongTo:       data.ProlongTo,
	}, nil
}

func (p *smsPVAProvider) RestoreRental(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("rental history order id is required")
	}
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, url.Values{"method": {"restore_user"}, "id": {id}}, &env); err != nil {
		return "", err
	}
	var data struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", err
	}
	restoredID := rawString(data.ID)
	if restoredID == "" {
		return "", errors.New("SMSPVA restore returned no order id")
	}
	return restoredID, nil
}

func (s *SMSService) RentalConstraints(ctx context.Context, userID int64, publicID string) (*SMSRentalConstraints, error) {
	if !s.Enabled(ctx) {
		return nil, ErrSMSFeatureDisabled
	}
	var providerOrder, providerCode, base, credential, productType, status string
	if err := s.db.QueryRowContext(ctx, `SELECT o.provider_order_id,p.code,p.base_url,p.credential_ref,o.product_type,o.status
		FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id
		WHERE o.user_id=$1 AND o.public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).
		Scan(&providerOrder, &providerCode, &base, &credential, &productType, &status); err != nil {
		return nil, err
	}
	if productType != "rental" || status != "active" || strings.TrimSpace(providerOrder) == "" {
		return nil, errors.New("rental extension is unavailable for this order")
	}
	provider := providerFor(providerCode, base, providerAPIKey(providerCode, credential, s.encryptor))
	if provider == nil || !provider.Capabilities(ctx).Extend {
		return nil, errors.New("rental extension is unavailable for this channel")
	}
	inspector, ok := provider.(SMSRentalOrderInspector)
	if !ok {
		return &SMSRentalConstraints{CanExtend: true}, nil
	}
	order, err := inspector.RentalOrder(ctx, providerOrder)
	if err != nil {
		return nil, sanitizeProviderError(err)
	}
	result := &SMSRentalConstraints{CanExtend: order.CanProlong, CanProlongMax: order.CanProlongMax}
	if order.Until > 0 {
		value := time.Unix(order.Until, 0)
		result.CurrentUntil = &value
	}
	if order.CanProlongUntil > 0 {
		value := time.Unix(order.CanProlongUntil, 0)
		result.CanProlongUntil = &value
	}
	if order.LastOnline > 0 {
		value := time.Unix(order.LastOnline, 0)
		result.LastOnline = &value
	}
	return result, nil
}
