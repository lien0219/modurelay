package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type loginSecuritySettingRepo struct {
	values map[string]string
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

func newLoginAbuseTestProtector(t *testing.T, settings LoginSecuritySettings) (*LoginAbuseProtector, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	repo := &loginSecuritySettingRepo{values: map[string]string{}}
	settingSvc := NewSettingService(repo, nil)
	require.NoError(t, settingSvc.SetLoginSecuritySettings(context.Background(), &settings))
	return NewLoginAbuseProtector(rdb, settingSvc), mr
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
}
