package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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

type smsPVARentalEnvelope struct {
	Status int             `json:"status"`
	Data   json.RawMessage `json:"data"`
	Msg    string          `json:"msg"`
}

func (p *smsPVAProvider) Capabilities(context.Context) SMSProviderCapabilities {
	return SMSProviderCapabilities{
		Temporary: true, Rental: true, RentalCancel: true, Polling: true, Cancel: true, Refund: true,
		Finish: true, Resend: true,
		Voice: true, VoiceSMS: true, VoiceCallerID: true, VoiceCall: true,
		OperatorSelection: true, ServiceSelection: true, Extend: true, ConversionStats: true,
	}
}

func (p *smsPVAProvider) requestJSON(ctx context.Context, method, path string, query url.Values, out any, accepted ...int) (int, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return 0, ErrSMSProviderCredentialMissing
	}
	base := strings.TrimRight(p.baseURL, "/")
	target := base + "/" + strings.TrimLeft(path, "/")
	u, err := url.Parse(target)
	if err != nil {
		return 0, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apikey", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("provider smspva request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return resp.StatusCode, err
	}

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	for _, code := range accepted {
		if resp.StatusCode == code {
			ok = true
			break
		}
	}
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

func (p *smsPVAProvider) rentalRequestJSON(ctx context.Context, query url.Values, out *smsPVARentalEnvelope) error {
	if query == nil { query = url.Values{} }
	if strings.TrimSpace(p.apiKey) != "" { query.Set("apikey", p.apiKey) }
	base := strings.TrimRight(p.baseURL, "/")
	targetBase := "https://smspva.com/api/rent.php"
	if u, err := url.Parse(base); err == nil && u.Host != "" && !strings.Contains(strings.ToLower(u.Host), "smspva.com") {
		targetBase = base + "/api/rent.php"
	}
	target := targetBase + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx,http.MethodGet,target,nil)
	if err != nil { return err }
	req.Header.Set("Accept","application/json")
	resp, err := p.client.Do(req)
	if err != nil { return fmt.Errorf("provider smspva rental request: %w",err) }
	defer func(){ _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body,2<<20))
	if err != nil { return err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("provider smspva rental returned HTTP %d",resp.StatusCode) }
	if err := json.Unmarshal(body,out); err != nil { return fmt.Errorf("provider smspva rental returned invalid response: %w",err) }
	if out.Status != 1 { if strings.TrimSpace(out.Msg)!="" { return errors.New(out.Msg) }; return ErrSMSProviderUnavailable }
	return nil
}

func smsPVARentalPeriod(value int, unit string) (string,int,int,error) {
	if value <= 0 { value = 1 }
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "week": return "week", value, value*7, nil
	case "month": return "month", value, value*30, nil
	default: return "",0,0,errors.New("SMSPVA rental purchase duration must use week or month")
	}
}

func smsPVARentalExtensionPeriod(value int, unit string) (string,int,error) {
	if value <= 0 { return "",0,errors.New("rental extension duration must be positive") }
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "week": return "week",value,nil
	case "month": return "month",value,nil
	case "day":
		if value > 6 { return "",0,errors.New("SMSPVA daily max extension is 6 days") }
		return "day",value,nil
	default: return "",0,errors.New("SMSPVA rental extension must use day, week, or month")
	}
}

