package setup

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestWriteConfigPreservesRedisClusterSettings(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	seeds := splitRedisClusterAddrs(" redis-0:6379,redis-1:6379 , ")
	require.Equal(t, []string{"redis-0:6379", "redis-1:6379"}, seeds)
	require.NoError(t, writeConfigFile(&SetupConfig{Redis: RedisConfig{Mode: "cluster", ClusterAddrs: seeds, Username: "cluster-user"}}))
	raw, err := os.ReadFile(GetConfigFilePath())
	require.NoError(t, err)
	var saved struct {
		Redis RedisConfig `yaml:"redis"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &saved))
	require.Equal(t, "cluster", saved.Redis.Mode)
	require.Equal(t, seeds, saved.Redis.ClusterAddrs)
	require.Equal(t, "cluster-user", saved.Redis.Username)
}

func TestRedisConnectionRejectsInvalidClusterConfigBeforeDial(t *testing.T) {
	err := TestRedisConnection(&RedisConfig{Mode: "cluster", ClusterAddrs: []string{"localhost:1"}, DB: 1})
	require.ErrorContains(t, err, "redis.db must be 0")
}
