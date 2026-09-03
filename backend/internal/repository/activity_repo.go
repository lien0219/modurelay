package repository

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

const activitySerializableAttempts = 4

const (
	lotteryQualificationSourcePayment = "payment_order"
	lotteryQualificationSourceRedeem  = "redeem_code"
)

type lotteryRechargeQualificationInput struct {
	sourceType     string
	sourceID       int64
	userID         int64
	rechargeAmount string
	currency       string
	qualifiedAt    time.Time
}

type activityRepository struct {
	db *sql.DB
}

type activityRowsQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type activityQueryer interface {
	activityRowsQueryer
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewActivityRepository(db *sql.DB) service.ActivityRepository {
	return &activityRepository{db: db}
}

func retryActivitySerializable[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; ; attempt++ {
		result, err := operation()
		if err == nil || !isActivityRetryableTransactionError(err) || attempt+1 >= activitySerializableAttempts {
			return result, err
		}
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		delay := time.Duration(attempt+1) * 10 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
}

func isActivityRetryableTransactionError(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr != nil && (pqErr.Code == "40001" || pqErr.Code == "40P01")
}

func (r *activityRepository) ListActivities(ctx context.Context, userID int64, includeArchived bool, now time.Time) ([]service.Activity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, slug, activity_type, title, description, status, enabled,
       sort_order, current_config_version, created_at, updated_at
FROM activities
WHERE $1 OR status = 'published'
ORDER BY sort_order ASC, id ASC`, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.Activity, 0)
	for rows.Next() {
		activity, scanErr := scanActivity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if err := r.hydrateActivity(ctx, r.db, activity, userID, now); err != nil {
			return nil, err
		}
		items = append(items, *activity)
	}
	return items, rows.Err()
}

func (r *activityRepository) GetActivity(ctx context.Context, slug string, userID int64, includeArchived bool, now time.Time) (*service.Activity, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, slug, activity_type, title, description, status, enabled,
       sort_order, current_config_version, created_at, updated_at
FROM activities
WHERE slug = $1 AND ($2 OR status = 'published')`, slug, includeArchived)
	activity, err := scanActivity(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrActivityNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := r.hydrateActivity(ctx, r.db, activity, userID, now); err != nil {
		return nil, err
	}
	return activity, nil
}

type activityScanner interface {
	Scan(dest ...any) error
}

func scanActivity(scanner activityScanner) (*service.Activity, error) {
	var activity service.Activity
	if err := scanner.Scan(
		&activity.ID,
		&activity.Slug,
		&activity.Type,
		&activity.Title,
		&activity.Description,
		&activity.Status,
		&activity.Enabled,
		&activity.SortOrder,
		&activity.CurrentConfigVersion,
		&activity.CreatedAt,
		&activity.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *activityRepository) hydrateActivity(ctx context.Context, queryer activityQueryer, activity *service.Activity, userID int64, now time.Time) error {
	switch activity.Type {
	case service.ActivityTypeRechargeLottery:
		config, err := loadLotteryConfig(ctx, queryer, activity.ID, activity.CurrentConfigVersion)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		activity.Lottery = config
		if userID > 0 && config != nil {
			participation, err := loadLotteryParticipation(ctx, queryer, activity.ID, userID, config, now)
			if err != nil {
				return err
			}
			activity.Participation = participation
		}
	case service.ActivityTypeLimitedBenefit:
		config, err := loadBenefitConfig(ctx, queryer, activity.ID, activity.CurrentConfigVersion)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		activity.Benefit = config
		if userID > 0 && config != nil {
			participation, err := loadBenefitParticipation(ctx, queryer, activity.ID, userID, config, now)
			if err != nil {
				return err
			}
			activity.Participation = participation
		}
	}
	setActivityAvailability(activity, now)
	return nil
}

func loadLotteryConfig(ctx context.Context, queryer activityQueryer, activityID int64, version int) (*service.LotteryConfig, error) {
	var config service.LotteryConfig
	var startsAt, endsAt sql.NullTime
	err := queryer.QueryRowContext(ctx, `
SELECT id, version, currency, recharge_threshold::text, draws_per_threshold,
       max_chances_per_order, per_user_draw_limit, daily_draw_limit,
       daily_limit_timezone, starts_at, ends_at
FROM activity_lottery_configs
WHERE activity_id = $1 AND version = $2`, activityID, version).Scan(
		&config.ID,
		&config.Version,
		&config.Currency,
		&config.RechargeThreshold,
		&config.DrawsPerThreshold,
		&config.MaxChancesPerOrder,
		&config.PerUserDrawLimit,
		&config.DailyDrawLimit,
		&config.DailyLimitTimezone,
		&startsAt,
		&endsAt,
	)
	if err != nil {
		return nil, err
	}
	config.StartsAt = nullableTime(startsAt)
	config.EndsAt = nullableTime(endsAt)
	config.Prizes = make([]service.ActivityPrize, 0)
	rows, err := queryer.QueryContext(ctx, `
SELECT id, name, amount::text, probability_ppm, sort_order
FROM activity_lottery_prizes
WHERE config_id = $1
ORDER BY sort_order ASC, id ASC`, config.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var prize service.ActivityPrize
		if err := rows.Scan(&prize.ID, &prize.Name, &prize.Amount, &prize.ProbabilityPPM, &prize.SortOrder); err != nil {
			return nil, err
		}
		config.Prizes = append(config.Prizes, prize)
	}
	return &config, rows.Err()
}

func loadBenefitConfig(ctx context.Context, queryer activityQueryer, activityID int64, version int) (*service.BenefitConfig, error) {
	var config service.BenefitConfig
	var startsAt, endsAt sql.NullTime
	err := queryer.QueryRowContext(ctx, `
SELECT c.id, c.version, c.currency, c.reward_amount::text,
       c.random_min_amount::text, c.random_max_amount::text, c.total_stock,
       c.per_user_limit, c.starts_at, c.ends_at, COUNT(cl.id)
FROM activity_benefit_configs c
LEFT JOIN activity_benefit_claims cl ON cl.config_id = c.id
WHERE c.activity_id = $1 AND c.version = $2
GROUP BY c.id`, activityID, version).Scan(
		&config.ID,
		&config.Version,
		&config.Currency,
		&config.RewardAmount,
		&config.RandomMinAmount,
		&config.RandomMaxAmount,
		&config.TotalStock,
		&config.PerUserLimit,
		&startsAt,
		&endsAt,
		&config.ClaimedCount,
	)
	if err != nil {
		return nil, err
	}
	config.StartsAt = nullableTime(startsAt)
	config.EndsAt = nullableTime(endsAt)
	config.DailyClaimLimit = service.BenefitDailyClaimLimit
	config.DailyLimitTimezone = service.BenefitDailyLimitTimezone
	return &config, nil
}

func loadLotteryParticipation(ctx context.Context, queryer activityQueryer, activityID, userID int64, config *service.LotteryConfig, now time.Time) (*service.ActivityParticipation, error) {
	participation := &service.ActivityParticipation{
		CumulativeRechargeAmount: "0",
		RechargeProgressAmount:   "0",
		NextDrawRechargeAmount:   config.RechargeThreshold,
		RewardTotal:              "0",
	}
	err := queryer.QueryRowContext(ctx, `
SELECT granted_draws, used_draws
FROM activity_lottery_draw_accounts
WHERE activity_id = $1 AND user_id = $2`, activityID, userID).Scan(&participation.GrantedDraws, &participation.UsedDraws)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	participation.AvailableDraws = max(0, participation.GrantedDraws-participation.UsedDraws)
	if config.PerUserDrawLimit > 0 {
		participation.AvailableDraws = min(participation.AvailableDraws, max(0, config.PerUserDrawLimit-participation.UsedDraws))
	}
	if config.DailyDrawLimit > 0 {
		dayStart, dayEnd := activityDayWindow(now, config.DailyLimitTimezone)
		if err := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM activity_lottery_draws
WHERE activity_id = $1 AND user_id = $2 AND created_at >= $3 AND created_at < $4`, activityID, userID, dayStart, dayEnd).Scan(&participation.DrawnToday); err != nil {
			return nil, err
		}
		participation.AvailableDraws = min(participation.AvailableDraws, max(0, config.DailyDrawLimit-participation.DrawnToday))
	}
	entitlement, err := calculateCumulativeLotteryEntitlement(ctx, queryer, userID, activityID, config.ID)
	if err != nil {
		return nil, err
	}
	participation.CumulativeRechargeAmount = entitlement.CurrentConfigRecharge.String()
	participation.RechargeProgressAmount = entitlement.CurrentConfigProgress.String()
	threshold, err := decimal.NewFromString(config.RechargeThreshold)
	if err != nil || !threshold.IsPositive() {
		return nil, service.ErrActivityConfigInvalid
	}
	nextAmount := threshold.Sub(entitlement.CurrentConfigProgress)
	if !nextAmount.IsPositive() {
		nextAmount = threshold
	}
	participation.NextDrawRechargeAmount = nextAmount.String()
	if err := queryer.QueryRowContext(ctx, `
SELECT COALESCE(SUM(amount), 0)::text
FROM activity_reward_ledger
WHERE activity_id = $1 AND user_id = $2 AND source_type = 'lottery_draw'`, activityID, userID).Scan(&participation.RewardTotal); err != nil {
		return nil, err
	}
	return participation, nil
}

type cumulativeLotteryEntitlement struct {
	TotalDraws            int
	CurrentConfigRecharge decimal.Decimal
	CurrentConfigProgress decimal.Decimal
}

func calculateCumulativeLotteryEntitlement(ctx context.Context, queryer activityRowsQueryer, userID, activityID, currentConfigID int64) (*cumulativeLotteryEntitlement, error) {
	rows, err := queryer.QueryContext(ctx, `
SELECT q.config_id, (q.recharge_amount - q.refunded_amount)::text,
       c.recharge_threshold::text, c.draws_per_threshold, c.max_chances_per_order
FROM activity_lottery_qualifications q
JOIN activity_lottery_configs c ON c.id = q.config_id
WHERE q.user_id = $1 AND q.activity_id = $2
ORDER BY q.config_id, q.created_at, q.id`, userID, activityID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &cumulativeLotteryEntitlement{}
	var activeConfigID int64
	remainder := decimal.Zero
	for rows.Next() {
		var configID int64
		var netAmountText, thresholdText string
		var drawsPerThreshold, maxPerOrder int
		if err := rows.Scan(&configID, &netAmountText, &thresholdText, &drawsPerThreshold, &maxPerOrder); err != nil {
			return nil, err
		}
		netAmount, amountErr := decimal.NewFromString(netAmountText)
		threshold, thresholdErr := decimal.NewFromString(thresholdText)
		if amountErr != nil || thresholdErr != nil || netAmount.IsNegative() || !threshold.IsPositive() || drawsPerThreshold <= 0 {
			return nil, service.ErrActivityConfigInvalid
		}
		if configID != activeConfigID {
			activeConfigID = configID
			remainder = decimal.Zero
		}
		combined := remainder.Add(netAmount)
		units := combined.Div(threshold).Floor()
		remainder = combined.Sub(threshold.Mul(units))
		eligible := units.Mul(decimal.NewFromInt(int64(drawsPerThreshold)))
		if maxPerOrder > 0 && eligible.GreaterThan(decimal.NewFromInt(int64(maxPerOrder))) {
			eligible = decimal.NewFromInt(int64(maxPerOrder))
		}
		maxAdditional := decimal.NewFromInt(int64(^uint(0)>>1) - int64(result.TotalDraws))
		if eligible.IsNegative() || eligible.GreaterThan(maxAdditional) {
			return nil, service.ErrActivityConfigInvalid
		}
		result.TotalDraws += int(eligible.IntPart())
		if configID == currentConfigID {
			result.CurrentConfigRecharge = result.CurrentConfigRecharge.Add(netAmount)
			result.CurrentConfigProgress = remainder
		}
	}
	return result, rows.Err()
}

func loadBenefitParticipation(ctx context.Context, queryer activityQueryer, activityID, userID int64, config *service.BenefitConfig, now time.Time) (*service.ActivityParticipation, error) {
	participation := &service.ActivityParticipation{RemainingStock: max(int64(0), config.TotalStock-config.ClaimedCount)}
	if err := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM activity_benefit_claims
WHERE activity_id = $1 AND config_id = $2 AND user_id = $3`, activityID, config.ID, userID).Scan(&participation.BenefitClaims); err != nil {
		return nil, err
	}
	claimsToday, err := countBenefitClaimsToday(ctx, queryer, activityID, userID, now)
	if err != nil {
		return nil, err
	}
	participation.BenefitClaimsToday = claimsToday
	return participation, nil
}

func countBenefitClaimsToday(ctx context.Context, queryer activityQueryer, activityID, userID int64, now time.Time) (int, error) {
	dayStart, dayEnd := activityDayWindow(now, service.BenefitDailyLimitTimezone)
	var count int
	err := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM activity_benefit_claims
WHERE activity_id = $1 AND user_id = $2 AND created_at >= $3 AND created_at < $4`, activityID, userID, dayStart, dayEnd).Scan(&count)
	return count, err
}

func setActivityAvailability(activity *service.Activity, now time.Time) {
	activity.Availability = "active"
	activity.ClosedReason = ""
	if activity.Status != service.ActivityStatusPublished {
		activity.Availability = "closed"
		activity.ClosedReason = "unpublished"
		return
	}
	if !activity.Enabled {
		activity.Availability = "closed"
		activity.ClosedReason = "disabled"
		return
	}
	var startsAt, endsAt *time.Time
	switch activity.Type {
	case service.ActivityTypeRechargeLottery:
		if activity.Lottery == nil {
			activity.Availability = "closed"
			activity.ClosedReason = "configuration_required"
			return
		}
		startsAt, endsAt = activity.Lottery.StartsAt, activity.Lottery.EndsAt
	case service.ActivityTypeLimitedBenefit:
		if activity.Benefit == nil {
			activity.Availability = "closed"
			activity.ClosedReason = "configuration_required"
			return
		}
		startsAt, endsAt = activity.Benefit.StartsAt, activity.Benefit.EndsAt
	}
	if startsAt != nil && now.Before(*startsAt) {
		activity.Availability = "upcoming"
		activity.ClosedReason = "not_started"
	} else if endsAt != nil && !now.Before(*endsAt) {
		activity.Availability = "ended"
		activity.ClosedReason = "ended"
	}
}

func (r *activityRepository) UpdateActivity(ctx context.Context, slug string, input service.UpdateActivityInput, now time.Time) (*service.Activity, error) {
	return retryActivitySerializable(ctx, func() (*service.Activity, error) {
		return r.updateActivityOnce(ctx, slug, input, now)
	})
}

func (r *activityRepository) updateActivityOnce(ctx context.Context, slug string, input service.UpdateActivityInput, now time.Time) (*service.Activity, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var activityID int64
	var activityType, currentStatus string
	var currentEnabled bool
	var currentVersion int
	if err := tx.QueryRowContext(ctx, `
SELECT id, activity_type, status, enabled, current_config_version
FROM activities WHERE slug = $1 FOR UPDATE`, slug).Scan(&activityID, &activityType, &currentStatus, &currentEnabled, &currentVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrActivityNotFound
		}
		return nil, err
	}
	effectiveStatus := currentStatus
	if input.Status != nil {
		effectiveStatus = *input.Status
	}
	effectiveEnabled := currentEnabled
	if input.Enabled != nil {
		effectiveEnabled = *input.Enabled
	}
	if effectiveEnabled {
		if effectiveStatus != service.ActivityStatusPublished || !validActiveConfig(ctx, tx, activityID, activityType, currentVersion, now) {
			return nil, service.ErrActivityConfigInvalid
		}
	}

	_, err = tx.ExecContext(ctx, `
UPDATE activities
SET title = COALESCE($2, title),
    description = COALESCE($3, description),
    status = COALESCE($4, status),
    enabled = COALESCE($5, enabled),
    sort_order = COALESCE($6, sort_order),
    updated_at = NOW()
WHERE id = $1`, activityID, pointerValue(input.Title), pointerValue(input.Description), pointerValue(input.Status), pointerValue(input.Enabled), pointerValue(input.SortOrder))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetActivity(ctx, slug, 0, true, time.Now().UTC())
}

func validActiveConfig(ctx context.Context, tx *sql.Tx, activityID int64, activityType string, version int, now time.Time) bool {
	switch activityType {
	case service.ActivityTypeRechargeLottery:
		var prizeCount, probabilityTotal, positiveRewards int
		var endsAt sql.NullTime
		err := tx.QueryRowContext(ctx, `
SELECT COUNT(p.id), COALESCE(SUM(p.probability_ppm), 0),
       COUNT(p.id) FILTER (WHERE p.amount > 0), c.ends_at
FROM activity_lottery_configs c
LEFT JOIN activity_lottery_prizes p ON p.config_id = c.id
WHERE c.activity_id = $1 AND c.version = $2
GROUP BY c.id`, activityID, version).Scan(&prizeCount, &probabilityTotal, &positiveRewards, &endsAt)
		return err == nil && prizeCount >= 2 && probabilityTotal == service.ActivityProbabilityScale && positiveRewards > 0 && (!endsAt.Valid || now.Before(endsAt.Time))
	case service.ActivityTypeLimitedBenefit:
		var valid bool
		err := tx.QueryRowContext(ctx, `
SELECT (
           (reward_amount > 0 AND random_min_amount = 0 AND random_max_amount = 0)
           OR
           (reward_amount = 0 AND random_min_amount > 0 AND random_max_amount >= random_min_amount)
       )
       AND total_stock > 0
       AND (ends_at IS NULL OR ends_at > $3)
FROM activity_benefit_configs
WHERE activity_id = $1 AND version = $2`, activityID, version, now).Scan(&valid)
		return err == nil && valid
	default:
		return false
	}
}

func (r *activityRepository) PublishLotteryConfig(ctx context.Context, slug string, input service.LotteryConfigInput) (*service.Activity, error) {
	return retryActivitySerializable(ctx, func() (*service.Activity, error) {
		return r.publishLotteryConfigOnce(ctx, slug, input)
	})
}

func (r *activityRepository) publishLotteryConfigOnce(ctx context.Context, slug string, input service.LotteryConfigInput) (*service.Activity, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var activityID int64
	var activityType string
	if err := tx.QueryRowContext(ctx, `SELECT id, activity_type FROM activities WHERE slug = $1 FOR UPDATE`, slug).Scan(&activityID, &activityType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrActivityNotFound
		}
		return nil, err
	}
	if activityType != service.ActivityTypeRechargeLottery {
		return nil, service.ErrActivityConfigInvalid
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM activity_lottery_configs WHERE activity_id = $1`, activityID).Scan(&version); err != nil {
		return nil, err
	}
	var configID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO activity_lottery_configs (
    activity_id, version, currency, recharge_threshold, draws_per_threshold,
    max_chances_per_order, per_user_draw_limit, daily_draw_limit,
    daily_limit_timezone, starts_at, ends_at, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, 0))
RETURNING id`, activityID, version, strings.ToUpper(strings.TrimSpace(input.Currency)), input.RechargeThreshold,
		input.DrawsPerThreshold, input.MaxChancesPerOrder, input.PerUserDrawLimit, input.DailyDrawLimit,
		strings.TrimSpace(input.DailyLimitTimezone), input.StartsAt, input.EndsAt, input.CreatedBy).Scan(&configID)
	if err != nil {
		return nil, err
	}
	for _, prize := range input.Prizes {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO activity_lottery_prizes (config_id, name, amount, probability_ppm, sort_order)
VALUES ($1, $2, $3, $4, $5)`, configID, strings.TrimSpace(prize.Name), prize.Amount, prize.ProbabilityPPM, prize.SortOrder); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activities SET current_config_version = $2, updated_at = NOW() WHERE id = $1`, activityID, version); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetActivity(ctx, slug, 0, true, time.Now().UTC())
}

func (r *activityRepository) PublishBenefitConfig(ctx context.Context, slug string, input service.BenefitConfigInput) (*service.Activity, error) {
	return retryActivitySerializable(ctx, func() (*service.Activity, error) {
		return r.publishBenefitConfigOnce(ctx, slug, input)
	})
}

func (r *activityRepository) publishBenefitConfigOnce(ctx context.Context, slug string, input service.BenefitConfigInput) (*service.Activity, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var activityID int64
	var activityType string
	if err := tx.QueryRowContext(ctx, `SELECT id, activity_type FROM activities WHERE slug = $1 FOR UPDATE`, slug).Scan(&activityID, &activityType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrActivityNotFound
		}
		return nil, err
	}
	if activityType != service.ActivityTypeLimitedBenefit {
		return nil, service.ErrActivityConfigInvalid
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM activity_benefit_configs WHERE activity_id = $1`, activityID).Scan(&version); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO activity_benefit_configs (
    activity_id, version, currency, reward_amount, random_min_amount,
    random_max_amount, total_stock, per_user_limit, starts_at, ends_at, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, 0))`, activityID, version,
		strings.ToUpper(strings.TrimSpace(input.Currency)), input.RewardAmount,
		normalizedActivityMoney(input.RandomMinAmount), normalizedActivityMoney(input.RandomMaxAmount),
		input.TotalStock, input.PerUserLimit, input.StartsAt, input.EndsAt, input.CreatedBy); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activities SET current_config_version = $2, updated_at = NOW() WHERE id = $1`, activityID, version); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetActivity(ctx, slug, 0, true, time.Now().UTC())
}

