package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type accountHealthCacheStub struct {
	snapshots  map[int64]*AccountHealthSnapshot
	records    []AccountHealthEvent
	getErr     error
	acquireErr error
	acquired   bool
	probeTTL   time.Duration
}

func (s *accountHealthCacheStub) Record(_ context.Context, event AccountHealthEvent, _ AccountHealthPolicy) (*AccountHealthSnapshot, error) {
	s.records = append(s.records, event)
	return s.snapshots[event.AccountID], nil
}

func (s *accountHealthCacheStub) GetBatch(_ context.Context, accountIDs []int64) (map[int64]*AccountHealthSnapshot, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	result := make(map[int64]*AccountHealthSnapshot, len(accountIDs))
	for _, accountID := range accountIDs {
		if snapshot := s.snapshots[accountID]; snapshot != nil {
			result[accountID] = snapshot
		}
	}
	return result, nil
}

func (s *accountHealthCacheStub) AcquireProbe(_ context.Context, _ int64, _ int, ttl time.Duration) (string, bool, error) {
	s.probeTTL = ttl
	if s.acquireErr != nil {
		return "", false, s.acquireErr
	}
	return "probe", s.acquired, nil
}

func (s *accountHealthCacheStub) ReleaseProbe(context.Context, int64, string) error { return nil }

func newAccountHealthRateLimitService(cache AccountHealthCache, enforce bool) *RateLimitService {
	cfg := &config.Config{Gateway: config.GatewayConfig{AccountHealth: config.GatewayAccountHealthConfig{
		Enabled:             true,
		EnforcementEnabled:  enforce,
		MinimumSamples:      10,
		DegradedScore:       80,
		OpenScore:           45,
		ConsecutiveFailures: 3,
		BaseCooldownSeconds: 30,
		MaxCooldownSeconds:  600,
		HalfOpenMaxProbes:   1,
		StateTTLSeconds:     86400,
		LocalCacheTTLMS:     500,
	}}}
	svc := NewRateLimitService(nil, nil, cfg, nil, nil)
	svc.SetAccountHealthCache(cache)
	return svc
}

func TestAccountHealthFailureAttribution(t *testing.T) {
	requestScoped := &UpstreamFailoverError{StatusCode: http.StatusBadGateway, Scope: GatewayFailureScopeRequest}
	providerScoped := &UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable, Scope: GatewayFailureScopeProvider}
	rateLimited := &UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}
	clientError := &UpstreamFailoverError{StatusCode: http.StatusBadRequest}
	accountFailure := &UpstreamFailoverError{StatusCode: http.StatusBadGateway, Scope: GatewayFailureScopeAccount}

	require.False(t, shouldPenalizeAccountHealth(context.Background(), requestScoped))
	require.False(t, shouldPenalizeAccountHealth(context.Background(), providerScoped))
	require.False(t, shouldPenalizeAccountHealth(context.Background(), rateLimited))
	require.False(t, shouldPenalizeAccountHealth(context.Background(), clientError))
	require.False(t, shouldPenalizeAccountHealth(context.Background(), context.Canceled))
	require.True(t, shouldPenalizeAccountHealth(context.Background(), accountFailure))
	require.True(t, shouldPenalizeAccountHealth(context.Background(), errors.New("proxy transport failed")))
}

func TestAccountHealthFailureReasonDoesNotPersistRawErrorDetails(t *testing.T) {
	require.Equal(t, "upstream_failure", accountHealthFailureReason(errors.New("request failed for https://secret.example/?token=sensitive")))
	require.Equal(t, "upstream_timeout", accountHealthFailureReason(context.DeadlineExceeded))
	require.Equal(t, "upstream_http_502", accountHealthFailureReason(&UpstreamFailoverError{StatusCode: http.StatusBadGateway}))
}

func TestAnnotateAccountsWithHealthFiltersOnlyOpenCircuit(t *testing.T) {
	cache := &accountHealthCacheStub{snapshots: map[int64]*AccountHealthSnapshot{
		1: {Score: 92, State: AccountHealthStateHealthy},
		2: {Score: 30, State: AccountHealthStateOpen, OpenUntilUnix: time.Now().Add(time.Minute).Unix()},
		3: {Score: 55, State: AccountHealthStateDegraded},
	}}
	svc := newAccountHealthRateLimitService(cache, true)
	accounts := svc.annotateAccountsWithHealth(context.Background(), []Account{{ID: 1}, {ID: 2}, {ID: 3}})

	require.Len(t, accounts, 2)
	require.Equal(t, []int64{1, 3}, []int64{accounts[0].ID, accounts[1].ID})
	require.Equal(t, 92.0, accountSchedulingHealthScore(&accounts[0]))
	require.Equal(t, 55.0, accountSchedulingHealthScore(&accounts[1]))
}

