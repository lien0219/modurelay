package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

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

func TestSMSOrderCapabilitiesPreferRegisteredAdapter(t *testing.T) {
	if got := resolveSMSCapabilities("pingme", "https://example.invalid", []byte(`{}`)); !got.Extend || !got.Rental {
		t.Fatalf("PingMe adapter capabilities = %#v", got)
	}
	fallback := resolveSMSCapabilities("custom", "https://example.invalid", []byte(`{"supports_cancel":true}`))
	if !fallback.Cancel {
		t.Fatalf("unknown provider should use stored capabilities: %#v", fallback)
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

func TestFiveSIMRecoversTimedOutPurchaseFromOrderHistory(t *testing.T) {
	startedAt := time.Date(2026, 9, 19, 2, 30, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/orders" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("category") != "activation" || r.URL.Query().Get("limit") != "50" || r.URL.Query().Get("reverse") != "true" {
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

func TestSMSOrderExpiryUsesProviderValueOrSafeDefaults(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	providerExpiry := now.Add(7 * time.Minute)
	if got := smsOrderExpiresAt(now, "temporary", 0, "", &providerExpiry); !got.Equal(providerExpiry) {
		t.Fatalf("provider expiry=%v, want %v", got, providerExpiry)
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

func TestSMSMappingValidationFailsClosed(t *testing.T) {
	svc := &SMSService{}
	if err := svc.UpsertProviderServiceMapping(context.Background(), 1, 2, "", "", true, false, true); err == nil {
		t.Fatal("enabled service mapping without provider code must fail")
	}
	if err := svc.UpsertProviderCountryMapping(context.Background(), 1, 2, "", "", true); err == nil {
		t.Fatal("enabled country mapping without provider identifier must fail")
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
		WithArgs(int64(7), int64(1), int64(2), int64(3), int64(4), "temporary", "any", 0, .4, .6, nil, "unavailable", "", 1.0, "idem-1", smsReconciliationPurchase, int(smsVerificationUnknownTimeout.Seconds())).
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
	want := []string{"create", "activate", "activate", "sms", "prolong", "delete"}
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
