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
	defer func() { _ = db.Close() }()
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
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT billing FROM email_providers`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"billing"}).AddRow([]byte(`{}`)))
	mock.ExpectExec(`INSERT INTO email_provider_usage`).WithArgs(int64(7), nil, "health_check", 0, true, int64(25), false, float64(0)).WillReturnResult(sqlmock.NewResult(1, 1))
	svc := &EmailVerificationService{db: db}
	svc.recordProviderUsage(context.Background(), 7, 0, "health_check", nil, 25*time.Millisecond)
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureEmailOrderKeepsInboxActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET captured_amount=sale_price_snapshot,refund_status='not_applicable'.*status IN \('email_received','verification_extracted'\)`).
		WithArgs(int64(31)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).
		WithArgs(int64(31), int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`INSERT INTO email_order_events`).
		WithArgs(int64(31), "balance_captured", "email_capture:31", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc := &EmailVerificationService{db: db}
	if err = svc.captureEmailOrder(context.Background(), 31, 11); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExpireEmailOrderCapturesAfterTargetMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	messageAt := time.Now().Add(-time.Minute)
	mock.ExpectQuery(`SELECT status,refund_policy_snapshot,sale_price_snapshot,first_message_at,captured_amount,refund_status FROM email_orders`).WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{"status", "refund_policy_snapshot", "sale_price_snapshot", "first_message_at", "captured_amount", "refund_status"}).AddRow("email_received", EmailRefundIfNoMessage, 1.25, messageAt, 0.0, "not_requested"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE email_orders SET status='completed',completed_at=NOW\(\),refund_status='not_applicable',captured_amount=sale_price_snapshot`).WithArgs(int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
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
	defer func() { _ = db.Close() }()
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
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders WHERE status='reconciling' OR \(status IN \('reserved','generating_inbox'\) AND updated_at <= NOW\(\) - \(\$1 \* INTERVAL '1 second'\)\)`).
		WithArgs(emailGenerationRecoveryGraceSeconds).
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
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders WHERE status='reconciling' OR \(status IN \('reserved','generating_inbox'\) AND updated_at <= NOW\(\) - \(\$1 \* INTERVAL '1 second'\)\)`).
		WithArgs(emailGenerationRecoveryGraceSeconds).
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
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders WHERE status='reconciling' OR \(status IN \('reserved','generating_inbox'\) AND updated_at <= NOW\(\) - \(\$1 \* INTERVAL '1 second'\)\)`).
		WithArgs(emailGenerationRecoveryGraceSeconds).
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

