// Package redisutil contains operations whose standalone Redis semantics need
// explicit routing when a ClusterClient is used.
package redisutil

import (
	"context"
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
)

func IsCluster(client redis.UniversalClient) bool {
	_, ok := client.(*redis.ClusterClient)
	return ok
}

// TaggedKey preserves existing standalone keys. Related atomic operations use
// a stable hash tag in cluster mode, without putting unrelated caches in one slot.
func TaggedKey(client redis.UniversalClient, tag, key string) string {
	if !IsCluster(client) {
		return key
	}
	return "{" + tag + "}:" + key
}

// MGet routes independent keys through a pipeline on their owning primaries.
// Like Redis MGET it preserves order and returns nil for missing/non-string keys.
func MGet(ctx context.Context, client redis.UniversalClient, keys ...string) ([]any, error) {
	if len(keys) == 0 {
		return []any{}, nil
	}
	if !IsCluster(client) {
		return client.MGet(ctx, keys...).Result()
	}
	pipe := client.Pipeline()
	commands := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		commands[i] = pipe.Get(ctx, key)
	}
	_, _ = pipe.Exec(ctx)
	values := make([]any, len(keys))
	for i, cmd := range commands {
		value, err := cmd.Result()
		if errors.Is(err, redis.Nil) || (err != nil && strings.HasPrefix(err.Error(), "WRONGTYPE")) {
			continue
		}
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

// Scan visits every primary, because ClusterClient.Scan only scans one node.
// The callback can stop its node scan by returning false. It may run concurrently
// for different primaries and must protect shared result state.
func Scan(ctx context.Context, client redis.UniversalClient, pattern string, count int64, visit func([]string) bool) error {
	scan := func(ctx context.Context, node redis.Cmdable) error {
		var cursor uint64
		for {
			keys, next, err := node.Scan(ctx, cursor, pattern, count).Result()
			if err != nil {
				return err
			}
			if !visit(keys) || next == 0 {
				return nil
			}
			cursor = next
		}
	}
	if cluster, ok := client.(*redis.ClusterClient); ok {
		return cluster.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error { return scan(ctx, node) })
	}
	return scan(ctx, client)
}
