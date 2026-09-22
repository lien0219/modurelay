package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type smsBatchLimitSettingRepoStub struct {
	value string
}

func (s *smsBatchLimitSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *smsBatchLimitSettingRepoStub) GetValue(context.Context, string) (string, error) {
	if s.value == "" {
		return "", ErrSettingNotFound
	}
	return s.value, nil
}

func (s *smsBatchLimitSettingRepoStub) Set(context.Context, string, string) error { return nil }

func (s *smsBatchLimitSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (s *smsBatchLimitSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (s *smsBatchLimitSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}

func (s *smsBatchLimitSettingRepoStub) Delete(context.Context, string) error { return nil }

func TestSMSProviderCapabilitiesAreSeparated(t *testing.T) {
	for _, code := range []string{"5sim", "smspva", "smspool", "sms_activate", "onlinesim", "pingme"} {
		provider := providerFor(code, "https://example.invalid", "test-key")
		if provider == nil {
			t.Fatalf("provider %s was not registered", code)
		}
		cap := provider.Capabilities(context.Background())
		if !cap.Temporary {
			t.Fatalf("provider %s must support temporary numbers", code)
		}
		if (code == "5sim" || code == "smspool" || code == "sms_activate") && cap.Rental {
			t.Fatalf("provider %s must not advertise rental", code)
		}
		if (code == "smspva" || code == "onlinesim" || code == "pingme") && !cap.Rental {
			t.Fatalf("provider %s must advertise its configured rental capability", code)
		}
	}
}

func TestSMSPVAWebhookIsFailClosedBeforeDatabaseLookup(t *testing.T) {
	svc := NewSMSService(nil, nil, nil)
	err := svc.ProcessWebhook(context.Background(), "SMSPVA", SMSStatusResult{Status: "completed"}, "501")
	if !errors.Is(err, ErrSMSProviderWebhookUnsupported) {
		t.Fatalf("SMSPVA webhook error = %v, want %v", err, ErrSMSProviderWebhookUnsupported)
	}
}

func TestSMSPVAAdvancedRentalCapabilitiesAreSafelyGated(t *testing.T) {
	p := providerFor("smspva", "https://example.invalid", "test-key")
	cap := p.Capabilities(context.Background())
	if !cap.Rental || !cap.Extend || !cap.RentalConstraints || cap.RentalRestore {
		t.Fatalf("SMSPVA safe rental capabilities = %#v", cap)
	}
	if cap.RentalMultiService || cap.RentalAddService {
		t.Fatalf("unverified SMSPVA billing features must remain gated: %#v", cap)
	}
}

func TestSMSOrderCapabilitiesPreferRegisteredAdapter(t *testing.T) {
	if got := resolveSMSCapabilities("pingme", "https://example.invalid", []byte(`{}`)); !got.Extend || !got.Rental {
		t.Fatalf("PingMe adapter capabilities = %#v", got)
	}
	fallback := resolveSMSCapabilities("custom", "https://example.invalid", []byte(`{"supports_cancel":true}`))
	if !fallback.Cancel {
		t.Fatalf("unknown provider should use stored capabilities: %#v", fallback)
	}
}

func TestSMSListPublicProvidersUsesOpaqueChannelAllowlist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s)SELECT c\.code,c\.public_name.*WHERE lower\(c\.code\) IN \('channel_1','channel_2'\).*ORDER BY c\.sort_order,c\.id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"code", "public_name", "provider_code", "base_url", "enabled", "health_status", "credential_ref", "capabilities", "channel_enabled", "channel_visible", "channel_healthy",
		}).AddRow("channel_2", "Channel 2", "smspva", "https://example.invalid", false, "disabled", "", []byte(`{}`), false, true, false))

	providers, err := (&SMSService{db: db}).ListPublicProviders(context.Background())
	if err != nil {
		t.Fatalf("ListPublicProviders: %v", err)
	}
	if len(providers) != 1 || providers[0].Code != "channel_2" {
		t.Fatalf("providers=%#v, want only the opaque channel projection", providers)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("public provider query lost its closed allow-list: %v", err)
	}
}

func TestSMSActivateUsesRealActionContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "secret" || r.URL.Query().Get("action") != "getPrices" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"google":{"1":{"cost":0.8,"count":4}}}`))
	}))
	defer server.Close()
	p := providerFor("sms_activate", server.URL, "secret")
	quote, err := p.Quote(context.Background(), SMSQuoteRequest{ServiceCode: "google", CountryCode: "US", ProductType: "temporary"})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Stock != 4 || quote.Cost.String() != "0.8" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestFiveSIMRentalIsFailClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("5SIM rental flow must not call upstream, got %s", r.URL.String())
	}))
	defer server.Close()

	p := providerFor("5sim", server.URL, "secret")
	if p.Capabilities(context.Background()).Rental {
		t.Fatal("5SIM must not advertise rental")
	}
	if catalog, ok := p.(SMSProductServiceCatalogProvider); !ok {
		t.Fatal("5SIM product catalog adapter missing")
	} else if items, err := catalog.CatalogServicesForProduct(context.Background(), "rental", 1, "day"); err != nil || len(items) != 0 {
		t.Fatalf("rental services=%#v err=%v, want empty", items, err)
	}
	countryProvider, ok := p.(SMSProductServiceCountryProvider)
	if !ok {
		t.Fatal("5SIM country catalog adapter missing")
	}
	if countries, err := countryProvider.CountriesForServiceProduct(context.Background(), "openai", "rental", 1, "day"); err != nil || len(countries) != 0 {
		t.Fatalf("rental countries=%#v err=%v, want empty", countries, err)
	}
	operatorProvider, ok := p.(SMSProductOperatorProvider)
	if !ok {
		t.Fatal("5SIM operator catalog adapter missing")
	}
	if operators, err := operatorProvider.OperatorsForProduct(context.Background(), "usa", "openai", "rental", 0, 1, "day"); err != nil || len(operators) != 0 {
		t.Fatalf("rental operators=%#v err=%v, want empty", operators, err)
	}
	if _, err := p.Quote(context.Background(), SMSQuoteRequest{CountryCode: "usa", ServiceCode: "openai", ProductType: "rental"}); !errors.Is(err, ErrSMSProviderUnavailable) {
		t.Fatalf("rental quote err=%v, want provider unavailable", err)
	}
	if _, err := p.PurchaseRental(context.Background(), SMSPurchaseRequest{CountryCode: "usa", ServiceCode: "openai", ProductType: "rental"}); !errors.Is(err, ErrSMSProviderUnavailable) {
		t.Fatalf("rental purchase err=%v, want provider unavailable", err)
	}
}

func TestSMSPurchaseOutcomeAmbiguousForUpstream5xx(t *testing.T) {
	if !isSMSPurchaseOutcomeAmbiguous(&smsProviderHTTPError{Provider: "5sim", StatusCode: http.StatusServiceUnavailable, Detail: "gateway timeout"}) {
		t.Fatal("provider 5xx must be treated as an ambiguous purchase outcome")
	}
	if isSMSPurchaseOutcomeAmbiguous(&smsProviderHTTPError{Provider: "5sim", StatusCode: http.StatusBadRequest, Detail: "no free phones"}) {
		t.Fatal("definitive provider 4xx must not be treated as ambiguous")
	}
}

func TestFiveSIMBuyTreatsHTTP200NoFreePhonesAsDefinitiveFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/buy/activation/usa/any/openai" {
			t.Fatalf("unexpected purchase path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("no free phones"))
	}))
	defer server.Close()

	provider := providerFor("5sim", server.URL, "secret")
	_, err := provider.PurchaseTemporary(context.Background(), SMSPurchaseRequest{
		CountryCode:  "usa",
		ServiceCode:  "openai",
		OperatorCode: "any",
	})
	if err == nil {
		t.Fatal("expected no-free-phones purchase rejection")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no free phones") {
		t.Fatalf("purchase error lost provider rejection: %v", err)
	}
	if isSMSPurchaseOutcomeAmbiguous(err) {
		t.Fatalf("HTTP 200 no-free-phones must be definitive, got ambiguous error: %v", err)
	}
}

func TestSMSPurchaseHTTP400ServerOfflineIsDefinitive(t *testing.T) {
	err := &smsProviderHTTPError{Provider: "5sim", StatusCode: http.StatusBadRequest, Detail: "server offline"}
	if isSMSPurchaseOutcomeAmbiguous(err) {
		t.Fatal("documented 5SIM HTTP 400 purchase rejection must not enter reconciliation")
	}
}

func TestFiveSIMRecoversTimedOutPurchaseFromOrderHistory(t *testing.T) {
	startedAt := time.Date(2026, 9, 19, 2, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/orders" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("category") != "activation" || r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("reverse") != "true" {
			t.Fatalf("unexpected history query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Data":[{"id":1094361636,"phone":"+542243424387","operator":"Virtual62","product":"openai","price":0.05,"status":"PENDING","expires":"2026-09-19T02:45:00Z","created_at":"2026-09-19T02:30:02Z","country":"argentina"}],"Total":1}`))
	}))
	defer server.Close()

	p := providerFor("5sim", server.URL, "secret")
	recovery, ok := p.(SMSPurchaseRecoveryProvider)
	if !ok {
		t.Fatal("5SIM purchase recovery adapter missing")
	}
	result, err := recovery.RecoverTemporaryPurchase(context.Background(), SMSPurchaseRequest{
		ServiceCode:       "openai",
		CountryCode:       "argentina",
		OperatorCode:      "any",
		ProviderCostLimit: 0.05,
	}, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.ProviderOrderID != "1094361636" || result.PhoneNumber != "+542243424387" || result.ProviderCost != 0.05 || result.ProviderOperatorCode != "virtual62" {
		t.Fatalf("unexpected recovered purchase: %#v", result)
	}
}