func (p *smsPVAProvider) Catalog(ctx context.Context) ([]SMSSvcCatalogItem, []SMSCountryCatalogItem, error) {
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/servicesprices", url.Values{"voice": {"0"}}, &env); err != nil {
		return nil, nil, err
	}
	if env.StatusCode != 0 && env.StatusCode != http.StatusOK {
		return nil, nil, ErrSMSProviderUnavailable
	}

	var rows []struct {
		Service     string          `json:"service"`
		Description string          `json:"serviceDescription"`
		Country     string          `json:"country"`
		Price       json.RawMessage `json:"price"`
	}
	if err := json.Unmarshal(env.Data, &rows); err != nil {
		return nil, nil, err
	}

	servicesMap := map[string]SMSSvcCatalogItem{}
	countriesMap := map[string]SMSCountryCatalogItem{}
	for _, row := range rows {
		service := strings.ToLower(strings.TrimSpace(row.Service))
		country := strings.ToUpper(strings.TrimSpace(row.Country))
		if service != "" {
			name := strings.TrimSpace(row.Description)
			if name == "" {
				name = service
			}
			servicesMap[service] = SMSSvcCatalogItem{
				Code:         service,
				Name:         name,
				ProviderCode: service,
				Category:     "activation",
				Available:    true,
			}
		}
		if len(country) == 2 {
			countriesMap[country] = SMSCountryCatalogItem{
				ISO2:         country,
				ProviderCode: country,
				NameEN:       country,
				Available:    true,
			}
		}
	}

	services := make([]SMSSvcCatalogItem, 0, len(servicesMap))
	for _, item := range servicesMap {
		services = append(services, item)
	}
	countries := make([]SMSCountryCatalogItem, 0, len(countriesMap))
	for _, item := range countriesMap {
		countries = append(countries, item)
	}
	return services, countries, nil
}

func (p *smsPVAProvider) CatalogServices(ctx context.Context, _ []SMSCountryCatalogItem) ([]SMSSvcCatalogItem, error) {
	services, _, err := p.Catalog(ctx)
	return services, err
}

func (p *smsPVAProvider) CatalogServicesForProduct(ctx context.Context, productType string, durationValue int, durationUnit string) ([]SMSSvcCatalogItem, error) {
	if !strings.EqualFold(strings.TrimSpace(productType), "rental") { return p.CatalogServices(ctx, nil) }
	var env smsPVARentalEnvelope
	if err := p.rentalRequestJSON(ctx, url.Values{"method":{"get_default_services"}}, &env); err != nil { return nil, err }
	var rows []map[string]any
	if err := json.Unmarshal(env.Data, &rows); err != nil { return nil, err }
	out := make([]SMSSvcCatalogItem,0,len(rows))
	for _, row := range rows {
		code := smsPVAFirstString(row, "service", "code", "opt")
		name := smsPVAFirstString(row, "name", "title", "service_name")
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" { continue }
		if name == "" { name = code }
		out = append(out, SMSSvcCatalogItem{Code:code,Name:name,ProviderCode:code,Category:"rental",Available:true})
	}
	if len(out) == 0 {
		// Some deployments return sparse default-service metadata. Build the
		// catalog from the provider's country data without fabricating service IDs.
		countries, err := p.rentalCountries(ctx)
		if err != nil { return nil, err }
		seen:=map[string]SMSSvcCatalogItem{}
		for _, country := range countries {
			dtype,dcount,_,_:=smsPVARentalPeriod(durationValue,durationUnit)
			var data smsPVARentalEnvelope
			if err:=p.rentalRequestJSON(ctx,url.Values{"method":{"getdata"},"country":{country.ProviderCode},"dtype":{dtype},"dcount":{strconv.Itoa(dcount)},"extend":{"1"}},&data);err!=nil{continue}
			var payload struct{ Services []struct{ Name string `json:"name"`; Service string `json:"service"`; PriceDay json.RawMessage `json:"price_day"`; Count int `json:"count"` } `json:"services"` }
			if json.Unmarshal(data.Data,&payload)!=nil{continue}
			for _,svc:=range payload.Services{code:=strings.ToLower(strings.TrimSpace(svc.Service));if code==""{continue};name:=strings.TrimSpace(svc.Name);if name==""{name=code};seen[code]=SMSSvcCatalogItem{Code:code,Name:name,ProviderCode:code,Category:"rental",Available:svc.Count>0}}
		}
		for _,item:=range seen{out=append(out,item)}
	}
	sort.Slice(out,func(i,j int)bool{return out[i].Name<out[j].Name})
	return out,nil
}

