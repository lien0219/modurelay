package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

var verificationRecordColumns = []string{
	"id", "order_no", "verification_type", "product_type", "user_id", "user_email",
	"service_code", "channel_code", "channel_name", "provider_code", "target", "region",
	"status", "outcome", "refund_status", "refund_reason", "sale_amount", "provider_cost",
	"user_debit_amount", "reserved_amount", "captured_amount", "released_amount",
	"refunded_amount", "currency", "provider_request_count", "error_code", "error_message",
	"public_error_message", "created_at", "updated_at", "completed_at", "expires_at",
	"provider_cost_estimated", "settlement_estimated",
}

func TestVerificationRecordServiceUserScopeAndRedaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	mock.ExpectQuery(`(?s)WITH .*records AS \(.*COUNT\(\*\).*FROM records WHERE user_id = \$1 AND verification_type = \$2 AND \(order_no ILIKE \$3`).
		WithArgs(int64(42), "email", "%openai%").
		WillReturnRows(sqlmock.NewRows([]string{
			"total", "processing", "success", "failed", "refunded", "free", "cancelled", "expired",
			"sale", "debit", "reserved", "captured", "released", "refunded_amount", "provider_cost",
		}).AddRow(1, 0, 0, 1, 0, 0, 0, 0, 3.5, 3.5, 0, 0, 3.5, 0, 0.4))
	mock.ExpectQuery(`(?s)WITH .*records AS \(.*SELECT id, order_no.*FROM records WHERE user_id = \$1 AND verification_type = \$2 AND \(order_no ILIKE \$3.*LIMIT \$4 OFFSET \$5`).
		WithArgs(int64(42), "email", "%openai%", 20, 0).
		WillReturnRows(sqlmock.NewRows(verificationRecordColumns).AddRow(
			"record-id", "EML-1", "email", "gmail", int64(42), "user@example.com",
			"openai", "email_channel_1", "邮箱通道1", "emailnator", "mail@example.com", "",
			"failed", "failed", "released", "provider rejected request", 3.5, 0.4,
			3.5, 0, 0, 3.5, 0, "CNY", 2, "EMAIL_GENERATION_FAILED", "secret upstream detail",
			"邮箱生成失败，金额已退回", now, now, nil, nil, true, false,
		))

	result, err := NewVerificationRecordService(db).List(context.Background(), VerificationRecordListOptions{
		Page:     1,
		PageSize: 20,
		UserID:   42,
		Type:     "email",
		Keyword:  "openai",
	}, false)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Nil(t, item.UserID)
	require.Empty(t, item.UserEmail)
	require.Empty(t, item.ProviderCode)
	require.Nil(t, item.ProviderCost)
	require.Empty(t, item.ErrorCode)
	require.Empty(t, item.ErrorMessage)
	require.Empty(t, item.RefundReason)
	require.Equal(t, "邮箱生成失败，金额已退回", item.PublicErrorMessage)
	require.Nil(t, result.Summary.ProviderCost)
	require.Nil(t, result.Summary.EstimatedProfit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerificationRecordServiceAdminIncludesOperationalFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	mock.ExpectQuery(`(?s)WITH .*records AS \(.*COUNT\(\*\).*FROM records$`).
		WillReturnRows(sqlmock.NewRows([]string{
			"total", "processing", "success", "failed", "refunded", "free", "cancelled", "expired",
			"sale", "debit", "reserved", "captured", "released", "refunded_amount", "provider_cost",
		}).AddRow(1, 0, 1, 0, 0, 0, 0, 0, 5.0, 5.0, 0, 5.0, 0, 0, 1.25))
	mock.ExpectQuery(`(?s)WITH .*records AS \(.*SELECT id, order_no.*FROM records.*LIMIT \$1 OFFSET \$2`).
		WithArgs(25, 25).
		WillReturnRows(sqlmock.NewRows(verificationRecordColumns).AddRow(
			"sms-id", "sms-id", "sms", "temporary", int64(9), "admin-visible@example.com",
			"google", "channel_1", "手机通道1", "5sim", "+15551234567", "US",
			"completed", "success", "not_requested", "", 5.0, 1.25,
			5.0, 0, 5.0, 0, 0, "USD", nil, "", "provider trace", "", now, now, now, now.Add(time.Hour), false, false,
		))

	result, err := NewVerificationRecordService(db).List(context.Background(), VerificationRecordListOptions{
		Page:     2,
		PageSize: 25,
	}, true)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.NotNil(t, item.UserID)
	require.Equal(t, int64(9), *item.UserID)
	require.Equal(t, "admin-visible@example.com", item.UserEmail)
	require.Equal(t, "5sim", item.ProviderCode)
	require.NotNil(t, item.ProviderCost)
	require.Equal(t, 1.25, *item.ProviderCost)
	require.Equal(t, "provider trace", item.ErrorMessage)
	require.NotNil(t, result.Summary.ProviderCost)
	require.Equal(t, 1.25, *result.Summary.ProviderCost)
	require.NotNil(t, result.Summary.EstimatedProfit)
	require.Equal(t, 3.75, *result.Summary.EstimatedProfit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerificationRecordServiceRejectsInvalidFiltersBeforeQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = NewVerificationRecordService(db).List(context.Background(), VerificationRecordListOptions{
		UserID: 1,
		Type:   "voice",
	}, false)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidVerificationRecordFilter))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerificationRecordServiceAnalyticsUsesFiltersAndCurrencyGroups(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)WITH .*records AS \(.*filtered AS \(SELECT \* FROM records WHERE service_code = \$1 AND region = \$2\).*SELECT dimension`).
		WithArgs("google", "US").
		WillReturnRows(sqlmock.NewRows([]string{"dimension", "key", "total", "success", "success_rate"}).
			AddRow("platform", "google", 10, 8, 0.8).
			AddRow("country", "US", 7, 6, 6.0/7.0).
			AddRow("type", "sms", 10, 8, 0.8))
	mock.ExpectQuery(`(?s)WITH .*records AS \(.*filtered AS \(SELECT \* FROM records WHERE service_code = \$1 AND region = \$2\).*SELECT currency`).
		WithArgs("google", "US").
		WillReturnRows(sqlmock.NewRows([]string{"currency", "sale", "cost", "captured", "refunded", "estimated"}).
			AddRow("USD", 50.0, 12.5, 45.0, 5.0, false))

	result, err := NewVerificationRecordService(db).Analytics(context.Background(), VerificationRecordListOptions{ServiceCode: " Google ", Region: "us"}, true)
	require.NoError(t, err)
	require.Len(t, result.ByPlatform, 1)
	require.Equal(t, 0.8, result.ByPlatform[0].SuccessRate)
	require.Len(t, result.ByCountry, 1)
	require.Len(t, result.Financial, 1)
	require.Equal(t, 40.0, result.Financial[0].NetRevenue)
	require.Equal(t, 27.5, result.Financial[0].EstimatedProfit)
	require.False(t, result.Financial[0].Estimated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerificationRecordServiceOptionsAreSearchableAndPaginated(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*email_services.*value ILIKE \$1`).
		WithArgs("%goo%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT value,MAX\(label\).*email_services.*ORDER BY label,value LIMIT \$2 OFFSET \$3`).
		WithArgs("%goo%", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"value", "label", "label_en", "icon"}).AddRow("google", "Google", "Google", ""))

	result, err := NewVerificationRecordService(db).ListOptions(context.Background(), VerificationRecordOptionListOptions{Kind: "platform", Type: "email", Query: "goo", Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Equal(t, "google", result.Items[0].Value)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerificationRecordCTEUsesDurableSettlementLedger(t *testing.T) {
	require.Contains(t, verificationRecordsCTE, "ELSE o.captured_amount::float8")
	require.Contains(t, verificationRecordsCTE, "ELSE o.refunded_amount::float8")
	require.Contains(t, verificationRecordsCTE, "o.provider_refund_status IN ('succeeded','not_required')")
	require.Contains(t, verificationRecordsCTE, "o.settlement_status='legacy'")
	require.NotContains(t, verificationRecordsCTE, "CASE WHEN o.status IN ('active', 'completed', 'expired')")
}

func TestVerificationRecordCTENetsProviderCostForConfirmedRefundsAndNoDelivery(t *testing.T) {
	require.Contains(t, verificationRecordsCTE, "o.provider_refund_status IN ('succeeded','not_required')")
	require.Contains(t, verificationRecordsCTE, "p.code='smspva' AND o.product_type='temporary' AND o.settlement_status='held' AND o.first_sms_received_at IS NULL")
	require.Contains(t, verificationRecordsCTE, "LEFT JOIN email_usage eu ON eu.email_order_id=o.id")
}

func TestVerificationRecordCTEIncludesRentalAddOnLedger(t *testing.T) {
	require.Contains(t, verificationRecordsCTE, "rental_service_totals AS")
	require.Contains(t, verificationRecordsCTE, "LEFT JOIN rental_service_totals rt ON rt.order_id=o.id")
	require.Contains(t, verificationRecordsCTE, "COALESCE(rt.captured_amount,0)")
	require.Contains(t, verificationRecordsCTE, "COALESCE(rt.released_amount,0)")
	require.Contains(t, verificationRecordsCTE, "COALESCE(rt.provider_cost,0)")
	require.Contains(t, verificationRecordsCTE, "GREATEST(o.reserved_amount-o.captured_amount-o.released_amount-o.refunded_amount,0)")
}


func TestVerificationRecordCTEClassifiesZeroCostEmailTerminalOrdersAsFree(t *testing.T) {
	require.Contains(t, verificationRecordsCTE, "WHEN o.sale_price_snapshot = 0 AND o.status IN ('cancelled','expired','refunded') THEN 'free'")
	require.Contains(t, verificationRecordsCTE, "CASE WHEN o.sale_price_snapshot = 0 THEN 'not_applicable' ELSE o.refund_status END AS refund_status")
	require.Contains(t, verificationRecordsCTE, "CASE WHEN o.sale_price_snapshot = 0 THEN 0::float8 ELSE o.refunded_amount::float8 END")
	require.Contains(t, verificationRecordsCTE, "COUNT(*) FILTER (WHERE outcome NOT IN ('processing','free'))::bigint")
}

func TestVerificationRecordServiceAcceptsFreeOutcomeFilter(t *testing.T) {
	require.True(t, isVerificationOutcome("free"))
}
