package service

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	SettingKeyActivityCenterEnabled = "activity_center_enabled"

	ActivityTypeRechargeLottery   = "recharge_lottery"
	ActivityTypeLimitedBenefit    = "limited_time_benefit"
	ActivityStatusDraft           = "draft"
	ActivityStatusPublished       = "published"
	ActivityStatusArchived        = "archived"
	ActivityProbabilityScale      = 1_000_000
	BenefitDailyClaimLimit        = 1
	BenefitDailyLimitTimezone     = "Asia/Shanghai"
	maxActivityPrizeCount         = 12
	maxActivityRewardAmount       = 1_000_000
	maxActivityConfigurationLimit = 1_000_000
)

var (
	ErrActivityCenterDisabled = infraerrors.NotFound("ACTIVITY_CENTER_DISABLED", "activity center is disabled")
	ErrActivityNotFound       = infraerrors.NotFound("ACTIVITY_NOT_FOUND", "activity not found")
	ErrActivityDisabled       = infraerrors.BadRequest("ACTIVITY_DISABLED", "activity is closed")
	ErrActivityNotActive      = infraerrors.BadRequest("ACTIVITY_NOT_ACTIVE", "activity is outside its active period")
	ErrActivityConfigInvalid  = infraerrors.BadRequest("ACTIVITY_CONFIG_INVALID", "activity configuration is invalid")
	ErrActivityRequestInvalid = infraerrors.BadRequest("ACTIVITY_REQUEST_INVALID", "activity request is invalid")
	ErrLotteryNoChance        = infraerrors.BadRequest("LOTTERY_NO_CHANCE", "no lottery chances available")
	ErrLotteryLimitReached    = infraerrors.BadRequest("LOTTERY_LIMIT_REACHED", "lottery draw limit reached")
	ErrBenefitLimitReached    = infraerrors.Conflict("BENEFIT_LIMIT_REACHED", "daily benefit claim limit reached")
	ErrBenefitExhausted       = infraerrors.Conflict("BENEFIT_EXHAUSTED", "benefit stock is exhausted")
)

var activityRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type ActivityPrize struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Amount         string `json:"amount"`
	ProbabilityPPM int    `json:"probability_ppm,omitempty"`
	SortOrder      int    `json:"sort_order"`
}

type LotteryConfig struct {
	ID                 int64           `json:"id"`
	Version            int             `json:"version"`
	Currency           string          `json:"currency"`
	RechargeThreshold  string          `json:"recharge_threshold"`
	DrawsPerThreshold  int             `json:"draws_per_threshold"`
	MaxChancesPerOrder int             `json:"max_chances_per_order"`
	PerUserDrawLimit   int             `json:"per_user_draw_limit"`
	DailyDrawLimit     int             `json:"daily_draw_limit"`
	DailyLimitTimezone string          `json:"daily_limit_timezone"`
	StartsAt           *time.Time      `json:"starts_at"`
	EndsAt             *time.Time      `json:"ends_at"`
	Prizes             []ActivityPrize `json:"prizes"`
}

type BenefitConfig struct {
	ID                 int64      `json:"id"`
	Version            int        `json:"version"`
	Currency           string     `json:"currency"`
	RewardAmount       string     `json:"reward_amount"`
	RandomMinAmount    string     `json:"random_min_amount"`
	RandomMaxAmount    string     `json:"random_max_amount"`
	TotalStock         int64      `json:"total_stock"`
	PerUserLimit       int        `json:"per_user_limit"`
	DailyClaimLimit    int        `json:"daily_claim_limit"`
	DailyLimitTimezone string     `json:"daily_limit_timezone"`
	StartsAt           *time.Time `json:"starts_at"`
	EndsAt             *time.Time `json:"ends_at"`
	ClaimedCount       int64      `json:"claimed_count"`
}

type ActivityParticipation struct {
	GrantedDraws             int    `json:"granted_draws,omitempty"`
	UsedDraws                int    `json:"used_draws,omitempty"`
	AvailableDraws           int    `json:"available_draws,omitempty"`
	DrawnToday               int    `json:"drawn_today,omitempty"`
	CumulativeRechargeAmount string `json:"cumulative_recharge_amount,omitempty"`
	RechargeProgressAmount   string `json:"recharge_progress_amount,omitempty"`
	NextDrawRechargeAmount   string `json:"next_draw_recharge_amount,omitempty"`
	RewardTotal              string `json:"reward_total,omitempty"`
	BenefitClaims            int    `json:"benefit_claims,omitempty"`
	BenefitClaimsToday       int    `json:"benefit_claims_today,omitempty"`
	RemainingStock           int64  `json:"remaining_stock,omitempty"`
}

