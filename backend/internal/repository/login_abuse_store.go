package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

var loginRequestRateScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
if current == 1 or ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
local audit = 0
if current > tonumber(ARGV[2]) then
  local logged = redis.call('SET', KEYS[2], '1', 'NX', 'PX', math.max(ttl, 1))
  if logged then audit = 1 end
end
return {current, ttl, audit}
`)

var loginFailureScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
if current == 1 or ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
local blocked = 0
if current >= tonumber(ARGV[2]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
  blocked = 1
end
return {current, ttl, blocked}
`)

// RedisLoginAbuseStore contains the atomic counter and TTL operations used by
// the service-level login protection policy.
type RedisLoginAbuseStore struct {
	client *redis.Client
}

func NewRedisLoginAbuseStore(client *redis.Client) *RedisLoginAbuseStore {
	return &RedisLoginAbuseStore{client: client}
}

func (s *RedisLoginAbuseStore) IncrementRequest(
	ctx context.Context,
	counterKey, auditKey string,
	window time.Duration,
	limit int,
) (int64, time.Duration, bool, error) {
	if s == nil || s.client == nil {
		return 0, 0, false, errors.New("login abuse redis client is not configured")
	}
	values, err := loginRequestRateScript.Run(ctx, s.client, []string{counterKey, auditKey}, window.Milliseconds(), limit).Slice()
	if err != nil {
		return 0, 0, false, fmt.Errorf("increment login request counter: %w", err)
	}
	if len(values) < 3 {
		return 0, 0, false, errors.New("invalid login request script response")
	}
	count, err := loginScriptInt64(values[0])
	if err != nil {
		return 0, 0, false, err
	}
	ttlMs, err := loginScriptInt64(values[1])
	if err != nil {
		return 0, 0, false, err
	}
	audit, err := loginScriptInt64(values[2])
	if err != nil {
		return 0, 0, false, err
	}
	return count, time.Duration(ttlMs) * time.Millisecond, audit == 1, nil
}

func (s *RedisLoginAbuseStore) CredentialBlockTTLs(
	ctx context.Context,
	accountIPKey, accountKey string,
) (time.Duration, time.Duration, error) {
	if s == nil || s.client == nil {
		return 0, 0, errors.New("login abuse redis client is not configured")
	}
	pipe := s.client.Pipeline()
	accountIPTTL := pipe.PTTL(ctx, accountIPKey)
	accountTTL := pipe.PTTL(ctx, accountKey)
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, fmt.Errorf("read login credential block TTLs: %w", err)
	}
	return positiveTTL(accountIPTTL.Val()), positiveTTL(accountTTL.Val()), nil
}

func (s *RedisLoginAbuseStore) IncrementFailure(
	ctx context.Context,
	counterKey, blockKey string,
	window time.Duration,
	limit int,
	blockTTL time.Duration,
) (int64, bool, error) {
	if s == nil || s.client == nil {
		return 0, false, errors.New("login abuse redis client is not configured")
	}
	values, err := loginFailureScript.Run(
		ctx,
		s.client,
		[]string{counterKey, blockKey},
		window.Milliseconds(),
		limit,
		blockTTL.Milliseconds(),
	).Slice()
	if err != nil {
		return 0, false, fmt.Errorf("increment login password failure counter: %w", err)
	}
	if len(values) < 3 {
		return 0, false, errors.New("invalid login failure script response")
	}
	count, err := loginScriptInt64(values[0])
	if err != nil {
		return 0, false, err
	}
	blocked, err := loginScriptInt64(values[2])
	if err != nil {
		return 0, false, err
	}
	return count, blocked == 1, nil
}

func (s *RedisLoginAbuseStore) DeleteCounter(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return errors.New("login abuse redis client is not configured")
	}
	return s.client.Del(ctx, key).Err()
}

func loginScriptInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		var parsed int64
		if _, err := fmt.Sscan(v, &parsed); err != nil {
			return 0, fmt.Errorf("parse login script integer: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected login script value type %T", value)
	}
}

func positiveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return 0
}

var _ service.LoginAbuseStore = (*RedisLoginAbuseStore)(nil)
