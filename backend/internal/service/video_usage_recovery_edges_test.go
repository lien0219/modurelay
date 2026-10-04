//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func videoRecoveryEdgeFixture(t *testing.T) (*OpenAIGatewayService, *videoUsageRecoveryCacheStub, *openAIRecordUsageBestEffortLogRepoStub, *openAIRecordUsageBillingRepoStub, *OpenAIRecordUsageInput, *GrokVideoPendingBilling) {
	t.Helper()
	input := videoUsageRecordingInputForTest()
	input.APIKey.User = input.User
	input.Account = seedanceFirstClassTestAccount()
	input.Result.ResponseID = input.Result.RequestID
	pending := &GrokVideoPendingBilling{
		RequestID: input.Result.RequestID, UserID: input.User.ID, APIKeyID: input.APIKey.ID,
		AccountID: input.Account.ID, GroupID: *input.APIKey.GroupID, Model: input.Result.Model,
		BillingModel: input.Result.Model, VideoResolution: "720p", VideoDurationSeconds: 5,
		QuotaPlatform: PlatformSeedance, NativeProtocol: true, CreatedAt: GrokVideoPendingCreatedAtNow(),
	}
	payload, err := json.Marshal(pending)
	require.NoError(t, err)
	cache := &videoUsageRecoveryCacheStub{key: grokVideoPendingBillingKey(pending.RequestID, pending.UserID, pending.APIKeyID), payload: payload}
	usageRepo := &openAIRecordUsageBestEffortLogRepoStub{}
	billingRepo := &openAIRecordUsageBillingRepoStub{}
	gateway := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	gateway.cache = cache
	gateway.accountRepo = &openAIRecordUsageAccountRepoStub{account: input.Account}
	gateway.billingCacheService.apiKeyRateLimitLoader = &videoUsageRecoveryAPIKeyLoaderStub{key: input.APIKey}
	gateway.httpUpstream = &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(
		`{"id":"task-1","status":"succeeded","content":{"video_url":"https://cdn.example.com/video.mp4"}}`,
	)}
	input.RequestPayloadHash = HashUsageRequestPayload([]byte(pending.RequestID))
	return gateway, cache, usageRepo, billingRepo, input, pending
}

func TestVideoRecoveryRetainsTaskWhileAnotherWorkerHoldsClaim(t *testing.T) {
	gateway, cache, _, billingRepo, _, _ := videoRecoveryEdgeFixture(t)
	cache.claimed = true
	gateway.recoverDueVideoBilling(context.Background())
	require.Zero(t, billingRepo.calls)
	require.Zero(t, cache.removals, "an in-flight claim is not proof that settlement finished")
	require.Positive(t, cache.schedules)
}

func TestVideoRecoveryPreservesAPIKeyQuotaAndRateWindows(t *testing.T) {
	gateway, _, usageRepo, billingRepo, input, pending := videoRecoveryEdgeFixture(t)
	input.APIKey.Quota, input.APIKey.RateLimit5h = 100, 100
	input.APIKeyService = &openAIRecordUsageAPIKeyQuotaStub{}
	writeErr := errors.New("usage insert failed")
	usageRepo.bestEffortErr, usageRepo.createErr = writeErr, writeErr
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), writeErr)
	first := *billingRepo.lastCmd
	require.InDelta(t, 1.25, first.APIKeyQuotaCost, 1e-12)
	require.InDelta(t, 1.25, first.APIKeyRateLimitCost, 1e-12)
	usageRepo.bestEffortErr, usageRepo.createErr = nil, nil
	billingRepo.result = &UsageBillingApplyResult{Applied: false}
	require.NoError(t, gateway.recordRecoveredVideoUsage(context.Background(), input.Account, pending, input.Result))
	require.Equal(t, first.RequestFingerprint, billingRepo.lastCmd.RequestFingerprint)
}

