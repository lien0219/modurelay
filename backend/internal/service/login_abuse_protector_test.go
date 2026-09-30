package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memoryLoginAbuseCounter struct {
	count     int64
	expiresAt time.Time
}

type memoryLoginAbuseStore struct {
	mu       sync.Mutex
	now      time.Time
	counters map[string]memoryLoginAbuseCounter
	blocks   map[string]time.Time
	audited  map[string]time.Time
	err      error
}

func newMemoryLoginAbuseStore() *memoryLoginAbuseStore {
	return &memoryLoginAbuseStore{
		now:      time.Now(),
		counters: make(map[string]memoryLoginAbuseCounter),
		blocks:   make(map[string]time.Time),
		audited:  make(map[string]time.Time),
	}
}

func (s *memoryLoginAbuseStore) IncrementRequest(_ context.Context, counterKey, auditKey string, window time.Duration, limit int) (int64, time.Duration, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, 0, false, s.err
	}
	counter := s.increment(counterKey, window)
	audit := false
	if counter.count > int64(limit) && !s.now.Before(s.audited[auditKey]) {
		s.audited[auditKey] = counter.expiresAt
		audit = true
	}
	return counter.count, counter.expiresAt.Sub(s.now), audit, nil
}

func (s *memoryLoginAbuseStore) CredentialBlockTTLs(_ context.Context, accountIPKey, accountKey string) (time.Duration, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, 0, s.err
	}
	return s.blockTTL(accountIPKey), s.blockTTL(accountKey), nil
}

func (s *memoryLoginAbuseStore) IncrementFailure(_ context.Context, counterKey, blockKey string, window time.Duration, limit int, blockTTL time.Duration) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, false, s.err
	}
	counter := s.increment(counterKey, window)
	blocked := counter.count >= int64(limit)
	if blocked {
		s.blocks[blockKey] = s.now.Add(blockTTL)
	}
	return counter.count, blocked, nil
}

func (s *memoryLoginAbuseStore) DeleteCounter(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	delete(s.counters, key)
	return nil
}

func (s *memoryLoginAbuseStore) increment(key string, ttl time.Duration) memoryLoginAbuseCounter {
	counter := s.counters[key]
	if counter.count == 0 || !s.now.Before(counter.expiresAt) {
		counter = memoryLoginAbuseCounter{expiresAt: s.now.Add(ttl)}
	}
	counter.count++
	s.counters[key] = counter
	return counter
}

func (s *memoryLoginAbuseStore) blockTTL(key string) time.Duration {
	expiresAt := s.blocks[key]
	if !s.now.Before(expiresAt) {
		delete(s.blocks, key)
		return 0
	}
	return expiresAt.Sub(s.now)
}

func (s *memoryLoginAbuseStore) setError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

func (s *memoryLoginAbuseStore) advance(d time.Duration) {
	s.mu.Lock()
	s.now = s.now.Add(d)
	s.mu.Unlock()
}

type loginSecuritySettingRepo struct {
	values map[string]string
}

type flakyLoginSecuritySettingRepo struct {
	SettingRepository
	mu    sync.Mutex
	err   error
	value string
	calls int
}

func (r *flakyLoginSecuritySettingRepo) GetValue(context.Context, string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return "", r.err
	}
	if r.value == "" {
		return "", ErrSettingNotFound
	}
	return r.value, nil
}

func (r *flakyLoginSecuritySettingRepo) setError(err error) {
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()
}

