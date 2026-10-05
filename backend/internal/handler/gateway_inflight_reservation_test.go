package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestMaxOutputTokens(t *testing.T) {
	require.Equal(t, 1024, requestMaxOutputTokens([]byte(`{"max_tokens":1024}`)))
	require.Equal(t, 2048, requestMaxOutputTokens([]byte(`{"max_completion_tokens":2048}`)))
	require.Equal(t, 4096, requestMaxOutputTokens([]byte(`{"max_output_tokens":4096}`)))
	require.Equal(t, 512, requestMaxOutputTokens([]byte(`{"generationConfig":{"maxOutputTokens":512}}`)))
	require.Equal(t, 0, requestMaxOutputTokens([]byte(`{"max_tokens":"x"}`)))
	require.Equal(t, 0, requestMaxOutputTokens([]byte(`{}`)))
}

type handlerTenantBudgetRepo struct {
	mu                 sync.Mutex
	reserved, released int
	attribution        service.BudgetAttribution
	err                error
	rejectAfter        int
	ids                []string
}

func (r *handlerTenantBudgetRepo) Reserve(_ context.Context, a service.BudgetAttribution, id string, cost float64) (*service.BudgetReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reserved++
	r.attribution = a
	r.ids = append(r.ids, id)
	if r.rejectAfter > 0 && r.reserved > r.rejectAfter {
		return nil, service.ErrProjectBudgetExceeded
	}
	if r.err != nil {
		return nil, r.err
	}
	return &service.BudgetReservation{ID: id, RequestID: id, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, ActorUserID: a.ActorUserID, APIKeyID: a.APIKeyID, Estimate: cost, Status: "pending"}, nil
}
func (r *handlerTenantBudgetRepo) Finalize(context.Context, string, float64) error { return nil }
func (r *handlerTenantBudgetRepo) Release(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.released++
	return nil
}
func tenantBudgetTestKey() *service.APIKey {
	return &service.APIKey{ID: 5, UserID: 4, User: &service.User{ID: 4}, Tenant: &service.TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}}
}

func TestTenantBudgetAdmissionIndependentOfWallet(t *testing.T) {
	for _, subscription := range []*service.UserSubscription{nil, {ID: 7}} {
		r := &handlerTenantBudgetRepo{}
		ctx := service.WithBudgetService(context.Background(), service.NewBudgetService(r))
		key := tenantBudgetTestKey()
		key.Group = &service.Group{SubscriptionType: service.SubscriptionTypeSubscription}
		ctx, done, err := reserveInflightBalanceCtx(ctx, nil, &countingEstimator{cost: 0.5, priced: true}, key, subscription, tokenInflightEstimate("m", nil))
		require.NoError(t, err)
		require.Equal(t, 1, r.reserved)
		require.Equal(t, int64(3), r.attribution.BillingPrincipalUserID)
		require.Empty(t, key.Tenant.BudgetReservationID, "admission must not mutate the shared API-key tenant snapshot")
		require.NotNil(t, service.BudgetReservationFromContext(ctx))
		require.NotEmpty(t, service.BudgetReservationIDFromContext(ctx))
		done()
		done()
		require.Equal(t, 1, r.released)
	}
}

func TestTenantBudgetAdmissionFailsClosed(t *testing.T) {
	_, _, err := reserveInflightBalanceCtx(context.Background(), nil, &countingEstimator{cost: 1, priced: true}, tenantBudgetTestKey(), nil, tokenInflightEstimate("m", nil))
	require.ErrorIs(t, err, service.ErrBudgetUnavailable)
	r := &handlerTenantBudgetRepo{err: service.ErrProjectBudgetExceeded}
	ctx := service.WithBudgetService(context.Background(), service.NewBudgetService(r))
	_, _, err = reserveInflightBalanceCtx(ctx, nil, &countingEstimator{cost: 1, priced: true}, tenantBudgetTestKey(), nil, tokenInflightEstimate("m", nil))
	require.ErrorIs(t, err, service.ErrProjectBudgetExceeded)
	_, _, err = reserveInflightBalanceCtx(ctx, nil, &countingEstimator{priced: false}, tenantBudgetTestKey(), nil, tokenInflightEstimate("m", nil))
	require.ErrorIs(t, err, service.ErrBudgetUnpriced)
	require.Equal(t, 1, r.reserved, "unknown prices must not reach storage")
}

func TestTenantBudgetWorkerKeepsReservationAndContext(t *testing.T) {
	r := &handlerTenantBudgetRepo{}
	ctx := service.WithBudgetService(context.Background(), service.NewBudgetService(r))
	ctx, done, err := reserveInflightBalanceCtx(ctx, nil, &countingEstimator{cost: 1, priced: true}, tenantBudgetTestKey(), nil, tokenInflightEstimate("m", nil))
	require.NoError(t, err)
	task, abandon := wrapUsageRecordTaskContext(ctx, func(worker context.Context) {
		h := service.BudgetReservationFromContext(worker)
		require.NotNil(t, h)
		require.NotNil(t, service.BudgetServiceFromContext(worker))
		h.MarkSettled()
	})
	done()
	require.Zero(t, r.released, "worker owns the reservation after HTTP handler returns")
	task(context.Background())
	abandon()
	require.Zero(t, r.released, "a committed reservation cannot be refunded by cleanup")
}

func TestTenantBudgetErrorsHaveLocalCodes(t *testing.T) {
	for _, err := range []error{service.ErrProjectBudgetExceeded, service.ErrWorkspaceBudgetExceeded} {
		status, code, _, _ := billingErrorDetails(err)
		require.Equal(t, http.StatusTooManyRequests, status)
		require.Equal(t, err.Error(), code)
	}
}

