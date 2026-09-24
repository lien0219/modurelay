package repository

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestSelectLotteryPrizeUsesPPMBoundaries(t *testing.T) {
	prizes := []service.ActivityPrize{
		{ID: 1, ProbabilityPPM: 100_000},
		{ID: 2, ProbabilityPPM: 900_000},
	}
	first, err := selectLotteryPrize(prizes, 99_999)
	require.NoError(t, err)
	require.Equal(t, int64(1), first.ID)
	second, err := selectLotteryPrize(prizes, 100_000)
	require.NoError(t, err)
	require.Equal(t, int64(2), second.ID)
	_, err = selectLotteryPrize(prizes, service.ActivityProbabilityScale)
	require.ErrorIs(t, err, service.ErrActivityConfigInvalid)
}

func TestSelectBenefitRewardAmountSupportsFixedAndInclusiveRandomRewards(t *testing.T) {
	fixed, err := selectBenefitRewardAmount(service.BenefitConfig{
		RewardAmount:    "5.00000000",
		RandomMinAmount: "0.00000000",
		RandomMaxAmount: "0.00000000",
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "5", fixed.String())

	deterministic, err := selectBenefitRewardAmount(service.BenefitConfig{
		RewardAmount:    "0.00000000",
		RandomMinAmount: "1.23000000",
		RandomMaxAmount: "1.23000000",
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "1.23", deterministic.String())

	config := service.BenefitConfig{
		RewardAmount:    "0.00000000",
		RandomMinAmount: "0.01000000",
		RandomMaxAmount: "8.88000000",
	}
	for range 100 {
		amount, randomErr := selectBenefitRewardAmount(config, cryptorand.Reader)
		require.NoError(t, randomErr)
		require.True(t, amount.GreaterThanOrEqual(decimal.RequireFromString("0.01")))
		require.True(t, amount.LessThanOrEqual(decimal.RequireFromString("8.88")))
		require.Equal(t, amount.Truncate(2), amount)
	}
}

func TestSelectBenefitRewardAmountRejectsIncompleteRandomConfiguration(t *testing.T) {
	_, err := selectBenefitRewardAmount(service.BenefitConfig{
		RewardAmount:    "0",
		RandomMinAmount: "0",
		RandomMaxAmount: "0",
	}, cryptorand.Reader)
	require.ErrorIs(t, err, service.ErrActivityConfigInvalid)
}

func TestFindLotteryDrawScopesRequestIDToActivity(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 9, 2, 13, 0, 0, 0, time.UTC)
	mock.ExpectQuery("JOIN activities activity ON activity.id = d.activity_id").
		WithArgs(int64(7), "lottery-autumn", "request-1234567890").
		WillReturnRows(sqlmock.NewRows([]string{"id", "prize_id", "prize_name", "reward_amount", "balance_after", "available_draws", "created_at"}).
			AddRow(31, 41, "Balance reward", "8.88", "108.88", 2, now))

	result, err := findLotteryDraw(context.Background(), db, 7, "lottery-autumn", "request-1234567890")
	require.NoError(t, err)
	require.Equal(t, int64(31), result.DrawID)
	require.Equal(t, "108.88", result.BalanceAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindBenefitClaimScopesRequestIDToActivity(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 9, 2, 13, 30, 0, 0, time.UTC)
	mock.ExpectQuery("JOIN activities activity ON activity.id = c.activity_id").
		WithArgs(int64(8), "benefit-autumn", "request-0987654321").
		WillReturnRows(sqlmock.NewRows([]string{"id", "reward_amount", "balance_after", "remaining_stock", "created_at"}).
			AddRow(51, "5", "25", 99, now))

	result, err := findBenefitClaim(context.Background(), db, 8, "benefit-autumn", "request-0987654321")
	require.NoError(t, err)
	require.Equal(t, int64(51), result.ClaimID)
	require.Equal(t, int64(99), result.RemainingStock)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCountBenefitClaimsTodayUsesChinaCalendarDayAcrossConfigVersions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 9, 2, 16, 30, 0, 0, time.UTC)
	dayStart := time.Date(2026, 9, 2, 16, 0, 0, 0, time.UTC)
	dayEnd := time.Date(2026, 9, 3, 16, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM activity_benefit_claims").
		WithArgs(int64(2), int64(8), dayStart, dayEnd).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	count, err := countBenefitClaimsToday(context.Background(), db, 2, 8, now)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "2026-09-03", activityLocalDate(now, service.BenefitDailyLimitTimezone))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRetryActivitySerializableRetriesPostgresSerializationFailures(t *testing.T) {
	attempts := 0
	result, err := retryActivitySerializable(context.Background(), func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, &pq.Error{Code: "40001"}
		}
		return 42, nil
	})
	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.Equal(t, 3, attempts)
}

