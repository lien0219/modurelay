//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func runtimeSample(memory, capacity int64, active, limit int, cpu float64) *OpsSystemMetricsSnapshot {
	return &OpsSystemMetricsSnapshot{
		CreatedAt: time.Now().UTC(), MemoryUsedMB: &memory, MemoryTotalMB: &capacity,
		CPUUsagePercent: &cpu, DBConnActive: &active, DBMaxOpenConns: &limit,
		DBOK: boolPtr(true), RedisOK: boolPtr(true), GoroutineCount: intPtr(10),
	}
}

func TestOpsClusterRuntimeAggregatesPodsAndExpiresMissingMembers(t *testing.T) {
	mr := miniredis.RunT(t)
	now := time.Now().UTC()
	mr.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	podA := &OpsMetricsCollector{redisClient: client, instanceID: "pod-a"}
	podB := &OpsMetricsCollector{redisClient: client, instanceID: "pod-b"}
	ctx := context.Background()
	require.NoError(t, podA.publishRuntimeSnapshot(ctx, runtimeSample(100, 200, 2, 10, 20)))
	require.NoError(t, podB.publishRuntimeSnapshot(ctx, runtimeSample(300, 800, 4, 20, 80)))
	for _, pod := range []*OpsMetricsCollector{podA, podB} {
		snapshot, err := pod.ClusterRuntimeSnapshot(ctx)
		require.NoError(t, err)
		require.Equal(t, "cluster", snapshot.Scope)
		require.Equal(t, 2, snapshot.InstanceCount)
		require.EqualValues(t, 400, *snapshot.MemoryUsedMB)
		require.EqualValues(t, 1000, *snapshot.MemoryTotalMB)
		require.Equal(t, 40.0, *snapshot.MemoryUsagePercent)
		require.Equal(t, 50.0, *snapshot.CPUUsagePercent)
		require.Equal(t, 6, *snapshot.DBConnActive)
		require.Equal(t, 30, *snapshot.DBMaxOpenConns)
	}
	// A replacement sample overwrites its pod, rather than accumulating twice.
	require.NoError(t, podA.publishRuntimeSnapshot(ctx, runtimeSample(150, 200, 3, 10, 30)))
	snapshot, err := podB.ClusterRuntimeSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, snapshot.InstanceCount)
	require.Equal(t, 7, *snapshot.DBConnActive)
	// Redis server time governs expiry; a pod's own clock is irrelevant.
	mr.SetTime(now.Add(46 * time.Second))
	require.NoError(t, podB.publishRuntimeSnapshot(ctx, runtimeSample(300, 800, 4, 20, 80)))
	snapshot, err = podA.ClusterRuntimeSnapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, snapshot.InstanceCount)
	require.Equal(t, 4, *snapshot.DBConnActive)
	require.Equal(t, int64(1), client.HLen(ctx, opsRuntimeSamplesKey).Val())
}

func TestOpsClusterRuntimeNoRedisSingleInstanceAndFailure(t *testing.T) {
	sample := runtimeSample(100, 200, 2, 10, 20)
	sample.Scope, sample.InstanceCount = "instance", 1
	collector := &OpsMetricsCollector{runtimeSnapshot: sample}
	snapshot, err := collector.ClusterRuntimeSnapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, snapshot.InstanceCount)
	sample.CreatedAt = time.Now().Add(-time.Minute)
	_, err = collector.ClusterRuntimeSnapshot(context.Background())
	require.Error(t, err)
	// A configured but failed Redis must not silently report one pod as total.
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	collector.redisClient = client
	require.NoError(t, client.Close())
	_, err = collector.ClusterRuntimeSnapshot(context.Background())
	require.Error(t, err)
}

func TestOpsClusterRuntimeDeduplicatesHostFallbacks(t *testing.T) {
	a := runtimeSample(100, 200, 2, 10, 20)
	b := runtimeSample(100, 200, 3, 20, 20)
	a.CPUResourceID, b.CPUResourceID = "host:shared", "host:shared"
	a.MemoryResourceID, b.MemoryResourceID = "host:shared", "host:shared"
	snapshot := aggregateOpsRuntimeSamples([]*OpsSystemMetricsSnapshot{a, b})
	require.Equal(t, 2, snapshot.InstanceCount)
	require.EqualValues(t, 100, *snapshot.MemoryUsedMB)
	require.EqualValues(t, 200, *snapshot.MemoryTotalMB)
	require.Equal(t, 5, *snapshot.DBConnActive)
	require.Equal(t, 30, *snapshot.DBMaxOpenConns)
	require.Empty(t, snapshot.CPUResourceID)
	require.Empty(t, snapshot.MemoryResourceID)
}

type collectedBucketRepository struct {
	opsRepoMock
	latest  *OpsSystemMetricsSnapshot
	err     error
	inserts int
}

func (r *collectedBucketRepository) GetLatestSystemMetrics(context.Context, int) (*OpsSystemMetricsSnapshot, error) {
	return r.latest, r.err
}
func (r *collectedBucketRepository) InsertSystemMetrics(context.Context, *OpsInsertSystemMetricsInput) error {
	r.inserts++
	return nil
}

func TestOpsMetricsCollectorSkipsAlreadyCollectedBucketAndReadFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		latest *OpsSystemMetricsSnapshot
		err    error
	}{
		{"completed", &OpsSystemMetricsSnapshot{CreatedAt: time.Now().UTC().Truncate(time.Minute)}, nil},
		{"read failed", nil, errors.New("database unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery("SELECT pg_try_advisory_lock").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			mock.ExpectExec("SELECT pg_advisory_unlock").WillReturnResult(sqlmock.NewResult(0, 1))
			repo := &collectedBucketRepository{latest: tc.latest, err: tc.err}
			collector := &OpsMetricsCollector{db: db, opsRepo: repo}
			collector.collectOnce()
			require.Zero(t, repo.inserts)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

var _ OpsRepository = (*collectedBucketRepository)(nil)