func TestFiveSIMRecoveryFallsBackToOppositeHistoryOrder(t *testing.T) {
	startedAt := time.Date(2026, 9, 19, 9, 22, 40, 0, time.UTC)
	var reverses []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/orders" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		reverse := r.URL.Query().Get("reverse")
		reverses = append(reverses, reverse)
		w.Header().Set("Content-Type", "application/json")
		if reverse == "true" {
			_, _ = w.Write([]byte(`{"Data":[],"Total":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"Data":[{"id":1094737019,"phone":"+31685440669","operator":"Virtual66","product":"openai","price":0.18,"status":"RECEIVED","expires":"2026-09-19T09:42:40Z","created_at":"2026-09-19T09:22:42Z","country":"netherlands"}],"Total":1}`))
	}))
	defer server.Close()

	p := providerFor("5sim", server.URL, "secret")
	recovery, ok := p.(SMSPurchaseRecoveryProvider)
	if !ok {
		t.Fatal("5sim provider does not implement SMSPurchaseRecoveryProvider")
	}
	result, err := recovery.RecoverTemporaryPurchase(context.Background(), SMSPurchaseRequest{
		ServiceCode:  "openai",
		CountryCode:  "netherlands",
		OperatorCode: "virtual66",
	}, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.ProviderOrderID != "1094737019" || result.PhoneNumber != "+31685440669" {
		t.Fatalf("unexpected recovered purchase: %#v", result)
	}
	if len(reverses) != 2 || reverses[0] != "true" || reverses[1] != "false" {
		t.Fatalf("history directions=%v", reverses)
	}
}

func TestFiveSIMPreservesExactNumericOrderID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1094675152,"phone":"+4367870333648","expires":"2026-09-19T16:00:00Z","price":0.3425,"operator":"Virtual66"}`))
	}))
	defer server.Close()

	p := providerFor("5sim", server.URL, "secret")
	result, err := p.PurchaseTemporary(context.Background(), SMSPurchaseRequest{CountryCode: "austria", ServiceCode: "openai", OperatorCode: "virtual66"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderOrderID != "1094675152" {
		t.Fatalf("provider order id=%q, want exact integer string", result.ProviderOrderID)
	}
}

func TestFiveSIMRecoveryRejectsAmbiguousMatches(t *testing.T) {
	startedAt := time.Date(2026, 9, 19, 2, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Data":[
			{"id":1,"phone":"+1","operator":"op1","product":"openai","price":0.05,"status":"PENDING","expires":"2026-09-19T02:45:00Z","created_at":"2026-09-19T02:30:01Z","country":"usa"},
			{"id":2,"phone":"+2","operator":"op2","product":"openai","price":0.05,"status":"PENDING","expires":"2026-09-19T02:45:00Z","created_at":"2026-09-19T02:30:03Z","country":"usa"}
		]}`))
	}))
	defer server.Close()

	p := providerFor("5sim", server.URL, "secret")
	recovery, ok := p.(SMSPurchaseRecoveryProvider)
	if !ok {
		t.Fatal("5SIM purchase recovery adapter missing")
	}
	if _, err := recovery.RecoverTemporaryPurchase(context.Background(), SMSPurchaseRequest{ServiceCode: "openai", CountryCode: "usa", OperatorCode: "any", ProviderCostLimit: 0.05}, startedAt); err == nil {
		t.Fatal("ambiguous 5SIM recovery must fail closed")
	}
}

func TestSMSPlatform30DayRateUsesDeliveredMessagesAndCaches(t *testing.T) {
	clearSMSPlatformDeliveryStatsCache()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT\s+COUNT\(\*\) FILTER \(WHERE o\.delivery_outcome='success'\),\s+COUNT\(\*\) FILTER \(WHERE o\.delivery_outcome='failed'\)`).
		WithArgs("5sim", "openai", "US", "virtual58").
		WillReturnRows(sqlmock.NewRows([]string{"successes", "failures"}).AddRow(18, 2))

	svc := &SMSService{db: db}
	grade, rate, sample := svc.successGrade(context.Background(), "5sim", "openai", "US", "virtual58")
	if grade != "A" || rate == nil || math.Abs(*rate-0.9) > 0.000001 || sample != 20 {
		t.Fatalf("grade=%q rate=%v sample=%d", grade, rate, sample)
	}
	// A second read of the same dimension must hit the short-lived cache.
	grade, rate, sample = svc.successGrade(context.Background(), "5sim", "openai", "US", "virtual58")
	if grade != "A" || rate == nil || sample != 20 {
		t.Fatalf("cached grade=%q rate=%v sample=%d", grade, rate, sample)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSPlatform30DayRateRequiresMinimumSample(t *testing.T) {
	clearSMSPlatformDeliveryStatsCache()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT\s+COUNT\(\*\) FILTER \(WHERE o\.delivery_outcome='success'\),\s+COUNT\(\*\) FILTER \(WHERE o\.delivery_outcome='failed'\)`).
		WithArgs("5sim", "telegram", "DE", "any").
		WillReturnRows(sqlmock.NewRows([]string{"successes", "failures"}).AddRow(9, 1))

	svc := &SMSService{db: db}
	grade, rate, sample := svc.successGrade(context.Background(), "5sim", "telegram", "DE", "any")
	if grade != "" || rate != nil || sample != 10 {
		t.Fatalf("grade=%q rate=%v sample=%d, want unpublished 10-sample rate", grade, rate, sample)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFiveSIMCountriesParseProductFilteredPricesShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/guest/prices":
			if r.URL.Query().Get("product") != "openai" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"openai":{"usa":{"att":{"cost":0.5,"count":4,"rate":95},"tmobile":{"cost":0.7,"count":3,"rate":90}}}}`))
		case "/guest/countries":
			_, _ = w.Write([]byte(`{"usa":{"iso":{"us":1},"text_en":"United States"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := providerFor("5sim", server.URL, "")
	countryProvider, ok := p.(SMSProductServiceCountryProvider)
	if !ok {
		t.Fatal("5SIM provider does not implement product-specific country catalog")
	}
	countries, err := countryProvider.CountriesForServiceProduct(context.Background(), "openai", "temporary", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(countries) != 1 || countries[0].ISO2 != "US" || countries[0].Stock != 7 || countries[0].ProviderCost != 0.5 || countries[0].ConversionRate != 95 || countries[0].RecommendedOperator != "att" || countries[0].RecommendedOperatorStock != 4 || countries[0].RecommendedProviderCost != 0.5 {
		t.Fatalf("unexpected countries: %#v", countries)
	}
}

func TestFiveSIMQuoteParsesCurrentGuestResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/guest/products/usa/any" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("guest quote must not require bearer auth")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"telegram":{"Category":"activation","Qty":93849,"Price":0.77}}`))
	}))
	defer server.Close()

	quote, err := providerFor("5sim", server.URL, "").Quote(context.Background(), SMSQuoteRequest{ServiceCode: "telegram", CountryCode: "usa", ProductType: "temporary", OperatorCode: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Stock != 93849 || quote.Cost.String() != "0.77" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestFiveSIMCatalogParsesGuestCountries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/guest/countries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usa":{"iso":{"us":1},"text_en":"USA"}}`))
	}))
	defer server.Close()

	provider := providerFor("5sim", server.URL, "")
	catalog, ok := provider.(SMSCatalogProvider)
	if !ok {
		t.Fatal("5SIM provider does not implement SMSCatalogProvider")
	}
	services, countries, err := catalog.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 0 || len(countries) != 1 || countries[0].ISO2 != "US" {
		t.Fatalf("unexpected catalog: services=%#v countries=%#v", services, countries)
	}
}