func (r *flakyLoginSecuritySettingRepo) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *loginSecuritySettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *loginSecuritySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (r *loginSecuritySettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}
func (r *loginSecuritySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}
func (r *loginSecuritySettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}
func (r *loginSecuritySettingRepo) GetAll(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}
func (r *loginSecuritySettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func newLoginAbuseTestProtector(t *testing.T, settings LoginSecuritySettings) (*LoginAbuseProtector, *memoryLoginAbuseStore) {
	t.Helper()
	store := newMemoryLoginAbuseStore()

	repo := &loginSecuritySettingRepo{values: map[string]string{}}
	settingSvc := NewSettingService(repo, nil)
	require.NoError(t, settingSvc.SetLoginSecuritySettings(context.Background(), &settings))
	return NewLoginAbuseProtector(store, settingSvc), store
}

func TestLoginAbuseProtectorAccountIPBlocksAfterFiveFailures(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	protector, _ := newLoginAbuseTestProtector(t, settings)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		decision, err := protector.CheckCredentialsAllowed(ctx, "user@example.com", "203.0.113.10")
		require.NoError(t, err)
		require.False(t, decision.Blocked)

		outcome, err := protector.RecordPasswordFailure(ctx, "user@example.com", "203.0.113.10")
		require.NoError(t, err)
		require.Equal(t, int64(i+1), outcome.AccountIPCount)
	}

	decision, err := protector.CheckCredentialsAllowed(ctx, "user@example.com", "203.0.113.10")
	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.Equal(t, "account_ip", decision.Scope)
}

func TestLoginAbuseProtectorGlobalAccountBlocksDistributedAttack(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	settings.AccountIPFailureLimit = 3
	settings.AccountFailureLimit = 4
	protector, _ := newLoginAbuseTestProtector(t, settings)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		_, err := protector.RecordPasswordFailure(ctx, "admin@example.com", fmt.Sprintf("203.0.113.%d", i+1))
		require.NoError(t, err)
	}

	decision, err := protector.CheckCredentialsAllowed(ctx, "admin@example.com", "198.51.100.99")
	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.Equal(t, "account", decision.Scope)
}

func TestLoginAbuseProtectorSuccessClearsOnlySourceCounter(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	protector, _ := newLoginAbuseTestProtector(t, settings)
	ctx := context.Background()

	_, err := protector.RecordPasswordFailure(ctx, "user@example.com", "203.0.113.10")
	require.NoError(t, err)
	require.NoError(t, protector.RecordSuccess(ctx, "user@example.com", "203.0.113.10"))

	outcome, err := protector.RecordPasswordFailure(ctx, "user@example.com", "203.0.113.10")
	require.NoError(t, err)
	require.Equal(t, int64(1), outcome.AccountIPCount)
	require.Equal(t, int64(2), outcome.AccountCount)
}

func TestLoginAbuseProtectorIPv6By64SharesRequestBucket(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	settings.RequestLimitPerMinute = 5
	protector, _ := newLoginAbuseTestProtector(t, settings)
	ctx := context.Background()

	addresses := []string{
		"2001:db8:abcd:1234:1111::1",
		"2001:db8:abcd:1234:2222::1",
		"2001:db8:abcd:1234:3333::1",
		"2001:db8:abcd:1234:4444::1",
		"2001:db8:abcd:1234:5555::1",
	}
	for _, address := range addresses {
		decision, err := protector.CheckRequestRate(ctx, address)
		require.NoError(t, err)
		require.False(t, decision.Blocked)
	}
	decision, err := protector.CheckRequestRate(ctx, "2001:db8:abcd:1234:ffff::1")
	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.Equal(t, "ip", decision.Scope)
	require.True(t, decision.Audit)

	decision, err = protector.CheckRequestRate(ctx, "2001:db8:abcd:1234:eeee::2")
	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.False(t, decision.Audit)

	decision, err = protector.CheckRequestRate(ctx, "2001:db8:abcd:1235:1111::1")
	require.NoError(t, err)
	require.False(t, decision.Blocked)
}

func TestLoginAbuseProtectorRequestLimitIsAtomicAndAuditsOneThrottle(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	settings.RequestLimitPerMinute = 20
	protector, _ := newLoginAbuseTestProtector(t, settings)

	var blocked atomic.Int64
	var audited atomic.Int64
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			decision, err := protector.CheckRequestRate(context.Background(), "203.0.113.80")
			require.NoError(t, err)
			if decision.Blocked {
				blocked.Add(1)
			}
			if decision.Audit {
				audited.Add(1)
			}
		}()
	}
	wait.Wait()
	require.EqualValues(t, 80, blocked.Load())
	require.EqualValues(t, 1, audited.Load())
}

