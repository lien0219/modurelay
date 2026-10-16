package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLoadRedisClusterEnvironment(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("REDIS_MODE", "cluster")
	t.Setenv("REDIS_CLUSTER_ADDRS", "redis-0:6379,redis-1:6379,redis-2:6379")
	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.Redis.IsCluster())
	require.Equal(t, []string{"redis-0:6379", "redis-1:6379", "redis-2:6379"}, cfg.Redis.ClusterAddrs)
}

func TestRedisModeValidation(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  RedisConfig
		invalid bool
	}{
		{name: "legacy defaults", config: RedisConfig{}},
		{name: "one seed cluster", config: RedisConfig{Mode: "cluster", ClusterAddrs: []string{"redis:6379"}}},
		{name: "invalid mode", config: RedisConfig{Mode: "clustr"}, invalid: true},
		{name: "empty seed host", config: RedisConfig{Mode: "cluster", ClusterAddrs: []string{":6379"}}, invalid: true},
		{name: "invalid seed port", config: RedisConfig{Mode: "cluster", ClusterAddrs: []string{"redis:65536"}}, invalid: true},
		{name: "no seeds", config: RedisConfig{Mode: "cluster"}, invalid: true},
		{name: "invalid seed", config: RedisConfig{Mode: "cluster", ClusterAddrs: []string{"redis"}}, invalid: true},
		{name: "nonzero database", config: RedisConfig{Mode: "cluster", ClusterAddrs: []string{"redis:6379"}, DB: 1}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.ValidateMode()
			if test.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