func TestSMSPoolQuoteUsesReadOnlyPriceEndpointAndKeyAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/request/price" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		if r.URL.Query().Get("key") != "secret" || r.URL.Query().Get("country") != "US" || r.URL.Query().Get("service") != "google" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("SMSPool must not receive bearer auth")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"price":"0.8","available":4}`))
	}))
	defer server.Close()

	quote, err := providerFor("smspool", server.URL, "secret").Quote(context.Background(), SMSQuoteRequest{ServiceCode: "google", CountryCode: "US", ProductType: "temporary"})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Stock != 4 || quote.Cost.String() != "0.8" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestSMSProviderConnectionChecksAreReadOnly(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		path  string
		query string
	}{
		{name: "5sim", code: "5sim", path: "/user/profile"},
		{name: "onlinesim", code: "onlinesim", path: "/getBalance.php", query: "apikey"},
		{name: "smspool", code: "smspool", path: "/request/balance", query: "key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tt.path {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					return
				}
				if tt.query != "" && r.URL.Query().Get(tt.query) != "secret" {
					t.Errorf("missing %s credential: %s", tt.query, r.URL.RawQuery)
					return
				}
				if r.Header.Get("Authorization") != "" && tt.code == "smspool" {
					t.Errorf("SMSPool must not receive bearer auth")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"balance":1,"response":"ACCESS_OK"}`))
			}))
			defer server.Close()
			provider := providerFor(tt.code, server.URL, "secret")
			checker, ok := provider.(smsProviderHealthChecker)
			if !ok {
				t.Fatalf("%s does not expose a health checker", tt.code)
			}
			if err := checker.TestConnection(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSMSPublicDTODoesNotExposeProviderFields(t *testing.T) {
	payload, err := json.Marshal(struct {
		Order    SMSOrder
		Channel  SMSPublicChannel
		Service  SMSSvcCatalogItem
		Country  SMSCountryCatalogItem
		Operator SMSOperatorOption
	}{
		Order:    SMSOrder{ID: "public", ChannelCode: "channel_1", Price: 1.2},
		Channel:  SMSPublicChannel{Code: "channel_1", ProviderCost: 0.8, GradeMultiplier: 1.2, GradeFixedMarkup: 0.1},
		Service:  SMSSvcCatalogItem{Code: "telegram", ProviderCost: 0.7},
		Country:  SMSCountryCatalogItem{ISO2: "US", ProviderCost: 0.7},
		Operator: SMSOperatorOption{Code: "any", ProviderCost: 0.7},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"provider_id", "provider_order_id", "provider_cost", "provider_raw_error", "credential"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public DTO leaked %q: %s", forbidden, text)
		}
	}
}

func TestSMSUnknownProviderStatesDoNotBecomeSuccess(t *testing.T) {
	if got := normalizeSMSStatus("STATUS_WAIT_CODE"); got != "active" {
		t.Fatalf("waiting status = %q", got)
	}
	if got := normalizeSMSStatus("provider_timeout"); got != "provider_unknown" {
		t.Fatalf("unknown status = %q", got)
	}
	if got := normalizeSMSStatus("canceled"); got != "cancelled" {
		t.Fatalf("canceled status = %q", got)
	}
	if got := normalizeSMSStatus("TIMEOUT"); got != "expired" {
		t.Fatalf("5SIM timeout status = %q, want expired", got)
	}
}

func TestSMSProviderTerminalStateNeedsVerificationCode(t *testing.T) {
	if got := smsStatusFromProvider(&SMSStatusResult{Status: "completed"}); got != "active" {
		t.Fatalf("terminal provider state without a code = %q, want active", got)
	}
	if got := smsStatusFromProvider(&SMSStatusResult{Status: "completed", Messages: []string{"Your code is 482913"}}); got != "completed" {
		t.Fatalf("provider message with a code = %q, want completed", got)
	}
	if got := smsStatusFromProvider(nil); got != "provider_unknown" {
		t.Fatalf("nil provider result = %q, want provider_unknown", got)
	}
}

func TestSMSProviderExpiryWaitsForPlatformExpiry(t *testing.T) {
	now := time.Date(2026, 9, 19, 15, 10, 0, 0, time.UTC)
	platformExpiry := now.Add(5 * time.Minute)
	if got := deferSMSProviderExpiry("temporary", "expired", "", &platformExpiry, now); got != "active" {
		t.Fatalf("provider timeout before platform expiry = %q, want active", got)
	}
	if got := deferSMSProviderExpiry("temporary", "expired", "", &platformExpiry, platformExpiry); got != "expired" {
		t.Fatalf("provider timeout at platform expiry = %q, want expired", got)
	}
	if got := deferSMSProviderExpiry("temporary", "cancelled", "", &platformExpiry, now); got != "active" {
		t.Fatalf("provider cancellation before platform expiry = %q, want active", got)
	}
	if got := deferSMSProviderExpiry("temporary", "TIMEOUT", "", &platformExpiry, now); got != "active" {
		t.Fatalf("raw provider timeout before platform expiry = %q, want active", got)
	}
	if got := deferSMSProviderExpiry("temporary", "CANCELED", "", &platformExpiry, now); got != "active" {
		t.Fatalf("raw provider cancellation before platform expiry = %q, want active", got)
	}
	if got := deferSMSProviderExpiry("temporary", "cancelled", smsReconciliationCancel, &platformExpiry, now); got != "cancelled" {
		t.Fatalf("manual cancellation before platform expiry = %q, want cancelled", got)
	}
	if got := deferSMSProviderExpiry("rental", "expired", "", &platformExpiry, now); got != "expired" {
		t.Fatalf("rental provider expiry = %q, want expired", got)
	}
}

func TestSMSCodeExtractionHandlesPunctuationAndRejectsLongNumbers(t *testing.T) {
	if got := extractSMSCode("验证码：482913"); got != "482913" {
		t.Fatalf("Chinese punctuation code = %q", got)
	}
	if got := extractSMSCode("Your code is 482913."); got != "482913" {
		t.Fatalf("punctuated code = %q", got)
	}
	if got := extractSMSCode("Reference 123456789"); got != "" {
		t.Fatalf("long numeric reference = %q, want no code", got)
	}
}

func TestSMSTimeoutDetection(t *testing.T) {
	if !isSMSProviderTimeout(context.DeadlineExceeded) {
		t.Fatal("deadline should be treated as provider timeout")
	}
	if isSMSProviderTimeout(errors.New("bad request")) {
		t.Fatal("ordinary errors must not be treated as timeout")
	}
	if _, ok := rentalDuration(2, "hour"); !ok {
		t.Fatal("hour duration should be accepted")
	}
	if got, _ := rentalDuration(2, "hour"); got != 2*time.Hour {
		t.Fatalf("unexpected duration: %s", got)
	}
}

func TestSMSReleaseHoldIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status=\$1.*settlement_status='held'.*RETURNING reserved_amount`).
		WithArgs("failed", "not_requested", "purchase failed", "released", int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(1.25))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1,frozen_balance`).
		WithArgs(1.25, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status=\$1.*settlement_status='held'.*RETURNING reserved_amount`).
		WithArgs("failed", "not_requested", "purchase failed", "released", int64(11)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	svc := &SMSService{db: db}
	if err = svc.releaseSMSHold(context.Background(), 11, 7, "failed", "purchase failed"); err != nil {
		t.Fatal(err)
	}
	if err = svc.releaseSMSHold(context.Background(), 11, 7, "failed", "purchase failed"); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSCaptureIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET captured_amount=reserved_amount.*settlement_status='held'.*RETURNING reserved_amount`).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(2.5))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).WithArgs(2.5, int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET captured_amount=reserved_amount.*settlement_status='held'.*RETURNING reserved_amount`).
		WithArgs(int64(12)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	svc := &SMSService{db: db}
	if err = svc.captureSMSSettlement(context.Background(), 12, 8); err != nil {
		t.Fatal(err)
	}
	if err = svc.captureSMSSettlement(context.Background(), 12, 8); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSActivateOrderPersistsFractionalProviderCost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status='active'.*provider_cost_snapshot=CASE WHEN \$4::numeric>0 THEN \$4::numeric`).
		WithArgs("1094764603", "+44 7536658308", nil, 0.2, "virtual66", int64(28)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(5.8))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).
		WithArgs(5.8, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE sms_order_services`).
		WithArgs("1094764603", int64(28)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	svc := &SMSService{db: db}
	if err := svc.activateSMSOrder(context.Background(), 28, 1, "1094764603", "+44 7536658308", nil, 0.2, "virtual66"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMS5SIMRefundReconciliationCallsCancelOnlyOnce(t *testing.T) {
	var cancelCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/cancel/order-1" {
			t.Fatalf("unexpected provider path: %s", r.URL.Path)
		}
		cancelCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	t.Setenv("SMS_5SIM_API_KEY", "secret")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`UPDATE sms_orders SET provider_refund_status=\$1`).
		WithArgs("succeeded", "", int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status=\$1.*settlement_status='captured'.*RETURNING reserved_amount`).
		WithArgs("refunded", "approved", "provider refund confirmed during reconciliation", "refunded", int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(0.75))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1,updated_at`).
		WithArgs(0.75, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := &SMSService{db: db}
	if err := svc.reconcileSMSAction(context.Background(), 11, 7, "temporary", "order-1", "5sim", server.URL, "env:SMS_5SIM_API_KEY", "captured", smsReconciliationRefund, time.Now()); err != nil {
		t.Fatal(err)
	}
	if cancelCalls != 1 {
		t.Fatalf("5SIM cancellation/refund endpoint called %d times, want 1", cancelCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSRefundCaptureIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status=\$1.*settlement_status='captured'.*RETURNING reserved_amount`).
		WithArgs("refunded", "approved", "provider confirmed", "refunded", int64(13)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(.75))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1,updated_at`).WithArgs(.75, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status=\$1.*settlement_status='captured'.*RETURNING reserved_amount`).
		WithArgs("refunded", "approved", "provider confirmed", "refunded", int64(13)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	svc := &SMSService{db: db}
	if err = svc.refundSMSCapture(context.Background(), 13, 9, "refunded", "provider confirmed"); err != nil {
		t.Fatal(err)
	}
	if err = svc.refundSMSCapture(context.Background(), 13, 9, "refunded", "provider confirmed"); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSQuoteLookupIsUserScopedAndRejectsConsumedQuote(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`FROM sms_quotes q.*WHERE q.id=\$1 AND q.user_id=\$2 AND q.consumed_at IS NULL`).
		WithArgs("quote-1", int64(42)).
		WillReturnError(sql.ErrNoRows)

	svc := &SMSService{db: db}
	if _, err = svc.loadQuote(context.Background(), 42, "quote-1"); !errors.Is(err, ErrSMSQuoteInvalid) {
		t.Fatalf("loadQuote error=%v, want ErrSMSQuoteInvalid", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSPricingSettingsDriveUnknownAndGradeFormulas(t *testing.T) {
	pricing := SMSPricingSettings{
		CostMultiplier:          7,
		FixedMarkup:             1,
		UnknownGradeMultiplier:  2,
		UnknownGradeFixedMarkup: 2,
		GradeMultipliers:        map[string]float64{"S": 1.25},
		GradeFixedMarkups:       map[string]float64{"S": 3},
	}
	svc := &SMSService{}
	unknownMultiplier, unknownFixed := svc.gradePricing(context.Background(), "", pricing)
	if unknownMultiplier != 2 || unknownFixed != 3 {
		t.Fatalf("unknown pricing = multiplier %v fixed %v, want 2 and 3", unknownMultiplier, unknownFixed)
	}
	unknownSale := 0.6*pricing.CostMultiplier*unknownMultiplier + unknownFixed
	if math.Abs(unknownSale-11.4) > 1e-9 {
		t.Fatalf("unknown sale price = %v, want 11.4", unknownSale)
	}

	gradeMultiplier, gradeFixed := svc.gradePricing(context.Background(), "S", pricing)
	if gradeMultiplier != 1.25 || gradeFixed != 4 {
		t.Fatalf("S pricing = multiplier %v fixed %v, want 1.25 and 4", gradeMultiplier, gradeFixed)
	}
	gradeSale := 0.6*pricing.CostMultiplier*gradeMultiplier + gradeFixed
	if math.Abs(gradeSale-9.25) > 1e-9 {
		t.Fatalf("S sale price = %v, want 9.25", gradeSale)
	}
}

func TestSMSBatchPurchaseLimitDefaultsAndValidates(t *testing.T) {
	defaults := defaultSMSPricingSettings()
	if defaults.BatchPurchaseLimit != defaultSMSBatchPurchaseLimit {
		t.Fatalf("default batch purchase limit=%d, want %d", defaults.BatchPurchaseLimit, defaultSMSBatchPurchaseLimit)
	}
	for _, limit := range []int{1, maxSMSBatchPurchaseLimit} {
		settings := defaults
		settings.BatchPurchaseLimit = limit
		if err := settings.Validate(); err != nil {
			t.Fatalf("batch purchase limit %d should be valid: %v", limit, err)
		}
	}
	for _, limit := range []int{0, maxSMSBatchPurchaseLimit + 1} {
		settings := defaults
		settings.BatchPurchaseLimit = limit
		if err := settings.Validate(); err == nil {
			t.Fatalf("batch purchase limit %d should be rejected", limit)
		}
	}
}

func TestSMSBatchPurchaseLimitReadsConfiguredSettingAndRejectsOverflow(t *testing.T) {
	settingService := NewSettingService(&smsBatchLimitSettingRepoStub{value: `{"batch_purchase_limit":2}`}, nil)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer func() { _ = db.Close() }()
	svc := &SMSService{db: db, settings: settingService}
	settings, err := svc.GetPricingSettings(context.Background())
	if err != nil {
		t.Fatalf("GetPricingSettings error: %v", err)
	}
	if settings.BatchPurchaseLimit != 2 {
		t.Fatalf("configured batch purchase limit=%d, want 2", settings.BatchPurchaseLimit)
	}
	items := make([]SMSPurchaseRequest, 3)
	mock.ExpectQuery(`SELECT id FROM sms_orders WHERE user_id=\$1 AND idempotency_key=\$2`).
		WithArgs(int64(1), "batch-key-0").
		WillReturnError(sql.ErrNoRows)
	if _, err := svc.PurchaseBatch(context.Background(), 1, items, "batch-key", nil); err == nil || err.Error() != "batch size must be between 1 and 2" {
		t.Fatalf("PurchaseBatch overflow error=%v, want configured limit error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet database expectations: %v", err)
	}
}

func TestSMSBatchPurchaseLimitPreservesLegacyDefaultAndRejectsInvalidUpdates(t *testing.T) {
	settingService := NewSettingService(&smsBatchLimitSettingRepoStub{value: `{"cost_multiplier":1.4}`}, nil)
	svc := &SMSService{settings: settingService}
	settings, err := svc.GetPricingSettings(context.Background())
	if err != nil {
		t.Fatalf("GetPricingSettings error: %v", err)
	}
	if settings.BatchPurchaseLimit != defaultSMSBatchPurchaseLimit {
		t.Fatalf("legacy batch purchase limit=%d, want %d", settings.BatchPurchaseLimit, defaultSMSBatchPurchaseLimit)
	}
	settings.BatchPurchaseLimit = 0
	if err := svc.SetPricingSettings(context.Background(), settings); err == nil {
		t.Fatal("SetPricingSettings should reject an explicit zero batch purchase limit")
	}
}

func TestSMSBatchPurchaseLimitFailsClosedWhenSettingsCannotBeRead(t *testing.T) {
	settingService := NewSettingService(&smsBatchLimitSettingRepoStub{value: `{invalid-json`}, nil)
	svc := &SMSService{settings: settingService}
	if _, err := svc.PurchaseBatch(context.Background(), 1, []SMSPurchaseRequest{{}}, "batch-key", nil); err == nil || err.Error() != "invalid SMS pricing settings" {
		t.Fatalf("PurchaseBatch settings error=%v, want invalid settings error", err)
	}
}

func TestSMSBatchPurchaseLimitAllowsCompleteIdempotentReplayAfterLimitReduction(t *testing.T) {
	settingService := NewSettingService(&smsBatchLimitSettingRepoStub{value: `{"batch_purchase_limit":1}`}, nil)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer func() { _ = db.Close() }()
	svc := &SMSService{db: db, settings: settingService}
	createdAt := time.Now().Add(-time.Minute)
	orderColumns := []string{"public_id", "product_type", "status", "reconciliation_action", "channel_code", "channel_name", "service_code", "country_code", "phone_number", "operator_code", "voice_mode", "sale_price_snapshot", "success_rate_snapshot", "success_rate_grade_snapshot", "success_rate_source_snapshot", "refund_status", "refund_reason", "expires_at", "created_at", "provider_code", "base_url", "capabilities"}
	for i, orderID := range []int64{101, 102} {
		itemKey := fmt.Sprintf("replay-key-%d", i)
		publicID := fmt.Sprintf("00000000-0000-0000-0000-%012d", orderID)
		mock.ExpectQuery(`SELECT id FROM sms_orders WHERE user_id=\$1 AND idempotency_key=\$2`).
			WithArgs(int64(1), itemKey).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(orderID))
		mock.ExpectQuery(`SELECT o.public_id::text`).
			WithArgs(int64(1), orderID).
			WillReturnRows(sqlmock.NewRows(orderColumns).AddRow(publicID, "temporary", "active", "", "channel_1", "Channel 1", "openai", "US", "+12025550123", "any", 0, 1.25, nil, "", "unavailable", "not_requested", "", nil, createdAt, "5sim", "https://5sim.net", []byte(`{}`)))
		mock.ExpectQuery(`SELECT id,message_text,verification_code,received_at FROM sms_messages`).
			WithArgs(orderID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "message_text", "verification_code", "received_at"}))
	}

	orders, err := svc.PurchaseBatch(context.Background(), 1, []SMSPurchaseRequest{{}, {}}, "replay-key", nil)
	if err != nil {
		t.Fatalf("PurchaseBatch replay error: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("replayed order count=%d, want 2", len(orders))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet database expectations: %v", err)
	}
}

func TestSMSOrderExpiryUsesPlatformPolicyOrSafeDefaults(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	providerExpiry := now.Add(7 * time.Minute)
	if got := smsOrderExpiresAt(now, "temporary", 0, "", &providerExpiry); !got.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("temporary platform expiry=%v, want %v", got, now.Add(10*time.Minute))
	}
	longProviderExpiry := now.Add(15 * time.Minute)
	if got := smsOrderExpiresAt(now, "temporary", 0, "", &longProviderExpiry, 3*time.Minute); !got.Equal(now.Add(3 * time.Minute)) {
		t.Fatalf("platform temporary expiry=%v, want %v", got, now.Add(3*time.Minute))
	}
	if got := smsOrderExpiresAt(now, "temporary", 0, "", nil); !got.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("temporary expiry=%v", got)
	}
	if got := smsOrderExpiresAt(now, "rental", 2, "hour", nil); !got.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("rental expiry=%v", got)
	}
	if got := smsOrderExpiresAt(now, "rental", 0, "", nil); !got.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("fallback rental expiry=%v", got)
	}
}

func TestSMSOrderCancellationWindowIsIndependentFromExpiry(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 5, 0, 0, time.UTC)
	created := now.Add(-30 * time.Second)
	expires := now.Add(9 * time.Minute)
	order := SMSOrder{
		ProductType: "temporary",
		Status:      "active",
		CreatedAt:   created,
		ExpiresAt:   &expires,
		Capabilities: SMSProviderCapabilities{
			Refund: true,
		},
	}
	setSMSOrderCancellationState(&order, SMSPricingSettings{SelfServiceCancelAfterMinutes: 1}, now)
	if order.CanCancel {
		t.Fatal("order should remain non-cancellable during the safety window")
	}
	if order.CancelAvailableAt == nil || !order.CancelAvailableAt.Equal(created.Add(time.Minute)) {
		t.Fatalf("cancel_available_at=%v, want %v", order.CancelAvailableAt, created.Add(time.Minute))
	}
	if order.CancelRemainingSeconds != 30 {
		t.Fatalf("cancel_remaining_seconds=%d, want 30", order.CancelRemainingSeconds)
	}

	now = created.Add(2 * time.Minute)
	setSMSOrderCancellationState(&order, SMSPricingSettings{SelfServiceCancelAfterMinutes: 1}, now)
	if !order.CanCancel || order.CancelRemainingSeconds != 0 {
		t.Fatalf("order should be cancellable after safety window: can=%v remaining=%d", order.CanCancel, order.CancelRemainingSeconds)
	}

	order.Status = "cancelled"
	setSMSOrderCancellationState(&order, SMSPricingSettings{SelfServiceCancelAfterMinutes: 1}, now)
	if order.CanCancel || order.CancelAvailableAt != nil || order.CancelRemainingSeconds != 0 {
		t.Fatalf("terminal order should have no cancellation window: %#v", order)
	}
}

func TestSMSOrderRemainingStopsDuringRefundReconciliation(t *testing.T) {
	now := time.Now()
	expires := now.Add(5 * time.Minute)
	order := SMSOrder{Status: "reconciling", ReconciliationAction: smsReconciliationRefund, ExpiresAt: &expires}
	setSMSOrderRemaining(&order)
	if order.RemainingSeconds != 0 {
		t.Fatalf("refund reconciliation remaining=%d, want 0", order.RemainingSeconds)
	}
}

func TestSMSExpiryReleasesHeldSettlement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT settlement_status FROM sms_orders WHERE id=\$1`).
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"settlement_status"}).AddRow("held"))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET status=\$1,refund_status=\$2.*settlement_status='held'.*RETURNING reserved_amount`).
		WithArgs("refunded", "approved", "expired while provider refund was confirmed", "released", int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(1.25))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1,frozen_balance`).
		WithArgs(1.25, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := &SMSService{db: db}
	if err := svc.settleSMSExpiry(context.Background(), 44, 9, "refunded", "approved", "expired while provider refund was confirmed"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSMappingValidationFailsClosed(t *testing.T) {
	svc := &SMSService{}
	if err := svc.UpsertProviderServiceMapping(context.Background(), 1, 2, "", "", true, false, true); err == nil {
		t.Fatal("enabled service mapping without provider code must fail")
	}
	if err := svc.UpsertProviderCountryMapping(context.Background(), 1, 2, "", "", true); err == nil {
		t.Fatal("enabled country mapping without provider identifier must fail")
	}
}

func TestSMSReservationStartsInPurchaseReconciliationState(t *testing.T) {
	const insertSQL = "INSERT INTO sms_orders"
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectQuery(insertSQL).
		WithArgs(int64(7), int64(1), int64(2), int64(3), int64(4), "temporary", "any", 0, .4, .6, nil, "unavailable", "", 1.0, "idem-crash-safe").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(17)))
	mock.ExpectExec(`UPDATE sms_quotes SET consumed_at=NOW\(\),consumed_order_id=\$1`).
		WithArgs(int64(17), "quote-17", int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET balance=balance-\$1,frozen_balance`).
		WithArgs(.6, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := &SMSService{db: db}
	selected := &SMSPublicChannel{ProviderCost: .4, SalePrice: .6, SuccessRateSource: "unavailable", GradeMultiplier: 1}
	if _, err := svc.reserveSMSPurchase(context.Background(), 7, 1, 2, 3, 4, SMSPurchaseRequest{ProductType: "temporary", QuoteID: "quote-17"}, selected, "idem-crash-safe"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSReservationConsumesQuoteAndFreezesBalanceInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO sms_orders .* RETURNING id`).
		WithArgs(int64(7), int64(1), int64(2), int64(3), int64(4), "temporary", "any", 0, .4, .6, nil, "unavailable", "", 1.0, "idem-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(15)))
	mock.ExpectExec(`UPDATE sms_quotes SET consumed_at=NOW\(\),consumed_order_id=\$1`).
		WithArgs(int64(15), "quote-15", int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET balance=balance-\$1,frozen_balance`).
		WithArgs(.6, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := &SMSService{db: db}
	selected := &SMSPublicChannel{ProviderCost: .4, SalePrice: .6, SuccessRateSource: "unavailable", GradeMultiplier: 1}
	id, err := svc.reserveSMSPurchase(context.Background(), 7, 1, 2, 3, 4, SMSPurchaseRequest{ProductType: "temporary", QuoteID: "quote-15"}, selected, "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if id != 15 {
		t.Fatalf("order id=%d", id)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSReservationRollsBackWhenQuoteWasAlreadyConsumed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO sms_orders .* RETURNING id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(16)))
	mock.ExpectExec(`UPDATE sms_quotes SET consumed_at=NOW\(\),consumed_order_id=\$1`).
		WithArgs(int64(16), "quote-used", int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	svc := &SMSService{db: db}
	selected := &SMSPublicChannel{ProviderCost: .4, SalePrice: .6, SuccessRateSource: "unavailable", GradeMultiplier: 1}
	_, err = svc.reserveSMSPurchase(context.Background(), 7, 1, 2, 3, 4, SMSPurchaseRequest{ProductType: "temporary", QuoteID: "quote-used"}, selected, "idem-2")
	if !errors.Is(err, ErrSMSQuoteExpired) {
		t.Fatalf("reserve error=%v, want ErrSMSQuoteExpired", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFiveSIMOperatorsUsePricesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/guest/products/usa/any":
			_, _ = w.Write([]byte(`{"telegram":{"Category":"activation","Qty":10,"Price":0.8}}`))
		case "/guest/prices":
			if r.URL.Query().Get("country") != "usa" || r.URL.Query().Get("product") != "telegram" {
				t.Errorf("unexpected prices query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"usa":{"telegram":{"att":{"cost":0.9,"count":4,"rate":95},"tmobile":{"cost":1.1,"count":2,"rate":91}}}}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	provider := providerFor("5sim", server.URL, "")
	operatorProvider, ok := provider.(SMSProductOperatorProvider)
	if !ok {
		t.Fatal("5SIM provider does not implement product operator catalog")
	}
	ops, err := operatorProvider.OperatorsForProduct(context.Background(), "usa", "telegram", "temporary", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 || ops[0].Code != "any" || ops[1].Code != "att" || ops[1].ProviderRate != 95 || ops[2].Code != "tmobile" || ops[2].ProviderRate != 91 {
		t.Fatalf("unexpected operators: %#v", ops)
	}
}

func TestFiveSIMPurchaseFinishAndBanContracts(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/user/buy/activation/") {
			if got := r.URL.Query().Get("maxPrice"); got != "0.6" {
				t.Errorf("maxPrice=%q want 0.6", got)
			}
			_, _ = w.Write([]byte(`{"id":123,"phone":"+12025550123","expires":"2030-01-01T00:00:00Z","price":0.55,"operator":"Virtual58"}`))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()
	p := providerFor("5sim", server.URL, "secret")
	result, err := p.PurchaseTemporary(context.Background(), SMSPurchaseRequest{CountryCode: "usa", ServiceCode: "telegram", OperatorCode: "any", ProviderCostLimit: 0.6})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderOrderID != "123" || result.PhoneNumber != "+12025550123" || result.ProviderCost != 0.55 || result.ProviderOperatorCode != "virtual58" {
		t.Fatalf("unexpected purchase: %#v", result)
	}
	action, ok := p.(SMSOrderActionProvider)
	if !ok {
		t.Fatal("5SIM provider does not implement order actions")
	}
	if err = action.FinishTemporary(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}
	if err = action.BanTemporary(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}
	want := []string{"/user/buy/activation/usa/any/telegram", "/user/finish/123", "/user/ban/123"}
	if len(paths) != len(want) {
		t.Fatalf("paths=%v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("path[%d]=%q want %q", i, paths[i], want[i])
		}
	}
}

func TestFiveSIMStatusReturnsActualCostAndOperator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/check/123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"PENDING","phone":"+12025550123","price":0.6,"operator":"Virtual58","sms":[]}`))
	}))
	defer server.Close()

	result, err := providerFor("5sim", server.URL, "secret").GetTemporaryStatus(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderCost != 0.6 || result.ProviderOperatorCode != "virtual58" {
		t.Fatalf("unexpected status actuals: %#v", result)
	}
}