func smsPVAFirstString(row map[string]any, keys ...string) string {
	for _, key := range keys { if value,ok:=row[key].(string);ok && strings.TrimSpace(value)!=""{return value} }
	return ""
}

func (p *smsPVAProvider) rentalCountries(ctx context.Context) ([]SMSCountryCatalogItem,error) {
	var env smsPVARentalEnvelope
	if err:=p.rentalRequestJSON(ctx,url.Values{"method":{"getcountries"}},&env);err!=nil {
		// The provider currently has deployments that return status=0 while still
		// carrying a valid public country list. Retry through the raw endpoint.
		if env.Data==nil{return nil,err}
	}
	var rows []struct{Name string `json:"name"`; Code string `json:"code"`}
	if err:=json.Unmarshal(env.Data,&rows);err!=nil{return nil,err}
	out:=make([]SMSCountryCatalogItem,0,len(rows));for _,row:=range rows{code:=strings.ToUpper(strings.TrimSpace(row.Code));if code==""{continue};out=append(out,SMSCountryCatalogItem{ISO2:code,ProviderCode:code,NameEN:strings.TrimSpace(row.Name),Available:true})}
	return out,nil
}

func (p *smsPVAProvider) CountriesForServiceProduct(ctx context.Context, serviceCode, productType string, durationValue int, durationUnit string) ([]SMSCountryCatalogItem, error) {
	if !strings.EqualFold(strings.TrimSpace(productType), "rental") { return p.CountriesForService(ctx,serviceCode) }
	dtype,dcount,_,err:=smsPVARentalPeriod(durationValue,durationUnit);if err!=nil{return nil,err}
	var env smsPVARentalEnvelope
	if err:=p.rentalRequestJSON(ctx,url.Values{"method":{"get_country_by_service"},"service":{strings.ToLower(strings.TrimSpace(serviceCode))},"dtype":{dtype},"dcount":{strconv.Itoa(dcount)}},&env);err!=nil{return nil,err}
	var rows []struct{Name string `json:"name"`; Code string `json:"code"`}
	if err:=json.Unmarshal(env.Data,&rows);err!=nil{return nil,err}
	out:=make([]SMSCountryCatalogItem,0,len(rows))
	for _,row:=range rows{code:=strings.ToUpper(strings.TrimSpace(row.Code));if code==""{continue};out=append(out,SMSCountryCatalogItem{ISO2:code,ProviderCode:code,NameEN:strings.TrimSpace(row.Name),Available:true})}
	return out,nil
}

func (p *smsPVAProvider) OperatorsForProduct(ctx context.Context, countryCode, serviceCode, productType string, voiceMode, durationValue int, durationUnit string) ([]SMSOperatorOption,error) {
	if !strings.EqualFold(strings.TrimSpace(productType),"rental"){return p.Operators(ctx,countryCode,serviceCode,voiceMode)}
	dtype,dcount,_,err:=smsPVARentalPeriod(durationValue,durationUnit);if err!=nil{return nil,err}
	var env smsPVARentalEnvelope
	if err:=p.rentalRequestJSON(ctx,url.Values{"method":{"getdataWithProviders"},"country":{strings.ToUpper(strings.TrimSpace(countryCode))},"dtype":{dtype},"dcount":{strconv.Itoa(dcount)},"extend":{"1"}},&env);err!=nil{return nil,err}
	var data struct{Services []struct{Service string `json:"service"`;PriceDay json.RawMessage `json:"price_day"`;Count map[string]int `json:"count"`} `json:"services"`}
	if err:=json.Unmarshal(env.Data,&data);err!=nil{return nil,err}
	out:=[]SMSOperatorOption{}
	for _,svc:=range data.Services{if !strings.EqualFold(svc.Service,serviceCode){continue};price,_:=jsonNumber(svc.PriceDay);for op,count:=range svc.Count{out=append(out,SMSOperatorOption{Code:op,Name:op,Stock:count,ProviderCost:price,Available:count>0})}}
	sort.Slice(out,func(i,j int)bool{if out[i].Stock==out[j].Stock{return out[i].Name<out[j].Name};return out[i].Stock>out[j].Stock})
	return out,nil
}

