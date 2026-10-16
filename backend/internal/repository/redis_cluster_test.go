package repository

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/redisutil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// This deliberately uses real Redis Cluster: miniredis does not enforce hash
// slots, and would silently pass the CROSSSLOT regressions covered here.
func newLocalRedisCluster(t *testing.T) *redis.ClusterClient {
	t.Helper()
	server, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server is required for real Cluster coverage")
	}
	cli, err := exec.LookPath("redis-cli")
	if err != nil {
		t.Skip("redis-cli is required for real Cluster coverage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var addresses []string
	for i := 0; i < 3; i++ {
		var port int
		for candidate := 22000; candidate < 32000; candidate++ {
			listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidate))
			if err != nil {
				continue
			}
			bus, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidate+10000))
			_ = listener.Close()
			if err != nil {
				continue
			}
			_ = bus.Close()
			port = candidate
			break
		}
		require.NotZero(t, port)
		directory := t.TempDir()
		log, err := os.Create(filepath.Join(directory, "redis.log"))
		require.NoError(t, err)
		cmd := exec.Command(server, "--bind", "127.0.0.1", "--port", strconv.Itoa(port),
			"--cluster-enabled", "yes", "--cluster-config-file", "nodes.conf", "--cluster-node-timeout", "1000",
			"--appendonly", "no", "--save", "", "--dir", directory)
		cmd.Stdout, cmd.Stderr = log, log
		require.NoError(t, cmd.Start())
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = log.Close() })
		address := fmt.Sprintf("127.0.0.1:%d", port)
		probe := redis.NewClient(&redis.Options{Addr: address, MaxRetries: -1})
		require.Eventually(t, func() bool { return probe.Ping(ctx).Err() == nil }, 5*time.Second, 20*time.Millisecond)
		_ = probe.Close()
		addresses = append(addresses, address)
	}
	args := append([]string{"--cluster", "create"}, addresses...)
	args = append(args, "--cluster-replicas", "0", "--cluster-yes")
	output, err := exec.CommandContext(ctx, cli, args...).CombinedOutput()
	require.NoError(t, err, string(output))
	client, ok := InitRedis(&config.Config{Redis: config.RedisConfig{Mode: "cluster", ClusterAddrs: addresses, DialTimeoutSeconds: 2, ReadTimeoutSeconds: 2, WriteTimeoutSeconds: 2}}).(*redis.ClusterClient)
	require.True(t, ok)
	t.Cleanup(func() { _ = client.Close() })
	require.Eventually(t, func() bool {
		return client.ForEachMaster(ctx, func(ctx context.Context, node *redis.Client) error {
			info, err := node.ClusterInfo(ctx).Result()
			if err == nil && !strings.Contains(info, "cluster_state:ok") {
				return fmt.Errorf("cluster is converging")
			}
			return err
		}) == nil
	}, 10*time.Second, 50*time.Millisecond)
	return client
}

