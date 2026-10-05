package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type usageBillingRepository struct {
	db *sql.DB
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	return r.apply(ctx, cmd, nil)
}

func (r *usageBillingRepository) ApplyVideoUsage(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) (*service.UsageBillingApplyResult, error) {
	if cmd == nil || log == nil || log.VideoCount <= 0 || !strings.HasPrefix(cmd.RequestID, "grok-video:") ||
		cmd.RequestID != log.RequestID || cmd.UserID != log.UserID || cmd.APIKeyID != log.APIKeyID || cmd.AccountID != log.AccountID {
		return nil, errors.New("video billing usage identity mismatch")
	}
	if cmd.WorkspaceID > 0 || cmd.ProjectID > 0 || cmd.BillingPrincipalUserID > 0 || strings.TrimSpace(cmd.BudgetReservationID) != "" ||
		log.WorkspaceID != nil || log.ProjectID != nil || log.BillingPrincipalUserID != nil || log.BudgetReservationID != nil {
		if err := validateTenantUsageSnapshot(cmd, log); err != nil {
			return nil, err
		}
	}
	result, err := r.apply(ctx, cmd, log)
	if err != nil && cmd.WorkspaceID > 0 && strings.TrimSpace(cmd.BudgetReservationID) != "" {
		// The money transaction has rolled back. Persist the actual pending
		// obligation and its alert together on a separate committed transaction.
		if pendingErr := r.recordPendingVideoSettlement(ctx, cmd, log, err); pendingErr != nil {
			return nil, fmt.Errorf("%w (persist pending settlement: %v)", err, pendingErr)
		}
	}
	return result, err
}

