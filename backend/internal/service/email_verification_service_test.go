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
	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestExtractVerificationPrefersContextAndRejectsNoise(t *testing.T) {
	got := ExtractVerification("Security code", "Year 2026, order 823492, amount 123456. Your security code: 8432", "")
	if got.Code != "8432" || got.Confidence < 0.9 {
		t.Fatalf("unexpected extraction: %+v", got)
	}
	got = ExtractVerification("登录验证", "验证码：381924，订单号 823492，金额 123456", "")
	if got.Code != "381924" || got.Method != "context_otp" {
		t.Fatalf("unexpected Chinese extraction: %+v", got)
	}
	got = ExtractVerification("Verify Email", "Open https://example.test/verify?id=abc and ignore https://example.test/unsubscribe", "")
	if got.URL != "https://example.test/verify?id=abc" {
		t.Fatalf("unexpected URL extraction: %+v", got)
	}
	got = ExtractVerification("Verify Email", "Click the button below", `<a href="https://example.test/verify?token=abc">Verify</a>`)
	if got.URL != "https://example.test/verify?token=abc" {
		t.Fatalf("anchor URL was not extracted: %+v", got)
	}
}

func TestSanitizeEmailHTMLRemovesActiveContentAndRemoteAttributes(t *testing.T) {
	clean := SanitizeEmailHTML(`<div onclick="alert(1)"><script>alert(1)</script><img src="https://tracker.test/pixel"><a href="javascript:alert(1)">bad</a><a href="https://example.test/verify">verify</a></div>`)
	if strings.Contains(strings.ToLower(clean), "script") || strings.Contains(strings.ToLower(clean), "onclick") || strings.Contains(strings.ToLower(clean), "javascript:") {
		t.Fatalf("unsafe HTML survived: %s", clean)
	}
	if !strings.Contains(clean, `href="https://example.test/verify"`) {
		t.Fatalf("safe verification link missing: %s", clean)
	}
	if !strings.Contains(clean, `rel="noopener noreferrer"`) {
		t.Fatalf("safe link isolation missing: %s", clean)
	}
}

