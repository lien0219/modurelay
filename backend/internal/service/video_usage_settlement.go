package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// VideoUsageBillingRepository commits the money event and its usage row in one
// transaction. Other gateway protocols keep their existing logging paths.
type VideoUsageBillingRepository interface {
	ApplyVideoUsage(context.Context, *UsageBillingCommand, *UsageLog) (*UsageBillingApplyResult, error)
	IsVideoUsageSettled(ctx context.Context, requestID string, userID, apiKeyID, accountID int64, logOnly bool) (bool, error)
}

// VideoUsageSettlement freezes the first settlement attempt before any money
// moves. It contains accounting data only, never credentials or request bodies.
type VideoUsageSettlement struct {
	Command                    *UsageBillingCommand `json:"command,omitempty"`
	UsageLog                   *UsageLog            `json:"usage_log"`
	QuotaPlatform              string               `json:"quota_platform,omitempty"`
	LogOnly                    bool                 `json:"log_only,omitempty"`
	SimpleModeKeyRateLimitOnly bool                 `json:"simple_mode_key_rate_limit_only,omitempty"`
}

type GrokVideoSettlementPreparer interface {
	PrepareGrokVideoSettlement(ctx context.Context, key string, payload []byte) ([]byte, error)
}

type GrokVideoRecoveryFinalizer interface {
	CompleteGrokVideoRecovery(ctx context.Context, key string, ttl time.Duration) error
}

// A replay may follow a crash between database commit and cache updates.
// Invalidate authoritative snapshots without repeating any quota increments.
func invalidateVideoUsageCaches(ctx context.Context, cache *BillingCacheService, keyService APIKeyQuotaUpdater, userID, keyID, groupID int64, credential string) error {
	cacheCtx, cancel := detachedBillingContext(ctx)
	defer cancel()
	var invalidationErr error
	if cache != nil {
		invalidationErr = errors.Join(cache.InvalidateUserBalance(cacheCtx, userID), cache.InvalidateAPIKeyRateLimit(cacheCtx, keyID))
		if groupID > 0 {
			invalidationErr = errors.Join(invalidationErr, cache.InvalidateSubscription(cacheCtx, userID, groupID))
		}
	}
	if invalidator, ok := keyService.(apiKeyAuthCacheInvalidator); ok && credential != "" {
		invalidator.InvalidateAuthCacheByKey(cacheCtx, credential)
	} else if invalidator, ok := keyService.(interface{ InvalidateAuthCacheByUserID(context.Context, int64) }); ok {
		invalidator.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}
	return invalidationErr
}

func (s *OpenAIGatewayService) invalidateSettledVideoCaches(ctx context.Context, userID, keyID, groupID int64, credential string, keyService APIKeyQuotaUpdater) error {
	if keyService == nil {
		if keys := s.videoRecoveryAPIKeyService.Load(); keys != nil {
			keyService = keys
		}
	}
	return invalidateVideoUsageCaches(ctx, s.billingCacheService, keyService, userID, keyID, groupID, credential)
}

func (s *OpenAIGatewayService) finalizeSettledVideoUsage(ctx context.Context, input *OpenAIRecordUsageInput, groupID int64) error {
	if groupID == 0 && input.APIKey.GroupID != nil {
		groupID = *input.APIKey.GroupID
	}
	if err := s.invalidateSettledVideoCaches(ctx, input.User.ID, input.APIKey.ID, groupID, input.APIKey.Key, input.APIKeyService); err != nil {
		return err
	}
	return s.completeVideoRecovery(ctx, grokVideoPendingBillingKey(videoUsageTaskID(ctx, input), input.User.ID, input.APIKey.ID))
}

func videoUsageTaskID(ctx context.Context, input *OpenAIRecordUsageInput) string {
	if taskID := firstNonEmpty(input.VideoTaskID,
		strings.TrimPrefix(strings.TrimSpace(input.Result.RequestID), "grok-video:"),
		input.Result.ResponseID); taskID != "" {
		return strings.TrimPrefix(strings.TrimSpace(taskID), "grok-video:")
	}
	if ctx != nil {
		clientID, _ := ctx.Value(ctxkey.ClientRequestID).(string)
		localID, _ := ctx.Value(ctxkey.RequestID).(string)
		if strings.TrimSpace(clientID) != "" || strings.TrimSpace(localID) != "" {
			return strings.TrimPrefix(resolveUsageBillingRequestID(ctx, ""), "grok-video:")
		}
	}
	return ""
}

func (s *OpenAIGatewayService) videoUsageSettled(ctx context.Context, requestID string, userID, apiKeyID, accountID int64, logOnly bool) (bool, error) {
	if repo, ok := s.usageBillingRepo.(VideoUsageBillingRepository); ok {
		return repo.IsVideoUsageSettled(ctx, StableGrokVideoBillingRequestID(requestID), userID, apiKeyID, accountID, logOnly)
	}
	return false, nil
}