func TestLoginAbuseProtectorFailureCountersAreScopedByAccountAndSource(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	protector, _ := newLoginAbuseTestProtector(t, settings)
	ctx := context.Background()

	for range 4 {
		_, err := protector.RecordPasswordFailure(ctx, "one@example.com", "203.0.113.12")
		require.NoError(t, err)
	}

	otherAccount, err := protector.RecordPasswordFailure(ctx, "two@example.com", "203.0.113.12")
	require.NoError(t, err)
	require.EqualValues(t, 1, otherAccount.AccountIPCount)
	require.EqualValues(t, 1, otherAccount.AccountCount)

	otherSource, err := protector.RecordPasswordFailure(ctx, "one@example.com", "203.0.113.13")
	require.NoError(t, err)
	require.EqualValues(t, 1, otherSource.AccountIPCount)
	require.EqualValues(t, 5, otherSource.AccountCount)

	decision, err := protector.CheckCredentialsAllowed(ctx, "one@example.com", "203.0.113.13")
	require.NoError(t, err)
	require.False(t, decision.Blocked)
}

func TestLoginAbuseProtectorGlobalDefaultBlocksAtTwentyAcrossSources(t *testing.T) {
	protector, _ := newLoginAbuseTestProtector(t, *DefaultLoginSecuritySettings())
	ctx := context.Background()
	for i := range 20 {
		outcome, err := protector.RecordPasswordFailure(ctx, "distributed@example.com", fmt.Sprintf("198.51.100.%d", i+1))
		require.NoError(t, err)
		if i < 19 {
			require.False(t, outcome.AccountBlocked)
		} else {
			require.True(t, outcome.AccountBlocked)
		}
	}
	decision, err := protector.CheckCredentialsAllowed(ctx, "distributed@example.com", "192.0.2.200")
	require.NoError(t, err)
	require.True(t, decision.Blocked)
	require.Equal(t, "account", decision.Scope)
}

func TestLoginAbuseProtectorConcurrentPasswordFailuresRemainAtomic(t *testing.T) {
	protector, _ := newLoginAbuseTestProtector(t, *DefaultLoginSecuritySettings())
	const attempts = 40
	counts := make(chan int64, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			outcome, err := protector.RecordPasswordFailure(context.Background(), "race@example.com", "203.0.113.44")
			require.NoError(t, err)
			counts <- outcome.AccountIPCount
		}()
	}
	wait.Wait()
	close(counts)
	seen := make(map[int64]struct{}, attempts)
	for count := range counts {
		seen[count] = struct{}{}
	}
	require.Len(t, seen, attempts)
	_, err := protector.CheckCredentialsAllowed(context.Background(), "race@example.com", "203.0.113.44")
	require.NoError(t, err)
	decision, err := protector.CheckCredentialsAllowed(context.Background(), "race@example.com", "203.0.113.44")
	require.NoError(t, err)
	require.True(t, decision.Blocked)
}

func TestLoginAbuseProtectorRedisFailureDoesNotFailOpen(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	protector, store := newLoginAbuseTestProtector(t, settings)
	store.setError(errors.New("redis unavailable"))

	_, err := protector.CheckRequestRate(context.Background(), "203.0.113.99")
	require.Error(t, err)
	_, err = protector.CheckCredentialsAllowed(context.Background(), "user@example.com", "203.0.113.99")
	require.Error(t, err)
	_, err = protector.RecordPasswordFailure(context.Background(), "user@example.com", "203.0.113.99")
	require.Error(t, err)
}

func TestLoginAbuseProtectorRequestCounterExpires(t *testing.T) {
	settings := *DefaultLoginSecuritySettings()
	settings.RequestLimitPerMinute = 5
	protector, store := newLoginAbuseTestProtector(t, settings)
	for range 6 {
		_, err := protector.CheckRequestRate(context.Background(), "203.0.113.55")
		require.NoError(t, err)
	}
	store.advance(time.Minute + time.Second)
	decision, err := protector.CheckRequestRate(context.Background(), "203.0.113.55")
	require.NoError(t, err)
	require.False(t, decision.Blocked)
}

