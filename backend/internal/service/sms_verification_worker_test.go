package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSMSProviderPollDelayKeepsFiveSIMBaseline(t *testing.T) {
	now := time.Now()
	for _, age := range []time.Duration{0, 2 * time.Minute, 20 * time.Minute} {
		if got := smsProviderPollDelay("5sim", now.Add(-age), now); got != smsVerificationPollInterval {
			t.Fatalf("5SIM age %s: got %s, want %s", age, got, smsVerificationPollInterval)
		}
	}
}

func TestSMSProviderPollDelayBacksOffOnlySMSPVA(t *testing.T) {
	now := time.Now()
	cases := []struct {
		age  time.Duration
		want time.Duration
	}{
		{30 * time.Second, 5 * time.Second},
		{2 * time.Minute, 10 * time.Second},
		{10 * time.Minute, 25 * time.Second},
	}
	for _, tc := range cases {
		if got := smsProviderPollDelay("smspva", now.Add(-tc.age), now); got != tc.want {
			t.Fatalf("SMSPVA age %s: got %s, want %s", tc.age, got, tc.want)
		}
	}
}

func TestSMSPurchaseNeedsFailClosedReview(t *testing.T) {
	if !smsPurchaseNeedsFailClosedReview("smspva") {
		t.Fatal("SMSPVA ambiguous purchases must fail closed")
	}
	if smsPurchaseNeedsFailClosedReview("5sim") {
		t.Fatal("5SIM must preserve its deterministic recovery path")
	}
}

func TestHoldUnknownSMSPurchaseForManualReviewDoesNotReleaseFunds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`UPDATE sms_orders[[:space:]]+SET status='reconciling'`).
		WithArgs(smsReconciliationPurchase, int(smsVerificationManualReviewRetry.Seconds()), "ambiguous purchase", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO sms_order_events`).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc := &SMSService{db: db}
	if err := svc.holdUnknownSMSPurchaseForManualReview(context.Background(), 42, "ambiguous purchase"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSMSReconcileCapturesDeliveredMessageEvenWithoutFirstSMSMarker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	now := time.Now()
	mock.ExpectQuery(`SELECT o.id,o.user_id,o.status`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "product_type", "provider_order_id", "refund_status", "provider_refund_status", "code", "base_url", "credential_ref", "expires_at", "updated_at", "created_at", "settlement_status", "reconciliation_action",
		}).AddRow(int64(61), int64(8), "expired", "temporary", "provider-61", "rejected", "not_requested", "smspva", "https://example.invalid", "env:SMSPVA_API_KEY", now.Add(-time.Minute), now, now.Add(-10*time.Minute), "held", ""))
	mock.ExpectQuery(`SELECT first_sms_received_at,\(SELECT COUNT\(\*\) FROM sms_messages`).
		WithArgs(int64(61)).
		WillReturnRows(sqlmock.NewRows([]string{"first_sms_received_at", "message_count"}).AddRow(nil, 1))
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE sms_orders SET captured_amount=reserved_amount`).
		WithArgs(int64(61)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved_amount"}).AddRow(1.25))
	mock.ExpectExec(`UPDATE users SET frozen_balance`).
		WithArgs(1.25, int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`DELETE FROM sms_quotes`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`DELETE FROM sms_rental_service_quotes`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`DELETE FROM sms_rental_restore_quotes`).WillReturnResult(sqlmock.NewResult(0, 0))

	svc := &SMSService{db: db}
	if err = svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