func TestRetryActivitySerializableDoesNotRetryDomainErrors(t *testing.T) {
	attempts := 0
	_, err := retryActivitySerializable(context.Background(), func() (int, error) {
		attempts++
		return 0, service.ErrLotteryNoChance
	})
	require.ErrorIs(t, err, service.ErrLotteryNoChance)
	require.Equal(t, 1, attempts)
}

func TestCalculateCumulativeLotteryEntitlementAccumulatesAcrossPayments(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(12, "30", "50", 1, 0).
			AddRow(12, "20", "50", 1, 0))

	result, err := calculateCumulativeLotteryEntitlement(context.Background(), db, 7, 11, 12)
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalDraws)
	require.Equal(t, "50", result.CurrentConfigRecharge.String())
	require.True(t, result.CurrentConfigProgress.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCalculateCumulativeLotteryEntitlementKeepsConfigRemaindersSeparate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(11, "30", "50", 1, 0).
			AddRow(12, "20", "50", 1, 0))

	result, err := calculateCumulativeLotteryEntitlement(context.Background(), db, 7, 11, 12)
	require.NoError(t, err)
	require.Zero(t, result.TotalDraws)
	require.Equal(t, "20", result.CurrentConfigRecharge.String())
	require.Equal(t, "20", result.CurrentConfigProgress.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCalculateCumulativeLotteryEntitlementPreservesPerOrderCap(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(12, "250", "50", 1, 2).
			AddRow(12, "50", "50", 1, 2))

	result, err := calculateCumulativeLotteryEntitlement(context.Background(), db, 7, 11, 12)
	require.NoError(t, err)
	require.Equal(t, 3, result.TotalDraws)
	require.Equal(t, "300", result.CurrentConfigRecharge.String())
	require.True(t, result.CurrentConfigProgress.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeRechargeQualificationTxRecalculatesPartialRefundEligibility(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT q.id, q.user_id, q.activity_id, q.recharge_amount").
		WithArgs(int64(91)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "activity_id", "recharge_amount"}).
			AddRow(1, 7, 11, "300"))
	mock.ExpectExec("UPDATE activity_lottery_qualifications").
		WithArgs(int64(1), "150").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(12, "150", "100", 1, 0))
	mock.ExpectQuery("SELECT granted_draws, used_draws").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"granted_draws", "used_draws"}).AddRow(3, 0))
	mock.ExpectExec("UPDATE activity_lottery_draw_accounts").
		WithArgs(int64(7), int64(11), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id, granted_draws - revoked_draws").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "available"}).AddRow(1, 3))
	mock.ExpectExec("UPDATE activity_lottery_qualifications").
		WithArgs(int64(1), 2, now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &activityRepository{db: db}
	revoked, err := repo.RevokeRechargeQualificationTx(context.Background(), db, 91, "150", now)
	require.NoError(t, err)
	require.Equal(t, 2, revoked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeRechargeQualificationTxNeverRevokesConsumedDraws(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT q.id, q.user_id, q.activity_id, q.recharge_amount").
		WithArgs(int64(92)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "activity_id", "recharge_amount"}).
			AddRow(2, 8, 12, "300"))
	mock.ExpectExec("UPDATE activity_lottery_qualifications").
		WithArgs(int64(2), "300").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(8), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(13, "0", "100", 1, 0))
	mock.ExpectQuery("SELECT granted_draws, used_draws").
		WithArgs(int64(8), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"granted_draws", "used_draws"}).AddRow(3, 2))
	mock.ExpectExec("UPDATE activity_lottery_draw_accounts").
		WithArgs(int64(8), int64(12), 2, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id, granted_draws - revoked_draws").
		WithArgs(int64(8), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "available"}).AddRow(2, 3))
	mock.ExpectExec("UPDATE activity_lottery_qualifications").
		WithArgs(int64(2), 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &activityRepository{db: db}
	revoked, err := repo.RevokeRechargeQualificationTx(context.Background(), db, 92, "300", now)
	require.NoError(t, err)
	require.Equal(t, 1, revoked)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeRechargeQualificationTxRejectsInvalidRefundAmount(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &activityRepository{db: db}
	_, err = repo.RevokeRechargeQualificationTx(context.Background(), db, 1, "0", time.Now())
	require.True(t, errors.Is(err, service.ErrActivityRequestInvalid))
}

func TestCompleteBalancePaymentGrantsQualificationExactlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	completedAt := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	leaseVersion := completedAt.Add(-time.Minute)
	input := service.CompleteActivityPaymentInput{
		OrderID:        101,
		UserID:         7,
		RechargeAmount: "250",
		Currency:       "CNY",
		LeaseVersion:   leaseVersion,
		CompletedAt:    completedAt,
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE payment_orders").
		WithArgs(int64(101), completedAt, leaseVersion, service.OrderStatusCompleted, service.OrderStatusRecharging).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(service.SettingKeyActivityCenterEnabled).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT a.id, c.id").
		WithArgs("CNY", completedAt).
		WillReturnRows(sqlmock.NewRows([]string{"activity_id", "config_id"}).AddRow(11, 12))
	mock.ExpectExec("INSERT INTO activity_lottery_qualifications").
		WithArgs(int64(11), int64(12), int64(101), int64(7), "250", "CNY", 0, completedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT granted_draws, used_draws").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"granted_draws", "used_draws"}))
	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(12, "250", "100", 2, 3))
	mock.ExpectExec("UPDATE activity_lottery_qualifications").
		WithArgs(int64(11), 3, int64(101)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO activity_lottery_draw_accounts").
		WithArgs(int64(7), int64(11), 3).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE payment_orders").
		WithArgs(int64(101), completedAt, leaseVersion, service.OrderStatusCompleted, service.OrderStatusRecharging).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT status FROM payment_orders").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(service.OrderStatusCompleted))
	mock.ExpectRollback()

	repo := &activityRepository{db: db}
	first, err := repo.CompleteBalancePayment(context.Background(), input)
	require.NoError(t, err)
	require.True(t, first.Completed)
	require.Equal(t, 3, first.GrantedDraws)

	second, err := repo.CompleteBalancePayment(context.Background(), input)
	require.NoError(t, err)
	require.False(t, second.Completed)
	require.Zero(t, second.GrantedDraws)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrantBalanceRedeemQualificationTxGrantsExactlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	redeemedAt := time.Date(2026, 9, 2, 14, 0, 0, 0, time.UTC)
	input := service.BalanceRedeemQualificationInput{
		RedeemCodeID:   201,
		UserID:         7,
		RechargeAmount: "100",
		Currency:       "cny",
		RedeemedAt:     redeemedAt,
	}

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(service.SettingKeyActivityCenterEnabled).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT a.id, c.id").
		WithArgs("CNY", redeemedAt).
		WillReturnRows(sqlmock.NewRows([]string{"activity_id", "config_id"}).AddRow(11, 12))
	mock.ExpectExec("INSERT INTO activity_lottery_qualifications").
		WithArgs(int64(11), int64(12), int64(201), int64(7), "100", "CNY", 0, redeemedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT granted_draws, used_draws").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"granted_draws", "used_draws"}))
	mock.ExpectQuery("SELECT q.config_id, \\(q.recharge_amount - q.refunded_amount\\)::text").
		WithArgs(int64(7), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"config_id", "net_amount", "threshold", "draws", "max_per_order"}).
			AddRow(12, "100", "50", 1, 0))
	mock.ExpectExec("UPDATE activity_lottery_qualifications").
		WithArgs(int64(11), 2, int64(201)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO activity_lottery_draw_accounts").
		WithArgs(int64(7), int64(11), 2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	first, err := (&activityRepository{db: db}).GrantBalanceRedeemQualificationTx(context.Background(), db, input)
	require.NoError(t, err)
	require.Equal(t, 2, first)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(service.SettingKeyActivityCenterEnabled).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT a.id, c.id").
		WithArgs("CNY", redeemedAt).
		WillReturnRows(sqlmock.NewRows([]string{"activity_id", "config_id"}).AddRow(11, 12))
	mock.ExpectExec("INSERT INTO activity_lottery_qualifications").
		WithArgs(int64(11), int64(12), int64(201), int64(7), "100", "CNY", 0, redeemedAt).
		WillReturnResult(sqlmock.NewResult(0, 0))

	second, err := (&activityRepository{db: db}).GrantBalanceRedeemQualificationTx(context.Background(), db, input)
	require.NoError(t, err)
	require.Zero(t, second)
	require.NoError(t, mock.ExpectationsWereMet())
}