func TestAnnotateAccountsWithHealthFailsOpenOnCacheError(t *testing.T) {
	cache := &accountHealthCacheStub{getErr: errors.New("redis unavailable")}
	svc := newAccountHealthRateLimitService(cache, true)
	accounts := svc.annotateAccountsWithHealth(context.Background(), []Account{{ID: 1}, {ID: 2}})
	require.Len(t, accounts, 2)
	require.Equal(t, 100.0, accountSchedulingHealthScore(&accounts[0]))
}

func TestFilterByMaxHealthStaysInsidePriorityTier(t *testing.T) {
	highPriorityLowHealth := &Account{ID: 1, Priority: 1}
	highPriorityLowHealth.setSchedulingHealth(&AccountHealthSnapshot{Score: 60, State: AccountHealthStateDegraded}, true)
	highPriorityHighHealth := &Account{ID: 2, Priority: 1}
	highPriorityHighHealth.setSchedulingHealth(&AccountHealthSnapshot{Score: 95, State: AccountHealthStateHealthy}, true)
	lowerPriorityPerfectHealth := &Account{ID: 3, Priority: 2}
	lowerPriorityPerfectHealth.setSchedulingHealth(&AccountHealthSnapshot{Score: 100, State: AccountHealthStateHealthy}, true)

	priorityTier := filterByMinPriority([]accountWithLoad{
		{account: highPriorityLowHealth, loadInfo: &AccountLoadInfo{}},
		{account: lowerPriorityPerfectHealth, loadInfo: &AccountLoadInfo{}},
		{account: highPriorityHighHealth, loadInfo: &AccountLoadInfo{}},
	})
	healthTier := filterByMaxHealth(priorityTier)
	require.Len(t, healthTier, 1)
	require.Equal(t, int64(2), healthTier[0].account.ID)
}

func TestHalfOpenProbeFailureFailsOpenButBusyProbeDoesNot(t *testing.T) {
	account := &Account{ID: 9}
	account.setSchedulingHealth(&AccountHealthSnapshot{Score: 40, State: AccountHealthStateHalfOpen}, true)

	cache := &accountHealthCacheStub{acquireErr: errors.New("redis unavailable")}
	svc := newAccountHealthRateLimitService(cache, true)
	_, allowed := svc.acquireAccountHealthProbe(context.Background(), account)
	require.True(t, allowed)

	cache.acquireErr = nil
	cache.acquired = false
	_, allowed = svc.acquireAccountHealthProbe(context.Background(), account)
	require.False(t, allowed)
}

func TestHalfOpenProbeLeaseMatchesLongerConcurrencySlotTTL(t *testing.T) {
	account := &Account{ID: 9}
	account.setSchedulingHealth(&AccountHealthSnapshot{Score: 40, State: AccountHealthStateHalfOpen}, true)
	cache := &accountHealthCacheStub{acquired: true}
	svc := newAccountHealthRateLimitService(cache, true)
	svc.cfg.Gateway.ConcurrencySlotTTLMinutes = 30

	_, allowed := svc.acquireAccountHealthProbe(context.Background(), account)
	require.True(t, allowed)
	require.Equal(t, 30*time.Minute, cache.probeTTL)
}

func TestAccountHealthPolicyKeepsMaxCooldownAtLeastBase(t *testing.T) {
	cfg := &config.Config{Gateway: config.GatewayConfig{AccountHealth: config.GatewayAccountHealthConfig{
		Enabled: true, BaseCooldownSeconds: 3600, MaxCooldownSeconds: 1,
	}}}
	policy, enabled, _ := accountHealthPolicyFromConfig(cfg)
	require.True(t, enabled)
	require.GreaterOrEqual(t, policy.MaxCooldown, policy.BaseCooldown)
}

func TestGatewayAccountSlotReleaseWithoutHealthService(t *testing.T) {
	result, err := (&GatewayService{}).tryAcquireAccountSlot(context.Background(), &Account{ID: 1})
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
	require.NotPanics(t, result.ReleaseFunc)
}

func TestOpenAIAccountSlotReleaseWithoutHealthService(t *testing.T) {
	result, err := (&OpenAIGatewayService{}).tryAcquireAccountSlot(context.Background(), &Account{ID: 1})
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
	require.NotPanics(t, result.ReleaseFunc)
}

func TestObserveOpenAIAccountHealthFailureRecordsUnifiedHealth(t *testing.T) {
	cache := &accountHealthCacheStub{snapshots: map[int64]*AccountHealthSnapshot{
		12: {Score: 80, State: AccountHealthStateDegraded},
	}}
	rateLimitService := newAccountHealthRateLimitService(cache, true)
	gateway := &OpenAIGatewayService{rateLimitService: rateLimitService}
	observedErr := &UpstreamFailoverError{StatusCode: http.StatusBadGateway, Scope: GatewayFailureScopeAccount}

	require.False(t, gateway.ObserveOpenAIAccountHealthFailure(context.Background(), &Account{ID: 12}, observedErr))
	require.Equal(t, []AccountHealthEvent{{
		AccountID:     12,
		FailureReason: "upstream_http_502",
	}}, cache.records)
}