func TestSMSPVAQuoteUsesSelectedOperatorPriceAndStock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "secret" {
			t.Errorf("missing apikey header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/activation/serviceprice/US/opt20":
			_, _ = w.Write([]byte(`{"statusCode":200,"data":{"price":"0.80","priceByOperators":{"att":"1.25","tmobile":"0.95"}}}`))
		case "/activation/countnumbers/US":
			_, _ = w.Write([]byte(`{"statusCode":200,"data":[{"operator":"att","country":"US","services":[{"service":"opt20","total":3}]},{"operator":"tmobile","country":"US","services":[{"service":"opt20","total":9}]}]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	p := providerFor("smspva", server.URL, "secret")
	quote, err := p.Quote(context.Background(), SMSQuoteRequest{CountryCode: "US", ServiceCode: "opt20", ProductType: "temporary", OperatorCode: "att"})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Cost.String() != "1.25" || quote.Stock != 3 {
		t.Fatalf("unexpected selected-operator quote: %#v", quote)
	}
}

func TestSMSPVAPurchaseCarriesOperatorAndVoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/activation/number/US/telegram/att" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("voice") != "2" {
			t.Errorf("voice=%s", r.URL.Query().Get("voice"))
		}
		if r.Header.Get("apikey") != "secret" {
			t.Errorf("missing apikey header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statusCode":200,"data":{"orderId":"77","phoneNumber":"+12025550199","orderExpireIn":600}}`))
	}))
	defer server.Close()
	p := providerFor("smspva", server.URL, "secret")
	got, err := p.PurchaseTemporary(context.Background(), SMSPurchaseRequest{CountryCode: "US", ServiceCode: "telegram", OperatorCode: "att", VoiceMode: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderOrderID != "77" || got.PhoneNumber != "+12025550199" {
		t.Fatalf("unexpected purchase: %#v", got)
	}
}

func TestSMSPVARentalLifecycleUsesOfficialRentContract(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/rent.php" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("apikey") != "secret" {
			t.Errorf("missing rental apikey")
		}
		methods = append(methods, r.URL.Query().Get("method"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "create":
			if r.URL.Query().Get("dtype") != "week" || r.URL.Query().Get("dcount") != "1" || r.URL.Query().Get("provider") != "att" {
				t.Errorf("unexpected create query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"status":1,"data":{"id":"501","pnumber":"+12025550001","until":1893456000}}`))
		case "activate":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"id":"501"}]}`))
		case "orders":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"id":"501","scode":"telegram","sname":"Telegram","state":"0","pnumber":"+12025550001","cname":"US","hasnewsms":true,"until":1893456000,"canprolong":true,"canprolongmax":6,"canprolonguntil":1893459600,"lastonline":1893455000}]}`))
		case "sms":
			_, _ = w.Write([]byte(`{"status":1,"data":{"SmsList":[{"text":"Your code is 482913"}],"OtherSms":[]}}`))
		case "prolong":
			_, _ = w.Write([]byte(`{"status":1,"data":{}}`))
		case "delete":
			_, _ = w.Write([]byte(`{"status":1,"data":{}}`))
		default:
			t.Errorf("unexpected method: %s", r.URL.Query().Get("method"))
		}
	}))
	defer server.Close()
	p := providerFor("smspva", server.URL, "secret")
	order, err := p.PurchaseRental(context.Background(), SMSPurchaseRequest{CountryCode: "US", ServiceCode: "telegram", OperatorCode: "att", DurationValue: 1, DurationUnit: "week"})
	if err != nil {
		t.Fatal(err)
	}
	if order.ProviderOrderID != "501" {
		t.Fatalf("order=%#v", order)
	}
	status, err := p.GetRentalStatus(context.Background(), "501")
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Messages) != 1 || status.Messages[0] != "Your code is 482913" {
		t.Fatalf("status=%#v", status)
	}
	if err = p.ExtendRental(context.Background(), "501", 1, "week"); err != nil {
		t.Fatal(err)
	}
	if err = p.CancelRental(context.Background(), "501"); err != nil {
		t.Fatal(err)
	}
	want := []string{"create", "activate", "activate", "orders", "sms", "prolong", "delete"}
	if len(methods) != len(want) {
		t.Fatalf("methods=%v", methods)
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("method[%d]=%q want %q", i, methods[i], want[i])
		}
	}
}

