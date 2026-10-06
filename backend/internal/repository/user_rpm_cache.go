package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// 用户/分组级 RPM 计数器 Redis 实现。
//
// 设计说明：
//   - key 形式：rpm:ug:{uid}:{gid}:{minute}、rpm:u:{uid}:{minute}
//   - 时间来源：rdb.Time()（Redis 服务端时间），避免多实例时钟漂移。
//   - 原子操作：TxPipeline (MULTI/EXEC) 执行 INCR+EXPIRE，兼容 Redis Cluster。
//   - TTL：120s，覆盖当前分钟窗口 + 少量冗余。
//   - 返回值语义：超限判断由调用方（billing_cache_service.checkRPM）与 RPMLimit 比较完成。
const (
	userGroupRPMKeyPrefix = "rpm:ug:"
	userRPMKeyPrefix      = "rpm:u:"

	userRPMKeyTTL = 120 * time.Second
)

var multiScopeRPMAdmissionScript = redis.NewScript(`
local limited = 0
local limited_count = 0
local limited_limit = 0
for i = 1, #KEYS do
  local count = redis.call('INCR', KEYS[i])
  local ttl = redis.call('TTL', KEYS[i])
  if count == 1 or ttl < 0 then
    redis.call('EXPIRE', KEYS[i], ARGV[#KEYS + 1])
  end
  local limit = tonumber(ARGV[i])
  if limited == 0 and count > limit then
    limited = i
    limited_count = count
    limited_limit = limit
  end
end
return {limited, limited_count, limited_limit}
`)

type userRPMCacheImpl struct {
	rdb *redis.Client
}

// NewUserRPMCache 创建用户/分组级 RPM 计数器。
func NewUserRPMCache(rdb *redis.Client) service.UserRPMCache {
	return &userRPMCacheImpl{rdb: rdb}
}

// minuteTS 获取当前 Redis 服务端分钟时间戳。
func (c *userRPMCacheImpl) minuteTS(ctx context.Context) (int64, error) {
	t, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return 0, fmt.Errorf("redis TIME: %w", err)
	}
	return t.Unix() / 60, nil
}

// atomicIncr 原子 INCR+EXPIRE。
func (c *userRPMCacheImpl) atomicIncr(ctx context.Context, key string) (int, error) {
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, userRPMKeyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("user rpm increment: %w", err)
	}
	return int(incr.Val()), nil
}

// IncrementUserGroupRPM 递增 (user, group) 分钟计数。
func (c *userRPMCacheImpl) IncrementUserGroupRPM(ctx context.Context, userID, groupID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d:%d", userGroupRPMKeyPrefix, userID, groupID, minute)
	return c.atomicIncr(ctx, key)
}

// IncrementUserRPM 递增用户分钟计数。
func (c *userRPMCacheImpl) IncrementUserRPM(ctx context.Context, userID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d", userRPMKeyPrefix, userID, minute)
	return c.atomicIncr(ctx, key)
}

func (c *userRPMCacheImpl) AdmitMultiScopeRPM(ctx context.Context, counters []service.RPMCounter) (service.RPMAdmissionResult, error) {
	if len(counters) == 0 {
		return service.RPMAdmissionResult{Allowed: true}, nil
	}
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return service.RPMAdmissionResult{}, err
	}
	keys := make([]string, len(counters))
	args := make([]any, len(counters)+1)
	for i, counter := range counters {
		if counter.Key == "" || counter.Scope == "" || counter.Limit <= 0 {
			return service.RPMAdmissionResult{}, fmt.Errorf("invalid multi-scope rpm counter at index %d", i)
		}
		keys[i] = fmt.Sprintf("rpm:%s:%d", counter.Key, minute)
		args[i] = counter.Limit
	}
	args[len(counters)] = int64(userRPMKeyTTL / time.Second)
	values, err := multiScopeRPMAdmissionScript.Run(ctx, c.rdb, keys, args...).Slice()
	if err != nil {
		return service.RPMAdmissionResult{}, fmt.Errorf("multi-scope rpm admission: %w", err)
	}
	if len(values) != 3 {
		return service.RPMAdmissionResult{}, fmt.Errorf("multi-scope rpm admission returned %d values", len(values))
	}
	limitedIndex, err := parseRPMInt64(values[0])
	if err != nil {
		return service.RPMAdmissionResult{}, err
	}
	count, err := parseRPMInt64(values[1])
	if err != nil {
		return service.RPMAdmissionResult{}, err
	}
	limit, err := parseRPMInt64(values[2])
	if err != nil {
		return service.RPMAdmissionResult{}, err
	}
	if limitedIndex == 0 {
		return service.RPMAdmissionResult{Allowed: true}, nil
	}
	if limitedIndex < 1 || limitedIndex > int64(len(counters)) {
		return service.RPMAdmissionResult{}, fmt.Errorf("multi-scope rpm admission returned invalid counter index %d", limitedIndex)
	}
	return service.RPMAdmissionResult{
		Allowed: false,
		Scope:   counters[limitedIndex-1].Scope,
		Count:   count,
		Limit:   limit,
	}, nil
}

func parseRPMInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected multi-scope rpm value type %T", value)
	}
}

// GetUserGroupRPM 获取 (user, group) 当前分钟已用 RPM（只读）。
func (c *userRPMCacheImpl) GetUserGroupRPM(ctx context.Context, userID, groupID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d:%d", userGroupRPMKeyPrefix, userID, groupID, minute)
	val, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("user group rpm get: %w", err)
	}
	return val, nil
}

// GetUserRPM 获取用户当前分钟已用 RPM（只读）。
func (c *userRPMCacheImpl) GetUserRPM(ctx context.Context, userID int64) (int, error) {
	minute, err := c.minuteTS(ctx)
	if err != nil {
		return 0, err
	}
	key := fmt.Sprintf("%s%d:%d", userRPMKeyPrefix, userID, minute)
	val, err := c.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("user rpm get: %w", err)
	}
	return val, nil
}
