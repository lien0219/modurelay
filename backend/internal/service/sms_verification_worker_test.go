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
	if err != nil { t.Fatal(err) }
	defer func() { _ = db.Close() }()

	mock.ExpectExec(`UPDATE sms_orders[[:space:]]+SET status='reconciling'`).
		WithArgs(smsReconciliationPurchase, int(smsVerificationManualReviewRetry.Seconds()), "ambiguous purchase", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO sms_order_events`).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc := &SMSService{db: db}
	if err := svc.holdUnknownSMSPurchaseForManualReview(context.Background(), 42, "ambiguous purchase"); err != nil { t.Fatal(err) }
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
}