// ApplyTenantUsage commits the balance/quota effects, budget settlement, and
// immutable FinOps usage row in one transaction. A usage row without its money
// effects would make tenant reports permanently diverge from the wallet.
func (r *usageBillingRepository) ApplyTenantUsage(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) (*service.UsageBillingApplyResult, error) {
	if cmd == nil || log == nil {
		return nil, service.ErrBudgetReservationInvalid
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}
	if err := validateTenantUsageSnapshot(cmd, log); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		matched, verifyErr := verifyTenantUsageLog(ctx, tx, cmd)
		if verifyErr != nil {
			return nil, verifyErr
		}
		if !matched {
			return nil, service.ErrUsageBillingRequestConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		tx = nil
		return &service.UsageBillingApplyResult{Applied: false, TenantUsageLogPersisted: true}, nil
	}
	// A legacy crash can leave the usage row committed while the dedup claim was
	// archived or absent. Detect that durable row before applying money again.
	if existing, verifyErr := verifyTenantUsageLog(ctx, tx, cmd); verifyErr != nil {
		return nil, verifyErr
	} else if existing {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		tx = nil
		return &service.UsageBillingApplyResult{Applied: false, TenantUsageLogPersisted: true}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	// Deleted keys remain valid for settlement of an already admitted tenant
	// request; the reservation carries the immutable actor/key attribution.
	if err = r.applyUsageBillingEffects(ctx, tx, cmd, result, true); err != nil {
		return nil, err
	}
	query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(log)}, tenantUsageLogConflictClause)
	if _, err = tx.ExecContext(ctx, query, args...); err != nil {
		return nil, err
	}
	matched, err := verifyTenantUsageLog(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, service.ErrUsageBillingRequestConflict
	}
	result.TenantUsageLogPersisted = true
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func valueOrZeroRepository(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func validateTenantUsageSnapshot(cmd *service.UsageBillingCommand, log *service.UsageLog) error {
	if cmd == nil || log == nil || !service.ValidExecutionAttribution(cmd.UserID, cmd.ServiceAccountID) || cmd.APIKeyID <= 0 || cmd.AccountID <= 0 ||
		cmd.WorkspaceID <= 0 || cmd.ProjectID <= 0 || cmd.BillingPrincipalUserID <= 0 || strings.TrimSpace(cmd.BudgetReservationID) == "" ||
		(log.UserID != cmd.UserID || valueOrZeroRepository(log.ServiceAccountID) != cmd.ServiceAccountID) || log.APIKeyID != cmd.APIKeyID || log.AccountID != cmd.AccountID || strings.TrimSpace(log.RequestID) != strings.TrimSpace(cmd.RequestID) {
		return service.ErrBudgetReservationConflict
	}
	if log.WorkspaceID == nil || *log.WorkspaceID != cmd.WorkspaceID || log.ProjectID == nil || *log.ProjectID != cmd.ProjectID ||
		log.BillingPrincipalUserID == nil || *log.BillingPrincipalUserID != cmd.BillingPrincipalUserID ||
		log.BudgetReservationID == nil || strings.TrimSpace(*log.BudgetReservationID) != strings.TrimSpace(cmd.BudgetReservationID) {
		return service.ErrBudgetReservationConflict
	}
	if log.ResolvedPlatform != nil && strings.TrimSpace(*log.ResolvedPlatform) != strings.TrimSpace(cmd.ResolvedPlatform) {
		return service.ErrBudgetReservationConflict
	}
	if !validBudgetAmount(log.ActualCost) || !validBudgetAmount(cmd.BudgetActualCost) {
		return service.ErrBudgetReservationConflict
	}
	if cmd.UsageLogCostTelemetryOnly {
		if cmd.BudgetActualCost != 0 || cmd.BalanceCost != 0 || cmd.SubscriptionCost != 0 || cmd.APIKeyQuotaCost != 0 || cmd.AccountQuotaCost != 0 {
			return service.ErrBudgetReservationConflict
		}
	} else if service.QuantizeUsageBillingAmount(log.ActualCost) != service.QuantizeUsageBillingAmount(cmd.BudgetActualCost) {
		return service.ErrBudgetReservationConflict
	}
	if _, err := uuid.Parse(strings.TrimSpace(cmd.BudgetReservationID)); err != nil {
		return service.ErrBudgetReservationConflict
	}
	return nil
}

const tenantUsageLogConflictClause = `
	ON CONFLICT (request_id, api_key_id) DO NOTHING
`

func verifyTenantUsageLog(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	var (
		keyID, accountID, workspaceID, projectID, principalID int64
		userID, machineID                                     sql.NullInt64
		actual, total                                         float64
		model, platform, reservation                          sql.NullString
	)
	err := tx.QueryRowContext(ctx, `SELECT user_id,service_account_id,api_key_id,account_id,workspace_id,project_id,billing_principal_user_id,
		budget_reservation_id::text,resolved_platform,model,actual_cost,total_cost
		FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, cmd.APIKeyID).Scan(
		&userID, &machineID, &keyID, &accountID, &workspaceID, &projectID, &principalID, &reservation, &platform, &model, &actual, &total)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	amountMatches := service.QuantizeUsageBillingAmount(actual) == service.QuantizeUsageBillingAmount(cmd.BudgetActualCost)
	if cmd.UsageLogCostTelemetryOnly {
		amountMatches = cmd.BudgetActualCost == 0 && cmd.BalanceCost == 0 && cmd.SubscriptionCost == 0 && cmd.APIKeyQuotaCost == 0 && cmd.AccountQuotaCost == 0
	}
	return userID.Int64 == cmd.UserID && userID.Valid == (cmd.UserID > 0) && machineID.Int64 == cmd.ServiceAccountID && machineID.Valid == (cmd.ServiceAccountID > 0) && keyID == cmd.APIKeyID && accountID == cmd.AccountID && workspaceID == cmd.WorkspaceID &&
		projectID == cmd.ProjectID && principalID == cmd.BillingPrincipalUserID && reservation.Valid && strings.TrimSpace(reservation.String) == strings.TrimSpace(cmd.BudgetReservationID) &&
		(!platform.Valid || strings.TrimSpace(platform.String) == strings.TrimSpace(cmd.ResolvedPlatform)) &&
		(strings.TrimSpace(cmd.Model) == "" || strings.TrimSpace(model.String) == strings.TrimSpace(cmd.Model)) &&
		amountMatches && total >= actual, nil
}

func (r *usageBillingRepository) IsVideoUsageSettled(ctx context.Context, requestID string, userID, apiKeyID, accountID int64, logOnly bool) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("usage billing repository db is nil")
	}
	var settled bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM usage_logs ul
			WHERE ul.request_id = $1 AND (( $2::bigint>0 AND ul.user_id=$2 AND ul.service_account_id IS NULL) OR ($2::bigint<0 AND ul.user_id IS NULL AND ul.service_account_id=-$2)) AND ul.api_key_id = $3
				AND ul.account_id = $4 AND ul.video_count > 0
				AND ($5 OR NOT (ul.actual_cost = 0 AND ul.total_cost > 0 AND ul.rate_multiplier > 0))
				AND ($5 OR EXISTS (
					SELECT 1 FROM usage_billing_dedup d WHERE d.request_id = $1 AND d.api_key_id = $3
				) OR EXISTS (
					SELECT 1 FROM usage_billing_dedup_archive d WHERE d.request_id = $1 AND d.api_key_id = $3
				))
		)
	`, requestID, userID, apiKeyID, accountID, logOnly).Scan(&settled)
	return settled, err
}

func (r *usageBillingRepository) apply(ctx context.Context, cmd *service.UsageBillingCommand, videoLog *service.UsageLog) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied && videoLog == nil {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: applied}
	if applied {
		if err := r.applyUsageBillingEffects(ctx, tx, cmd, result, videoLog != nil); err != nil {
			return nil, err
		}
	}
	if videoLog != nil {
		query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(videoLog)}, videoUsageLogConflictClause)
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return nil, err
		}
		var owned bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM usage_logs WHERE request_id = $1 AND api_key_id = $2
				AND user_id IS NOT DISTINCT FROM NULLIF($3,0) AND service_account_id IS NOT DISTINCT FROM NULLIF($5,0) AND account_id = $4 AND video_count > 0
		)`, cmd.RequestID, cmd.APIKeyID, cmd.UserID, cmd.AccountID, cmd.ServiceAccountID).Scan(&owned); err != nil {
			return nil, err
		}
		if !owned {
			return nil, errors.New("video usage log ownership mismatch")
		}
		if cmd.WorkspaceID > 0 || cmd.ProjectID > 0 || cmd.BillingPrincipalUserID > 0 || strings.TrimSpace(cmd.BudgetReservationID) != "" {
			matched, verifyErr := verifyTenantUsageLog(ctx, tx, cmd)
			if verifyErr != nil {
				return nil, verifyErr
			}
			if !matched {
				return nil, service.ErrUsageBillingRequestConflict
			}
		}
		result.VideoUsageLogPersisted = true
		if cmd.WorkspaceID > 0 {
			if err := recordRecoveredVideoSettlementTx(ctx, tx, cmd); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

// Only failed historical placeholders may be corrected by a validated settlement.
// Free and simple-mode logs, already paid rows and other owners remain intact.
const videoUsageLogConflictClause = `
	ON CONFLICT (request_id, api_key_id) DO UPDATE SET
		model = EXCLUDED.model,
		requested_model = EXCLUDED.requested_model,
		upstream_model = EXCLUDED.upstream_model,
		upstream_response_model = EXCLUDED.upstream_response_model,
		upstream_model_mismatch = EXCLUDED.upstream_model_mismatch,
		group_id = EXCLUDED.group_id,
		subscription_id = EXCLUDED.subscription_id,
		input_tokens = EXCLUDED.input_tokens,
		output_tokens = EXCLUDED.output_tokens,
		input_cost = EXCLUDED.input_cost,
		output_cost = EXCLUDED.output_cost,
		cache_creation_cost = EXCLUDED.cache_creation_cost,
		cache_read_cost = EXCLUDED.cache_read_cost,
		total_cost = EXCLUDED.total_cost,
		actual_cost = EXCLUDED.actual_cost,
		rate_multiplier = EXCLUDED.rate_multiplier,
		account_rate_multiplier = EXCLUDED.account_rate_multiplier,
		billing_type = EXCLUDED.billing_type,
		video_count = EXCLUDED.video_count,
		video_resolution = EXCLUDED.video_resolution,
		video_duration_seconds = EXCLUDED.video_duration_seconds,
		model_mapping_chain = EXCLUDED.model_mapping_chain,
		billing_tier = EXCLUDED.billing_tier,
		billing_mode = EXCLUDED.billing_mode,
		account_stats_cost = EXCLUDED.account_stats_cost,
		workspace_id = EXCLUDED.workspace_id,
		project_id = EXCLUDED.project_id,
		billing_principal_user_id = EXCLUDED.billing_principal_user_id,
		resolved_platform = EXCLUDED.resolved_platform,
		budget_reservation_id = EXCLUDED.budget_reservation_id
	WHERE usage_logs.user_id = EXCLUDED.user_id AND usage_logs.account_id = EXCLUDED.account_id
		AND usage_logs.video_count > 0 AND usage_logs.actual_cost = 0
		AND usage_logs.total_cost > 0 AND usage_logs.rate_multiplier > 0
		AND usage_logs.workspace_id IS NULL AND usage_logs.project_id IS NULL
		AND usage_logs.billing_principal_user_id IS NULL AND usage_logs.budget_reservation_id IS NULL
		AND usage_logs.resolved_platform IS NULL
		AND EXCLUDED.actual_cost >= 0