func TestTempTFProviderGenerateListAndRead(t *testing.T) {
	var accountQuery string
	checkCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/account":
			accountQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"email":"demo+abc@gmail.com"}`))
		case "/check":
			checkCalls++
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["email"] != "demo+abc@gmail.com" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"subject":"Verify your email","from":"noreply@example.com","date":"2026-09-20T12:00:00Z","body":"<p>Your code is 123456</p>","bodyContentType":"html","id":"42"}]}`))
		case "/stats":
			_, _ = w.Write([]byte(`{"totalAddresses":10}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := &tempTFProvider{code: "temp_tf", baseURL: server.URL, client: server.Client(), limiter: newEmailProviderLimiter(2, 0)}
	if err := p.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	inbox, err := p.GenerateInbox(context.Background(), GenerateInboxRequest{AddressType: "gmail"})
	if err != nil || inbox.EmailAddress != "demo+abc@gmail.com" {
		t.Fatalf("generate inbox=%+v err=%v", inbox, err)
	}
	if !strings.Contains(accountQuery, "providers=gmail") || !strings.Contains(accountQuery, "plus=1") || !strings.Contains(accountQuery, "dot=1") {
		t.Fatalf("unexpected account query: %s", accountQuery)
	}
	list, err := p.ListMessages(context.Background(), ListMessagesRequest{EmailAddress: inbox.EmailAddress})
	if err != nil || len(list.Messages) != 1 || list.Messages[0].ProviderMessageID != "42" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	msg, err := p.GetMessage(context.Background(), GetMessageRequest{EmailAddress: inbox.EmailAddress, ProviderMessageID: "42"})
	if err != nil || msg.TextBody == "" || msg.Subject != "Verify your email" {
		t.Fatalf("message=%+v err=%v", msg, err)
	}
	if checkCalls != 2 {
		t.Fatalf("check calls=%d want 2", checkCalls)
	}
}

func TestSonjjProviderGmailAndOutlookContracts(t *testing.T) {
	var sawAPIKey bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "sonjj-key" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		sawAPIKey = true
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/temp_email/domains":
			_, _ = w.Write([]byte(`{"domains":["example.test"]}`))
		case "/v1/temp_gmail/random":
			if r.URL.Query().Get("type") != "real" {
				http.Error(w, "wrong type", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"email":"real@gmail.com","timestamp":1780000000,"type":"real"}`))
		case "/v1/temp_gmail/inbox":
			if r.URL.Query().Get("email") != "real@gmail.com" || r.URL.Query().Get("timestamp") != "1780000000" {
				http.Error(w, "wrong inbox query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"messages":[{"mid":"m1","textDate":"Thu, 09 May 2024 01:10:51 +0000 (UTC)","textFrom":"noreply@openai.com","textSubject":"Verify","textTo":"real@gmail.com"}]}`))
		case "/v1/temp_gmail/message":
			if r.URL.Query().Get("mid") != "m1" {
				http.Error(w, "wrong message", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"body":"Your verification code is 654321"}`))
		case "/v1/temp_outlook/random":
			if r.URL.Query().Get("type") != "alias" {
				http.Error(w, "wrong outlook type", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"email":"demo+abc@outlook.com","timestamp":1780000100,"type":"alias"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := &sonjjProvider{code: "sonjj", baseURL: server.URL, apiKey: "sonjj-key", client: server.Client(), limiter: newEmailProviderLimiter(4, 0)}
	if err := p.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	gmail, err := p.GenerateInbox(context.Background(), GenerateInboxRequest{AddressType: "gmail_real"})
	if err != nil || gmail.EmailAddress != "real@gmail.com" || !strings.Contains(gmail.ProviderInboxID, "|gmail|real") {
		t.Fatalf("gmail=%+v err=%v", gmail, err)
	}
	list, err := p.ListMessages(context.Background(), ListMessagesRequest{ProviderInboxID: gmail.ProviderInboxID, EmailAddress: gmail.EmailAddress})
	if err != nil || len(list.Messages) != 1 || list.Messages[0].FromAddress != "noreply@openai.com" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if list.Messages[0].ReceivedAt.IsZero() {
		t.Fatal("Sonjj RFC mail date was not parsed")
	}
	msg, err := p.GetMessage(context.Background(), GetMessageRequest{ProviderInboxID: gmail.ProviderInboxID, ProviderMessageID: "m1", EmailAddress: gmail.EmailAddress})
	if err != nil || !strings.Contains(msg.TextBody, "654321") || !msg.ReceivedAt.IsZero() {
		t.Fatalf("message=%+v err=%v", msg, err)
	}
	outlook, err := p.GenerateInbox(context.Background(), GenerateInboxRequest{AddressType: "outlook_alias"})
	if err != nil || outlook.EmailAddress != "demo+abc@outlook.com" {
		t.Fatalf("outlook=%+v err=%v", outlook, err)
	}
	if !sawAPIKey {
		t.Fatal("Sonjj API key header was not sent")
	}
}

func TestEmailProviderAddressTypeMatrix(t *testing.T) {
	cases := []struct {
		provider string
		address  string
		want     bool
	}{
		{"temp_tf", "gmail", true},
		{"temp_tf", "hotmail", true},
		{"temp_tf", "gmail_real", false},
		{"sonjj", "gmail_real", true},
		{"sonjj", "outlook_alias", true},
		{"sonjj", "hotmail", false},
		{"emailnator", "gmail", true},
		{"emailnator", "outlook", false},
	}
	for _, tc := range cases {
		if got := emailProviderSupportsAddressType(tc.provider, tc.address); got != tc.want {
			t.Fatalf("%s/%s=%v want %v", tc.provider, tc.address, got, tc.want)
		}
	}
	if emailProviderRequiresCredential("temp_tf") {
		t.Fatal("temp.tf must not require a credential")
	}
	if !emailProviderRequiresCredential("sonjj") {
		t.Fatal("Sonjj must require a credential")
	}
}

func TestNormalizeEmailAddressTypes(t *testing.T) {
	for _, value := range []string{"gmail", "outlook", "hotmail", "gmail_real", "gmail_alias", "outlook_real", "outlook_alias"} {
		got, err := normalizeEmailAddressType(value)
		if err != nil || got != value {
			t.Fatalf("normalize %s => %s err=%v", value, got, err)
		}
	}
	if _, err := normalizeEmailAddressType("unsupported"); err == nil {
		t.Fatal("unsupported email address type should fail")
	}
}


type emailAdminTestSettingRepo struct {
	values map[string]string
}

func newEmailAdminTestSettingRepo() *emailAdminTestSettingRepo {
	return &emailAdminTestSettingRepo{values: map[string]string{}}
}

func (r *emailAdminTestSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *emailAdminTestSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *emailAdminTestSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *emailAdminTestSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *emailAdminTestSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *emailAdminTestSettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *emailAdminTestSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestEmailAdminSettingsDefaultsAndUpdate(t *testing.T) {
	repo := newEmailAdminTestSettingRepo()
	settings := NewSettingService(repo, nil)
	svc := &EmailVerificationService{settings: settings}

	defaults := svc.AdminSettings(context.Background())
	if defaults.Enabled || defaults.FreeDailyLimit != 20 || defaults.FreeActiveLimit != 3 || defaults.FreeGenerationIntervalSec != 5 {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	want := EmailAdminSettings{
		Enabled:                   true,
		FreeDailyLimit:            40,
		FreeActiveLimit:           4,
		FreeGenerationIntervalSec: 8,
	}
	if err := svc.UpdateAdminSettings(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got := svc.AdminSettings(context.Background())
	if got != want {
		t.Fatalf("settings=%+v want %+v", got, want)
	}
}

func TestEmailAdminSettingsRejectInvalidLimits(t *testing.T) {
	repo := newEmailAdminTestSettingRepo()
	settings := NewSettingService(repo, nil)
	svc := &EmailVerificationService{settings: settings}

	for _, tc := range []EmailAdminSettings{
		{FreeDailyLimit: -1, FreeActiveLimit: 3, FreeGenerationIntervalSec: 5},
		{FreeDailyLimit: 20, FreeActiveLimit: -1, FreeGenerationIntervalSec: 5},
		{FreeDailyLimit: 20, FreeActiveLimit: 3, FreeGenerationIntervalSec: -1},
		{FreeDailyLimit: 100001, FreeActiveLimit: 3, FreeGenerationIntervalSec: 5},
		{FreeDailyLimit: 20, FreeActiveLimit: 1001, FreeGenerationIntervalSec: 5},
		{FreeDailyLimit: 20, FreeActiveLimit: 3, FreeGenerationIntervalSec: 3601},
	} {
		if err := svc.UpdateAdminSettings(context.Background(), tc); err == nil {
			t.Fatalf("expected invalid settings rejection for %+v", tc)
		}
	}
}


func TestNormalizeEmailServiceCodeDefaultsToGeneric(t *testing.T) {
	if got := normalizeEmailServiceCode(""); got != "other" {
		t.Fatalf("empty service code normalized to %q, want other", got)
	}
	if got := normalizeEmailServiceCode("  GOOGLE  "); got != "google" {
		t.Fatalf("explicit service code normalized to %q, want google", got)
	}
}