func TestVideoRecoveryPreservesMappedModelFingerprint(t *testing.T) {
	gateway, cache, usageRepo, billingRepo, input, pending := videoRecoveryEdgeFixture(t)
	input.Account.Platform, input.APIKey.Group.Platform, input.QuotaPlatform = PlatformOpenAI, PlatformOpenAI, PlatformOpenAI
	input.APIKey.Group.VideoModelPrices = map[string]map[string]float64{
		"public-video": {"720p": 0.25}, "vendor-video": {"720p": 0.25},
	}
	input.Result.RequestID, input.Result.ResponseID = "compatible-task-1", "compatible-task-1"
	input.Result.Model, input.Result.BillingModel = "vendor-video", "vendor-video"
	pending.RequestID, pending.Model, pending.BillingModel, pending.OriginalModel = "compatible-task-1", "public-video", "public-video", "public-video"
	pending.QuotaPlatform, pending.NativeProtocol = PlatformOpenAI, false
	cache.key = grokVideoPendingBillingKey(pending.RequestID, pending.UserID, pending.APIKeyID)
	var err error
	cache.payload, err = json.Marshal(pending)
	require.NoError(t, err)
	input.RequestPayloadHash = HashUsageRequestPayload([]byte(pending.RequestID))
	writeErr := errors.New("usage insert failed")
	usageRepo.bestEffortErr, usageRepo.createErr = writeErr, writeErr
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), writeErr)
	first := *billingRepo.lastCmd
	usageRepo.bestEffortErr, usageRepo.createErr = nil, nil
	billingRepo.result = &UsageBillingApplyResult{Applied: false}
	require.NoError(t, gateway.recordRecoveredVideoUsage(context.Background(), input.Account, pending, input.Result))
	require.Equal(t, first.RequestFingerprint, billingRepo.lastCmd.RequestFingerprint)
	require.Equal(t, "vendor-video", usageRepo.lastLog.Model)
}

func TestVideoRecoveryUsesOriginalCostAfterPricingChanges(t *testing.T) {
	gateway, _, _, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	billingRepo.err = ErrInsufficientBalance
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), ErrInsufficientBalance)
	first := *billingRepo.lastCmd
	input.APIKey.Group.VideoModelPrices["seedance-2.0"]["720p"] = 9
	input.APIKey.Group.RateMultiplier = 7
	billingRepo.err = nil
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	require.Equal(t, first.RequestFingerprint, billingRepo.lastCmd.RequestFingerprint)
	require.InDelta(t, 1.25, billingRepo.lastCmd.BalanceCost, 1e-12)
}