func (p *smsPVAProvider) CountriesForService(ctx context.Context, serviceCode string) ([]SMSCountryCatalogItem, error) {
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/serviceprices/"+url.PathEscape(serviceCode), url.Values{"voice": {"0"}}, &env); err != nil {
		return nil, err
	}

	var data struct {
		ServiceCode string `json:"scode"`
		ServiceName string `json:"sname"`
		Countries   []struct {
			Code      string `json:"ccode"`
			Name      string `json:"cname"`
			Operators []struct {
				Price json.RawMessage `json:"price"`
				Count int             `json:"count"`
			} `json:"opers"`
		} `json:"clist"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}

	conversionRates := map[string]float64{}
	var conversionEnv smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/conversions/"+url.PathEscape(serviceCode), nil, &conversionEnv); err == nil {
		var conversionData struct { Conversions map[string]json.RawMessage `json:"conversions"` }
		if json.Unmarshal(conversionEnv.Data, &conversionData) == nil {
			for code, raw := range conversionData.Conversions {
				if value, ok := jsonNumber(raw); ok { conversionRates[strings.ToUpper(strings.TrimSpace(code))] = value }
			}
		}
	}

	out := make([]SMSCountryCatalogItem, 0, len(data.Countries))
	for _, country := range data.Countries {
		code := strings.ToUpper(strings.TrimSpace(country.Code))
		if code == "" {
			continue
		}
		stock := 0
		minPrice := 0.0
		for _, operator := range country.Operators {
			stock += operator.Count
			if price, ok := jsonNumber(operator.Price); ok && price > 0 && (minPrice == 0 || price < minPrice) {
				minPrice = price
			}
		}
		out = append(out, SMSCountryCatalogItem{
			ISO2:         code,
			ProviderCode: code,
			NameEN:       strings.TrimSpace(country.Name),
			Stock:        stock,
			ProviderCost: minPrice,
			ConversionRate: conversionRates[code],
			Available:    stock > 0,
		})
	}
	return out, nil
}

func (p *smsPVAProvider) Operators(ctx context.Context, countryCode, serviceCode string, voiceMode int) ([]SMSOperatorOption, error) {
	country := strings.ToUpper(strings.TrimSpace(countryCode))
	service := strings.ToLower(strings.TrimSpace(serviceCode))
	voice := strconv.Itoa(voiceMode)
	if voiceMode < 0 || voiceMode > 2 { voice = "0" }
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/serviceprice/"+url.PathEscape(country)+"/"+url.PathEscape(service), url.Values{"voice":{voice}}, &env); err != nil { return nil, err }
	var data struct { PriceByOperators map[string]json.RawMessage `json:"priceByOperators"` }
	if err := json.Unmarshal(env.Data,&data); err != nil { return nil, err }
	out:=make([]SMSOperatorOption,0,len(data.PriceByOperators)+1)
	for code, raw := range data.PriceByOperators { if price,ok:=jsonNumber(raw); ok { out=append(out,SMSOperatorOption{Code:code,Name:code,ProviderCost:price,Available:true}) } }
	if len(out)==0 {
		var ops smsPVAEnvelope
		if _,err:=p.requestJSON(ctx,http.MethodGet,"activation/operators/"+url.PathEscape(country),nil,&ops); err==nil {
			var d struct{ Operators []string `json:"operators"` }; if json.Unmarshal(ops.Data,&d)==nil { for _,code:=range d.Operators { out=append(out,SMSOperatorOption{Code:code,Name:code,Available:true}) } }
		}
	}
	sort.Slice(out,func(i,j int)bool{return out[i].ProviderCost<out[j].ProviderCost})
	return out,nil
}

func (p *smsPVAProvider) TestConnection(ctx context.Context) error {
	var env smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/userinfo", nil, &env); err != nil {
		return err
	}
	if env.StatusCode != 0 && env.StatusCode != http.StatusOK {
		return errors.New("SMSPVA credential was rejected")
	}
	return nil
}

func (p *smsPVAProvider) Quote(ctx context.Context, req SMSQuoteRequest) (*SMSProviderQuote, error) {
	country := strings.ToUpper(strings.TrimSpace(req.CountryCode))
	service := strings.ToLower(strings.TrimSpace(req.ServiceCode))
	if country == "" || service == "" { return nil, ErrSMSProviderUnavailable }
	if strings.EqualFold(req.ProductType,"rental") {
		dtype,dcount,days,err := smsPVARentalPeriod(req.DurationValue,req.DurationUnit)
		if err != nil { return nil,err }
		var env smsPVARentalEnvelope
		q:=url.Values{"method":{"getdataWithProviders"},"country":{country},"dtype":{dtype},"dcount":{strconv.Itoa(dcount)},"extend":{"1"}}
		if err:=p.rentalRequestJSON(ctx,q,&env); err!=nil { return nil,err }
		var data struct{ Services []struct{ Service string `json:"service"`; PriceDay json.RawMessage `json:"price_day"`; TotalCount int `json:"totalCount"`; Count map[string]int `json:"count"` } `json:"services"` }
		if err:=json.Unmarshal(env.Data,&data); err!=nil { return nil,err }
		for _,item:=range data.Services {
			if !strings.EqualFold(item.Service,service) { continue }
			priceDay,ok:=jsonNumber(item.PriceDay); if !ok || priceDay<=0 { return nil,ErrSMSProviderUnavailable }
			stock:=item.TotalCount
			if op:=strings.TrimSpace(req.OperatorCode); op!="" && !strings.EqualFold(op,"any") { stock=item.Count[op] }
			if stock<=0 { return nil,ErrSMSProviderUnavailable }
			return &SMSProviderQuote{Cost:decimal.NewFromFloat(priceDay*float64(days)),Currency:"USD",Stock:stock,ExpiresAt:time.Now().Add(30*time.Second),EstimatedDeliverySeconds:90},nil
		}
		return nil,ErrSMSProviderUnavailable
	}

	var env smsPVAEnvelope
	path := "activation/serviceprice/" + url.PathEscape(country) + "/" + url.PathEscape(service)
	voice := strconv.Itoa(req.VoiceMode)
	if req.VoiceMode < 0 || req.VoiceMode > 2 { voice = "0" }
	if _, err := p.requestJSON(ctx, http.MethodGet, path, url.Values{"voice": {voice}}, &env); err != nil {
		return nil, err
	}
	var data struct {
		Price json.RawMessage `json:"price"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	price, ok := jsonNumber(data.Price)
	if !ok || price <= 0 {
		return nil, ErrSMSProviderUnavailable
	}

	stock := 1
	var counts smsPVAEnvelope
	if _, err := p.requestJSON(ctx, http.MethodGet, "activation/countnumbers/"+url.PathEscape(country), nil, &counts); err == nil {
		var operators []struct {
			Services []struct {
				Service string `json:"service"`
				Total   int    `json:"total"`
			} `json:"services"`
		}
		if json.Unmarshal(counts.Data, &operators) == nil {
			total := 0
			for _, operator := range operators {
				for _, svc := range operator.Services {
					if strings.EqualFold(svc.Service, service) {
						total += svc.Total
					}
				}
			}
			if total > 0 {
				stock = total
			}
		}
	}

	return &SMSProviderQuote{
		Cost:                     decimal.NewFromFloat(price),
		Currency:                 "USD",
		Stock:                    stock,
		ExpiresAt:                time.Now().Add(30 * time.Second),
		EstimatedDeliverySeconds: 90,
	}, nil
}