func TestSMSPVARentalPeriodValidation(t *testing.T) {
	if _, _, _, err := smsPVARentalPeriod(1, "hour"); err == nil {
		t.Fatal("hour rental purchase must be rejected")
	}
	if kind, count, days, err := smsPVARentalPeriod(2, "week"); err != nil || kind != "week" || count != 2 || days != 14 {
		t.Fatalf("period=%q %d %d err=%v", kind, count, days, err)
	}
	if _, _, err := smsPVARentalExtensionPeriod(7, "day"); err == nil {
		t.Fatal("daily extension above six days must fail")
	}
}

func TestSMSPVARentalRequestFailsClosedWithoutCredential(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	p := providerFor("smspva", server.URL, "")
	_, err := p.PurchaseRental(context.Background(), SMSPurchaseRequest{
		CountryCode: "US", ServiceCode: "telegram", DurationValue: 1, DurationUnit: "week",
	})
	if !errors.Is(err, ErrSMSProviderCredentialMissing) {
		t.Fatalf("error=%v, want missing-credential error", err)
	}
	if called {
		t.Fatal("rental request must not reach upstream without an API key")
	}
}

func TestSMSPVAEmbeddedBusinessErrorsAreDefinitiveAndRedacted(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		definitive bool
	}{
		{name: "low balance", body: `{"statusCode":407,"error":{"type":"LOW_BALANCE","description":"account balance is too low"}}`, definitive: true},
		{name: "order closed", body: `{"statusCode":410,"error":{"type":"ORDER_CLOSED","description":"order is closed"}}`, definitive: true},
		{name: "unknown upstream", body: `{"statusCode":499,"error":{"type":"UNKNOWN","description":"temporary provider issue"}}`, definitive: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			err := func() error {
				_, err := providerFor("smspva", server.URL, "secret-key").PurchaseTemporary(context.Background(), SMSPurchaseRequest{CountryCode: "US", ServiceCode: "telegram"})
				return err
			}()
			if err == nil {
				t.Fatal("expected provider error")
			}
			if strings.Contains(err.Error(), "secret-key") {
				t.Fatalf("provider credential leaked in error: %v", err)
			}
			if got := isSMSPurchaseDefinitiveRejection(err); got != tc.definitive {
				t.Fatalf("definitive=%v, want %v, err=%v", got, tc.definitive, err)
			}
		})
	}
}