func TestRedisClusterAtomicCaches(t *testing.T) {
	client := newLocalRedisCluster(t)
	ctx := context.Background()
	t.Run("waiters expire independently across instances", func(t *testing.T) {
		first := NewConcurrencyCache(client, 1, 60).(*concurrencyCache)
		second := NewConcurrencyCache(client, 1, 60).(*concurrencyCache)
		for _, scope := range []string{"account", "user"} {
			queue := waitLeaseScope(first, scope, 991)
			peer := waitLeaseScope(second, scope, 991)
			for _, id := range []string{"crashed-pod", "live-pod"} {
				ok, err := queue.acquire(ctx, 991, 2, id)
				require.NoError(t, err)
				require.True(t, ok)
			}
			now, err := first.redisUnixSeconds(ctx)
			require.NoError(t, err)
			require.NoError(t, client.ZAdd(ctx, queue.key, redis.Z{Score: float64(now - 1), Member: "crashed-pod"}).Err())
			ok, err := peer.acquire(ctx, 991, 2, "replacement")
			require.NoError(t, err)
			require.True(t, ok)
			require.NoError(t, queue.release(ctx, 991, "crashed-pod"))
			count, err := peer.count(ctx, 991)
			require.NoError(t, err)
			require.Equal(t, 2, count, "late release must preserve both live waiters")
			ok, err = queue.acquire(ctx, 991, 2, "over-limit")
			require.NoError(t, err)
			require.False(t, ok)
		}
	})
	t.Run("concurrency survives another pod startup and live admission stays atomic", func(t *testing.T) {
		first, ok := NewConcurrencyCache(client, 1, 60).(*concurrencyCache)
		require.True(t, ok)
		second, ok := NewConcurrencyCache(client, 1, 60).(*concurrencyCache)
		require.True(t, ok)
		acquired, err := first.AcquireAccountSlot(ctx, 41, 1, "pod-a-request")
		require.NoError(t, err)
		require.True(t, acquired)
		_, err = first.IncrementAccountWaitCount(ctx, 41, 3, "waiter-1")
		require.NoError(t, err)
		require.NoError(t, second.CleanupStaleProcessSlots(ctx, "pod-b-"))
		acquired, err = second.AcquireAccountSlot(ctx, 41, 1, "pod-b-request")
		require.NoError(t, err)
		require.False(t, acquired)
		count, err := first.GetAccountWaitingCount(ctx, 41)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.NoError(t, first.ReleaseAccountSlot(ctx, 41, "pod-a-request"))
		acquired, err = first.AcquireLiveLease(ctx, 41, 1, 42, 1, 43, "lease-a", false)
		require.NoError(t, err)
		require.True(t, acquired)
		acquired, err = second.AcquireLiveLease(ctx, 44, 1, 42, 1, 45, "lease-b", false)
		require.NoError(t, err)
		require.False(t, acquired)
		alive, err := first.RefreshLiveLease(ctx, 41, 42, 43, "lease-a")
		require.NoError(t, err)
		require.True(t, alive)
		require.NoError(t, first.ReleaseLiveLease(ctx, 41, 42, 43, "lease-a"))
	})
	t.Run("billing reservations serialize contenders", func(t *testing.T) {
		cache, ok := NewBillingCache(client).(*billingCache)
		require.True(t, ok)
		var wg sync.WaitGroup
		accepted := make(chan bool, 20)
		failures := make(chan error, 20)
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				ok, _, err := cache.ReserveInflightBalance(ctx, 71, fmt.Sprintf("request-%d", i), 1, 5, time.Minute)
				accepted <- ok
				failures <- err
			}(i)
		}
		wg.Wait()
		close(accepted)
		close(failures)
		total := 0
		for ok := range accepted {
			if ok {
				total++
			}
		}
		for err := range failures {
			require.NoError(t, err)
		}
		require.Equal(t, 5, total)
		require.NoError(t, cache.ReleaseInflightBalance(ctx, 71, "request-0"))
	})
	t.Run("balance cache rejects stale refill after deduction", func(t *testing.T) {
		cache, ok := NewBillingCache(client).(*billingCache)
		require.True(t, ok)
		require.NoError(t, cache.SetUserBalance(ctx, 72, 10))
		token, err := cache.BeginUserBalanceLoad(ctx, 72)
		require.NoError(t, err)
		require.NoError(t, cache.DeductUserBalance(ctx, 72, 3))
		require.NoError(t, cache.SetUserBalanceIfUnchanged(ctx, 72, 10, token))
		balance, err := cache.GetUserBalance(ctx, 72)
		require.NoError(t, err)
		require.Equal(t, 7.0, balance)
		require.NoError(t, cache.InvalidateUserBalance(ctx, 72))
		require.NoError(t, cache.SetUserBalanceIfUnchanged(ctx, 72, 10, token))
		_, err = cache.GetUserBalance(ctx, 72)
		require.ErrorIs(t, err, redis.Nil)
	})
	t.Run("quota increment and dirty index remain atomic", func(t *testing.T) {
		cache, ok := NewBillingCache(client).(*billingCache)
		require.True(t, ok)
		require.NoError(t, cache.SetUserPlatformQuotaCache(ctx, 73, "openai", &service.UserPlatformQuotaCacheEntry{SchemaVersion: service.UserPlatformQuotaCacheSchemaV1}, time.Minute))
		require.NoError(t, cache.IncrUserPlatformQuotaUsageCache(ctx, 73, "openai", 2.5, time.Minute, true))
		entry, hit, err := cache.GetUserPlatformQuotaCache(ctx, 73, "openai")
		require.NoError(t, err)
		require.True(t, hit)
		require.Equal(t, 2.5, entry.DailyUsageUSD)
		keys, err := cache.PopDirtyUserPlatformQuotaKeys(ctx, 10)
		require.NoError(t, err)
		require.Len(t, keys, 1)
		require.NoError(t, cache.ReaddDirtyUserPlatformQuotaKeys(ctx, keys))
		entries, err := cache.BatchGetUserPlatformQuotaCache(ctx, keys)
		require.NoError(t, err)
		require.Equal(t, 2.5, entries[0].DailyUsageUSD)
		require.NoError(t, cache.DeleteUserPlatformQuotaCache(ctx, 73, "openai"))
	})
	t.Run("login request and credential counters enforce cluster limits", func(t *testing.T) {
		store := NewRedisLoginAbuseStore(client)
		n, _, _, err := store.IncrementRequest(ctx, "login_security:req:203.0.113.1", "login_security:req:203.0.113.1:audit", time.Minute, 1)
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
		n, _, audited, err := store.IncrementRequest(ctx, "login_security:req:203.0.113.1", "login_security:req:203.0.113.1:audit", time.Minute, 1)
		require.NoError(t, err)
		require.EqualValues(t, 2, n)
		require.True(t, audited)
		_, blocked, err := store.IncrementFailure(ctx, "login_security:account:user:fail", "login_security:account:user:block", time.Minute, 1, time.Minute)
		require.NoError(t, err)
		require.True(t, blocked)
		ttl, _, err := store.CredentialBlockTTLs(ctx, "login_security:account:user:block", "login_security:account:missing:block")
		require.NoError(t, err)
		require.Positive(t, ttl)
		require.NoError(t, store.DeleteCounter(ctx, "login_security:account:user:fail"))
	})
	t.Run("scheduler fencing and accounts span shards", func(t *testing.T) {
		cache, ok := NewSchedulerCache(client).(*schedulerCache)
		require.True(t, ok)
		bucket := service.SchedulerBucket{GroupID: 91, Platform: service.PlatformOpenAI, Mode: "default"}
		token, err := cache.CaptureBucketWriteToken(ctx, bucket)
		require.NoError(t, err)
		require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{{ID: 92}, {ID: 93}}))
		require.NoError(t, cache.UpdateLastUsed(ctx, map[int64]time.Time{92: time.Now(), 93: time.Now()}))
		accounts, hit, err := cache.GetSnapshot(ctx, bucket)
		require.NoError(t, err)
		require.True(t, hit)
		require.Len(t, accounts, 2)
		require.NotNil(t, accounts[0].LastUsedAt)
		require.NoError(t, cache.RetireBucket(ctx, bucket))
		require.ErrorIs(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{{ID: 94}}), service.ErrSchedulerBucketRetired)
		_, err = cache.ReopenBucket(ctx, bucket)
		require.NoError(t, err)
		require.NoError(t, cache.DeleteAccount(ctx, 92))
	})
	t.Run("expired scheduler owner cannot release successor lease", func(t *testing.T) {
		cache, ok := NewSchedulerCache(client).(*schedulerCache)
		require.True(t, ok)
		bucket := service.SchedulerBucket{GroupID: 99, Platform: service.PlatformOpenAI, Mode: "default"}
		oldOwner, acquired, err := cache.TryAcquireBucketLease(ctx, bucket, time.Minute)
		require.NoError(t, err)
		require.True(t, acquired)
		// Expire deterministically, then let another acquisition own the same key.
		require.NoError(t, client.Del(ctx, cache.schedulerBucketKey(schedulerLockPrefix, bucket)).Err())
		nextOwner, acquired, err := cache.TryAcquireBucketLease(ctx, bucket, time.Minute)
		require.NoError(t, err)
		require.True(t, acquired)
		require.NotEqual(t, oldOwner, nextOwner)
		require.NoError(t, cache.ReleaseBucketLease(ctx, bucket, oldOwner))
		_, acquired, err = cache.TryAcquireBucketLease(ctx, bucket, time.Minute)
		require.NoError(t, err)
		require.False(t, acquired)
		require.NoError(t, cache.ReleaseBucketLease(ctx, bucket, nextOwner))
	})
	t.Run("verification code and attempts share a slot", func(t *testing.T) {
		cache := NewEmailCache(client)
		require.NoError(t, cache.SetVerificationCode(ctx, "cluster@example.test", &service.VerificationCodeData{Code: "123456"}, time.Minute))
		n, err := cache.IncrVerificationCodeAttempts(ctx, "cluster@example.test")
		require.NoError(t, err)
		require.Equal(t, 1, n)
		code, err := cache.GetVerificationCode(ctx, "cluster@example.test")
		require.NoError(t, err)
		require.Equal(t, 1, code.Attempts)
		require.NoError(t, cache.DeleteVerificationCode(ctx, "cluster@example.test"))
	})
	t.Run("queue reserve requeue ack are cross-pod safe", func(t *testing.T) {
		queue := newBatchImageQueueWithOptions(client, batchImageQueueOptions{})
		require.NoError(t, queue.Enqueue(ctx, "imgbatch_cluster"))
		task, err := queue.Reserve(ctx, time.Millisecond)
		require.NoError(t, err)
		require.Equal(t, "imgbatch_cluster", task.BatchID)
		require.NoError(t, queue.RequeueAfter(ctx, task.BatchID, 0))
		task, err = queue.Reserve(ctx, time.Millisecond)
		require.NoError(t, err)
		require.NoError(t, queue.Ack(ctx, task.BatchID))
	})
	t.Run("session batch works with cold script caches on every shard", func(t *testing.T) {
		cache := NewSessionLimitCache(client, 10)
		counts, err := cache.GetActiveSessionCountBatch(ctx, []int64{101, 102, 103}, nil)
		require.NoError(t, err)
		require.Equal(t, map[int64]int{101: 0, 102: 0, 103: 0}, counts)
	})
	t.Run("multi-get and plugin scan visit all shards", func(t *testing.T) {
		store := NewPluginKVStore(client)
		var keys []string
		for i := 0; i < 30; i++ {
			key := fmt.Sprintf("item-%d", i)
			require.NoError(t, store.Set(ctx, "cluster", "test", key, []byte(key), time.Minute))
			keys = append(keys, "plugin:kv:v1:cluster:test:"+key)
		}
		keys = append(keys, "missing:cluster:test")
		values, err := redisutil.MGet(ctx, client, keys...)
		require.NoError(t, err)
		require.Len(t, values, 31)
		require.Nil(t, values[30])
		for i := 0; i < 30; i++ {
			require.Equal(t, fmt.Sprintf("item-%d", i), values[i])
		}
		listed, err := store.List(ctx, "cluster", "test", "item-", 100)
		require.NoError(t, err)
		require.Len(t, listed, 30)
	})
}

func TestConcurrencyStartupPreservesOtherInstancesStandalone(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	ctx := context.Background()
	first, isConcrete := NewConcurrencyCache(client, 1, 60).(*concurrencyCache)
	require.True(t, isConcrete)
	second, isConcrete := NewConcurrencyCache(client, 1, 60).(*concurrencyCache)
	require.True(t, isConcrete)
	ok, err := first.AcquireAccountSlot(ctx, 1, 1, "pod-a-request")
	require.NoError(t, err)
	require.True(t, ok)
	_, err = first.IncrementAccountWaitCount(ctx, 1, 2, "waiter-1")
	require.NoError(t, err)
	require.NoError(t, second.CleanupStaleProcessSlots(ctx, "pod-b-"))
	ok, err = second.AcquireAccountSlot(ctx, 1, 1, "pod-b-request")
	require.NoError(t, err)
	require.False(t, ok)
	count, err := first.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.True(t, server.Exists(accountSlotKey(1)), "standalone key names remain compatible")
}