func (s *OpenAIGatewayService) loadVideoUsageSettlement(ctx context.Context, input *OpenAIRecordUsageInput) (*VideoUsageSettlement, bool, error) {
	if input.APIKey == nil || input.User == nil || input.Account == nil {
		return nil, false, errors.New("video settlement identity is incomplete")
	}
	taskID := videoUsageTaskID(ctx, input)
	if taskID == "" {
		return nil, false, ErrUsageBillingRequestIDRequired
	}
	logOnly := s.cfg != nil && s.cfg.RunMode == config.RunModeSimple && !simpleModeKeyRateLimitBillingEnabled(s.cfg, input.APIKey)
	// A durable row wins over a stale cache, even after model/price changes. A
	// temporary DB outage must still allow us to freeze a new retry in Redis.
	if settled, err := s.videoUsageSettled(ctx, taskID, input.User.ID, input.APIKey.ID, input.Account.ID, false); err == nil && settled {
		return nil, true, s.finalizeSettledVideoUsage(ctx, input, 0)
	}
	if s.cache != nil {
		pending, err := s.LoadGrokVideoPendingBilling(ctx, taskID, input.User.ID, input.APIKey.ID)
		if err != nil {
			return nil, false, err
		}
		if pending != nil {
			if pending.Cancelled {
				return nil, false, ErrGrokVideoBillingCancelled
			}
			if pending.UserID != input.User.ID || pending.APIKeyID != input.APIKey.ID || pending.AccountID != input.Account.ID {
				return nil, false, errors.New("video settlement ownership mismatch")
			}
			if pending.Settlement != nil {
				if settled, err := s.videoUsageSettled(ctx, taskID, input.User.ID, input.APIKey.ID, input.Account.ID, pending.Settlement.LogOnly); err == nil && settled {
					return nil, true, s.finalizeSettledVideoUsage(ctx, input, pending.GroupID)
				}
				return pending.Settlement, false, nil
			}
		}
	}
	// Only an unfrozen legacy task may use the current mode's log-only lookup.
	// Frozen standard-mode money effects survive later configuration changes.
	if logOnly {
		if settled, err := s.videoUsageSettled(ctx, taskID, input.User.ID, input.APIKey.ID, input.Account.ID, true); err == nil && settled {
			return nil, true, s.finalizeSettledVideoUsage(ctx, input, 0)
		}
	}
	return nil, false, nil
}

func (s *OpenAIGatewayService) prepareVideoUsageSettlement(ctx context.Context, input *OpenAIRecordUsageInput, usageLog *UsageLog, p *postUsageBillingParams, logOnly bool) (*VideoUsageSettlement, error) {
	logCopy := *usageLog
	logCopy.User, logCopy.APIKey, logCopy.Account, logCopy.Group, logCopy.Subscription = nil, nil, nil, nil, nil
	p.VideoUsageLog = &logCopy
	settlement := &VideoUsageSettlement{
		UsageLog: &logCopy, QuotaPlatform: p.Platform, LogOnly: logOnly,
		SimpleModeKeyRateLimitOnly: p.SimpleModeKeyRateLimitOnly,
	}
	billingParams := *p
	if billingParams.BudgetReservationID == "" {
		billingParams.BudgetReservationID = BudgetReservationIDFromContext(ctx)
	}
	if logOnly {
		// Record a durable zero-money receipt too. A later run-mode change or
		// loss of Redis state must never make this completed video payable.
		billingParams.Cost = &CostBreakdown{}
		billingParams.UsageLogCostTelemetryOnly = true
	}
	settlement.Command = buildUsageBillingCommand(usageLog.RequestID, &logCopy, &billingParams)
	if s.cache == nil {
		return settlement, nil
	}
	taskID := videoUsageTaskID(ctx, input)
	pending, err := s.LoadGrokVideoPendingBilling(ctx, taskID, input.User.ID, input.APIKey.ID)
	if err != nil {
		return nil, err
	}
	if pending == nil {
		pending = &GrokVideoPendingBilling{
			RequestID: taskID, UserID: input.User.ID, APIKeyID: input.APIKey.ID, AccountID: input.Account.ID,
			Model: usageLog.Model, OriginalModel: usageLog.RequestedModel, QuotaPlatform: p.Platform,
			CreatedAt: usageLog.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if usageLog.GroupID != nil {
			pending.GroupID = *usageLog.GroupID
		}
		if usageLog.SubscriptionID != nil {
			pending.SubscriptionID = *usageLog.SubscriptionID
		}
	}
	if pending.UserID != input.User.ID || pending.APIKeyID != input.APIKey.ID || pending.AccountID != input.Account.ID {
		return nil, errors.New("video settlement ownership mismatch")
	}
	pending.Settlement = settlement
	payload, err := json.Marshal(pending)
	if err != nil {
		return nil, err
	}
	key := grokVideoPendingBillingKey(taskID, input.User.ID, input.APIKey.ID)
	if preparer, ok := s.cache.(GrokVideoSettlementPreparer); ok {
		// First writer wins, including after a lease expires. Competing workers
		// must use the same amounts, model and request fingerprint.
		payload, err = preparer.PrepareGrokVideoSettlement(ctx, key, payload)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, pending); err != nil {
			return nil, err
		}
		if pending.Cancelled {
			return nil, ErrGrokVideoBillingCancelled
		}
		return pending.Settlement, nil
	}
	if err := s.cache.SetGrokVideoPendingBilling(ctx, key, payload, grokVideoPendingBillingTTL(s.cfg)); err != nil {
		return nil, err
	}
	if recovery, ok := s.cache.(GrokVideoRecoveryCache); ok {
		if err := recovery.ScheduleGrokVideoRecovery(ctx, key, time.Now().Add(grokVideoRecoveryRetryDelay), grokVideoPendingBillingTTL(s.cfg)); err != nil {
			return nil, err
		}
	}
	return settlement, nil
}