type Activity struct {
	ID                   int64                  `json:"id"`
	Slug                 string                 `json:"slug"`
	Type                 string                 `json:"type"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description"`
	Status               string                 `json:"status"`
	Enabled              bool                   `json:"enabled"`
	SortOrder            int                    `json:"sort_order"`
	CurrentConfigVersion int                    `json:"current_config_version"`
	Availability         string                 `json:"availability"`
	ClosedReason         string                 `json:"closed_reason,omitempty"`
	Lottery              *LotteryConfig         `json:"lottery,omitempty"`
	Benefit              *BenefitConfig         `json:"benefit,omitempty"`
	Participation        *ActivityParticipation `json:"participation,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

type ActivityCenterAdminView struct {
	Enabled    bool       `json:"enabled"`
	Activities []Activity `json:"activities"`
}

type UpdateActivityInput struct {
	Title       *string
	Description *string
	Status      *string
	Enabled     *bool
	SortOrder   *int
}

type LotteryPrizeInput struct {
	Name           string `json:"name"`
	Amount         string `json:"amount"`
	ProbabilityPPM int    `json:"probability_ppm"`
	SortOrder      int    `json:"sort_order"`
}

type LotteryConfigInput struct {
	Currency           string              `json:"currency"`
	RechargeThreshold  string              `json:"recharge_threshold"`
	DrawsPerThreshold  int                 `json:"draws_per_threshold"`
	MaxChancesPerOrder int                 `json:"max_chances_per_order"`
	PerUserDrawLimit   int                 `json:"per_user_draw_limit"`
	DailyDrawLimit     int                 `json:"daily_draw_limit"`
	DailyLimitTimezone string              `json:"daily_limit_timezone"`
	StartsAt           *time.Time          `json:"starts_at"`
	EndsAt             *time.Time          `json:"ends_at"`
	Prizes             []LotteryPrizeInput `json:"prizes"`
	CreatedBy          int64               `json:"-"`
}

type BenefitConfigInput struct {
	Currency        string     `json:"currency"`
	RewardAmount    string     `json:"reward_amount"`
	RandomMinAmount string     `json:"random_min_amount"`
	RandomMaxAmount string     `json:"random_max_amount"`
	TotalStock      int64      `json:"total_stock"`
	PerUserLimit    int        `json:"per_user_limit"`
	StartsAt        *time.Time `json:"starts_at"`
	EndsAt          *time.Time `json:"ends_at"`
	CreatedBy       int64      `json:"-"`
}

type CompleteActivityPaymentInput struct {
	OrderID        int64
	UserID         int64
	RechargeAmount string
	Currency       string
	LeaseVersion   time.Time
	CompletedAt    time.Time
}

type BalanceRedeemQualificationInput struct {
	RedeemCodeID   int64
	UserID         int64
	RechargeAmount string
	Currency       string
	RedeemedAt     time.Time
}

type BalanceRedeemActivityQualifier interface {
	GrantBalanceRedeemQualificationTx(ctx context.Context, executor ActivityTxExecutor, input BalanceRedeemQualificationInput) (int, error)
}

type ActivityPaymentCompletion struct {
	Completed    bool
	GrantedDraws int
}

type LotteryDrawResult struct {
	DrawID         int64     `json:"draw_id"`
	PrizeID        int64     `json:"prize_id"`
	PrizeName      string    `json:"prize_name"`
	RewardAmount   string    `json:"reward_amount"`
	BalanceAfter   string    `json:"balance_after,omitempty"`
	AvailableDraws int       `json:"available_draws"`
	CreatedAt      time.Time `json:"created_at"`
}

type BenefitClaimResult struct {
	ClaimID        int64     `json:"claim_id"`
	RewardAmount   string    `json:"reward_amount"`
	BalanceAfter   string    `json:"balance_after"`
	RemainingStock int64     `json:"remaining_stock"`
	CreatedAt      time.Time `json:"created_at"`
}

type ActivityReward struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	ActivityID    int64     `json:"activity_id"`
	ActivityTitle string    `json:"activity_title"`
	ActivityType  string    `json:"activity_type"`
	SourceType    string    `json:"source_type"`
	SourceID      int64     `json:"source_id"`
	Amount        string    `json:"amount"`
	BalanceAfter  string    `json:"balance_after"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}

