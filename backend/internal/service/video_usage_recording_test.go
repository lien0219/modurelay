//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func videoUsageRecordingInputForTest() *OpenAIRecordUsageInput {
	groupID := int64(24)
	return &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "seedance:kz-cgt-" + strings.Repeat("a", 50), Model: "seedance-2.0",
			VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 5,
		},
		APIKey: &APIKey{ID: 20, UserID: 10, GroupID: &groupID, Group: &Group{
			ID: groupID, Platform: PlatformSeedance, RateMultiplier: 1,
			VideoModelPrices: map[string]map[string]float64{"seedance-2.0": {"720p": 0.25}},
		}},
		User: &User{ID: 10}, Account: &Account{ID: 1, Platform: PlatformSeedance},
		QuotaPlatform: PlatformSeedance,
	}
}

func TestOpenAIGatewayServiceRecordUsage_VideoUsageLogFailureCanRetry(t *testing.T) {
	for _, runMode := range []string{config.RunModeStandard, config.RunModeSimple} {
		t.Run(runMode, func(t *testing.T) {
			writeErr := errors.New("usage log persistence unavailable")
			usageRepo := &openAIRecordUsageBestEffortLogRepoStub{bestEffortErr: writeErr, createErr: writeErr}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			gateway := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			gateway.cfg.RunMode = runMode
			input := videoUsageRecordingInputForTest()
			requestID := StableGrokVideoBillingRequestID(input.Result.RequestID)

			err := gateway.RecordUsage(context.Background(), input)
			require.ErrorIs(t, err, writeErr, "video recovery must see the failed insert and retain the pending task")
			require.Equal(t, 1, usageRepo.bestEffortCalls)
			require.Equal(t, 1, usageRepo.createCalls)
			require.Equal(t, requestID, usageRepo.lastLog.RequestID)
			if runMode == config.RunModeStandard {
				require.Equal(t, 1, billingRepo.calls)
				require.Equal(t, requestID, billingRepo.lastCmd.RequestID)
				require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
			}

			// A retry repairs the log with the same durable billing key. Billing has
			// already committed, so the billing repository reports a duplicate.
			usageRepo.bestEffortErr = nil
			usageRepo.createErr = nil
			billingRepo.result = &UsageBillingApplyResult{Applied: false}
			require.NoError(t, gateway.RecordUsage(context.Background(), input))
			require.Equal(t, requestID, usageRepo.lastLog.RequestID)
			require.Equal(t, 1, usageRepo.lastLog.VideoCount)
			require.Equal(t, "720p", *usageRepo.lastLog.VideoResolution)
			require.Equal(t, 5, *usageRepo.lastLog.VideoDurationSeconds)
			if runMode == config.RunModeStandard {
				require.Equal(t, 2, billingRepo.calls)
				require.Equal(t, requestID, billingRepo.lastCmd.RequestID)
				require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
			} else {
				require.Zero(t, billingRepo.calls)
			}
		})
	}
}

func TestOpenAIGatewayServiceRecordUsage_VideoUsageLogSyncFailureCanRetry(t *testing.T) {
	writeErr := errors.New("usage log insert failed")
	usageRepo := &openAIRecordUsageLogRepoStub{err: writeErr}
	gateway := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), videoUsageRecordingInputForTest()), writeErr)
}

func TestOpenAIGatewayServiceRecordUsage_VideoUsageLogSyncFallbackSucceeds(t *testing.T) {
	usageRepo := &openAIRecordUsageBestEffortLogRepoStub{bestEffortErr: MarkUsageLogCreateDropped(errors.New("usage log queue full"))}
	gateway := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	require.NoError(t, gateway.RecordUsage(context.Background(), videoUsageRecordingInputForTest()))
	require.Equal(t, 1, usageRepo.bestEffortCalls)
	require.Equal(t, 1, usageRepo.createCalls)
}

func TestOpenAIGatewayServiceRecordUsage_VideoBillingFailureDefersUsageLog(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{err: ErrInsufficientBalance}
	gateway := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	input := videoUsageRecordingInputForTest()
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), ErrInsufficientBalance)
	// A zero-cost placeholder would occupy the unique key and hide the actual
	// charged cost after replenishment, since usage logs are append-only.
	require.Zero(t, usageRepo.calls)

	billingRepo.err = nil
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	require.Equal(t, 1, usageRepo.calls)
	require.Equal(t, StableGrokVideoBillingRequestID(input.Result.RequestID), usageRepo.lastLog.RequestID)
	require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
}

