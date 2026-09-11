package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type AccountHealthState string

const (
	AccountHealthStateWarming  AccountHealthState = "warming"
	AccountHealthStateHealthy  AccountHealthState = "healthy"
	AccountHealthStateDegraded AccountHealthState = "degraded"
	AccountHealthStateOpen     AccountHealthState = "open"
	AccountHealthStateHalfOpen AccountHealthState = "half_open"
)

type AccountHealthEvent struct {
	AccountID     int64
	Success       bool
	LatencyMs     int64
	FailureReason string
}

type AccountHealthSnapshot struct {
	Score               float64            `json:"score"`
	State               AccountHealthState `json:"state"`
	SampleCount         int64              `json:"sample_count"`
	ErrorRateEWMA       float64            `json:"error_rate_ewma"`
	LatencyEWMAMs       float64            `json:"latency_ewma_ms"`
	ConsecutiveFailures int                `json:"consecutive_failures"`
	OpenCount           int                `json:"open_count"`
	OpenUntilUnix       int64              `json:"open_until_unix,omitempty"`
	LastFailureReason   string             `json:"last_failure_reason,omitempty"`
	UpdatedAtUnix       int64              `json:"updated_at_unix"`
}

type AccountHealthPolicy struct {
	MinimumSamples      int
	DegradedScore       float64
	OpenScore           float64
	ConsecutiveFailures int
	BaseCooldown        time.Duration
	MaxCooldown         time.Duration
	HalfOpenMaxProbes   int
	StateTTL            time.Duration
	LocalCacheTTL       time.Duration
}

type AccountHealthCache interface {
	Record(ctx context.Context, event AccountHealthEvent, policy AccountHealthPolicy) (*AccountHealthSnapshot, error)
	GetBatch(ctx context.Context, accountIDs []int64) (map[int64]*AccountHealthSnapshot, error)
	AcquireProbe(ctx context.Context, accountID int64, limit int, ttl time.Duration) (token string, acquired bool, err error)
	ReleaseProbe(ctx context.Context, accountID int64, token string) error
}

type accountHealthLocalEntry struct {
	snapshot  *AccountHealthSnapshot
	expiresAt time.Time
}

func accountHealthPolicyFromConfig(cfg *config.Config) (AccountHealthPolicy, bool, bool) {
	if cfg == nil || !cfg.Gateway.AccountHealth.Enabled {
		return AccountHealthPolicy{}, false, false
	}
	health := cfg.Gateway.AccountHealth
	policy := AccountHealthPolicy{
		MinimumSamples:      health.MinimumSamples,
		DegradedScore:       health.DegradedScore,
		OpenScore:           health.OpenScore,
		ConsecutiveFailures: health.ConsecutiveFailures,
		BaseCooldown:        time.Duration(health.BaseCooldownSeconds) * time.Second,
		MaxCooldown:         time.Duration(health.MaxCooldownSeconds) * time.Second,
		HalfOpenMaxProbes:   health.HalfOpenMaxProbes,
		StateTTL:            time.Duration(health.StateTTLSeconds) * time.Second,
		LocalCacheTTL:       time.Duration(health.LocalCacheTTLMS) * time.Millisecond,
	}
	if policy.MinimumSamples < 1 {
		policy.MinimumSamples = 10
	}
	if policy.DegradedScore <= 0 || policy.DegradedScore > 100 {
		policy.DegradedScore = 80
	}
	if policy.OpenScore <= 0 || policy.OpenScore >= policy.DegradedScore {
		policy.OpenScore = 45
	}
	if policy.ConsecutiveFailures < 1 {
		policy.ConsecutiveFailures = 3
	}
	if policy.BaseCooldown <= 0 {
		policy.BaseCooldown = 30 * time.Second
	}
	if policy.MaxCooldown < policy.BaseCooldown {
		policy.MaxCooldown = policy.BaseCooldown
		if policy.MaxCooldown < 10*time.Minute {
			policy.MaxCooldown = 10 * time.Minute
		}
	}
	if policy.HalfOpenMaxProbes < 1 {
		policy.HalfOpenMaxProbes = 1
	}
	if policy.StateTTL <= 0 {
		policy.StateTTL = 24 * time.Hour
	}
	if policy.LocalCacheTTL <= 0 {
		policy.LocalCacheTTL = 500 * time.Millisecond
	}
	return policy, true, health.EnforcementEnabled
}

func defaultAccountHealthSnapshot() *AccountHealthSnapshot {
	return &AccountHealthSnapshot{Score: 100, State: AccountHealthStateWarming}
}

func normalizeAccountHealthSnapshot(snapshot *AccountHealthSnapshot, now time.Time) *AccountHealthSnapshot {
	if snapshot == nil {
		return defaultAccountHealthSnapshot()
	}
	copy := *snapshot
	copy.Score = math.Max(0, math.Min(100, copy.Score))
	if copy.State == AccountHealthStateOpen && copy.OpenUntilUnix > 0 && now.Unix() >= copy.OpenUntilUnix {
		copy.State = AccountHealthStateHalfOpen
	}
	if copy.State == "" {
		copy.State = AccountHealthStateWarming
	}
	return &copy
}