func (p *smsPVAProvider) PurchaseTemporary(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	var env smsPVAEnvelope
	path := "activation/number/" + url.PathEscape(strings.ToUpper(req.CountryCode)) + "/" + url.PathEscape(strings.ToLower(req.ServiceCode))
	if op := strings.TrimSpace(req.OperatorCode); op != "" && !strings.EqualFold(op, "any") { path += "/" + url.PathEscape(op) }
	voice := strconv.Itoa(req.VoiceMode)
	if req.VoiceMode < 0 || req.VoiceMode > 2 { voice = "0" }
	if _, err := p.requestJSON(ctx, http.MethodGet, path, url.Values{"voice": {voice}}, &env); err != nil {
		return nil, err
	}

	var data struct {
		OrderID       json.RawMessage `json:"orderId"`
		PhoneNumber   json.RawMessage `json:"phoneNumber"`
		OrderExpireIn int             `json:"orderExpireIn"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	orderID := strings.Trim(string(data.OrderID), "\"")
	phone := strings.Trim(string(data.PhoneNumber), "\"")
	if orderID == "" || orderID == "null" {
		return nil, errors.New("SMSPVA returned no order id")
	}
	expires := time.Now().Add(time.Duration(data.OrderExpireIn) * time.Second)
	return &SMSPurchaseResult{
		ProviderOrderID: orderID,
		PhoneNumber:     phone,
		ExpiresAt:       &expires,
	}, nil
}

func (p *smsPVAProvider) GetTemporaryStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	var env smsPVAEnvelope
	path := "activation/sms/" + url.PathEscape(strings.TrimSpace(id))
	httpStatus, err := p.requestJSON(ctx, http.MethodGet, path, nil, &env, http.StatusAccepted, http.StatusGone)
	if err != nil {
		return nil, err
	}
	switch httpStatus {
	case http.StatusAccepted:
		return &SMSStatusResult{Status: "active"}, nil
	case http.StatusGone:
		return &SMSStatusResult{Status: "expired"}, nil
	}

	var data struct {
		OrderID       json.RawMessage `json:"orderId"`
		PhoneNumber   json.RawMessage `json:"phoneNumber"`
		OrderExpireIn int             `json:"orderExpireIn"`
		SMS           *struct {
			Code     string `json:"code"`
			FullText string `json:"fullText"`
		} `json:"sms"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}

	messages := []string{}
	if data.SMS != nil {
		if strings.TrimSpace(data.SMS.FullText) != "" {
			messages = append(messages, data.SMS.FullText)
		}
		if strings.TrimSpace(data.SMS.Code) != "" {
			messages = append(messages, data.SMS.Code)
		}
	}
	status := "active"
	if len(messages) > 0 {
		status = "completed"
	}
	return &SMSStatusResult{
		Status:      status,
		PhoneNumber: rawString(data.PhoneNumber),
		Messages:    messages,
	}, nil
}

