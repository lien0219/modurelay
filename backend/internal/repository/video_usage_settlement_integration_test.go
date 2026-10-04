//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func videoSettlementFixture(t *testing.T) (service.VideoUsageBillingRepository, *service.UsageBillingCommand, *service.UsageLog) {
	t.Helper()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: uuid.NewString() + "@video-settlement.test", Balance: 100})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-video-settlement-" + uuid.NewString(), Name: "video", Quota: 100, RateLimit5h: 100})
	account := mustCreateAccount(t, client, &service.Account{Name: "video-settlement-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformSeedance})
	requestID := "grok-video:seedance:" + uuid.NewString()
	cmd := &service.UsageBillingCommand{
		RequestID: requestID, APIKeyID: key.ID, UserID: user.ID, AccountID: account.ID,
		AccountType: service.AccountTypeAPIKey, Model: "vendor-video", BalanceCost: 1.25, APIKeyQuotaCost: 1.25, APIKeyRateLimitCost: 1.25,
	}
	resolution, duration, mode := "720p", 5, "video"
	log := &service.UsageLog{
		UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: requestID, Model: "vendor-video",
		VideoCount: 1, VideoResolution: &resolution, VideoDurationSeconds: &duration, BillingMode: &mode,
		ActualCost: 1.25, TotalCost: 1.25, RateMultiplier: 1, CreatedAt: time.Now().UTC(),
	}
	repo, ok := NewUsageBillingRepository(client, integrationDB).(service.VideoUsageBillingRepository)
	require.True(t, ok, "video billing must expose atomic settlement")
	return repo, cmd, log
}

func assertVideoSettlementTotals(t *testing.T, cmd *service.UsageBillingCommand, balance, quota, window float64, rows int) {
	t.Helper()
	ctx := context.Background()
	var gotBalance, gotQuota, gotWindow float64
	var gotRows int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", cmd.UserID).Scan(&gotBalance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used, usage_5h FROM api_keys WHERE id = $1", cmd.APIKeyID).Scan(&gotQuota, &gotWindow))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", cmd.RequestID, cmd.APIKeyID).Scan(&gotRows))
	require.Equal(t, balance, gotBalance)
	require.Equal(t, quota, gotQuota)
	require.Equal(t, window, gotWindow)
	require.Equal(t, rows, gotRows)
}

func TestVideoSettlementRollsBackBillingWhenUsageInsertFails(t *testing.T) {
	repo, cmd, log := videoSettlementFixture(t)
	ctx := context.Background()
	invalid := strings.Repeat("x", 100)
	log.VideoResolution = &invalid
	_, err := repo.ApplyVideoUsage(ctx, cmd, log)
	require.Error(t, err)
	assertVideoSettlementTotals(t, cmd, 100, 0, 0, 0)
	var dedup int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1 AND api_key_id = $2", cmd.RequestID, cmd.APIKeyID).Scan(&dedup))
	require.Zero(t, dedup)
	resolution := "720p"
	log.VideoResolution = &resolution
	result, err := repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.True(t, result.VideoUsageLogPersisted)
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
	result, err = repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.True(t, result.VideoUsageLogPersisted)
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
	settled, err := repo.IsVideoUsageSettled(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, cmd.AccountID, false)
	require.NoError(t, err)
	require.True(t, settled)
	settled, err = repo.IsVideoUsageSettled(ctx, cmd.RequestID, cmd.UserID+1, cmd.APIKeyID, cmd.AccountID, false)
	require.NoError(t, err)
	require.False(t, settled, "another owner must never inherit the settled task")
}