func (r *activityRepository) CompleteBalancePayment(ctx context.Context, input service.CompleteActivityPaymentInput) (*service.ActivityPaymentCompletion, error) {
	return retryActivitySerializable(ctx, func() (*service.ActivityPaymentCompletion, error) {
		return r.completeBalancePaymentOnce(ctx, input)
	})
}

func (r *activityRepository) completeBalancePaymentOnce(ctx context.Context, input service.CompleteActivityPaymentInput) (*service.ActivityPaymentCompletion, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
UPDATE payment_orders
SET status = 'completed', completed_at = $2, updated_at = $2
WHERE id = $1 AND status = 'recharging' AND updated_at = $3`, input.OrderID, input.CompletedAt, input.LeaseVersion)
	if err != nil {
		return nil, fmt.Errorf("complete payment order: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM payment_orders WHERE id = $1`, input.OrderID).Scan(&status); err != nil {
			return nil, err
		}
		if status == service.OrderStatusCompleted {
			return &service.ActivityPaymentCompletion{}, nil
		}
		return nil, infraerrors.Conflict("CONFLICT", "fulfillment lease was lost before completion")
	}

	completion := &service.ActivityPaymentCompletion{Completed: true}
	if strings.ToUpper(strings.TrimSpace(input.Currency)) == "CNY" {
		granted, err := grantRechargeLotteryQualifications(ctx, tx, lotteryRechargeQualificationInput{
			sourceType:     lotteryQualificationSourcePayment,
			sourceID:       input.OrderID,
			userID:         input.UserID,
			rechargeAmount: input.RechargeAmount,
			currency:       input.Currency,
			qualifiedAt:    input.CompletedAt,
		})
		if err != nil {
			return nil, err
		}
		completion.GrantedDraws = granted
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return completion, nil
}