func TestSMSPVAPurchaseRecoveryIsFailClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("SMSPVA recovery must not guess from provider history: %s", r.URL.String())
	}))
	defer server.Close()
	p := providerFor("smspva", server.URL, "secret")
	temporary, ok := p.(SMSPurchaseRecoveryProvider)
	if !ok {
		t.Fatal("SMSPVA temporary recovery adapter missing")
	}
	if recovered, err := temporary.RecoverTemporaryPurchase(context.Background(), SMSPurchaseRequest{CountryCode: "US", ServiceCode: "telegram"}, time.Now()); err != nil || recovered != nil {
		t.Fatalf("temporary recovery=%#v err=%v, want nil,nil", recovered, err)
	}
	rental, ok := p.(SMSRentalPurchaseRecoveryProvider)
	if !ok {
		t.Fatal("SMSPVA rental recovery adapter missing")
	}
	if recovered, err := rental.RecoverRentalPurchase(context.Background(), SMSPurchaseRequest{CountryCode: "US", ServiceCode: "telegram", ProductType: "rental"}, time.Now()); err != nil || recovered != nil {
		t.Fatalf("rental recovery=%#v err=%v, want nil,nil", recovered, err)
	}
}

func TestSMSPVARentalStatusPreservesSenderDateAndOtherSMS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "activate":
			_, _ = w.Write([]byte(`{"status":1,"data":[]}`))
		case "orders":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"id":"501","scode":"telegram","sname":"Telegram","state":"0","pnumber":"+12025550001","cname":"US","hasnewsms":true,"until":1893456000,"canprolong":true,"canprolongmax":6,"canprolonguntil":1893459600,"lastonline":1893455000}]}`))
		case "sms":
			_, _ = w.Write([]byte(`{"status":1,"data":{"SmsList":[{"text":"Code 482913","sender":"Example","date":1893456000}],"OtherSms":[{"text":"Service notice"}]}}`))
		default:
			t.Fatalf("unexpected rental method %q", r.URL.Query().Get("method"))
		}
	}))
	defer server.Close()
	status, err := providerFor("smspva", server.URL, "secret").GetRentalStatus(context.Background(), "501")
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Messages) != 2 || status.Messages[0] != "Code 482913" || status.Messages[1] != "Service notice" {
		t.Fatalf("messages=%#v", status.Messages)
	}
	items, ok := status.Metadata["messages"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("metadata=%#v", status.Metadata)
	}
	if items[0]["sender"] != "Example" || items[0]["message_type"] != "service" || items[0]["other_sms"] != false {
		t.Fatalf("service metadata=%#v", items[0])
	}
	if got, ok := items[0]["provider_received_at"].(time.Time); !ok || !got.Equal(time.Unix(1893456000, 0).UTC()) {
		t.Fatalf("provider time=%#v", items[0]["provider_received_at"])
	}
	if items[1]["message_type"] != "other" || items[1]["other_sms"] != true {
		t.Fatalf("other metadata=%#v", items[1])
	}
}

func TestSMSPVARentalOrderConstraints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/rent.php" || r.URL.Query().Get("method") != "orders" {
			t.Fatalf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"data":[{"id":"40300","scode":"opt16","sname":"Instagram","state":"0","pnumber":"9096068511","ccode":"+7","cname":"KZ","hasnewsms":false,"until":"1587552240","canprolong":true,"canprolongmax":6,"canprolonguntil":1634883780,"lastonline":1586947920}]}`))
	}))
	defer server.Close()

	provider := providerFor("smspva", server.URL, "secret")
	inspector, ok := provider.(SMSRentalOrderInspector)
	if !ok {
		t.Fatal("SMSPVA provider must expose rental order inspection")
	}
	order, err := inspector.RentalOrder(context.Background(), "40300")
	if err != nil {
		t.Fatal(err)
	}
	if !order.CanProlong || order.CanProlongMax != 6 || order.CanProlongUntil != 1634883780 {
		t.Fatalf("unexpected rental constraints: %#v", order)
	}
	if order.ServiceCode != "opt16" || order.CountryCode != "KZ" {
		t.Fatalf("unexpected rental identity: %#v", order)
	}
}