func cloneAccountHealthSnapshot(snapshot *AccountHealthSnapshot) *AccountHealthSnapshot {
	if snapshot == nil {
		return nil
	}
	copy := *snapshot
	return &copy
}

func (s *RateLimitService) SetAccountHealthCache(cache AccountHealthCache) {
	if s == nil {
		return
	}
	s.accountHealthCache = cache
}

func (s *RateLimitService) accountHealthPolicy() (AccountHealthPolicy, bool, bool) {
	if s == nil || s.accountHealthCache == nil {
		return AccountHealthPolicy{}, false, false
	}
	return accountHealthPolicyFromConfig(s.cfg)
}

func (s *RateLimitService) GetAccountHealthSnapshots(ctx context.Context, accountIDs []int64) map[int64]*AccountHealthSnapshot {
	policy, enabled, _ := s.accountHealthPolicy()
	if !enabled || len(accountIDs) == 0 {
		return nil
	}

	now := time.Now()
	result := make(map[int64]*AccountHealthSnapshot, len(accountIDs))
	missing := make([]int64, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		if _, ok := seen[accountID]; ok {
			continue
		}
		seen[accountID] = struct{}{}
		if cached, ok := s.accountHealthLocal.Load(accountID); ok {
			entry, valid := cached.(accountHealthLocalEntry)
			if valid && now.Before(entry.expiresAt) {
				result[accountID] = normalizeAccountHealthSnapshot(entry.snapshot, now)
				continue
			}
			s.accountHealthLocal.Delete(accountID)
		}
		missing = append(missing, accountID)
	}

	if len(missing) > 0 {
		fetched, err := s.accountHealthCache.GetBatch(ctx, missing)
		if err != nil {
			slog.Warn("account_health_batch_read_failed", "count", len(missing), "error", err)
		} else {
			for _, accountID := range missing {
				snapshot := normalizeAccountHealthSnapshot(fetched[accountID], now)
				result[accountID] = snapshot
				s.accountHealthLocal.Store(accountID, accountHealthLocalEntry{
					snapshot:  cloneAccountHealthSnapshot(snapshot),
					expiresAt: now.Add(policy.LocalCacheTTL),
				})
			}
		}
	}

	return result
}

func (s *RateLimitService) recordAccountHealth(ctx context.Context, event AccountHealthEvent) {
	policy, enabled, _ := s.accountHealthPolicy()
	if !enabled || event.AccountID <= 0 {
		return
	}
	if len(event.FailureReason) > 256 {
		event.FailureReason = event.FailureReason[:256]
	}
	snapshot, err := s.accountHealthCache.Record(ctx, event, policy)
	if err != nil {
		slog.Warn("account_health_record_failed", "account_id", event.AccountID, "success", event.Success, "error", err)
		return
	}
	if snapshot != nil {
		s.accountHealthLocal.Store(event.AccountID, accountHealthLocalEntry{
			snapshot:  normalizeAccountHealthSnapshot(snapshot, time.Now()),
			expiresAt: time.Now().Add(policy.LocalCacheTTL),
		})
	}
}

func (s *RateLimitService) ObserveAccountHealthSuccess(ctx context.Context, account *Account, latency time.Duration) {
	if account == nil {
		return
	}
	latencyMs := latency.Milliseconds()
	if latencyMs < 0 {
		latencyMs = 0
	}
	s.recordAccountHealth(ctx, AccountHealthEvent{
		AccountID: account.ID,
		Success:   true,
		LatencyMs: latencyMs,
	})
}

func (s *RateLimitService) ObserveAccountHealthFailure(ctx context.Context, accountID int64, observedErr error) {
	if accountID <= 0 || !shouldPenalizeAccountHealth(ctx, observedErr) {
		return
	}
	s.recordAccountHealth(ctx, AccountHealthEvent{
		AccountID:     accountID,
		FailureReason: accountHealthFailureReason(observedErr),
	})
}

func shouldPenalizeAccountHealth(ctx context.Context, observedErr error) bool {
	if observedErr == nil || (ctx != nil && ctx.Err() != nil) || errors.Is(observedErr, context.Canceled) {
		return false
	}
	var failoverErr *UpstreamFailoverError
	if !errors.As(observedErr, &failoverErr) {
		return true
	}
	if failoverErr.RequestScopedTransient || failoverErr.Scope == GatewayFailureScopeProvider || failoverErr.Scope == GatewayFailureScopeRequest {
		return false
	}
	if failoverErr.StatusCode == http.StatusTooManyRequests {
		return false
	}
	if failoverErr.StatusCode >= 400 && failoverErr.StatusCode < 500 && failoverErr.StatusCode != http.StatusUnauthorized && failoverErr.StatusCode != http.StatusForbidden {
		return false
	}
	return failoverErr.ShouldReportAccountScheduleFailure()
}