func TestVideoSettlementAfterAPIKeyDeletionKeepsFinancialIdentity(t *testing.T) {
	repo, cmd, log := videoSettlementFixture(t)
	ctx := context.Background()
	keyRepo := NewAPIKeyRepository(testEntClient(t), integrationDB)
	key, err := keyRepo.GetByID(ctx, cmd.APIKeyID)
	require.NoError(t, err)
	credential := key.Key
	require.NoError(t, keyRepo.DeleteWithAudit(ctx, cmd.APIKeyID))
	_, err = keyRepo.GetByID(ctx, cmd.APIKeyID)
	require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
	_, err = keyRepo.GetByKey(ctx, credential)
	require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
	loader, ok := keyRepo.(interface {
		GetByIDForVideoBilling(context.Context, int64) (*service.APIKey, error)
	})
	require.True(t, ok)
	billingKey, err := loader.GetByIDForVideoBilling(ctx, cmd.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, cmd.UserID, billingKey.UserID)
	require.NotEqual(t, credential, billingKey.Key)
	_, err = repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
	// Normal requests still reject the deleted key and roll back their balance.
	ordinary := *cmd
	ordinary.RequestID = "ordinary:" + uuid.NewString()
	ordinary.RequestFingerprint = ""
	_, err = NewUsageBillingRepository(testEntClient(t), integrationDB).Apply(ctx, &ordinary)
	require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
}

func TestConcurrentVideoSettlementChargesAndLogsOnce(t *testing.T) {
	repo, cmd, log := videoSettlementFixture(t)
	var workers sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			commandCopy, logCopy := *cmd, *log
			_, err := repo.ApplyVideoUsage(context.Background(), &commandCopy, &logCopy)
			errs <- err
		}()
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
}

func TestVideoSettlementRepairsHistoricalUnbilledPlaceholder(t *testing.T) {
	repo, cmd, log := videoSettlementFixture(t)
	ctx := context.Background()
	placeholder := *log
	placeholder.ActualCost = 0
	_, err := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB).Create(ctx, &placeholder)
	require.NoError(t, err)
	_, err = repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
	var recordedCost float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT actual_cost FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", cmd.RequestID, cmd.APIKeyID).Scan(&recordedCost))
	require.Equal(t, 1.25, recordedCost, "the charged amount must replace the old failed zero-cost placeholder")
}

func TestVideoSettlementRepairsHistoricalPaidPlaceholderWithoutChargingAgain(t *testing.T) {
	repo, cmd, log := videoSettlementFixture(t)
	ctx := context.Background()
	_, err := NewUsageBillingRepository(testEntClient(t), integrationDB).Apply(ctx, cmd)
	require.NoError(t, err)
	placeholder := *log
	placeholder.ActualCost = 0
	_, err = newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB).Create(ctx, &placeholder)
	require.NoError(t, err)
	settled, err := repo.IsVideoUsageSettled(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, cmd.AccountID, false)
	require.NoError(t, err)
	require.False(t, settled, "a paid dedup receipt must not hide an inaccurate usage placeholder")
	result, err := repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.False(t, result.Applied)
	assertVideoSettlementTotals(t, cmd, 98.75, 1.25, 1.25, 1)
	var recordedCost float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT actual_cost FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", cmd.RequestID, cmd.APIKeyID).Scan(&recordedCost))
	require.Equal(t, 1.25, recordedCost)
	settled, err = repo.IsVideoUsageSettled(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, cmd.AccountID, false)
	require.NoError(t, err)
	require.True(t, settled)
}

func TestVideoSettlementRecognizesFreeAndLogOnlyUsage(t *testing.T) {
	for _, logOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "free", true: "simple"}[logOnly], func(t *testing.T) {
			repo, cmd, log := videoSettlementFixture(t)
			ctx := context.Background()
			log.ActualCost = 0
			if !logOnly {
				log.RateMultiplier = 0
				cmd.BalanceCost, cmd.APIKeyQuotaCost, cmd.APIKeyRateLimitCost = 0, 0, 0
				_, err := repo.ApplyVideoUsage(ctx, cmd, log)
				require.NoError(t, err)
			} else {
				_, err := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB).Create(ctx, log)
				require.NoError(t, err)
			}
			settled, err := repo.IsVideoUsageSettled(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, cmd.AccountID, logOnly)
			require.NoError(t, err)
			require.True(t, settled, "intentional free/log-only usage must remain settled")
			assertVideoSettlementTotals(t, cmd, 100, 0, 0, 1)
		})
	}
}