func (s *OpenAIGatewayService) recordVideoUsageSettlement(ctx context.Context, input *OpenAIRecordUsageInput, settlement *VideoUsageSettlement) error {
	if settlement == nil || settlement.UsageLog == nil {
		return errors.New("video settlement usage is missing")
	}
	log := settlement.UsageLog
	if log.UserID != input.User.ID || log.APIKeyID != input.APIKey.ID || log.AccountID != input.Account.ID ||
		log.RequestID != StableGrokVideoBillingRequestID(videoUsageTaskID(ctx, input)) || log.VideoCount <= 0 {
		return errors.New("video settlement usage ownership mismatch")
	}
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: log.ActualCost, TotalCost: log.TotalCost},
		User: input.User, APIKey: input.APIKey, Account: input.Account, Subscription: input.Subscription,
		IsSubscriptionBill: log.BillingType == BillingTypeSubscription,
		APIKeyService:      input.APIKeyService, Platform: settlement.QuotaPlatform,
		SimpleModeKeyRateLimitOnly: settlement.SimpleModeKeyRateLimitOnly, VideoUsageLog: log,
	}
	if log.AccountRateMultiplier != nil {
		p.AccountRateMultiplier = *log.AccountRateMultiplier
	}
	if !settlement.LogOnly {
		cmd := settlement.Command
		if cmd == nil || cmd.RequestID != log.RequestID || cmd.UserID != log.UserID || cmd.APIKeyID != log.APIKeyID || cmd.AccountID != log.AccountID {
			return errors.New("video settlement billing ownership mismatch")
		}
		result, err := applyPreparedUsageBilling(ctx, cmd, p, s.billingDeps(), s.usageBillingRepo)
		if err != nil {
			return err
		}
		if result == nil || !result.VideoUsageLogPersisted {
			if err := writeUsageLog(ctx, s.usageLogRepo, log, "service.openai_gateway"); err != nil {
				return err
			}
		}
	} else {
		var result *UsageBillingApplyResult
		if _, ok := s.usageBillingRepo.(VideoUsageBillingRepository); ok {
			p.Cost = &CostBreakdown{}
			cmd := settlement.Command
			if cmd == nil {
				// Older log-only snapshots did not contain a receipt command.
				cmd = buildUsageBillingCommand(log.RequestID, log, p)
			}
			if cmd == nil || cmd.RequestID != log.RequestID || cmd.UserID != log.UserID || cmd.APIKeyID != log.APIKeyID || cmd.AccountID != log.AccountID ||
				cmd.BalanceCost != 0 || cmd.SubscriptionCost != 0 || cmd.APIKeyQuotaCost != 0 || cmd.APIKeyRateLimitCost != 0 || cmd.AccountQuotaCost != 0 {
				return errors.New("log-only video settlement must have zero money effects")
			}
			var err error
			result, err = applyPreparedUsageBilling(ctx, cmd, p, s.billingDeps(), s.usageBillingRepo)
			if err != nil {
				return err
			}
		}
		if result == nil || !result.VideoUsageLogPersisted {
			if err := writeUsageLog(ctx, s.usageLogRepo, log, "service.openai_gateway"); err != nil {
				return err
			}
			s.deferredService.ScheduleLastUsedUpdate(input.Account.ID)
		}
	}
	return s.completeVideoRecovery(ctx, grokVideoPendingBillingKey(videoUsageTaskID(ctx, input), input.User.ID, input.APIKey.ID))
}

func (s *OpenAIGatewayService) completeVideoRecovery(ctx context.Context, key string) error {
	if finalizer, ok := s.cache.(GrokVideoRecoveryFinalizer); ok {
		if err := finalizer.CompleteGrokVideoRecovery(ctx, key, 24*time.Hour); err != nil {
			return fmt.Errorf("finalize video settlement: %w", err)
		}
	}
	return nil
}
