package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	videoBillingRecoveryPollInterval = 10 * time.Second
	videoBillingRecoveryQueryTimeout = 30 * time.Second
)

type videoRecoveryAPIKeyLoader interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

func (s *OpenAIGatewayService) videoRecoveryAPIKeyLoader() videoRecoveryAPIKeyLoader {
	if s == nil || s.billingCacheService == nil || s.billingCacheService.apiKeyRateLimitLoader == nil {
		return nil
	}
	loader, _ := s.billingCacheService.apiKeyRateLimitLoader.(videoRecoveryAPIKeyLoader)
	return loader
}

func (s *OpenAIGatewayService) startVideoBillingRecovery() {
	if s == nil || s.cache == nil || s.accountRepo == nil || s.usageBillingRepo == nil || s.videoRecoveryAPIKeyLoader() == nil {
		return
	}
	if _, ok := s.cache.(GrokVideoRecoveryCache); !ok {
		return
	}
	s.videoRecoveryMu.Lock()
	defer s.videoRecoveryMu.Unlock()
	if s.videoRecoveryCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.videoRecoveryCancel = cancel
	s.videoRecoveryDone = done
	go func() {
		defer close(done)
		s.runVideoBillingRecovery(ctx)
	}()
}

func (s *OpenAIGatewayService) stopVideoBillingRecovery() {
	if s == nil {
		return
	}
	s.videoRecoveryMu.Lock()
	cancel := s.videoRecoveryCancel
	done := s.videoRecoveryDone
	s.videoRecoveryCancel = nil
	s.videoRecoveryDone = nil
	s.videoRecoveryMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *OpenAIGatewayService) runVideoBillingRecovery(ctx context.Context) {
	ticker := time.NewTicker(videoBillingRecoveryPollInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		s.recoverDueVideoBilling(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *OpenAIGatewayService) recoverDueVideoBilling(ctx context.Context) {
	recovery, ok := s.cache.(GrokVideoRecoveryCache)
	if !ok {
		return
	}
	keys, err := recovery.ListDueGrokVideoRecovery(ctx, time.Now(), grokVideoRecoveryBatchLimit)
	if err != nil {
		logger.L().Warn("video_billing_recovery.list_failed", zap.Error(err))
		return
	}
	for _, key := range keys {
		if ctx.Err() != nil {
			return
		}
		queryCtx, cancel := context.WithTimeout(ctx, videoBillingRecoveryQueryTimeout)
		terminal, processErr := s.recoverVideoBillingKey(queryCtx, key)
		cancel()
		if processErr != nil {
			logger.L().Warn("video_billing_recovery.process_failed",
				zap.String("key", key),
				zap.Error(processErr),
			)
			_ = recovery.ScheduleGrokVideoRecovery(ctx, key, time.Now().Add(grokVideoRecoveryRetryDelay), grokVideoPendingBillingTTL(s.cfg))
			continue
		}
		if terminal {
			if err := recovery.RemoveGrokVideoRecovery(ctx, key); err != nil {
				logger.L().Warn("video_billing_recovery.remove_failed", zap.String("key", key), zap.Error(err))
			}
			continue
		}
		if err := recovery.ScheduleGrokVideoRecovery(ctx, key, time.Now().Add(grokVideoRecoveryRetryDelay), grokVideoPendingBillingTTL(s.cfg)); err != nil {
			logger.L().Warn("video_billing_recovery.reschedule_failed", zap.String("key", key), zap.Error(err))
		}
	}
}

func (s *OpenAIGatewayService) recoverVideoBillingKey(ctx context.Context, key string) (bool, error) {
	payload, err := s.cache.GetGrokVideoPendingBilling(ctx, strings.TrimSpace(key))
	if err != nil {
		return false, err
	}
	if len(payload) == 0 {
		return true, nil
	}
	var pending GrokVideoPendingBilling
	if err := json.Unmarshal(payload, &pending); err != nil {
		return true, fmt.Errorf("decode pending video billing: %w", err)
	}
	if pending.RequestID == "" || pending.UserID <= 0 || pending.APIKeyID <= 0 || pending.AccountID <= 0 {
		return true, fmt.Errorf("pending video recovery metadata is incomplete")
	}

	account, err := s.accountRepo.GetByID(ctx, pending.AccountID)
	if err != nil || account == nil {
		return false, fmt.Errorf("load video account %d: %w", pending.AccountID, err)
	}
	result, terminalStatus, err := s.queryRecoveredVideoStatus(ctx, account, &pending)
	if err != nil {
		return false, err
	}
	if terminalStatus && (result == nil || !videoRecoveryResultBillable(pending.RequestID, result)) {
		return true, nil
	}
	if result == nil || !videoRecoveryResultBillable(pending.RequestID, result) {
		return false, nil
	}

	claimed, err := s.ClaimGrokVideoBilling(ctx, pending.RequestID, pending.UserID, pending.APIKeyID)
	if err != nil {
		return false, err
	}
	if !claimed {
		return true, nil
	}

	if err := s.recordRecoveredVideoUsage(ctx, account, &pending, result); err != nil {
		if releaseErr := s.ReleaseGrokVideoBilling(ctx, pending.RequestID, pending.UserID, pending.APIKeyID); releaseErr != nil {
			logger.L().Error("video_billing_recovery.claim_release_failed",
				zap.String("request_id", pending.RequestID),
				zap.Error(releaseErr),
			)
		}
		return false, err
	}
	return true, nil
}

func videoRecoveryResultBillable(requestID string, result *OpenAIForwardResult) bool {
	if result == nil {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(requestID), "seedance:") {
		return result.Usage.OutputTokens > 0
	}
	return result.VideoCount > 0
}

func (s *OpenAIGatewayService) queryRecoveredVideoStatus(
	ctx context.Context,
	account *Account,
	pending *GrokVideoPendingBilling,
) (*OpenAIForwardResult, bool, error) {
	if account == nil || pending == nil {
		return nil, false, errors.New("video recovery status input is incomplete")
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/video-billing-recovery", nil).WithContext(ctx)

	requestID := strings.TrimSpace(pending.RequestID)
	var result *OpenAIForwardResult
	var err error
	switch {
	case strings.HasPrefix(requestID, "aistarslab:"):
		result, err = s.ForwardAIStarsLabOpenAPIVideo(ctx, c, account, GrokMediaEndpointVideoStatus, requestID, nil, "", pending.Model)
	case strings.HasPrefix(requestID, "seedance:"):
		result, err = s.ForwardSeedanceCompatibleVideo(ctx, c, account, GrokMediaEndpointVideoStatus, requestID, nil, "", pending.Model)
	case account.Platform == PlatformGrok:
		result, err = s.ForwardGrokMedia(ctx, c, account, GrokMediaEndpointVideoStatus, requestID, nil, "")
	default:
		result, err = s.ForwardCompatibleVideo(ctx, c, account, GrokMediaEndpointVideoStatus, requestID, nil, "", pending.Model)
	}
	if err != nil {
		return nil, false, err
	}
	return result, videoRecoveryTerminalStatus(rec.Body.Bytes()), nil
}

func videoRecoveryTerminalStatus(body []byte) bool {
	status := strings.ToLower(strings.TrimSpace(compatibleVideoJSONField(
		body,
		"status", "data.status", "result.status", "video.status",
	)))
	switch status {
	case "completed", "done", "succeeded", "success", "finished",
		"failed", "error", "expired", "canceled", "cancelled", "rejected":
		return true
	}
	// AIStarsLab/OpenAPI adapter rewrites numeric upstream states to strings
	// before this recovery helper sees the response body. Keep a numeric fallback
	// for direct compatible providers that expose 3=completed / 4=failed.
	if status == "" {
		raw := gjson.GetBytes(body, "status")
		return raw.Type == gjson.Number && (raw.Int() == 3 || raw.Int() == 4)
	}
	return false
}

func (s *OpenAIGatewayService) recordRecoveredVideoUsage(
	ctx context.Context,
	account *Account,
	pending *GrokVideoPendingBilling,
	result *OpenAIForwardResult,
) error {
	loader := s.videoRecoveryAPIKeyLoader()
	if loader == nil {
		return errors.New("video recovery api key loader is unavailable")
	}
	apiKey, err := loader.GetByID(ctx, pending.APIKeyID)
	if err != nil || apiKey == nil {
		return fmt.Errorf("load api key %d: %w", pending.APIKeyID, err)
	}
	if apiKey.User == nil || apiKey.UserID != pending.UserID {
		return errors.New("video recovery api key ownership mismatch")
	}
	if pending.GroupID > 0 && (apiKey.GroupID == nil || *apiKey.GroupID != pending.GroupID) {
		return errors.New("video recovery api key group changed")
	}

	var subscription *UserSubscription
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		if s.userSubRepo == nil || apiKey.GroupID == nil {
			return errors.New("video recovery subscription repository is unavailable")
		}
		subscription, err = s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, pending.UserID, *apiKey.GroupID)
		if err != nil {
			return fmt.Errorf("load video recovery subscription: %w", err)
		}
		if subscription == nil {
			return errors.New("video recovery subscription is no longer active")
		}
	}

	merged := *result
	merged.Model = firstNonEmpty(pending.Model, merged.Model)
	merged.BillingModel = firstNonEmpty(pending.BillingModel, pending.Model, merged.BillingModel, merged.Model)
	merged.UpstreamModel = firstNonEmpty(pending.UpstreamModel, merged.UpstreamModel)
	if pending.VideoResolution != "" {
		merged.VideoResolution = pending.VideoResolution
	}
	if merged.VideoDurationSeconds <= 0 {
		merged.VideoDurationSeconds = pending.VideoDurationSeconds
	}
	merged.ResponseID = firstNonEmpty(merged.ResponseID, pending.RequestID)
	merged.RequestID = StableGrokVideoBillingRequestID(pending.RequestID)
	merged.Duration = GrokVideoE2EDuration(pending.CreatedAt, time.Now())
	if strings.HasPrefix(strings.TrimSpace(pending.RequestID), "seedance:") {
		merged.ForceTokenBilling = true
		merged.VideoCount = 1
	} else {
		merged.VideoCount = 1
	}

	pricingAt := time.Time{}
	if created := strings.TrimSpace(pending.CreatedAt); created != "" {
		pricingAt, _ = time.Parse(time.RFC3339Nano, created)
	}
	return s.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result:             &merged,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    "/v1/videos/:id",
		UpstreamEndpoint:   "/v1/videos/:id",
		UserAgent:          "modurelay-video-recovery",
		RequestPayloadHash: HashUsageRequestPayload([]byte(pending.RequestID)),
		QuotaPlatform:      PlatformFromAPIKey(apiKey),
		PricingAt:          pricingAt,
		ChannelUsageFields: ChannelUsageFields{
			OriginalModel:      pending.OriginalModel,
			ChannelMappedModel: pending.Model,
		},
	})
}