`

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	return r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
}

func (r *usageBillingRepository) claimUsageBillingRequest(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, requestFingerprint string) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, requestID, apiKeyID, requestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, requestID, apiKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) ReserveBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, "reserve", reserveUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, "capture", captureUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) ReleaseBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, "release", releaseUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) applyBatchImageBalanceHold(
	ctx context.Context,
	cmd *service.BatchImageBalanceHoldCommand,
	operation string,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (_ *service.BatchImageBalanceHoldResult, err error) {
	if cmd == nil {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	if strings.TrimSpace(cmd.RequestID) == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	job, budgetCmd, err := validateBatchImageHoldSnapshotTx(ctx, tx, cmd, operation)
	if err != nil {
		return nil, err
	}
	if operation == "release" && cmd.WorkspaceID > 0 && budgetCmd == nil {
		if err := validateBatchImageHoldOperationTx(ctx, tx, cmd, budgetCmd, operation); err != nil {
			return nil, err
		}
		// A budget reservation may still be committing. Leave the release
		// request ID unclaimed so recovery can reclaim a late reservation.
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}
	cmd.Normalize()

	applied, err := r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if !applied {
		if operation == "capture" && budgetCmd != nil {
			if err := persistBatchImageSummaryTx(ctx, tx, job, cmd); err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			tx = nil
			return &service.BatchImageBalanceHoldResult{Applied: false, UsageLogPersisted: true}, nil
		}
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}
	if err := validateBatchImageHoldOperationTx(ctx, tx, cmd, budgetCmd, operation); err != nil {
		return nil, err
	}

	result, err := apply(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &service.BatchImageBalanceHoldResult{}
	}
	result.Applied = true
	if budgetCmd != nil {
		switch operation {
		case "capture":
			if err := finalizeTenantBudgetTx(ctx, tx, budgetCmd); err != nil {
				return nil, err
			}
			if err := persistBatchImageSummaryTx(ctx, tx, job, cmd); err != nil {
				return nil, err
			}
			result.UsageLogPersisted = true
		case "release":
			if err := releaseTenantBudgetTx(ctx, tx, budgetCmd); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult, allowDeletedAPIKey ...bool) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		payerID := cmd.BillingPrincipalUserID
		if payerID <= 0 {
			payerID = cmd.UserID
		}
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, payerID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 {
		payerID := cmd.BillingPrincipalUserID
		if payerID <= 0 {
			payerID = cmd.UserID
		}
		newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, payerID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost, allowDeletedAPIKey...)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost, allowDeletedAPIKey...); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	if err := finalizeTenantBudgetTx(ctx, tx, cmd); err != nil {
		return err
	}

	return nil
}

// finalizeTenantBudgetTx moves a durable reservation from reserved to spent on
// the same transaction as the wallet/quota effects and billing dedup claim.
// The reservation is an immutable admission snapshot; current key/workspace
// state is deliberately never consulted during settlement.
func finalizeTenantBudgetTx(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	if cmd == nil || (cmd.WorkspaceID == 0 && cmd.ProjectID == 0 && cmd.BillingPrincipalUserID == 0 && strings.TrimSpace(cmd.BudgetReservationID) == "") {
		return nil
	}
	if !validBudgetAmount(cmd.BudgetActualCost) {
		return service.ErrBudgetReservationInvalid
	}
	res, err := tenantBudgetReservationTx(ctx, tx, cmd)
	if err != nil {
		return err
	}
	// Billing dedup already handles retries of the original command. A fresh
	// billing claim must never reuse a finalized reservation to debit again.
	return settleBudgetReservationTx(ctx, tx, res, service.QuantizeUsageBillingAmount(cmd.BudgetActualCost), "finalized", false)
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID, payerID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.user_id = $3
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID, payerID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	var newBalance float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if err == nil {
		return newBalance, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}

	exists, existsErr := userExistsForBilling(ctx, tx, userID)
	if existsErr != nil {
		return 0, false, existsErr
	}
	if !exists {
		return 0, false, service.ErrUserNotFound
	}
	return 0, false, service.ErrInsufficientBalance
}

func reserveUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			frozen_balance = COALESCE(frozen_balance, 0) + $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, batchImageHoldPayerID(cmd)).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, batchImageHoldPayerID(cmd)); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, service.ErrBatchImageInsufficientBalance
}

func captureUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 && cmd.ActualAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if cmd.ActualAmount-cmd.HoldAmount > 0.00000001 {
		return nil, service.ErrBatchImageSettlementCostExceedsHold
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance
				+ CASE WHEN $1 > $2 THEN $1 - $2 ELSE 0 END
				- CASE WHEN $2 > $1 THEN $2 - $1 ELSE 0 END,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.ActualAmount, batchImageHoldPayerID(cmd)).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, batchImageHoldPayerID(cmd)); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func releaseUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	// 释放前校验该 job 确实预留过 hold（hold request id 已被 claim），
	// 防止从未成功冻结的 job 触发"幻影释放"，从其他用户的冻结资金池中凭空生成余额。
	held, heldErr := batchImageHoldClaimExists(ctx, tx, service.BatchImageHoldRequestID(cmd.BatchID), cmd.APIKeyID)
	if heldErr != nil {
		return nil, heldErr
	}
	if !held {
		logger.LegacyPrintf("repository.usage_billing", "[BatchImage] release skipped, hold was never reserved: batch=%s", cmd.BatchID)
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, batchImageHoldPayerID(cmd)).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, batchImageHoldPayerID(cmd)); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

// batchImageHoldClaimExists 检查 hold request id 是否已在 dedup（或归档）表中被 claim，
// 即该 batch 的冻结操作确实成功提交过。
func batchImageHoldClaimExists(ctx context.Context, tx *sql.Tx, holdRequestID string, apiKeyID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func userExistsForBilling(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64, allowDeleted ...bool) (bool, error) {
	activeFilter := " AND deleted_at IS NULL"
	if len(allowDeleted) > 0 && allowDeleted[0] {
		activeFilter = ""
	}
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2`+activeFilter+`
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64, allowDeleted ...bool) error {
	activeFilter := " AND deleted_at IS NULL"
	if len(allowDeleted) > 0 && allowDeleted[0] {
		activeFilter = ""
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2`+activeFilter+`
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