type ActivityRepository interface {
	ListActivities(ctx context.Context, userID int64, includeArchived bool, now time.Time) ([]Activity, error)
	GetActivity(ctx context.Context, slug string, userID int64, includeArchived bool, now time.Time) (*Activity, error)
	UpdateActivity(ctx context.Context, slug string, input UpdateActivityInput, now time.Time) (*Activity, error)
	PublishLotteryConfig(ctx context.Context, slug string, input LotteryConfigInput) (*Activity, error)
	PublishBenefitConfig(ctx context.Context, slug string, input BenefitConfigInput) (*Activity, error)
	CompleteBalancePayment(ctx context.Context, input CompleteActivityPaymentInput) (*ActivityPaymentCompletion, error)
	GrantBalanceRedeemQualificationTx(ctx context.Context, executor ActivityTxExecutor, input BalanceRedeemQualificationInput) (int, error)
	RevokeRechargeQualification(ctx context.Context, orderID int64, refundAmount string, now time.Time) (int, error)
	RevokeRechargeQualificationTx(ctx context.Context, executor ActivityTxExecutor, orderID int64, refundAmount string, now time.Time) (int, error)
	DrawLottery(ctx context.Context, userID int64, slug, requestID string, randomValue int, now time.Time) (*LotteryDrawResult, error)
	ClaimBenefit(ctx context.Context, userID int64, slug, requestID string, now time.Time) (*BenefitClaimResult, error)
	ListRewards(ctx context.Context, userID int64, limit int) ([]ActivityReward, error)
}

type ActivityTxExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type ActivityService struct {
	repo                 ActivityRepository
	settingRepo          SettingRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
	onUpdate             func() // Callback when the activity-center setting changes
}