func accountHealthFailureReason(observedErr error) string {
	var failoverErr *UpstreamFailoverError
	if errors.As(observedErr, &failoverErr) {
		if failoverErr.Reason != "" {
			return string(failoverErr.Reason)
		}
		if failoverErr.StatusCode > 0 {
			return fmt.Sprintf("upstream_http_%d", failoverErr.StatusCode)
		}
	}
	if errors.Is(observedErr, context.DeadlineExceeded) {
		return "upstream_timeout"
	}
	var netErr net.Error
	if errors.As(observedErr, &netErr) && netErr.Timeout() {
		return "upstream_timeout"
	}
	return "upstream_failure"
}

func (s *RateLimitService) annotateAccountsWithHealth(ctx context.Context, accounts []Account) []Account {
	_, enabled, enforce := s.accountHealthPolicy()
	if !enabled || len(accounts) == 0 {
		return accounts
	}
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	snapshots := s.GetAccountHealthSnapshots(ctx, ids)
	if len(snapshots) == 0 {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		snapshot := snapshots[account.ID]
		if snapshot == nil {
			snapshot = defaultAccountHealthSnapshot()
		}
		account.setSchedulingHealth(snapshot, enforce)
		if enforce && snapshot.State == AccountHealthStateOpen {
			continue
		}
		filtered = append(filtered, account)
	}
	return filtered
}

func (s *RateLimitService) accountHealthAllowsWait(account *Account) bool {
	if account == nil || !account.schedulingHealthEnforced {
		return true
	}
	return account.schedulingHealthState != AccountHealthStateHalfOpen
}

func (s *RateLimitService) acquireAccountHealthProbe(ctx context.Context, account *Account) (string, bool) {
	policy, enabled, enforce := s.accountHealthPolicy()
	if !enabled || !enforce || account == nil {
		return "", true
	}
	state := account.schedulingHealthState
	if !account.schedulingHealthKnown {
		snapshot := s.GetAccountHealthSnapshots(ctx, []int64{account.ID})[account.ID]
		if snapshot == nil {
			return "", true
		}
		account.setSchedulingHealth(snapshot, true)
		state = snapshot.State
	}
	if state == AccountHealthStateOpen {
		return "", false
	}
	if state != AccountHealthStateHalfOpen {
		return "", true
	}
	token, acquired, err := s.accountHealthCache.AcquireProbe(ctx, account.ID, policy.HalfOpenMaxProbes, s.accountHealthProbeLeaseTTL(policy))
	if err != nil {
		slog.Warn("account_health_probe_acquire_failed", "account_id", account.ID, "error", err)
		return "", true
	}
	return token, acquired
}

func (s *RateLimitService) accountHealthProbeLeaseTTL(policy AccountHealthPolicy) time.Duration {
	ttl := policy.BaseCooldown
	if s != nil && s.cfg != nil && s.cfg.Gateway.ConcurrencySlotTTLMinutes > 0 {
		slotTTL := time.Duration(s.cfg.Gateway.ConcurrencySlotTTLMinutes) * time.Minute
		if slotTTL > ttl {
			ttl = slotTTL
		}
	}
	return ttl
}

func (s *RateLimitService) releaseAccountHealthProbe(_ context.Context, accountID int64, token string) {
	if s == nil || s.accountHealthCache == nil || token == "" {
		return
	}
	releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.accountHealthCache.ReleaseProbe(releaseCtx, accountID, token); err != nil {
		slog.Warn("account_health_probe_release_failed", "account_id", accountID, "error", err)
	}
}

func (a *Account) setSchedulingHealth(snapshot *AccountHealthSnapshot, enforce bool) {
	if a == nil || snapshot == nil {
		return
	}
	a.schedulingHealthKnown = true
	a.schedulingHealthEnforced = enforce
	a.schedulingHealthScore = snapshot.Score
	a.schedulingHealthState = snapshot.State
}

func copySchedulingHealthAnnotation(source, target *Account) {
	if source == nil || target == nil || !source.schedulingHealthKnown {
		return
	}
	target.schedulingHealthKnown = source.schedulingHealthKnown
	target.schedulingHealthEnforced = source.schedulingHealthEnforced
	target.schedulingHealthScore = source.schedulingHealthScore
	target.schedulingHealthState = source.schedulingHealthState
}

func accountSchedulingHealthScore(account *Account) float64 {
	if account == nil || !account.schedulingHealthKnown || !account.schedulingHealthEnforced {
		return 100
	}
	return account.schedulingHealthScore
}

func compareAccountSchedulingHealth(left, right *Account) int {
	leftScore := accountSchedulingHealthScore(left)
	rightScore := accountSchedulingHealthScore(right)
	switch {
	case leftScore > rightScore:
		return -1
	case leftScore < rightScore:
		return 1
	default:
		return 0
	}
}