func TestVideoSettlementRechargeReplaysFrozenCostAcrossGatewayRestart(t *testing.T) {
	_, cmd, _ := videoSettlementFixture(t)
	ctx := context.Background()
	client := testEntClient(t)
	keyRepo := NewAPIKeyRepository(client, integrationDB)
	key, err := keyRepo.GetByID(ctx, cmd.APIKeyID)
	require.NoError(t, err)
	userRepo := NewUserRepository(client, integrationDB)
	user, err := userRepo.GetByID(ctx, cmd.UserID)
	require.NoError(t, err)
	accountRepo := NewAccountRepository(client, integrationDB, nil)
	account, err := accountRepo.GetByID(ctx, cmd.AccountID)
	require.NoError(t, err)
	group := mustCreateGroup(t, client, &service.Group{
		Name: "video-recharge-" + uuid.NewString(), Platform: service.PlatformSeedance, RateMultiplier: 1,
	})
	group.VideoModelPrices = map[string]map[string]float64{"vendor-video": {"720p": 0.25}}
	key.GroupID, key.Group, key.User = &group.ID, group, user
	rdb := testRedis(t)
	cache := NewGatewayCache(rdb).(*gatewayCache)
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	newGateway := func() *service.OpenAIGatewayService {
		return service.NewOpenAIGatewayService(accountRepo, newUsageLogRepositoryWithSQL(client, integrationDB),
			NewUsageBillingRepository(client, integrationDB), userRepo, NewUserSubscriptionRepository(client), nil,
			cache, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil,
			&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	}
	taskID := strings.TrimPrefix(cmd.RequestID, "grok-video:")
	input := &service.OpenAIRecordUsageInput{
		Result: &service.OpenAIForwardResult{RequestID: taskID, Model: "vendor-video", BillingModel: "vendor-video",
			VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 5},
		APIKey: key, User: user, Account: account, QuotaPlatform: service.PlatformSeedance,
		VideoTaskID: taskID, PricingAt: time.Now().UTC(),
	}
	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET balance = 0 WHERE id = $1", user.ID)
	require.NoError(t, err)
	gateway := newGateway()
	require.ErrorIs(t, gateway.RecordUsage(ctx, input), service.ErrInsufficientBalance)
	assertVideoSettlementTotals(t, cmd, 0, 0, 0, 0)
	cacheKey := fmt.Sprintf("%d:%d:%s", user.ID, key.ID, taskID)
	frozen, err := cache.GetGrokVideoPendingBilling(ctx, cacheKey)
	require.NoError(t, err)
	require.NotEmpty(t, frozen)
	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET balance = 10 WHERE id = $1", user.ID)
	require.NoError(t, err)
	group.VideoModelPrices["vendor-video"]["720p"], group.RateMultiplier = 9, 7
	input.Result.Model, input.Result.BillingModel = "changed-model", "changed-model"
	// A fresh gateway instance simulates a process restart. Pricing and models
	// have changed, but the original 1.25 money event must be replayed exactly.
	require.NoError(t, newGateway().RecordUsage(ctx, input))
	assertVideoSettlementTotals(t, cmd, 8.75, 1.25, 1.25, 1)
	var recordedCost float64
	var recordedModel string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT actual_cost, model FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", cmd.RequestID, cmd.APIKeyID).Scan(&recordedCost, &recordedModel))
	require.Equal(t, 1.25, recordedCost)
	require.Equal(t, "vendor-video", recordedModel)
	require.NoError(t, rdb.Del(ctx, grokVideoPendingBillingPrefix+cacheKey).Err())
	// Redis state is now absent: the durable database receipt must prevent a
	// new charge or any attempt to look up the changed model's price.
	require.NoError(t, newGateway().RecordUsage(ctx, input))
	assertVideoSettlementTotals(t, cmd, 8.75, 1.25, 1.25, 1)
}

func TestVideoSettlementRepairsHistoricalPlaceholderWithFreePrice(t *testing.T) {
	repo, cmd, log := videoSettlementFixture(t)
	ctx := context.Background()
	placeholder := *log
	placeholder.ActualCost = 0
	_, err := newUsageLogRepositoryWithSQL(testEntClient(t), integrationDB).Create(ctx, &placeholder)
	require.NoError(t, err)
	log.ActualCost, log.RateMultiplier = 0, 0
	cmd.BalanceCost, cmd.APIKeyQuotaCost, cmd.APIKeyRateLimitCost = 0, 0, 0
	_, err = repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	settled, err := repo.IsVideoUsageSettled(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, cmd.AccountID, false)
	require.NoError(t, err)
	require.True(t, settled, "a free settlement must repair the inaccurate placeholder too")
	assertVideoSettlementTotals(t, cmd, 100, 0, 0, 1)
	var recordedMultiplier float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT rate_multiplier FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", cmd.RequestID, cmd.APIKeyID).Scan(&recordedMultiplier))
	require.Zero(t, recordedMultiplier)
}

func TestVideoLogOnlySettlementKeepsReceiptAfterCacheLossAndModeChange(t *testing.T) {
	_, cmd, _ := videoSettlementFixture(t)
	ctx := context.Background()
	client := testEntClient(t)
	key, err := NewAPIKeyRepository(client, integrationDB).GetByID(ctx, cmd.APIKeyID)
	require.NoError(t, err)
	userRepo := NewUserRepository(client, integrationDB)
	user, err := userRepo.GetByID(ctx, cmd.UserID)
	require.NoError(t, err)
	accountRepo := NewAccountRepository(client, integrationDB, nil)
	account, err := accountRepo.GetByID(ctx, cmd.AccountID)
	require.NoError(t, err)
	group := mustCreateGroup(t, client, &service.Group{Name: "video-simple-" + uuid.NewString(), Platform: service.PlatformSeedance, RateMultiplier: 1})
	group.VideoModelPrices = map[string]map[string]float64{"vendor-video": {"720p": 0.25}}
	key.GroupID, key.Group, key.User = &group.ID, group, user
	// No key windows in this fixture: simple mode only records usage.
	key.RateLimit5h = 0
	rdb := testRedis(t)
	cache := NewGatewayCache(rdb).(*gatewayCache)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	gateway := service.NewOpenAIGatewayService(accountRepo, newUsageLogRepositoryWithSQL(client, integrationDB),
		NewUsageBillingRepository(client, integrationDB), userRepo, NewUserSubscriptionRepository(client), nil,
		cache, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	taskID := strings.TrimPrefix(cmd.RequestID, "grok-video:")
	input := &service.OpenAIRecordUsageInput{
		Result: &service.OpenAIForwardResult{RequestID: taskID, Model: "vendor-video", BillingModel: "vendor-video",
			VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 5},
		APIKey: key, User: user, Account: account, QuotaPlatform: service.PlatformSeedance, VideoTaskID: taskID,
	}
	require.NoError(t, gateway.RecordUsage(ctx, input))
	assertVideoSettlementTotals(t, cmd, 100, 0, 0, 1)
	cacheKey := fmt.Sprintf("%d:%d:%s", user.ID, key.ID, taskID)
	require.NoError(t, rdb.Del(ctx, grokVideoPendingBillingPrefix+cacheKey).Err())
	cfg.RunMode = config.RunModeStandard
	require.NoError(t, gateway.RecordUsage(ctx, input))
	assertVideoSettlementTotals(t, cmd, 100, 0, 0, 1)
}