type videoUsageRecoveryCacheStub struct {
	GatewayCache
	key       string
	payload   []byte
	claimed   bool
	releases  int
	schedules int
	removals  int
}

func (s *videoUsageRecoveryCacheStub) GetGrokVideoPendingBilling(context.Context, string) ([]byte, error) {
	return s.payload, nil
}

func (s *videoUsageRecoveryCacheStub) ClaimGrokVideoBilled(context.Context, string, time.Duration) (bool, error) {
	if s.claimed {
		return false, nil
	}
	s.claimed = true
	return true, nil
}

func (s *videoUsageRecoveryCacheStub) ReleaseGrokVideoBilled(context.Context, string) error {
	s.claimed = false
	s.releases++
	return nil
}

func (s *videoUsageRecoveryCacheStub) ListDueGrokVideoRecovery(context.Context, time.Time, int) ([]string, error) {
	return []string{s.key}, nil
}

func (s *videoUsageRecoveryCacheStub) ScheduleGrokVideoRecovery(context.Context, string, time.Time, time.Duration) error {
	s.schedules++
	return nil
}

func (s *videoUsageRecoveryCacheStub) RemoveGrokVideoRecovery(context.Context, string) error {
	s.removals++
	return nil
}

type videoUsageRecoveryAPIKeyLoaderStub struct {
	APIKeyRepository
	key *APIKey
}

func (s *videoUsageRecoveryAPIKeyLoaderStub) GetByID(context.Context, int64) (*APIKey, error) {
	return s.key, nil
}

func TestVideoBillingRecoveryRetriesUsageLogPersistenceFailure(t *testing.T) {
	input := videoUsageRecordingInputForTest()
	input.APIKey.User = input.User
	account := seedanceFirstClassTestAccount()
	pending := GrokVideoPendingBilling{
		RequestID: input.Result.RequestID, UserID: input.User.ID, APIKeyID: input.APIKey.ID,
		AccountID: account.ID, GroupID: *input.APIKey.GroupID, Model: input.Result.Model,
		VideoResolution: "720p", VideoDurationSeconds: 5, QuotaPlatform: PlatformSeedance,
		NativeProtocol: true, CreatedAt: GrokVideoPendingCreatedAtNow(),
	}
	payload, err := json.Marshal(pending)
	require.NoError(t, err)
	cache := &videoUsageRecoveryCacheStub{
		key: grokVideoPendingBillingKey(pending.RequestID, pending.UserID, pending.APIKeyID), payload: payload,
	}
	writeErr := errors.New("usage log temporarily unavailable")
	usageRepo := &openAIRecordUsageBestEffortLogRepoStub{bestEffortErr: writeErr, createErr: writeErr}
	billingRepo := &openAIRecordUsageBillingRepoStub{}
	gateway := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	gateway.cache = cache
	gateway.accountRepo = &openAIRecordUsageAccountRepoStub{account: account}
	gateway.billingCacheService.apiKeyRateLimitLoader = &videoUsageRecoveryAPIKeyLoaderStub{key: input.APIKey}
	statusBody := `{"id":"task-1","status":"succeeded","content":{"video_url":"https://cdn.example.com/video.mp4"}}`
	gateway.httpUpstream = &grokMediaContentUpstreamStub{responses: []*http.Response{
		grokMediaContentStatusResponse(statusBody), grokMediaContentStatusResponse(statusBody),
	}}

	gateway.recoverDueVideoBilling(context.Background())
	require.Equal(t, 1, cache.releases, "failed usage persistence releases the claim for recovery")
	require.Equal(t, 1, cache.schedules)
	require.Zero(t, cache.removals, "pending recovery must survive a missing usage record")
	require.False(t, cache.claimed)
	require.Equal(t, StableGrokVideoBillingRequestID(pending.RequestID), billingRepo.lastCmd.RequestID)

	usageRepo.bestEffortErr, usageRepo.createErr = nil, nil
	billingRepo.result = &UsageBillingApplyResult{Applied: false}
	gateway.recoverDueVideoBilling(context.Background())
	require.Equal(t, 1, cache.removals)
	require.Equal(t, 1, cache.releases)
	require.True(t, cache.claimed)
	require.Equal(t, 2, billingRepo.calls)
	require.Equal(t, StableGrokVideoBillingRequestID(pending.RequestID), billingRepo.lastCmd.RequestID)
	require.Equal(t, billingRepo.lastCmd.RequestID, usageRepo.lastLog.RequestID)
	require.Equal(t, 1, usageRepo.lastLog.VideoCount)
	require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
}