func TestEmailnatorProviderGenerateAndList(t *testing.T) {
	generateContractOK := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-RapidAPI-Key") != "test-key" || r.Header.Get("X-RapidAPI-Host") != "test-host" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/generate":
			var body struct {
				Options []int `json:"options"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Options) != 3 || body.Options[0] != 1 {
				http.Error(w, "invalid Gmailnator generate contract", http.StatusBadRequest)
				return
			}
			generateContractOK = true
			_, _ = w.Write([]byte(`{"email":["abc@gmail.com"]}`))
		case "/list":
			_, _ = w.Write([]byte(`{"messages":[{"id":"m1","from":"noreply@example.com","subject":"Verify Email"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &emailnatorProvider{code: "emailnator", baseURL: server.URL, rapidAPIKey: "test-key", rapidAPIHost: "test-host", client: server.Client(), cap: emailnatorCapabilities(), endpoints: map[string]string{"generate": "/generate", "list": "/list"}, limiter: newEmailProviderLimiter(1, 0)}
	inbox, err := p.GenerateInbox(context.Background(), GenerateInboxRequest{AddressType: "gmail"})
	if err != nil || inbox.EmailAddress != "abc@gmail.com" {
		t.Fatalf("generate: inbox=%+v err=%v", inbox, err)
	}
	if !generateContractOK {
		t.Fatal("generate request did not use the Gmailnator options contract")
	}
	list, err := p.ListMessages(context.Background(), ListMessagesRequest{EmailAddress: inbox.EmailAddress})
	if err != nil || len(list.Messages) != 1 || list.Messages[0].ProviderMessageID != "m1" {
		t.Fatalf("list: result=%+v err=%v", list, err)
	}
}

func TestEmailnatorProviderMessageAndDelete(t *testing.T) {
	var deleteMethod string
	messageContractOK := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-RapidAPI-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/message":
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["messageid"] != "m1" {
				http.Error(w, "invalid Gmailnator message contract", http.StatusBadRequest)
				return
			}
			messageContractOK = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"m1","from":"noreply@example.com","subject":"Verify","text":"code 123456","html":"<b>code</b>"}`))
		case "/delete":
			deleteMethod = r.Method
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p := &emailnatorProvider{code: "emailnator", baseURL: server.URL, rapidAPIKey: "test-key", rapidAPIHost: "test-host", client: server.Client(), cap: emailnatorCapabilities(), endpoints: map[string]string{"message": "/message", "delete": "/delete"}, limiter: newEmailProviderLimiter(1, 0)}
	msg, err := p.GetMessage(context.Background(), GetMessageRequest{EmailAddress: "a@gmail.com", ProviderMessageID: "m1"})
	if err != nil || msg.ProviderMessageID != "m1" || msg.TextBody == "" {
		t.Fatalf("get message: %+v err=%v", msg, err)
	}
	if !messageContractOK {
		t.Fatal("message request did not include messageid")
	}
	if err := p.DeleteMessage(context.Background(), DeleteMessageRequest{EmailAddress: "a@gmail.com", ProviderMessageID: "m1"}); err != nil || deleteMethod != http.MethodDelete {
		t.Fatalf("delete message: method=%q err=%v", deleteMethod, err)
	}
}

func TestEmailnatorProviderErrorsAndRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	p := &emailnatorProvider{code: "emailnator", baseURL: server.URL, rapidAPIKey: "test-key", rapidAPIHost: "test-host", client: server.Client(), cap: emailnatorCapabilities(), limiter: newEmailProviderLimiter(1, 0)}
	_, err := p.ListMessages(context.Background(), ListMessagesRequest{EmailAddress: "a@gmail.com"})
	var providerErr *EmailProviderHTTPError
	if !errors.As(err, &providerErr) || providerErr.StatusCode != http.StatusTooManyRequests || providerErr.RetryAfter != 3*time.Second {
		t.Fatalf("unexpected provider error: %#v", err)
	}
}

func TestEmailQuoteTokenEnforcesTTLAndSignature(t *testing.T) {
	key := []byte("test-quote-secret")
	expires := time.Now().Add(time.Minute)
	id := makeEmailQuoteID(key, "google", "email_channel_1", "gmail", expires)
	if err := validateEmailQuoteID(key, id, "google", "email_channel_1", "gmail", time.Now()); err != nil {
		t.Fatalf("valid quote rejected: %v", err)
	}
	if err := validateEmailQuoteID(key, id, "google", "email_channel_1", "gmail", expires.Add(time.Second)); err != ErrEmailQuoteExpired {
		t.Fatalf("expired quote error=%v", err)
	}
	if err := validateEmailQuoteID(key, id, "github", "email_channel_1", "gmail", time.Now()); err != ErrEmailQuoteInvalid {
		t.Fatalf("tampered quote error=%v", err)
	}
}

func TestEmailQuoteSigningKeyUsesInitializedConfigFallback(t *testing.T) {
	t.Setenv("EMAIL_QUOTE_SIGNING_KEY", "")
	t.Setenv("JWT_SECRET", "")
	cfg := &config.Config{}
	cfg.JWT.Secret = "persisted-auto-generated-jwt-secret"
	if got := string(resolveEmailQuoteSigningKey(cfg)); got != cfg.JWT.Secret {
		t.Fatalf("quote signing key=%q, want initialized JWT secret", got)
	}
}

func TestEmailSalePriceUsesConfiguredCostAndGradeRules(t *testing.T) {
	price, snapshot := emailSalePrice(.05, .10,
		map[string]any{"base_markup": .30, "minimum_profit": .20},
		map[string]any{"fixed_markup": .01}, "A", 1.15, .02)
	// Cost floor: .10 + .20. The grade-adjusted formula is lower, so the
	// configured minimum profit must win.
	if price < .299999 || price > .300001 {
		t.Fatalf("unexpected dynamic price: %v", price)
	}
	if snapshot["grade"] != "A" || snapshot["grade_multiplier"] != 1.15 {
		t.Fatalf("pricing snapshot lost grade inputs: %#v", snapshot)
	}

	price, _ = emailSalePrice(.01, .10,
		map[string]any{"base_markup": .50},
		map[string]any{"fixed_markup": .02}, "S", 1.25, .03)
	if price <= .10 {
		t.Fatalf("expected cost and grade markup to raise price: %v", price)
	}
}

func TestEmailUserOrderJSONDoesNotExposeProviderFields(t *testing.T) {
	payload, err := json.Marshal(EmailOrder{ID: "public", OrderNo: "order", EmailAddress: "a@example.com", Status: "waiting_email"})
	if err != nil {
		t.Fatal(err)
	}
	serialized := strings.ToLower(string(payload))
	for _, forbidden := range []string{"emailnator", "gmailnator", "rapidapi", "provider_id", "provider_inbox_id", "provider_message_id", "provider_raw", "credential"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("user JSON contains %q: %s", forbidden, serialized)
		}
	}
}

func TestGetEmailOrderScopesLookupToAuthenticatedUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id FROM email_orders WHERE user_id=\$1 AND public_id=\$2::uuid`).
		WithArgs(int64(42), "other-users-order").
		WillReturnError(sql.ErrNoRows)

	svc := &EmailVerificationService{db: db}
	if _, err = svc.GetOrder(context.Background(), 42, "other-users-order"); err != ErrEmailNotFound {
		t.Fatalf("cross-user lookup error=%v, want ErrEmailNotFound", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmailPollingBackoffUsesChannelScheduleAndRetryAfter(t *testing.T) {
	schedule := parseEmailPollingBackoff([]byte(`[3,9,30]`))
	if len(schedule) != 3 || schedule[1] != 9 {
		t.Fatalf("unexpected schedule: %#v", schedule)
	}
	delay := emailPollDelayWithSchedule(1, nil, schedule)
	if delay < 9*time.Second || delay > 12*time.Second {
		t.Fatalf("custom delay out of jitter range: %v", delay)
	}
	delay = emailPollDelayWithSchedule(0, &EmailProviderHTTPError{StatusCode: 429, RetryAfter: 17 * time.Second}, schedule)
	if delay != 17*time.Second {
		t.Fatalf("Retry-After was not authoritative: %v", delay)
	}
}

func TestEmailOrderCostSupportsFixedAndEstimatedRequestModes(t *testing.T) {
	if got := estimateEmailOrderCost(map[string]any{"cost_mode": "fixed_per_order", "fixed_cost_per_order": .42}); got != .42 {
		t.Fatalf("fixed order cost=%v", got)
	}
	got := estimateEmailOrderCost(map[string]any{"cost_mode": "request_based", "overage_price_per_request": .03, "estimated_requests_per_order": 4})
	if got < .119999 || got > .120001 {
		t.Fatalf("estimated request cost=%v", got)
	}
}

func TestRecordProviderUsageUsesNullOrderForAdminHealthCheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT billing FROM email_providers`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"billing"}).AddRow([]byte(`{}`)))
	mock.ExpectExec(`INSERT INTO email_provider_usage`).WithArgs(int64(7), nil, "health_check", 0, true, int64(25), false, float64(0)).WillReturnResult(sqlmock.NewResult(1, 1))
	svc := &EmailVerificationService{db: db}
	svc.recordProviderUsage(context.Background(), 7, 0, "health_check", nil, 25*time.Millisecond)
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExpireEmailOrderCapturesAfterTargetMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	messageAt := time.Now().Add(-time.Minute)
	mock.ExpectQuery(`SELECT status,refund_policy_snapshot,sale_price_snapshot,first_message_at FROM email_orders`).WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{"status", "refund_policy_snapshot", "sale_price_snapshot", "first_message_at"}).AddRow("email_received", EmailRefundIfNoMessage, 1.25, messageAt))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET status='expired',refund_status='not_applicable',captured_amount=sale_price_snapshot`).WithArgs(int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).WithArgs(1.25, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`INSERT INTO email_order_events`).WithArgs(int64(12), "balance_captured", "email_capture:12", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	svc := &EmailVerificationService{db: db}
	if err = svc.expireOrderV2(context.Background(), 12, 9); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmailRefundRechecksNoMessageConditionInsideTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(`SELECT id,status,refund_policy_snapshot,sale_price_snapshot,first_message_at FROM email_orders`).
		WithArgs(int64(7), "order-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "refund_policy_snapshot", "sale_price_snapshot", "first_message_at"}).
			AddRow(int64(14), "waiting_email", EmailRefundIfNoMessage, 0.5, nil))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET status='refunded'.*first_message_at IS NULL`).
		WithArgs(int64(14), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	svc := &EmailVerificationService{db: db}
	if err = svc.RequestRefund(context.Background(), 7, "order-id"); err == nil {
		t.Fatal("refund should be rejected when the transactional eligibility condition no longer matches")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileRefundsUndeliveredInboxForNoRefundAfterDeliveryPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "expires_at", "refund_policy_snapshot", "sale_price_snapshot", "provider_inbox_id", "email_address"}).
			AddRow(int64(21), int64(8), "reconciling", time.Now().Add(-time.Minute), EmailNoRefundAfterDelivery, 0.75, "", ""))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET status='refunded'.*provider_inbox_id='' AND email_address=''`).
		WithArgs(int64(21)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET balance=balance\+\$1`).
		WithArgs(0.75, int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`INSERT INTO email_order_events`).
		WithArgs(int64(21), "reconciled", "email_reconcile:21", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT id,user_id,status,refund_status,sale_price_snapshot`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "refund_status", "sale_price_snapshot", "reserved_amount", "captured_amount", "released_amount", "refunded_amount"}))

	svc := &EmailVerificationService{db: db}
	if err = svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileCapturesDeliveredInboxForNoRefundAfterDeliveryPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "expires_at", "refund_policy_snapshot", "sale_price_snapshot", "provider_inbox_id", "email_address"}).
			AddRow(int64(22), int64(9), "reconciling", time.Now().Add(-time.Minute), EmailNoRefundAfterDelivery, 0.8, "inbox-22", "user@gmail.com"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET status='expired'.*provider_inbox_id<>'' AND email_address<>''`).
		WithArgs(int64(22)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).
		WithArgs(0.8, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`INSERT INTO email_order_events`).
		WithArgs(int64(22), "balance_captured", "email_capture:22", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT id,user_id,status,refund_status,sale_price_snapshot`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "refund_status", "sale_price_snapshot", "reserved_amount", "captured_amount", "released_amount", "refunded_amount"}))

	svc := &EmailVerificationService{db: db}
	if err = svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileCapturesDeliveredInboxForRefundIfNoMessagePolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "expires_at", "refund_policy_snapshot", "sale_price_snapshot", "provider_inbox_id", "email_address"}).
			AddRow(int64(23), int64(10), "reconciling", time.Now().Add(-time.Minute), EmailRefundIfNoMessage, 0.9, "inbox-23", "user@gmail.com"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET status='expired'.*provider_inbox_id<>'' AND email_address<>''`).
		WithArgs(int64(23)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).
		WithArgs(0.9, int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`INSERT INTO email_order_events`).
		WithArgs(int64(23), "balance_captured", "email_capture:23", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT id,user_id,status,refund_status,sale_price_snapshot`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "refund_status", "sale_price_snapshot", "reserved_amount", "captured_amount", "released_amount", "refunded_amount"}))

	svc := &EmailVerificationService{db: db}
	if err = svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