type countingEstimator struct {
	calls  int
	cost   float64
	priced bool
}

func (e *countingEstimator) EstimateInflightReservation(context.Context, *service.APIKey, service.InflightEstimateRequest) (float64, bool) {
	e.calls++
	return e.cost, e.priced
}

func newInflightTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c
}

func TestReserveInflightBalance_SkipsWhenDisabledOrSubscription(t *testing.T) {
	cfg := &config.Config{}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	est := &countingEstimator{cost: 1, priced: true}
	apiKey := &service.APIKey{User: &service.User{ID: 1}}

	done, err := reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("m", []byte(`{}`)))
	require.NoError(t, err)
	done()
	require.Equal(t, 0, est.calls, "disabled switch must not even estimate")

	cfg.Billing.InflightReservation.Enabled = true
	apiKey.Group = &service.Group{SubscriptionType: service.SubscriptionTypeSubscription}
	done, err = reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, &service.UserSubscription{}, tokenInflightEstimate("m", []byte(`{}`)))
	require.NoError(t, err)
	done()
	require.Equal(t, 0, est.calls, "subscription mode must be unaffected")
}

func TestReserveInflightBalance_UnpricedFailOpenByDefaultFailClosedOptIn(t *testing.T) {
	cache := newHandlerInflightCache(10)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	apiKey := &service.APIKey{User: &service.User{ID: 1}}
	est := &countingEstimator{priced: false}

	done, err := reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("unknown", nil))
	require.NoError(t, err)
	done()

	cfg.Billing.InflightReservation.FailClosedOnUnpriced = true
	_, err = reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("unknown", nil))
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
}

// handlerInflightCache 内存版余额缓存 + 在途预留（语义同 Redis Lua）。
type handlerInflightCache struct {
	service.BillingCache
	mu      sync.Mutex
	balance float64
	res     map[string]float64
}

func newHandlerInflightCache(balance float64) *handlerInflightCache {
	return &handlerInflightCache{balance: balance, res: map[string]float64{}}
}

func (m *handlerInflightCache) GetUserBalance(context.Context, int64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balance, nil
}

func (m *handlerInflightCache) GetUserPlatformQuotaCache(context.Context, int64, string) (*service.UserPlatformQuotaCacheEntry, bool, error) {
	return nil, false, nil
}

func (m *handlerInflightCache) ReserveInflightBalance(_ context.Context, _ int64, id string, amount, balance float64, _ time.Duration) (bool, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sum := 0.0
	for _, v := range m.res {
		sum += v
	}
	if len(m.res) > 0 && balance-sum < amount {
		return false, sum, nil
	}
	m.res[id] = amount
	return true, sum, nil
}

func (m *handlerInflightCache) ReleaseInflightBalance(_ context.Context, _ int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.res, id)
	return nil
}

func (m *handlerInflightCache) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.res)
}

func TestWrapUsageRecordTaskContext_HandsReservationToBillingTask(t *testing.T) {
	cache := newHandlerInflightCache(1)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	apiKey := &service.APIKey{User: &service.User{ID: 5}}

	c := newInflightTestGinContext()
	done, err := reserveInflightBalance(c, billing, &countingEstimator{cost: 0.9, priced: true}, apiKey, nil, tokenInflightEstimate("m", nil))
	require.NoError(t, err)
	require.Equal(t, 1, cache.count())

	ran := false
	task, abandon := wrapUsageRecordTaskContext(c.Request.Context(), func(context.Context) { ran = true })
	done() // handler returns; billing still pending
	require.Equal(t, 1, cache.count(), "reservation held until the billing task finishes")
	task(context.Background())
	require.True(t, ran)
	require.Equal(t, 0, cache.count())
	abandon() // idempotent with the task's own done

	// Dropped task: the submitter abandons it and the reservation is released.
	c2 := newInflightTestGinContext()
	done2, err := reserveInflightBalance(c2, billing, &countingEstimator{cost: 0.9, priced: true}, apiKey, nil, tokenInflightEstimate("m", nil))
	require.NoError(t, err)
	_, abandon2 := wrapUsageRecordTaskContext(c2.Request.Context(), func(context.Context) {})
	done2()
	require.Equal(t, 1, cache.count())
	abandon2()
	require.Equal(t, 0, cache.count())
}

// 新接入的端点（独立 web_search）：在途预留超过余额时拒绝，且不残留预留。
func TestWebSearch_RejectsWhenInflightExceedsBalance(t *testing.T) {
	cache := newHandlerInflightCache(1.5)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billingCache := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	billing := service.NewBillingService(cfg, nil)
	gw := service.NewGatewayService(
		nil, nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, billing, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, service.NewModelPricingResolver(nil, billing), nil, nil, nil,
	)
	h := &GatewayHandler{gatewayService: gw, billingCacheService: billingCache}

	groupID := int64(3)
	perK := 1000.0 // $1 per search
	apiKey := &service.APIKey{
		ID: 9, User: &service.User{ID: 42, Balance: 1.5}, GroupID: &groupID,
		Group: &service.Group{ID: groupID, Platform: service.PlatformGrok, RateMultiplier: 1, SearchPricePer1k: &perK},
	}

	// Another in-flight request of this user already holds $1.
	held, err := billingCache.ReserveInflight(context.Background(), apiKey.User, apiKey.Group, nil, 1.0)
	require.NoError(t, err)
	defer held.HandlerDone()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/web_search", bytes.NewBufferString(`{"query":"sub2api"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)

	h.WebSearch(c)

	require.NotEqual(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "balance")
	require.Equal(t, 1, cache.count(), "rejected request must not leave a reservation behind")
}