func TestLoginSecuritySettingsCacheBacksOffSettingReadFailures(t *testing.T) {
	t.Run("expired disabled cache fails closed without retrying every login", func(t *testing.T) {
		settings := *DefaultLoginSecuritySettings()
		settings.Enabled = false
		raw, err := json.Marshal(settings)
		require.NoError(t, err)
		repo := &flakyLoginSecuritySettingRepo{value: string(raw)}
		settingService := NewSettingService(repo, nil)
		protector := NewLoginAbuseProtector(nil, settingService)

		cached, err := protector.Settings(context.Background())
		require.NoError(t, err)
		require.False(t, cached.Enabled)
		repo.setError(errors.New("settings backend unavailable"))

		protector.cacheMu.Lock()
		protector.cache.expiresAt = time.Now().Add(-time.Second)
		protector.cacheMu.Unlock()

		for range 2 {
			_, err = protector.CheckRequestRate(context.Background(), "203.0.113.30")
			require.Error(t, err)
		}
		require.Equal(t, 2, repo.callCount())
	})

	t.Run("initial read errors are throttled and remain fail closed", func(t *testing.T) {
		repo := &flakyLoginSecuritySettingRepo{err: errors.New("settings backend unavailable")}
		settingService := NewSettingService(repo, nil)
		protector := NewLoginAbuseProtector(nil, settingService)

		for range 2 {
			_, err := protector.Settings(context.Background())
			require.Error(t, err)
		}
		require.Equal(t, 1, repo.callCount())
	})
}

func TestGetLoginSecuritySettingsRejectsMalformedPersistedData(t *testing.T) {
	repo := &loginSecuritySettingRepo{values: map[string]string{settingKeyLoginSecuritySettings: `{"enabled":true`}}
	settingSvc := NewSettingService(repo, nil)
	_, err := settingSvc.GetLoginSecuritySettings(context.Background())
	require.Error(t, err)
}

func TestLoginSecuritySettingsDefaultsAndBounds(t *testing.T) {
	defaults := DefaultLoginSecuritySettings()
	require.Equal(t, LoginSecuritySettings{
		Enabled: true, RequestLimitPerMinute: 20, GroupIPv6By64: true,
		AccountIPFailureLimit: 5, AccountIPWindowMinutes: 30, AccountIPBlockMinutes: 30,
		AccountFailureLimit: 20, AccountWindowMinutes: 30, AccountBlockMinutes: 30,
		AdminMFARequired: false,
	}, *defaults)
	require.NoError(t, validateLoginSecuritySettings(defaults))

	mutations := []func(*LoginSecuritySettings){
		func(s *LoginSecuritySettings) { s.RequestLimitPerMinute = 4 },
		func(s *LoginSecuritySettings) { s.RequestLimitPerMinute = 301 },
		func(s *LoginSecuritySettings) { s.AccountIPFailureLimit = 2 },
		func(s *LoginSecuritySettings) { s.AccountIPFailureLimit = 21 },
		func(s *LoginSecuritySettings) { s.AccountIPWindowMinutes = 4 },
		func(s *LoginSecuritySettings) { s.AccountIPWindowMinutes = 121 },
		func(s *LoginSecuritySettings) { s.AccountIPBlockMinutes = 4 },
		func(s *LoginSecuritySettings) { s.AccountIPBlockMinutes = 1441 },
		func(s *LoginSecuritySettings) { s.AccountFailureLimit = 4 },
		func(s *LoginSecuritySettings) { s.AccountFailureLimit = 201 },
		func(s *LoginSecuritySettings) { s.AccountWindowMinutes = 4 },
		func(s *LoginSecuritySettings) { s.AccountWindowMinutes = 241 },
		func(s *LoginSecuritySettings) { s.AccountBlockMinutes = 4 },
		func(s *LoginSecuritySettings) { s.AccountBlockMinutes = 1441 },
	}
	for i, mutate := range mutations {
		t.Run(fmt.Sprintf("invalid_%d", i), func(t *testing.T) {
			settings := *DefaultLoginSecuritySettings()
			mutate(&settings)
			require.Error(t, validateLoginSecuritySettings(&settings))
		})
	}
}