func (r *activityRepository) GrantBalanceRedeemQualificationTx(ctx context.Context, executor service.ActivityTxExecutor, input service.BalanceRedeemQualificationInput) (int, error) {
	if executor == nil || input.RedeemCodeID <= 0 || input.UserID <= 0 {
		return 0, service.ErrActivityRequestInvalid
	}
	return grantRechargeLotteryQualifications(ctx, executor, lotteryRechargeQualificationInput{
		sourceType:     lotteryQualificationSourceRedeem,
		sourceID:       input.RedeemCodeID,
		userID:         input.UserID,
		rechargeAmount: input.RechargeAmount,
		currency:       input.Currency,
		qualifiedAt:    input.RedeemedAt,
	})
}

func grantRechargeLotteryQualifications(ctx context.Context, executor service.ActivityTxExecutor, input lotteryRechargeQualificationInput) (int, error) {
	centerEnabled, err := activityCenterEnabledExecutor(ctx, executor)
	if err != nil || !centerEnabled {
		return 0, err
	}
	amount, err := decimal.NewFromString(input.rechargeAmount)
	if err != nil || !amount.IsPositive() || input.sourceID <= 0 || input.userID <= 0 {
		return 0, nil
	}
	qualifiedAt := input.qualifiedAt.UTC()
	if qualifiedAt.IsZero() {
		qualifiedAt = time.Now().UTC()
	}
	currency := strings.ToUpper(strings.TrimSpace(input.currency))
	rows, err := executor.QueryContext(ctx, `
SELECT a.id, c.id
FROM activities a
JOIN activity_lottery_configs c
  ON c.activity_id = a.id AND c.version = a.current_config_version
WHERE a.activity_type = 'recharge_lottery'
  AND a.status = 'published'
  AND a.enabled = TRUE
  AND c.currency = $1
  AND (c.starts_at IS NULL OR c.starts_at <= $2)
  AND (c.ends_at IS NULL OR c.ends_at > $2)
ORDER BY a.id
FOR UPDATE OF a`, currency, qualifiedAt)
	if err != nil {
		return 0, err
	}
	type qualification struct {
		activityID, configID int64
	}
	qualifications := make([]qualification, 0)
	for rows.Next() {
		var item qualification
		if err := rows.Scan(&item.activityID, &item.configID); err != nil {
			_ = rows.Close()
			return 0, err
		}
		qualifications = append(qualifications, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	totalGranted := 0
	for _, item := range qualifications {
		inserted, err := insertLotteryRechargeQualification(ctx, executor, item.activityID, item.configID, input, currency, qualifiedAt)
		if err != nil {
			return 0, err
		}
		if !inserted {
			continue
		}
		oldGranted, used, err := queryActivityDrawAccount(ctx, executor, input.userID, item.activityID)
		if errors.Is(err, sql.ErrNoRows) {
			oldGranted, used, err = 0, 0, nil
		}
		if err != nil {
			return 0, err
		}
		entitlement, err := calculateCumulativeLotteryEntitlement(ctx, executor, input.userID, item.activityID, item.configID)
		if err != nil {
			return 0, err
		}
		newGranted := max(used, entitlement.TotalDraws)
		granted := max(0, newGranted-oldGranted)
		if err := updateLotteryRechargeQualificationGrant(ctx, executor, item.activityID, input.sourceType, input.sourceID, granted); err != nil {
			return 0, err
		}
		if _, err := executor.ExecContext(ctx, `
INSERT INTO activity_lottery_draw_accounts (user_id, activity_id, granted_draws, used_draws, updated_at)
VALUES ($1, $2, $3, 0, NOW())
ON CONFLICT (user_id, activity_id) DO UPDATE
SET granted_draws = EXCLUDED.granted_draws,
	    updated_at = NOW()`, input.userID, item.activityID, newGranted); err != nil {
			return 0, err
		}
		totalGranted += granted
	}
	return totalGranted, nil
}

func activityCenterEnabledExecutor(ctx context.Context, executor service.ActivityTxExecutor) (bool, error) {
	rows, err := executor.QueryContext(ctx, `SELECT EXISTS (
    SELECT 1 FROM settings WHERE key = $1 AND value = 'true'
)`, service.SettingKeyActivityCenterEnabled)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, sql.ErrNoRows
	}
	var enabled bool
	if err := rows.Scan(&enabled); err != nil {
		return false, err
	}
	if rows.Next() {
		return false, errors.New("multiple activity center settings rows found")
	}
	return enabled, rows.Err()
}

func insertLotteryRechargeQualification(ctx context.Context, executor service.ActivityTxExecutor, activityID, configID int64, input lotteryRechargeQualificationInput, currency string, qualifiedAt time.Time) (bool, error) {
	var (
		result sql.Result
		err    error
	)
	switch input.sourceType {
	case lotteryQualificationSourcePayment:
		result, err = executor.ExecContext(ctx, `
INSERT INTO activity_lottery_qualifications (
    activity_id, config_id, payment_order_id, user_id, recharge_amount,
    currency, granted_draws, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (activity_id, payment_order_id) DO NOTHING`, activityID, configID,
			input.sourceID, input.userID, input.rechargeAmount, currency, 0, qualifiedAt)
	case lotteryQualificationSourceRedeem:
		result, err = executor.ExecContext(ctx, `
INSERT INTO activity_lottery_qualifications (
    activity_id, config_id, redeem_code_id, user_id, recharge_amount,
    currency, granted_draws, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (activity_id, redeem_code_id) DO NOTHING`, activityID, configID,
			input.sourceID, input.userID, input.rechargeAmount, currency, 0, qualifiedAt)
	default:
		return false, service.ErrActivityRequestInvalid
	}
	if err != nil {
		return false, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return inserted > 0, nil
}

func updateLotteryRechargeQualificationGrant(ctx context.Context, executor service.ActivityTxExecutor, activityID int64, sourceType string, sourceID int64, granted int) error {
	var err error
	switch sourceType {
	case lotteryQualificationSourcePayment:
		_, err = executor.ExecContext(ctx, `
UPDATE activity_lottery_qualifications
SET granted_draws = $2
WHERE activity_id = $1 AND payment_order_id = $3`, activityID, granted, sourceID)
	case lotteryQualificationSourceRedeem:
		_, err = executor.ExecContext(ctx, `
UPDATE activity_lottery_qualifications
SET granted_draws = $2
WHERE activity_id = $1 AND redeem_code_id = $3`, activityID, granted, sourceID)
	default:
		return service.ErrActivityRequestInvalid
	}
	return err
}

func (r *activityRepository) RevokeRechargeQualification(ctx context.Context, orderID int64, refundAmount string, now time.Time) (int, error) {
	return retryActivitySerializable(ctx, func() (int, error) {
		return r.revokeRechargeQualificationOnce(ctx, orderID, refundAmount, now)
	})
}

func (r *activityRepository) revokeRechargeQualificationOnce(ctx context.Context, orderID int64, refundAmount string, now time.Time) (int, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	totalRevoked, err := r.RevokeRechargeQualificationTx(ctx, tx, orderID, refundAmount, now)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return totalRevoked, nil
}

func (r *activityRepository) RevokeRechargeQualificationTx(ctx context.Context, executor service.ActivityTxExecutor, orderID int64, refundAmount string, now time.Time) (int, error) {
	if executor == nil || orderID <= 0 {
		return 0, service.ErrActivityRequestInvalid
	}
	refunded, err := decimal.NewFromString(strings.TrimSpace(refundAmount))
	if err != nil || !refunded.IsPositive() {
		return 0, service.ErrActivityRequestInvalid
	}
	rows, err := executor.QueryContext(ctx, `
SELECT q.id, q.user_id, q.activity_id, q.recharge_amount::text
FROM activity_lottery_qualifications q
WHERE q.payment_order_id = $1
ORDER BY q.id
FOR UPDATE OF q`, orderID)
	if err != nil {
		return 0, err
	}
	type qualification struct {
		id, userID, activityID int64
		rechargeAmount         string
	}
	items := make([]qualification, 0)
	for rows.Next() {
		var item qualification
		if err := rows.Scan(&item.id, &item.userID, &item.activityID, &item.rechargeAmount); err != nil {
			_ = rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	totalRevoked := 0
	for _, item := range items {
		rechargeAmount, amountErr := decimal.NewFromString(item.rechargeAmount)
		if amountErr != nil || !rechargeAmount.IsPositive() {
			return 0, service.ErrActivityConfigInvalid
		}
		refundedAmount := refunded
		if refundedAmount.GreaterThan(rechargeAmount) {
			refundedAmount = rechargeAmount
		}
		if _, err := executor.ExecContext(ctx, `
UPDATE activity_lottery_qualifications
SET refunded_amount = $2
WHERE id = $1`, item.id, refundedAmount.String()); err != nil {
			return 0, err
		}
		entitlement, err := calculateCumulativeLotteryEntitlement(ctx, executor, item.userID, item.activityID, 0)
		if err != nil {
			return 0, err
		}
		granted, used, err := queryActivityDrawAccount(ctx, executor, item.userID, item.activityID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		newGranted := max(used, entitlement.TotalDraws)
		revoke := max(0, granted-newGranted)
		if revoke > 0 {
			if _, err := executor.ExecContext(ctx, `
UPDATE activity_lottery_draw_accounts
SET granted_draws = $3, updated_at = $4
WHERE user_id = $1 AND activity_id = $2`, item.userID, item.activityID, newGranted, now); err != nil {
				return 0, err
			}
			if err := allocateLotteryRevocations(ctx, executor, item.userID, item.activityID, revoke, now); err != nil {
				return 0, err
			}
		}
		totalRevoked += revoke
	}
	return totalRevoked, nil
}

func allocateLotteryRevocations(ctx context.Context, executor service.ActivityTxExecutor, userID, activityID int64, revoke int, now time.Time) error {
	rows, err := executor.QueryContext(ctx, `
SELECT id, granted_draws - revoked_draws
FROM activity_lottery_qualifications
WHERE user_id = $1 AND activity_id = $2 AND granted_draws > revoked_draws
ORDER BY created_at DESC, id DESC
FOR UPDATE`, userID, activityID)
	if err != nil {
		return err
	}
	type revocableQualification struct {
		id        int64
		available int
	}
	items := make([]revocableQualification, 0)
	for rows.Next() {
		var item revocableQualification
		if err := rows.Scan(&item.id, &item.available); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	remaining := revoke
	for _, item := range items {
		if remaining == 0 {
			break
		}
		amount := min(remaining, item.available)
		if _, err := executor.ExecContext(ctx, `
UPDATE activity_lottery_qualifications
SET revoked_draws = revoked_draws + $2, revoked_at = $3
WHERE id = $1`, item.id, amount, now); err != nil {
			return err
		}
		remaining -= amount
	}
	if remaining != 0 {
		return fmt.Errorf("lottery qualification revocation ledger is short by %d draws", remaining)
	}
	return nil
}

func queryActivityDrawAccount(ctx context.Context, executor service.ActivityTxExecutor, userID, activityID int64) (int, int, error) {
	rows, err := executor.QueryContext(ctx, `
SELECT granted_draws, used_draws
FROM activity_lottery_draw_accounts
WHERE user_id = $1 AND activity_id = $2
FOR UPDATE`, userID, activityID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, err
		}
		return 0, 0, sql.ErrNoRows
	}
	var granted, used int
	if err := rows.Scan(&granted, &used); err != nil {
		return 0, 0, err
	}
	if rows.Next() {
		return 0, 0, errors.New("multiple activity draw accounts found")
	}
	return granted, used, rows.Err()
}

func (r *activityRepository) DrawLottery(ctx context.Context, userID int64, slug, requestID string, randomValue int, now time.Time) (*service.LotteryDrawResult, error) {
	return retryActivitySerializable(ctx, func() (*service.LotteryDrawResult, error) {
		return r.drawLotteryOnce(ctx, userID, slug, requestID, randomValue, now)
	})
}

func (r *activityRepository) drawLotteryOnce(ctx context.Context, userID int64, slug, requestID string, randomValue int, now time.Time) (*service.LotteryDrawResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, err := findLotteryDraw(ctx, tx, userID, slug, requestID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if enabled, err := activityCenterEnabledTx(ctx, tx); err != nil || !enabled {
		if err != nil {
			return nil, err
		}
		return nil, service.ErrActivityCenterDisabled
	}
	activity, err := lockActivity(ctx, tx, slug)
	if err != nil {
		return nil, err
	}
	if activity.Type != service.ActivityTypeRechargeLottery || activity.Status != service.ActivityStatusPublished || !activity.Enabled {
		return nil, service.ErrActivityDisabled
	}
	config, err := loadLotteryConfig(ctx, tx, activity.ID, activity.CurrentConfigVersion)
	if err != nil {
		return nil, service.ErrActivityConfigInvalid
	}
	if !activityWithinWindow(now, config.StartsAt, config.EndsAt) {
		return nil, service.ErrActivityNotActive
	}
	var granted, used int
	if err := tx.QueryRowContext(ctx, `
SELECT granted_draws, used_draws
FROM activity_lottery_draw_accounts
WHERE user_id = $1 AND activity_id = $2
FOR UPDATE`, userID, activity.ID).Scan(&granted, &used); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrLotteryNoChance
		}
		return nil, err
	}
	if existing, err := findLotteryDraw(ctx, tx, userID, slug, requestID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if used >= granted {
		return nil, service.ErrLotteryNoChance
	}
	if config.PerUserDrawLimit > 0 && used >= config.PerUserDrawLimit {
		return nil, service.ErrLotteryLimitReached
	}
	if config.DailyDrawLimit > 0 {
		dayStart, dayEnd := activityDayWindow(now, config.DailyLimitTimezone)
		var dailyCount int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM activity_lottery_draws
WHERE activity_id = $1 AND user_id = $2 AND created_at >= $3 AND created_at < $4`, activity.ID, userID, dayStart, dayEnd).Scan(&dailyCount); err != nil {
			return nil, err
		}
		if dailyCount >= config.DailyDrawLimit {
			return nil, service.ErrLotteryLimitReached
		}
	}
	prize, err := selectLotteryPrize(config.Prizes, randomValue)
	if err != nil {
		return nil, err
	}
	var drawID int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
INSERT INTO activity_lottery_draws (
    activity_id, config_id, prize_id, user_id, request_id,
    random_value, prize_name, reward_amount, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, created_at`, activity.ID, config.ID, prize.ID, userID, requestID,
		randomValue, prize.Name, prize.Amount, now).Scan(&drawID, &createdAt)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE activity_lottery_draw_accounts
SET used_draws = used_draws + 1, updated_at = NOW()
WHERE user_id = $1 AND activity_id = $2`, userID, activity.ID); err != nil {
		return nil, err
	}
	result := &service.LotteryDrawResult{
		DrawID:         drawID,
		PrizeID:        prize.ID,
		PrizeName:      prize.Name,
		RewardAmount:   prize.Amount,
		AvailableDraws: max(0, granted-used-1),
		CreatedAt:      createdAt,
	}
	amount, err := decimal.NewFromString(prize.Amount)
	if err != nil {
		return nil, err
	}
	if amount.IsPositive() {
		if err := creditActivityReward(ctx, tx, userID, activity, "lottery_draw", drawID, amount, config.Currency, map[string]any{"prize_id": prize.ID, "prize_name": prize.Name}, &result.BalanceAfter); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE activity_lottery_draws SET balance_after = $2 WHERE id = $1`, drawID, result.BalanceAfter); err != nil {
			return nil, err
		}
	}
	if config.PerUserDrawLimit > 0 {
		result.AvailableDraws = min(result.AvailableDraws, max(0, config.PerUserDrawLimit-used-1))
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *activityRepository) ClaimBenefit(ctx context.Context, userID int64, slug, requestID string, now time.Time) (*service.BenefitClaimResult, error) {
	return retryActivitySerializable(ctx, func() (*service.BenefitClaimResult, error) {
		return r.claimBenefitOnce(ctx, userID, slug, requestID, now)
	})
}

func (r *activityRepository) claimBenefitOnce(ctx context.Context, userID int64, slug, requestID string, now time.Time) (*service.BenefitClaimResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, err := findBenefitClaim(ctx, tx, userID, slug, requestID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if enabled, err := activityCenterEnabledTx(ctx, tx); err != nil || !enabled {
		if err != nil {
			return nil, err
		}
		return nil, service.ErrActivityCenterDisabled
	}
	activity, err := lockActivity(ctx, tx, slug)
	if err != nil {
		return nil, err
	}
	if activity.Type != service.ActivityTypeLimitedBenefit || activity.Status != service.ActivityStatusPublished || !activity.Enabled {
		return nil, service.ErrActivityDisabled
	}
	var config service.BenefitConfig
	var startsAt, endsAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
SELECT id, version, currency, reward_amount::text, random_min_amount::text,
       random_max_amount::text, total_stock, per_user_limit, starts_at, ends_at
FROM activity_benefit_configs
WHERE activity_id = $1 AND version = $2
FOR UPDATE`, activity.ID, activity.CurrentConfigVersion).Scan(&config.ID, &config.Version, &config.Currency,
		&config.RewardAmount, &config.RandomMinAmount, &config.RandomMaxAmount,
		&config.TotalStock, &config.PerUserLimit, &startsAt, &endsAt); err != nil {
		return nil, service.ErrActivityConfigInvalid
	}
	config.StartsAt, config.EndsAt = nullableTime(startsAt), nullableTime(endsAt)
	config.DailyClaimLimit = service.BenefitDailyClaimLimit
	config.DailyLimitTimezone = service.BenefitDailyLimitTimezone
	if !activityWithinWindow(now, config.StartsAt, config.EndsAt) {
		return nil, service.ErrActivityNotActive
	}
	if existing, err := findBenefitClaim(ctx, tx, userID, slug, requestID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var totalClaims int64
	var userClaimsToday int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_benefit_claims WHERE config_id = $1`, config.ID).Scan(&totalClaims); err != nil {
		return nil, err
	}
	if totalClaims >= config.TotalStock {
		return nil, service.ErrBenefitExhausted
	}
	userClaimsToday, err = countBenefitClaimsToday(ctx, tx, activity.ID, userID, now)
	if err != nil {
		return nil, err
	}
	if userClaimsToday >= service.BenefitDailyClaimLimit {
		return nil, service.ErrBenefitLimitReached
	}
	amount, err := selectBenefitRewardAmount(config, cryptorand.Reader)
	if err != nil {
		if errors.Is(err, service.ErrActivityConfigInvalid) {
			return nil, service.ErrActivityConfigInvalid
		}
		return nil, err
	}
	var balanceAfter string
	if err := tx.QueryRowContext(ctx, `
UPDATE users SET balance = balance + $2, updated_at = NOW()
WHERE id = $1
RETURNING balance::text`, userID, amount.String()).Scan(&balanceAfter); err != nil {
		return nil, err
	}
	var claimID int64
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `
INSERT INTO activity_benefit_claims (
    activity_id, config_id, user_id, request_id, reward_amount, balance_after, claim_day, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at`, activity.ID, config.ID, userID, requestID, amount.String(), balanceAfter,
		activityLocalDate(now, service.BenefitDailyLimitTimezone), now).Scan(&claimID, &createdAt); err != nil {
		return nil, err
	}
	if err := insertActivityRewardLedger(ctx, tx, userID, activity, "benefit_claim", claimID, amount, balanceAfter, config.Currency, nil); err != nil {
		return nil, err
	}
	result := &service.BenefitClaimResult{
		ClaimID:        claimID,
		RewardAmount:   amount.String(),
		BalanceAfter:   balanceAfter,
		RemainingStock: max(int64(0), config.TotalStock-totalClaims-1),
		CreatedAt:      createdAt,
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *activityRepository) ListRewards(ctx context.Context, userID int64, limit int) ([]service.ActivityReward, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT l.id, l.user_id, l.activity_id, a.title, l.activity_type,
       l.source_type, l.source_id, l.amount::text, l.balance_after::text,
       l.currency, l.created_at
FROM activity_reward_ledger l
JOIN activities a ON a.id = l.activity_id
WHERE l.user_id = $1
ORDER BY l.created_at DESC, l.id DESC
LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ActivityReward, 0)
	for rows.Next() {
		var item service.ActivityReward
		if err := rows.Scan(&item.ID, &item.UserID, &item.ActivityID, &item.ActivityTitle,
			&item.ActivityType, &item.SourceType, &item.SourceID, &item.Amount,
			&item.BalanceAfter, &item.Currency, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func lockActivity(ctx context.Context, tx *sql.Tx, slug string) (*service.Activity, error) {
	activity, err := scanActivity(tx.QueryRowContext(ctx, `
SELECT id, slug, activity_type, title, description, status, enabled,
       sort_order, current_config_version, created_at, updated_at
FROM activities WHERE slug = $1 FOR UPDATE`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrActivityNotFound
	}
	return activity, err
}

func activityCenterEnabledTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	var enabled bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (
    SELECT 1 FROM settings WHERE key = $1 AND value = 'true'
)`, service.SettingKeyActivityCenterEnabled).Scan(&enabled)
	return enabled, err
}

func findLotteryDraw(ctx context.Context, queryer activityQueryer, userID int64, slug, requestID string) (*service.LotteryDrawResult, error) {
	var result service.LotteryDrawResult
	var balanceAfter sql.NullString
	err := queryer.QueryRowContext(ctx, `
SELECT d.id, d.prize_id, d.prize_name, d.reward_amount::text,
       d.balance_after::text, COALESCE(a.granted_draws - a.used_draws, 0), d.created_at
FROM activity_lottery_draws d
JOIN activities activity ON activity.id = d.activity_id
LEFT JOIN activity_lottery_draw_accounts a ON a.user_id = d.user_id AND a.activity_id = d.activity_id
WHERE d.user_id = $1 AND activity.slug = $2 AND d.request_id = $3`, userID, slug, requestID).Scan(
		&result.DrawID, &result.PrizeID, &result.PrizeName, &result.RewardAmount,
		&balanceAfter, &result.AvailableDraws, &result.CreatedAt)
	if balanceAfter.Valid {
		result.BalanceAfter = balanceAfter.String
	}
	return &result, err
}

func findBenefitClaim(ctx context.Context, queryer activityQueryer, userID int64, slug, requestID string) (*service.BenefitClaimResult, error) {
	var result service.BenefitClaimResult
	err := queryer.QueryRowContext(ctx, `
SELECT c.id, c.reward_amount::text, c.balance_after::text,
       GREATEST(cfg.total_stock - (SELECT COUNT(*) FROM activity_benefit_claims all_claims WHERE all_claims.config_id = c.config_id), 0),
       c.created_at
FROM activity_benefit_claims c
JOIN activity_benefit_configs cfg ON cfg.id = c.config_id
JOIN activities activity ON activity.id = c.activity_id
WHERE c.user_id = $1 AND activity.slug = $2 AND c.request_id = $3`, userID, slug, requestID).Scan(
		&result.ClaimID, &result.RewardAmount, &result.BalanceAfter, &result.RemainingStock, &result.CreatedAt)
	return &result, err
}

func selectLotteryPrize(prizes []service.ActivityPrize, randomValue int) (*service.ActivityPrize, error) {
	cumulative := 0
	for index := range prizes {
		cumulative += prizes[index].ProbabilityPPM
		if randomValue < cumulative {
			return &prizes[index], nil
		}
	}
	return nil, service.ErrActivityConfigInvalid
}

func selectBenefitRewardAmount(config service.BenefitConfig, randomSource io.Reader) (decimal.Decimal, error) {
	fixedAmount, err := decimal.NewFromString(strings.TrimSpace(config.RewardAmount))
	if err != nil || fixedAmount.IsNegative() {
		return decimal.Zero, service.ErrActivityConfigInvalid
	}
	minAmount, minErr := decimal.NewFromString(strings.TrimSpace(config.RandomMinAmount))
	maxAmount, maxErr := decimal.NewFromString(strings.TrimSpace(config.RandomMaxAmount))
	if minErr != nil || maxErr != nil || minAmount.IsNegative() || maxAmount.IsNegative() {
		return decimal.Zero, service.ErrActivityConfigInvalid
	}
	if fixedAmount.IsPositive() {
		if !minAmount.IsZero() || !maxAmount.IsZero() {
			return decimal.Zero, service.ErrActivityConfigInvalid
		}
		return fixedAmount, nil
	}
	maximumReward := decimal.NewFromInt(1_000_000)
	if !minAmount.IsPositive() || maxAmount.LessThan(minAmount) || maxAmount.GreaterThan(maximumReward) ||
		!minAmount.Equal(minAmount.Truncate(2)) || !maxAmount.Equal(maxAmount.Truncate(2)) {
		return decimal.Zero, service.ErrActivityConfigInvalid
	}
	minCents := minAmount.Shift(2).IntPart()
	maxCents := maxAmount.Shift(2).IntPart()
	if minCents == maxCents {
		return decimal.NewFromInt(minCents).Shift(-2), nil
	}
	span := big.NewInt(maxCents - minCents + 1)
	offset, err := cryptorand.Int(randomSource, span)
	if err != nil {
		return decimal.Zero, fmt.Errorf("generate benefit reward amount: %w", err)
	}
	return decimal.NewFromInt(minCents + offset.Int64()).Shift(-2), nil
}

func normalizedActivityMoney(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0"
	}
	return strings.TrimSpace(value)
}

func creditActivityReward(ctx context.Context, tx *sql.Tx, userID int64, activity *service.Activity, sourceType string, sourceID int64, amount decimal.Decimal, currency string, metadata map[string]any, balanceAfter *string) error {
	if err := tx.QueryRowContext(ctx, `
UPDATE users SET balance = balance + $2, updated_at = NOW()
WHERE id = $1
RETURNING balance::text`, userID, amount.String()).Scan(balanceAfter); err != nil {
		return err
	}
	return insertActivityRewardLedger(ctx, tx, userID, activity, sourceType, sourceID, amount, *balanceAfter, currency, metadata)
}

func insertActivityRewardLedger(ctx context.Context, tx *sql.Tx, userID int64, activity *service.Activity, sourceType string, sourceID int64, amount decimal.Decimal, balanceAfter, currency string, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO activity_reward_ledger (
    user_id, activity_id, activity_type, source_type, source_id,
    amount, balance_after, currency, metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)`, userID, activity.ID,
		activity.Type, sourceType, sourceID, amount.String(), balanceAfter, strings.ToUpper(currency), string(payload))
	return err
}

func activityWithinWindow(now time.Time, startsAt, endsAt *time.Time) bool {
	return (startsAt == nil || !now.Before(*startsAt)) && (endsAt == nil || now.Before(*endsAt))
}

func activityDayWindow(now time.Time, timezone string) (time.Time, time.Time) {
	location := activityLocation(timezone)
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}

func activityLocalDate(now time.Time, timezone string) string {
	return now.In(activityLocation(timezone)).Format(time.DateOnly)
}

func activityLocation(timezone string) *time.Location {
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return time.UTC
	}
	return location
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