func (p *smsPVAProvider) CancelTemporary(ctx context.Context, id string) error {
	var env smsPVAEnvelope
	_, err := p.requestJSON(ctx, http.MethodPut, "activation/cancelorder/"+url.PathEscape(strings.TrimSpace(id)), nil, &env)
	return err
}

func (p *smsPVAProvider) RequestTemporaryRefund(ctx context.Context, id string) error {
	return p.CancelTemporary(ctx, id)
}
func (p *smsPVAProvider) ResendTemporary(ctx context.Context, id string) error {
	var env smsPVAEnvelope
	_, err := p.requestJSON(ctx, http.MethodPut, "activation/clearsms/"+url.PathEscape(strings.TrimSpace(id)), nil, &env)
	return err
}
func (p *smsPVAProvider) FinishTemporary(ctx context.Context, id string) error {
	var env smsPVAEnvelope
	_, err := p.requestJSON(ctx, http.MethodPut, "activation/stopsms/"+url.PathEscape(strings.TrimSpace(id)), nil, &env)
	return err
}
func (p *smsPVAProvider) BanTemporary(context.Context, string) error {
	return errors.New("SMSPVA does not expose a ban operation")
}

func (p *smsPVAProvider) PurchaseRental(ctx context.Context, req SMSPurchaseRequest) (*SMSPurchaseResult, error) {
	dtype,dcount,_,err:=smsPVARentalPeriod(req.DurationValue,req.DurationUnit); if err!=nil{return nil,err}
	q:=url.Values{"method":{"create"},"dtype":{dtype},"dcount":{strconv.Itoa(dcount)},"country":{strings.ToUpper(strings.TrimSpace(req.CountryCode))},"service":{strings.ToLower(strings.TrimSpace(req.ServiceCode))}}
	if op:=strings.TrimSpace(req.OperatorCode); op!="" && !strings.EqualFold(op,"any"){q.Set("provider",op)}
	var env smsPVARentalEnvelope
	if err:=p.rentalRequestJSON(ctx,q,&env);err!=nil{return nil,err}
	var data struct{ ID json.RawMessage `json:"id"`; Phone string `json:"pnumber"`; Until int64 `json:"until"` }
	if err:=json.Unmarshal(env.Data,&data);err!=nil{return nil,err}
	id:=rawString(data.ID); if id=="" { return nil,errors.New("SMSPVA rental returned no order id") }
	var expires *time.Time
	if data.Until>0 { t:=time.Unix(data.Until,0); expires=&t }
	// Rental numbers must be activated before SMS can be delivered. Activation
	// is retried by status polling as well, because providers may transiently
	// reject the immediate post-create activation call.
	var activation smsPVARentalEnvelope
	_ = p.rentalRequestJSON(ctx,url.Values{"method":{"activate"},"id":{id}},&activation)
	return &SMSPurchaseResult{ProviderOrderID:id,PhoneNumber:data.Phone,ExpiresAt:expires},nil
}

