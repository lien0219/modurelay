package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const accountHealthPrefix = "account_health:"

var accountHealthRecordScript = redis.NewScript(`
	local state_key = KEYS[1]
	local probe_key = KEYS[2]
	local success = tonumber(ARGV[1])
	local latency_ms = tonumber(ARGV[2])
	local failure_reason = ARGV[3]
	local minimum_samples = tonumber(ARGV[4])
	local degraded_score = tonumber(ARGV[5])
	local open_score = tonumber(ARGV[6])
	local consecutive_threshold = tonumber(ARGV[7])
	local base_cooldown = tonumber(ARGV[8])
	local max_cooldown = tonumber(ARGV[9])
	local state_ttl = tonumber(ARGV[10])

	local now_parts = redis.call('TIME')
	local now_unix = tonumber(now_parts[1])
	local state = {
		score = 100,
		state = 'warming',
		sample_count = 0,
		error_rate_ewma = 0,
		latency_ewma_ms = 0,
		consecutive_failures = 0,
		open_count = 0,
		open_until_unix = 0,
		last_failure_reason = '',
		updated_at_unix = now_unix
	}

	local raw = redis.call('GET', state_key)
	if raw then
		local ok, decoded = pcall(cjson.decode, raw)
		if ok and decoded then
			for key, value in pairs(decoded) do
				state[key] = value
			end
		end
	end

	state.sample_count = (tonumber(state.sample_count) or 0) + 1
	local previous_error = tonumber(state.error_rate_ewma) or 0
	local error_sample = success == 1 and 0 or 1
	state.error_rate_ewma = (previous_error * 0.8) + (error_sample * 0.2)

	if latency_ms and latency_ms > 0 then
		local previous_latency = tonumber(state.latency_ewma_ms) or 0
		if previous_latency <= 0 then
			state.latency_ewma_ms = latency_ms
		else
			state.latency_ewma_ms = (previous_latency * 0.8) + (latency_ms * 0.2)
		end
	end

	if success == 1 then
		state.consecutive_failures = 0
		state.last_failure_reason = ''
		state.open_until_unix = 0
		if state.state == 'open' or state.state == 'half_open' then
			state.error_rate_ewma = 0
			state.open_count = 0
		end
	else
		state.consecutive_failures = (tonumber(state.consecutive_failures) or 0) + 1
		state.last_failure_reason = failure_reason
	end

	state.score = math.max(0, math.min(100, 100 - ((tonumber(state.error_rate_ewma) or 0) * 100)))
	local should_open = success == 0 and (
		(tonumber(state.consecutive_failures) or 0) >= consecutive_threshold or
		(state.sample_count >= minimum_samples and state.score <= open_score)
	)
	if should_open then
		state.state = 'open'
		state.open_count = (tonumber(state.open_count) or 0) + 1
		local exponent = math.min(tonumber(state.open_count) - 1, 20)
		local cooldown = math.min(max_cooldown, base_cooldown * (2 ^ exponent))
		state.open_until_unix = now_unix + cooldown
	elseif state.sample_count < minimum_samples then
		state.state = 'warming'
	elseif state.score <= degraded_score then
		state.state = 'degraded'
	else
		state.state = 'healthy'
	end

	state.updated_at_unix = now_unix
	redis.call('SET', state_key, cjson.encode(state), 'EX', state_ttl)
	return cjson.encode(state)
`)

var accountHealthAcquireProbeScript = redis.NewScript(`
	local key = KEYS[1]
	local token = ARGV[1]
	local limit = tonumber(ARGV[2])
	local ttl_ms = tonumber(ARGV[3])
	local now_parts = redis.call('TIME')
	local now_ms = (tonumber(now_parts[1]) * 1000) + math.floor(tonumber(now_parts[2]) / 1000)

	redis.call('ZREMRANGEBYSCORE', key, '-inf', now_ms)
	if redis.call('ZCARD', key) >= limit then
		return 0
	end
	redis.call('ZADD', key, now_ms + ttl_ms, token)
	redis.call('PEXPIRE', key, ttl_ms)
	return 1
`)

func accountHealthStateKey(accountID int64) string {
	return fmt.Sprintf("%s{%d}:state", accountHealthPrefix, accountID)
}

func accountHealthProbeKey(accountID int64) string {
	return fmt.Sprintf("%s{%d}:probes", accountHealthPrefix, accountID)
}

func (c *tempUnschedCache) Record(ctx context.Context, event service.AccountHealthEvent, policy service.AccountHealthPolicy) (*service.AccountHealthSnapshot, error) {
	success := 0
	if event.Success {
		success = 1
	}
	result, err := accountHealthRecordScript.Run(ctx, c.rdb, []string{
		accountHealthStateKey(event.AccountID),
		accountHealthProbeKey(event.AccountID),
	}, success, event.LatencyMs, event.FailureReason, policy.MinimumSamples, policy.DegradedScore,
		policy.OpenScore, policy.ConsecutiveFailures, int64(policy.BaseCooldown.Seconds()),
		int64(policy.MaxCooldown.Seconds()), int64(policy.StateTTL.Seconds())).Text()
	if err != nil {
		return nil, fmt.Errorf("record account health: %w", err)
	}
	var snapshot service.AccountHealthSnapshot
	if err := json.Unmarshal([]byte(result), &snapshot); err != nil {
		return nil, fmt.Errorf("decode account health: %w", err)
	}
	return &snapshot, nil
}

func (c *tempUnschedCache) GetBatch(ctx context.Context, accountIDs []int64) (map[int64]*service.AccountHealthSnapshot, error) {
	result := make(map[int64]*service.AccountHealthSnapshot, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	keys := make([]string, len(accountIDs))
	for i, accountID := range accountIDs {
		keys[i] = accountHealthStateKey(accountID)
	}
	values, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get account health batch: %w", err)
	}
	for i, value := range values {
		if value == nil {
			continue
		}
		raw, ok := value.(string)
		if !ok || raw == "" {
			continue
		}
		var snapshot service.AccountHealthSnapshot
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			return nil, fmt.Errorf("decode account health %d: %w", accountIDs[i], err)
		}
		result[accountIDs[i]] = &snapshot
	}
	return result, nil
}

func (c *tempUnschedCache) AcquireProbe(ctx context.Context, accountID int64, limit int, ttl time.Duration) (string, bool, error) {
	if limit < 1 {
		limit = 1
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	var tokenBytes [16]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return "", false, fmt.Errorf("create account health probe token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes[:])
	acquired, err := accountHealthAcquireProbeScript.Run(ctx, c.rdb, []string{accountHealthProbeKey(accountID)}, token, limit, math.Ceil(float64(ttl.Milliseconds()))).Int64()
	if err != nil {
		return "", false, fmt.Errorf("acquire account health probe: %w", err)
	}
	if acquired != 1 {
		return "", false, nil
	}
	return token, true, nil
}

func (c *tempUnschedCache) ReleaseProbe(ctx context.Context, accountID int64, token string) error {
	if token == "" {
		return nil
	}
	if err := c.rdb.ZRem(ctx, accountHealthProbeKey(accountID), token).Err(); err != nil {
		return fmt.Errorf("release account health probe: %w", err)
	}
	return nil
}

var _ service.AccountHealthCache = (*tempUnschedCache)(nil)