func TestSynchronousVideoBillingFailurePersistsRecoverableSettlement(t *testing.T) {
	gateway, cache, usageRepo, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	input.Result.RequestID, input.Result.ResponseID = "sync-response-1", ""
	cache.payload = nil
	billingRepo.err = errors.New("temporary billing database outage")
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), billingRepo.err)
	require.NotEmpty(t, cache.payload, "a synchronous result needs settlement state even without an upstream task ID")
	require.Zero(t, usageRepo.bestEffortCalls+usageRepo.createCalls, "do not insert a zero-cost row before settlement")
	first := *billingRepo.lastCmd
	billingRepo.err = nil
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"status":"expired"}`)}
	gateway.httpUpstream = upstream
	terminal, err := gateway.recoverVideoBillingKey(context.Background(), cache.key)
	require.NoError(t, err, "frozen synchronous usage must recover without contacting the upstream")
	require.True(t, terminal)
	require.Empty(t, upstream.requests)
	require.Equal(t, first.RequestFingerprint, billingRepo.lastCmd.RequestFingerprint)
	require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
}

type settledVideoUsageBillingRepoStub struct {
	*openAIRecordUsageBillingRepoStub
	settled bool
}

func (s *settledVideoUsageBillingRepoStub) IsVideoUsageSettled(context.Context, string, int64, int64, int64, bool) (bool, error) {
	return s.settled, nil
}

func (s *settledVideoUsageBillingRepoStub) ApplyVideoUsage(ctx context.Context, command *UsageBillingCommand, _ *UsageLog) (*UsageBillingApplyResult, error) {
	result, err := s.Apply(ctx, command)
	if result != nil {
		result.VideoUsageLogPersisted = true
	}
	return result, err
}

type videoSettlementBillingCacheStub struct {
	billingCacheWorkerStub
	balances, windows, subscriptions int
}

func (s *videoSettlementBillingCacheStub) InvalidateUserBalance(context.Context, int64) error {
	s.balances++
	return nil
}
func (s *videoSettlementBillingCacheStub) InvalidateAPIKeyRateLimit(context.Context, int64) error {
	s.windows++
	return nil
}
func (s *videoSettlementBillingCacheStub) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	s.subscriptions++
	return nil
}

type videoSettlementAuthCacheStub struct {
	openAIRecordUsageAPIKeyQuotaStub
	invalidations int
}

func (s *videoSettlementAuthCacheStub) InvalidateAuthCacheByKey(context.Context, string) {
	s.invalidations++
}

func TestVideoSettledReceiptInvalidatesCachesWithoutRebilling(t *testing.T) {
	for _, fromRecovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreground", true: "recovery"}[fromRecovery], func(t *testing.T) {
			gateway, cache, usageRepo, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
			gateway.usageBillingRepo = &settledVideoUsageBillingRepoStub{openAIRecordUsageBillingRepoStub: billingRepo, settled: true}
			billingCache := &videoSettlementBillingCacheStub{}
			gateway.billingCacheService.cache = billingCache
			authCache := &videoSettlementAuthCacheStub{}
			input.APIKey.Key, input.APIKeyService = "sk-video-test", authCache
			if fromRecovery {
				terminal, err := gateway.recoverVideoBillingKey(context.Background(), cache.key)
				require.NoError(t, err)
				require.True(t, terminal)
			} else {
				require.NoError(t, gateway.RecordUsage(context.Background(), input))
				require.Positive(t, authCache.invalidations)
			}
			require.Positive(t, billingCache.balances)
			require.Positive(t, billingCache.windows)
			require.Positive(t, billingCache.subscriptions)
			require.Zero(t, billingRepo.calls)
			require.Zero(t, usageRepo.bestEffortCalls+usageRepo.createCalls)
			require.Zero(t, billingCache.balanceUpdates+billingCache.subscriptionUpdates, "repair only invalidates; it must never repeat cached increments")
		})
	}
}

func TestVideoDuplicateTransactionInvalidatesCachesWithoutRebilling(t *testing.T) {
	gateway, _, _, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	billingRepo.result = &UsageBillingApplyResult{Applied: false}
	gateway.usageBillingRepo = &settledVideoUsageBillingRepoStub{openAIRecordUsageBillingRepoStub: billingRepo}
	billingCache := &videoSettlementBillingCacheStub{}
	gateway.billingCacheService.cache = billingCache
	authCache := &videoSettlementAuthCacheStub{}
	input.APIKey.Key, input.APIKeyService = "sk-video-test", authCache
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	require.Equal(t, 1, billingRepo.calls)
	require.Positive(t, billingCache.balances)
	require.Positive(t, billingCache.windows)
	require.Positive(t, authCache.invalidations)
	require.Zero(t, billingCache.balanceUpdates+billingCache.subscriptionUpdates)
}

func TestVideoSettlementRequiresStableIdentity(t *testing.T) {
	gateway, cache, usageRepo, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	input.Result.RequestID, input.Result.ResponseID = "", ""
	cache.payload = nil
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), ErrUsageBillingRequestIDRequired)
	require.Zero(t, billingRepo.calls)
	require.Zero(t, usageRepo.bestEffortCalls+usageRepo.createCalls)
	require.Empty(t, cache.payload)
}

func TestVideoSettlementUsesOwnedTaskIdentity(t *testing.T) {
	gateway, cache, usageRepo, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	input.VideoTaskID = "original-owned-task"
	input.Result.RequestID = "vendor-poll-request"
	cache.payload = nil
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	require.Equal(t, "grok-video:original-owned-task", billingRepo.lastCmd.RequestID)
	require.Equal(t, billingRepo.lastCmd.RequestID, usageRepo.lastLog.RequestID)
}

func TestVideoSettlementUsesStableLocalIdentityForSynchronousResponse(t *testing.T) {
	gateway, cache, usageRepo, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	input.Result.RequestID, input.Result.ResponseID = "", ""
	cache.payload = nil
	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "synchronous-request-1")
	require.NoError(t, gateway.RecordUsage(ctx, input))
	require.Equal(t, "grok-video:local:synchronous-request-1", billingRepo.lastCmd.RequestID)
	require.Equal(t, billingRepo.lastCmd.RequestID, usageRepo.lastLog.RequestID)
}

type videoSettlementWindowCacheStub struct {
	videoSettlementBillingCacheStub
	windowUsage float64
}

func (s *videoSettlementWindowCacheStub) UpdateAPIKeyRateLimitUsage(_ context.Context, _ int64, cost float64) error {
	s.windowUsage += cost
	return nil
}

func TestVideoSettlementDoesNotDoubleCachedWindowsAfterReceiptReload(t *testing.T) {
	gateway, _, _, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	input.APIKey.RateLimit5h = 100
	billingRepo.result = &UsageBillingApplyResult{Applied: true}
	repo := &settledVideoUsageBillingRepoStub{openAIRecordUsageBillingRepoStub: billingRepo}
	gateway.usageBillingRepo = repo
	cache := &videoSettlementWindowCacheStub{}
	gateway.billingCacheService.cache = cache
	// Delay queue consumption until after a second worker repairs the cache.
	queue := make(chan cacheWriteTask, 8)
	gateway.billingCacheService.cacheWriteChan = queue
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	repo.settled = true
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	// A new preflight now reloads the already committed 1.25 from PostgreSQL.
	cache.windowUsage = 1.25
	close(queue)
	gateway.billingCacheService.cacheWriteWg.Add(1)
	gateway.billingCacheService.cacheWriteWorker(queue)
	gateway.billingCacheService.cacheWriteChan = nil
	require.Equal(t, 1.25, cache.windowUsage, "late video cache deltas must not count an already reloaded charge twice")
}

type videoLogOnlyReceiptRepoStub struct {
	*settledVideoUsageBillingRepoStub
}

func (s *videoLogOnlyReceiptRepoStub) IsVideoUsageSettled(_ context.Context, _ string, _, _, _ int64, logOnly bool) (bool, error) {
	return logOnly, nil
}

func TestVideoSettlementKeepsFrozenMoneyEffectsAfterModeChange(t *testing.T) {
	gateway, _, _, billingRepo, input, _ := videoRecoveryEdgeFixture(t)
	gateway.usageBillingRepo = &videoLogOnlyReceiptRepoStub{&settledVideoUsageBillingRepoStub{openAIRecordUsageBillingRepoStub: billingRepo}}
	billingRepo.err = ErrInsufficientBalance
	require.ErrorIs(t, gateway.RecordUsage(context.Background(), input), ErrInsufficientBalance)
	first := *billingRepo.lastCmd
	// A current log-only mode must not mistake a historical zero-cost
	// placeholder for settlement of this frozen standard-mode money event.
	gateway.cfg.RunMode = config.RunModeSimple
	billingRepo.err = nil
	require.NoError(t, gateway.RecordUsage(context.Background(), input))
	require.Equal(t, 2, billingRepo.calls)
	require.Equal(t, first.RequestFingerprint, billingRepo.lastCmd.RequestFingerprint)
	require.Equal(t, 1.25, billingRepo.lastCmd.BalanceCost)
}

type deletedVideoAPIKeyLoader struct {
	APIKeyRepository
	key *APIKey
}

func (s *deletedVideoAPIKeyLoader) GetByID(context.Context, int64) (*APIKey, error) {
	return nil, ErrAPIKeyNotFound
}

func (s *deletedVideoAPIKeyLoader) GetByIDForVideoBilling(context.Context, int64) (*APIKey, error) {
	return s.key, nil
}

func TestVideoRecoveryCanSettleAfterOriginalAPIKeyIsDeleted(t *testing.T) {
	gateway, _, usageRepo, _, input, pending := videoRecoveryEdgeFixture(t)
	gateway.billingCacheService.apiKeyRateLimitLoader = &deletedVideoAPIKeyLoader{key: input.APIKey}
	require.NoError(t, gateway.recordRecoveredVideoUsage(context.Background(), input.Account, pending, input.Result))
	require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
	require.Equal(t, input.APIKey.ID, usageRepo.lastLog.APIKeyID)
}

func TestLegacySeedanceRecoveryUsesVideoPriceWithoutCompletionTokens(t *testing.T) {
	gateway, cache, usageRepo, _, input, pending := videoRecoveryEdgeFixture(t)
	input.Account.Platform, input.APIKey.Group.Platform = PlatformOpenAI, PlatformOpenAI
	input.Account.Credentials["openai_capabilities"] = []string{"seedance"}
	pending.QuotaPlatform = PlatformOpenAI
	var err error
	cache.payload, err = json.Marshal(pending)
	require.NoError(t, err)
	terminal, err := gateway.recoverVideoBillingKey(context.Background(), cache.key)
	require.NoError(t, err)
	require.True(t, terminal)
	require.NotNil(t, usageRepo.lastLog, "a playable legacy Seedance video with an explicit price must be settled")
	require.InDelta(t, 1.25, usageRepo.lastLog.ActualCost, 1e-12)
}