func NewActivityService(repo ActivityRepository, settingRepo SettingRepository, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *ActivityService {
	return &ActivityService{
		repo:                 repo,
		settingRepo:          settingRepo,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

func (s *ActivityService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyActivityCenterEnabled)
	return err == nil && value == "true"
}

// SetOnUpdateCallback sets a callback invoked after the activity-center setting
// is persisted. The embedded frontend uses this to invalidate its injected
// public-settings HTML cache.
func (s *ActivityService) SetOnUpdateCallback(callback func()) {
	if s == nil {
		return
	}
	s.onUpdate = callback
}

func (s *ActivityService) SetEnabled(ctx context.Context, enabled bool) error {
	if s == nil || s.settingRepo == nil {
		return errors.New("activity setting repository is unavailable")
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{SettingKeyActivityCenterEnabled: fmt.Sprintf("%t", enabled)}); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}

func (s *ActivityService) ListUserActivities(ctx context.Context, userID int64) ([]Activity, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrActivityCenterDisabled
	}
	items, err := s.repo.ListActivities(ctx, userID, false, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	hideLotteryProbabilities(items)
	return items, nil
}

func (s *ActivityService) GetUserActivity(ctx context.Context, userID int64, slug string) (*Activity, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrActivityCenterDisabled
	}
	activity, err := s.repo.GetActivity(ctx, strings.TrimSpace(slug), userID, false, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	hideLotteryProbability(activity)
	return activity, nil
}

func hideLotteryProbabilities(items []Activity) {
	for index := range items {
		hideLotteryProbability(&items[index])
	}
}

func hideLotteryProbability(activity *Activity) {
	if activity == nil || activity.Lottery == nil {
		return
	}
	for index := range activity.Lottery.Prizes {
		activity.Lottery.Prizes[index].ProbabilityPPM = 0
	}
}

func (s *ActivityService) AdminView(ctx context.Context) (*ActivityCenterAdminView, error) {
	items, err := s.repo.ListActivities(ctx, 0, true, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &ActivityCenterAdminView{Enabled: s.IsEnabled(ctx), Activities: items}, nil
}

func (s *ActivityService) UpdateActivity(ctx context.Context, slug string, input UpdateActivityInput) (*Activity, error) {
	if input.Title != nil {
		value := strings.TrimSpace(*input.Title)
		if value == "" || len([]rune(value)) > 120 {
			return nil, ErrActivityConfigInvalid
		}
		input.Title = &value
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if len([]rune(value)) > 1000 {
			return nil, ErrActivityConfigInvalid
		}
		input.Description = &value
	}
	if input.Status != nil {
		switch *input.Status {
		case ActivityStatusDraft, ActivityStatusPublished, ActivityStatusArchived:
		default:
			return nil, ErrActivityConfigInvalid
		}
	}
	return s.repo.UpdateActivity(ctx, strings.TrimSpace(slug), input, time.Now().UTC())
}

func (s *ActivityService) PublishLotteryConfig(ctx context.Context, slug string, input LotteryConfigInput) (*Activity, error) {
	if err := validateLotteryConfig(input); err != nil {
		return nil, err
	}
	return s.repo.PublishLotteryConfig(ctx, strings.TrimSpace(slug), input)
}

func (s *ActivityService) PublishBenefitConfig(ctx context.Context, slug string, input BenefitConfigInput) (*Activity, error) {
	if err := validateBenefitConfig(input); err != nil {
		return nil, err
	}
	return s.repo.PublishBenefitConfig(ctx, strings.TrimSpace(slug), input)
}

func (s *ActivityService) CompleteBalancePayment(ctx context.Context, input CompleteActivityPaymentInput) (*ActivityPaymentCompletion, error) {
	return s.repo.CompleteBalancePayment(ctx, input)
}

func (s *ActivityService) GrantBalanceRedeemQualificationTx(ctx context.Context, executor ActivityTxExecutor, input BalanceRedeemQualificationInput) (int, error) {
	return s.repo.GrantBalanceRedeemQualificationTx(ctx, executor, input)
}

func (s *ActivityService) RevokeRechargeQualification(ctx context.Context, orderID int64, refundAmount string) (int, error) {
	return s.repo.RevokeRechargeQualification(ctx, orderID, refundAmount, time.Now().UTC())
}

func (s *ActivityService) RevokeRechargeQualificationTx(ctx context.Context, executor ActivityTxExecutor, orderID int64, refundAmount string, now time.Time) (int, error) {
	return s.repo.RevokeRechargeQualificationTx(ctx, executor, orderID, refundAmount, now)
}

func (s *ActivityService) DrawLottery(ctx context.Context, userID int64, slug, requestID string) (*LotteryDrawResult, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrActivityCenterDisabled
	}
	if userID <= 0 || !activityRequestIDPattern.MatchString(requestID) {
		return nil, ErrActivityRequestInvalid
	}
	randomNumber, err := cryptorand.Int(cryptorand.Reader, big.NewInt(ActivityProbabilityScale))
	if err != nil {
		return nil, fmt.Errorf("generate lottery random value: %w", err)
	}
	result, err := s.repo.DrawLottery(ctx, userID, strings.TrimSpace(slug), requestID, int(randomNumber.Int64()), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if amount, parseErr := decimal.NewFromString(result.RewardAmount); parseErr == nil && amount.IsPositive() {
		s.invalidateBalanceCaches(ctx, userID)
	}
	return result, nil
}

func (s *ActivityService) ClaimBenefit(ctx context.Context, userID int64, slug, requestID string) (*BenefitClaimResult, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrActivityCenterDisabled
	}
	if userID <= 0 || !activityRequestIDPattern.MatchString(requestID) {
		return nil, ErrActivityRequestInvalid
	}
	result, err := s.repo.ClaimBenefit(ctx, userID, strings.TrimSpace(slug), requestID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	s.invalidateBalanceCaches(ctx, userID)
	return result, nil
}

func (s *ActivityService) ListRewards(ctx context.Context, userID int64, limit int) ([]ActivityReward, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrActivityCenterDisabled
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListRewards(ctx, userID, limit)
}

func (s *ActivityService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Warn("invalidate activity reward balance cache", "user_id", userID, "error", err)
		}
	}
}

func validateLotteryConfig(input LotteryConfigInput) error {
	if strings.ToUpper(strings.TrimSpace(input.Currency)) != "CNY" {
		return ErrActivityConfigInvalid
	}
	threshold, err := parseActivityMoney(input.RechargeThreshold)
	if err != nil || !threshold.IsPositive() || threshold.GreaterThan(decimal.NewFromInt(maxActivityRewardAmount)) {
		return ErrActivityConfigInvalid
	}
	if input.DrawsPerThreshold <= 0 || input.DrawsPerThreshold > maxActivityConfigurationLimit ||
		input.MaxChancesPerOrder < 0 || input.MaxChancesPerOrder > maxActivityConfigurationLimit ||
		input.PerUserDrawLimit < 0 || input.PerUserDrawLimit > maxActivityConfigurationLimit ||
		input.DailyDrawLimit < 0 || input.DailyDrawLimit > maxActivityConfigurationLimit {
		return ErrActivityConfigInvalid
	}
	if _, err := time.LoadLocation(strings.TrimSpace(input.DailyLimitTimezone)); err != nil {
		return ErrActivityConfigInvalid
	}
	if !validActivityWindow(input.StartsAt, input.EndsAt) || len(input.Prizes) < 2 || len(input.Prizes) > maxActivityPrizeCount {
		return ErrActivityConfigInvalid
	}
	probabilityTotal := 0
	hasReward := false
	for index := range input.Prizes {
		prize := &input.Prizes[index]
		prize.Name = strings.TrimSpace(prize.Name)
		amount, amountErr := parseActivityMoney(prize.Amount)
		if prize.Name == "" || len([]rune(prize.Name)) > 120 || amountErr != nil || amount.IsNegative() || amount.GreaterThan(decimal.NewFromInt(maxActivityRewardAmount)) || prize.ProbabilityPPM <= 0 || prize.ProbabilityPPM > ActivityProbabilityScale {
			return ErrActivityConfigInvalid
		}
		if amount.IsPositive() {
			hasReward = true
		}
		probabilityTotal += prize.ProbabilityPPM
	}
	if probabilityTotal != ActivityProbabilityScale || !hasReward {
		return ErrActivityConfigInvalid
	}
	return nil
}

func validateBenefitConfig(input BenefitConfigInput) error {
	if strings.ToUpper(strings.TrimSpace(input.Currency)) != "CNY" {
		return ErrActivityConfigInvalid
	}
	amount, err := parseActivityMoney(input.RewardAmount)
	minAmount, minErr := parseOptionalActivityMoney(input.RandomMinAmount)
	maxAmount, maxErr := parseOptionalActivityMoney(input.RandomMaxAmount)
	if err != nil || minErr != nil || maxErr != nil || amount.IsNegative() ||
		amount.GreaterThan(decimal.NewFromInt(maxActivityRewardAmount)) ||
		input.TotalStock <= 0 || input.TotalStock > maxActivityConfigurationLimit ||
		input.PerUserLimit != BenefitDailyClaimLimit ||
		!validActivityWindow(input.StartsAt, input.EndsAt) {
		return ErrActivityConfigInvalid
	}
	if amount.IsPositive() {
		if !minAmount.IsZero() || !maxAmount.IsZero() {
			return ErrActivityConfigInvalid
		}
		return nil
	}
	if !minAmount.IsPositive() || maxAmount.LessThan(minAmount) ||
		maxAmount.GreaterThan(decimal.NewFromInt(maxActivityRewardAmount)) ||
		!hasCentPrecision(minAmount) || !hasCentPrecision(maxAmount) {
		return ErrActivityConfigInvalid
	}
	return nil
}

func parseOptionalActivityMoney(value string) (decimal.Decimal, error) {
	if strings.TrimSpace(value) == "" {
		return decimal.Zero, nil
	}
	return parseActivityMoney(value)
}

func hasCentPrecision(amount decimal.Decimal) bool {
	return amount.Equal(amount.Truncate(2))
}

func parseActivityMoney(value string) (decimal.Decimal, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil || amount.Exponent() < -8 {
		return decimal.Zero, ErrActivityConfigInvalid
	}
	return amount, nil
}

func validActivityWindow(startsAt, endsAt *time.Time) bool {
	return startsAt == nil || endsAt == nil || startsAt.Before(*endsAt)
}