func (p *smsPVAProvider) GetRentalStatus(ctx context.Context, id string) (*SMSStatusResult, error) {
	var activation smsPVARentalEnvelope
	_ = p.rentalRequestJSON(ctx,url.Values{"method":{"activate"},"id":{strings.TrimSpace(id)}},&activation)
	var env smsPVARentalEnvelope
	if err:=p.rentalRequestJSON(ctx,url.Values{"method":{"sms"},"id":{strings.TrimSpace(id)}},&env);err!=nil{return nil,err}
	var data struct{ SMSList []struct{ Text string `json:"text"` } `json:"SmsList"`; OtherSMS []any `json:"OtherSms"` }
	if err:=json.Unmarshal(env.Data,&data);err!=nil{return nil,err}
	messages:=make([]string,0,len(data.SMSList)); for _,m:=range data.SMSList { if strings.TrimSpace(m.Text)!="" { messages=append(messages,m.Text) } }
	return &SMSStatusResult{Status:"active",Messages:messages},nil
}

func (p *smsPVAProvider) ExtendRental(ctx context.Context, id string, value int, unit string) error {
	dtype,dcount,err:=smsPVARentalExtensionPeriod(value,unit); if err!=nil{return err}
	method:="prolong"; if dtype=="day" { method="prolong_max" }
	var env smsPVARentalEnvelope
	return p.rentalRequestJSON(ctx,url.Values{"method":{method},"id":{strings.TrimSpace(id)},"dtype":{dtype},"dcount":{strconv.Itoa(dcount)}},&env)
}

func (p *smsPVAProvider) CancelRental(ctx context.Context, id string) error {
	var env smsPVARentalEnvelope
	return p.rentalRequestJSON(ctx,url.Values{"method":{"delete"},"id":{strings.TrimSpace(id)}},&env)
}

func rawString(v json.RawMessage) string {
	value := strings.Trim(string(v), "\"")
	if value == "null" {
		return ""
	}
	return value
}
