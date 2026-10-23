//go:build unit

package service

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// This test intentionally shuts down one node and is opt-in for a dedicated,
// disposable three-master cluster. Never point these variables at a shared DB.
func TestMonitoringRedisClusterCapacityReal(t *testing.T) {
	raw := os.Getenv("MODURELAY_MONITORING_REDIS_CLUSTER_TEST_ADDRS")
	if raw == "" || os.Getenv("MODURELAY_MONITORING_REDIS_CLUSTER_ALLOW_SHUTDOWN") != "1" {
		t.Skip("requires dedicated disposable Redis Cluster and explicit shutdown opt-in")
	}
	addresses := strings.Split(raw, ",")
	require.Len(t, addresses, 3)
	ctx := context.Background()
	makeClient := func() *redis.ClusterClient {
		client := redis.NewClusterClient(&redis.ClusterOptions{Addrs: addresses, PoolSize: 4, MinIdleConns: 0, MaxRetries: -1, DialTimeout: 200 * time.Millisecond, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond, ContextTimeoutEnabled: true})
		t.Cleanup(func() { _ = client.Close() })
		return client
	}
	a, b := makeClient(), makeClient()
	require.Eventually(t, func() bool {
		return a.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
			raw, err := node.ClusterInfo(ctx).Result()
			if err == nil && !strings.Contains(raw, "cluster_state:ok") {
				return redis.Nil
			}
			return err
		}) == nil
	}, 10*time.Second, 50*time.Millisecond)
	// Pin real client connections on the 100-slot node; the other two have 1000.
	hot := redis.NewClient(&redis.Options{Addr: addresses[0], PoolSize: 90, MaxRetries: -1})
	t.Cleanup(func() { _ = hot.Close() })
	pinned := make([]*redis.Conn, 0, 80)
	t.Cleanup(func() {
		for _, conn := range pinned {
			_ = conn.Close()
		}
	})
	for range 80 {
		conn := hot.Conn()
		require.NoError(t, conn.Ping(ctx).Err())
		pinned = append(pinned, conn)
	}
	ca := &OpsMetricsCollector{redisClient: a, instanceID: "capacity-app-a"}
	cb := &OpsMetricsCollector{redisClient: b, instanceID: "capacity-app-b"}
	first := ca.collectDatabaseCapacity(ctx).Redis
	second := cb.collectDatabaseCapacity(ctx).Redis
	for _, capacity := range []OpsDatabaseInstanceCapacity{first, second} {
		require.Equal(t, "Available", capacity.State)
		require.Len(t, capacity.Nodes, 3)
		require.EqualValues(t, 100, *capacity.Limit)
		require.GreaterOrEqual(t, *capacity.Used, int64(80))
		percent, state := databaseInstanceUsage(capacity)
		require.Equal(t, alertMetricOK, state)
		require.GreaterOrEqual(t, percent, 80.0)
		var sumUsed, sumLimit int64
		for _, node := range capacity.Nodes {
			sumUsed += *node.Used
			sumLimit += *node.Limit
		}
		require.Less(t, 100*float64(sumUsed)/float64(sumLimit), 10.0, "aggregate denominator would hide this hot master")
	}
	for _, collector := range []*OpsMetricsCollector{ca, cb} {
		limit := collector.redisPoolCapacity(ctx)
		require.NotNil(t, limit)
		require.Equal(t, 12, *limit)
		total, idle, ok := collector.redisPoolStats()
		require.True(t, ok)
		require.GreaterOrEqual(t, total, 3)
		require.LessOrEqual(t, idle, total)
		t.Logf("instance=%s nodes=3 configured_pool_budget=%d actual_pool_connections=%d idle=%d", collector.instanceID, *limit, total, idle)
	}
	data, err := json.Marshal(first)
	require.NoError(t, err)
	t.Logf("healthy_hotspot=%s", data)
	failed := redis.NewClient(&redis.Options{Addr: addresses[2], MaxRetries: -1})
	defer failed.Close()
	_ = failed.Shutdown(ctx).Err() // Redis closes the response connection on shutdown.
	capacity := ca.collectDatabaseCapacity(ctx).Redis
	require.Equal(t, "Unknown", capacity.State)
	require.Nil(t, capacity.Used)
	require.Nil(t, capacity.Limit)
	data, err = json.Marshal(capacity)
	require.NoError(t, err)
	t.Logf("one_master_unreachable=%s", data)
}
