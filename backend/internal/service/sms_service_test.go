package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSMSProviderCapabilitiesAreSeparated(t *testing.T) {
	for _, code := range []string{"5sim", "smspool", "sms_activate", "onlinesim", "pingme"} {
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
		if (code == "onlinesim" || code == "pingme") && !cap.Rental {
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

func TestFiveSIMQuoteParsesCurrentGuestResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/guest/products/usa/any/telegram" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("guest quote must not require bearer auth")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Category":"activation","Qty":93849,"Price":0.77}`))
	}))
	defer server.Close()

	quote, err := providerFor("5sim", server.URL, "").Quote(context.Background(), SMSQuoteRequest{ServiceCode: "telegram", CountryCode: "usa", ProductType: "temporary"})
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
		Order   SMSOrder
		Channel SMSPublicChannel
	}{Order: SMSOrder{ID: "public", ChannelCode: "channel_1", Price: 1.2}, Channel: SMSPublicChannel{Code: "channel_1", ProviderCost: 0.8, GradeMultiplier: 1.2, GradeFixedMarkup: 0.1}})
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

func TestSMSOrderExpiryUsesProviderValueOrSafeDefaults(t *testing.T) {
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	providerExpiry := now.Add(7 * time.Minute)
	if got := smsOrderExpiresAt(now, "temporary", 0, "", &providerExpiry); !got.Equal(providerExpiry) {
		t.Fatalf("provider expiry=%v, want %v", got, providerExpiry)
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
		WithArgs(int64(7), int64(1), int64(2), int64(3), int64(4), "temporary", .4, .6, nil, "unavailable", "", 1.0, "idem-1", smsReconciliationPurchase, int(smsVerificationUnknownTimeout.Seconds())).
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