func TestSMSPVAAdvancedRentalContracts(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/rent.php" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		method := r.URL.Query().Get("method")
		methods = append(methods, method)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "create_multi":
			if got := r.URL.Query().Get("services"); got != "opt6,opt7" {
				t.Fatalf("services=%q", got)
			}
			_, _ = w.Write([]byte(`{"status":1,"data":{"id":40370,"pnumber":"9096037108","until":1893456000}}`))
		case "activate":
			_, _ = w.Write([]byte(`{"status":1,"data":{"id":40370}}`))
		case "add_service_to_order":
			if r.URL.Query().Get("pnumber") != "9096037108" || r.URL.Query().Get("service") != "opt89" {
				t.Fatalf("unexpected add-service query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"status":1,"data":{"id":4211321,"pnumber":"9096037108","service":"opt89","until":1893456000}}`))
		case "get_rent_history":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"orderId":857191,"resourceCode":"opt9","number":"37067787324","haveSms":false,"isAvailForRestore":true,"days":30,"country":"LT","begin":1768902473,"end":1771494473,"closed":1768903921}]}`))
		case "restore_user_precalc":
			_, _ = w.Write([]byte(`{"status":1,"data":{"ccode":"LT","scode":"opt9","price":1.5,"sname":"Instagram","pnumber":"37067787324","outdays":5,"orderid":857191,"prolongTo":30}}`))
		case "restore_user":
			_, _ = w.Write([]byte(`{"status":1,"data":{"id":123456}}`))
		default:
			t.Fatalf("unexpected method: %s", method)
		}
	}))
	defer server.Close()

	provider := providerFor("smspva", server.URL, "secret")
	advanced, ok := provider.(SMSRentalAdvancedProvider)
	if !ok {
		t.Fatal("SMSPVA provider must expose advanced rental operations")
	}
	multi, err := advanced.PurchaseRentalMulti(context.Background(), SMSPurchaseRequest{CountryCode: "KZ", DurationValue: 1, DurationUnit: "week"}, []string{"opt6", "opt7"})
	if err != nil || multi.ProviderOrderID != "40370" {
		t.Fatalf("multi=%#v err=%v", multi, err)
	}
	added, err := advanced.AddRentalService(context.Background(), "40370", "+9096037108", "opt89", 7)
	if err != nil || added.ProviderOrderID != "4211321" {
		t.Fatalf("added=%#v err=%v", added, err)
	}
	history, err := advanced.RentalHistory(context.Background(), 0, 10)
	if err != nil || len(history) != 1 || !history[0].CanRestore {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	quote, err := advanced.PrecalcRentalRestore(context.Background(), "857191")
	if err != nil || quote.ProviderCost != 1.5 || quote.ServiceCode != "opt9" {
		t.Fatalf("quote=%#v err=%v", quote, err)
	}
	restoredID, err := advanced.RestoreRental(context.Background(), "857191")
	if err != nil || restoredID != "123456" {
		t.Fatalf("restored=%q err=%v", restoredID, err)
	}
}

func TestNormalizeSMSPVAIconPath(t *testing.T) {
	valid := []string{
		"images/ico/example.ico",
		"/images/ico/example.png",
		"https://smspva.com/images/ico/example.webp",
	}
	for _, value := range valid {
		if _, err := normalizeSMSPVAIconPath(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	invalid := []string{
		"https://example.com/images/ico/example.ico",
		"http://smspva.com/images/ico/example.ico",
		"https://smspva.com:443/images/ico/example.ico",
		"https://user:pass@smspva.com/images/ico/example.ico",
		"https://smspva.com/images/ico/example.ico?x=1",
		"https://smspva.com/images/ico/example.ico?",
		"https://smspva.com/images/ico/example.ico#fragment",
		"../secret",
		"images/ico/%2e%2e/secret.ico",
		"images/ico/foo\\bar.ico",
		"images/other/example.ico",
	}
	for _, value := range invalid {
		if _, err := normalizeSMSPVAIconPath(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func newSMSIconServiceTest(t *testing.T, serviceCode, iconPath string) (*SMSService, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	mock.ExpectQuery(`SELECT p\.code,COALESCE\(c\.raw_metadata->>'icon_path',''\)`).
		WithArgs(serviceCode).
		WillReturnRows(sqlmock.NewRows([]string{"provider_code", "icon_path"}).AddRow("smspva", iconPath))
	return &SMSService{db: db}, db, mock
}

func withSMSIconHTTPClient(t *testing.T, service *SMSService, client *http.Client) {
	t.Helper()
	service.iconHTTPClient = client
}

func clearSMSIconCache(serviceCode, iconPath string) {
	if normalized, err := normalizeSMSPVAIconPath(iconPath); err == nil {
		smsProviderIconCache.Delete("smspva:" + normalized)
	}
	_ = serviceCode
}

func TestSMSServiceIconRejectsRedirect(t *testing.T) {
	service, db, mock := newSMSIconServiceTest(t, "redirect", "images/ico/redirect.png")
	defer db.Close()
	defer clearSMSIconCache("redirect", "images/ico/redirect.png")

	called := 0
	withSMSIconHTTPClient(t, service, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called++
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("redirect")),
				Request:    req,
			}, nil
		}),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
	_, _, err := service.ServiceIcon(context.Background(), "", "redirect")
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("redirect should be rejected, err=%v", err)
	}
	if called != 1 {
		t.Fatalf("redirect response should be fetched once, calls=%d", called)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSServiceIconRejectsUnsupportedMIME(t *testing.T) {
	service, db, mock := newSMSIconServiceTest(t, "mime", "images/ico/mime.png")
	defer db.Close()
	defer clearSMSIconCache("mime", "images/ico/mime.png")
	withSMSIconHTTPClient(t, service, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader("not an image")),
			Request:    req,
		}, nil
	})})
	_, _, err := service.ServiceIcon(context.Background(), "", "mime")
	if err == nil || !strings.Contains(err.Error(), "supported image") {
		t.Fatalf("unsupported MIME should be rejected, err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSServiceIconRejectsOversizedResponse(t *testing.T) {
	service, db, mock := newSMSIconServiceTest(t, "large", "images/ico/large.png")
	defer db.Close()
	defer clearSMSIconCache("large", "images/ico/large.png")
	withSMSIconHTTPClient(t, service, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", smsProviderIconMaxBytes+1))),
			Request:    req,
		}, nil
	})})
	_, _, err := service.ServiceIcon(context.Background(), "", "large")
	if err == nil || !strings.Contains(err.Error(), "exceeds allowed size") {
		t.Fatalf("oversized response should be rejected, err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSServiceIconHonorsTimeout(t *testing.T) {
	service, db, mock := newSMSIconServiceTest(t, "timeout", "images/ico/timeout.png")
	defer db.Close()
	defer clearSMSIconCache("timeout", "images/ico/timeout.png")
	withSMSIconHTTPClient(t, service, &http.Client{
		Timeout: 10 * time.Millisecond,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}),
	})
	started := time.Now()
	_, _, err := service.ServiceIcon(context.Background(), "", "timeout")
	if err == nil || time.Since(started) > time.Second || !strings.Contains(strings.ToLower(err.Error()), "deadline") {
		t.Fatalf("icon request should honor timeout, err=%v elapsed=%v", err, time.Since(started))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSServiceIconCachesSuccessfulResponse(t *testing.T) {
	service, db, mock := newSMSIconServiceTest(t, "cached", "images/ico/cached.png")
	defer db.Close()
	defer clearSMSIconCache("cached", "images/ico/cached.png")
	mock.ExpectQuery(`SELECT p\.code,COALESCE\(c\.raw_metadata->>'icon_path',''\)`).
		WithArgs("cached").
		WillReturnRows(sqlmock.NewRows([]string{"provider_code", "icon_path"}).AddRow("smspva", "images/ico/cached.png"))
	requests := 0
	withSMSIconHTTPClient(t, service, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png; charset=binary"}},
			Body:       io.NopCloser(strings.NewReader(string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}))),
			Request:    req,
		}, nil
	})})
	first, firstType, err := service.ServiceIcon(context.Background(), "", "cached")
	if err != nil {
		t.Fatal(err)
	}
	second, secondType, err := service.ServiceIcon(context.Background(), "", "cached")
	if err != nil {
		t.Fatal(err)
	}
	if firstType != "image/png" || secondType != "image/png" || len(first) == 0 || len(second) == 0 {
		t.Fatalf("unexpected cached payload: first=%q/%q second=%q/%q", first, firstType, second, secondType)
	}
	if requests != 1 {
		t.Fatalf("expected one upstream request for cache hit, got %d", requests)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFilterNewSMSPVARentalOrderRequiresMatchingRestoreEvidence(t *testing.T) {
	baseline := map[string]struct{}{"old": {}}
	orders := []SMSRentalProviderOrder{
		{ID: "old", ServiceCode: "opt9", CountryCode: "LT", PhoneNumber: "37067787324"},
		{ID: "wrong-phone", ServiceCode: "opt9", CountryCode: "LT", PhoneNumber: "37060000000"},
		{ID: "wrong-country", ServiceCode: "opt9", CountryCode: "US", PhoneNumber: "37067787324"},
		{ID: "restored", ServiceCode: "opt9", CountryCode: "LT", PhoneNumber: "+37067787324"},
	}
	got := filterNewSMSPVARentalOrder(orders, baseline, "opt9", "LT", "37067787324")
	if got == nil || got.ID != "restored" {
		t.Fatalf("restore candidate=%#v", got)
	}
}

func TestFilterNewSMSPVARentalOrderFailsClosedOnAmbiguousCandidates(t *testing.T) {
	orders := []SMSRentalProviderOrder{
		{ID: "a", ServiceCode: "opt9", CountryCode: "LT", PhoneNumber: "37067787324"},
		{ID: "b", ServiceCode: "opt9", CountryCode: "LT", PhoneNumber: "+37067787324"},
	}
	if got := filterNewSMSPVARentalOrder(orders, map[string]struct{}{}, "opt9", "LT", "37067787324"); got != nil {
		t.Fatalf("ambiguous restore must fail closed, got %#v", got)
	}
}

func TestSMSPVAServiceIconProxyURLDoesNotExposeProviderName(t *testing.T) {
	item := SMSSvcCatalogItem{Code: "opt9", ProviderIconPath: "images/ico/opt9.png"}
	if item.ProviderIconPath == "" {
		t.Fatal("test fixture requires an upstream icon path")
	}
	publicURL := "/api/v1/sms/service-icons/" + item.Code
	if strings.Contains(strings.ToLower(publicURL), "smspva") || strings.Contains(strings.ToLower(publicURL), "5sim") {
		t.Fatalf("public icon URL leaks provider identity: %s", publicURL)
	}
}

func TestResolvePublicChannelProviderUsesOnlyReadyPublicChannel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT p\.code\s+FROM sms_channels c\s+JOIN sms_providers p ON p\.id=c\.provider_id`).
		WithArgs("channel_2").
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("smspva"))

	svc := &SMSService{db: db}
	got, err := svc.ResolvePublicChannelProvider(context.Background(), " CHANNEL_2 ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "smspva" {
		t.Fatalf("resolved provider=%q, want smspva", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListPublicProvidersReturnsChannelAliasesNotSupplierIdentity(t *testing.T) {
	t.Setenv("SMS_5SIM_API_KEY", "five-key")
	t.Setenv("SMS_SMSPVA_API_KEY", "pva-key")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	rows := sqlmock.NewRows([]string{
		"channel_code", "public_name", "provider_code", "base_url", "provider_enabled",
		"health_status", "credential_ref", "capabilities", "channel_enabled", "visible", "healthy",
	}).
		AddRow("channel_1", "渠道1", "5sim", "https://5sim.net/v1", true, "healthy", "", []byte(`{"supports_temporary":true}`), true, true, true).
		AddRow("channel_2", "渠道2", "smspva", "https://api.smspva.com", true, "healthy", "", []byte(`{"supports_temporary":true,"supports_rental":true}`), true, true, true)

	mock.ExpectQuery(`SELECT c\.code,c\.public_name,p\.code,p\.base_url`).WillReturnRows(rows)

	svc := &SMSService{db: db}
	items, err := svc.ListPublicProviders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("providers=%#v", items)
	}
	if items[0].Code != "channel_1" || items[0].Name != "渠道1" || items[1].Code != "channel_2" || items[1].Name != "渠道2" {
		t.Fatalf("public channel aliases=%#v", items)
	}
	for _, item := range items {
		serialized, _ := json.Marshal(item)
		lower := strings.ToLower(string(serialized))
		if strings.Contains(lower, "5sim") || strings.Contains(lower, "smspva") {
			t.Fatalf("public provider response leaked supplier identity: %s", serialized)
		}
	}
	if !items[0].Selectable || !items[1].Selectable {
		t.Fatalf("ready channels must be selectable: %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
