//go:build unit

package service

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/dbpoolstats"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestDatabaseCapacityStandardQueryAndUnavailable(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	collector := &OpsMetricsCollector{db: db}
	mock.ExpectQuery("SELECT .*pg_stat_activity.*max_connections").WillReturnRows(sqlmock.NewRows([]string{"count", "max"}).AddRow(25, 100))
	result := collector.collectDatabaseCapacity(context.Background())
	require.Equal(t, "Available", result.PostgreSQL.State)
	require.EqualValues(t, 25, *result.PostgreSQL.Used)
	require.Equal(t, "Unknown", result.Redis.State)
	mock.ExpectQuery("SELECT .*pg_stat_activity.*max_connections").WillReturnError(errors.New("permission denied"))
	result = collector.collectDatabaseCapacity(context.Background())
	require.Equal(t, "Unknown", result.PostgreSQL.State)
	require.Nil(t, result.PostgreSQL.Used)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonitoringDatabaseCapacityConnectionSlotsPostgres(t *testing.T) {
	dsn := os.Getenv("MODURELAY_MONITORING_TEST_DSN")
	if dsn == "" {
		t.Skip("set disposable PostgreSQL DSN")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	db.SetMaxOpenConns(1)
	capacity := (&OpsMetricsCollector{db: db}).collectDatabaseCapacity(ctx).PostgreSQL
	require.Equal(t, "Available", capacity.State)
	var total, background int64
	require.NoError(t, db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER(WHERE backend_type<>'client backend') FROM pg_stat_activity`).Scan(&total, &background))
	require.Positive(t, background)
	require.Less(t, *capacity.Used, total, "background/IO workers must not inflate max_connections usage")
	require.Contains(t, capacity.Source, "client backend slots")
}

type capacityInfoHook struct {
	raw string
	err error
}

func (h capacityInfoHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (h capacityInfoHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error { cmd.(*redis.StringCmd).SetVal(h.raw); return h.err }
}
func (h capacityInfoHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestRedisClusterCapacityKeepsNodesAndDoesNotHideHotShard(t *testing.T) {
	clients := []*redis.Client{}
	for i, raw := range []string{"connected_clients:90\nmaxclients:100", "connected_clients:1\nmaxclients:1000"} {
		client := redis.NewClient(&redis.Options{Addr: []string{"one:6379", "two:6379"}[i]})
		client.AddHook(capacityInfoHook{raw: raw})
		defer client.Close()
		clients = append(clients, client)
	}
	each := func(ctx context.Context, fn func(context.Context, *redis.Client) error) error {
		var wg sync.WaitGroup
		for _, client := range clients {
			wg.Add(1)
			go func(c *redis.Client) { defer wg.Done(); require.NoError(t, fn(ctx, c)) }(client)
		}
		wg.Wait()
		return nil
	}
	result := collectRedisShardCapacity(context.Background(), each)
	require.Equal(t, "Available", result.State)
	require.Len(t, result.Nodes, 2)
	require.EqualValues(t, 90, *result.Used)
	require.EqualValues(t, 100, *result.Limit)
	value, status := databaseInstanceUsage(result)
	require.Equal(t, alertMetricOK, status)
	require.Equal(t, float64(90), value)
	failed := collectRedisShardCapacity(context.Background(), func(context.Context, func(context.Context, *redis.Client) error) error {
		return errors.New("one shard unavailable")
	})
	require.Equal(t, "Unknown", failed.State)
	require.Nil(t, failed.Used)
}

func TestDatabasePoolStatsIncludeAuxiliaryPoolsAndUnregister(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	defer primary.Close()
	lock, lm, err := sqlmock.New()
	require.NoError(t, err)
	defer lock.Close()
	replica, rm, err := sqlmock.New()
	require.NoError(t, err)
	defer replica.Close()
	primary.SetMaxOpenConns(10)
	lock.SetMaxOpenConns(16)
	replica.SetMaxOpenConns(64)
	removeLock := dbpoolstats.Register(primary, "advisory_lock", lock)
	defer removeLock()
	removeReplica := dbpoolstats.Register(primary, "read_replica", replica)
	defer removeReplica()
	collector := &OpsMetricsCollector{db: primary, instanceID: "a"}
	stats := collector.databasePoolStats()
	require.Len(t, stats, 3)
	aggregate := aggregateOpsRuntimeSamples([]*OpsSystemMetricsSnapshot{{DatabasePools: stats}, {DatabasePools: []OpsDatabasePoolStats{{Instance: "b", Role: "primary", Limit: 20}}}})
	require.Len(t, aggregate.DatabasePools, 4)
	removeReplica()
	require.Len(t, collector.databasePoolStats(), 2)
	require.NoError(t, pm.ExpectationsWereMet())
	require.NoError(t, lm.ExpectationsWereMet())
	require.NoError(t, rm.ExpectationsWereMet())
}
func TestRedisCapacityRequiresExplicitServerLimit(t *testing.T) {
	result := parseRedisInstanceCapacity("# Clients\r\nconnected_clients:12\r\nmaxclients:1000\r\n")
	require.Equal(t, "Available", result.State)
	require.EqualValues(t, 12, *result.Used)
	require.EqualValues(t, 1000, *result.Limit)
	missing := parseRedisInstanceCapacity("connected_clients:12\r\n")
	require.Equal(t, "Unknown", missing.State)
	require.Nil(t, missing.Limit)
	require.Equal(t, "Unknown", parseRedisInstanceCapacity("connected_clients:-1\nmaxclients:0").State)
}
func TestPoolCountersAreProcessOwnedAndDoNotQueryDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(7)
	svc := &OpsService{metricsCollector: &OpsMetricsCollector{db: db}}
	values := svc.InstancePoolCounters()
	require.Equal(t, float64(7), values["modurelay_db_pool_max_open_connections"])
	require.Equal(t, float64(0), values["modurelay_db_pool_wait_total"])
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestDatabaseCapacityAlertsRejectStaleOrUnknownSnapshots(t *testing.T) {
	used, limit := int64(8), int64(10)
	capacity := &OpsDatabaseCapacity{PostgreSQL: OpsDatabaseInstanceCapacity{State: "Available", Used: &used, Limit: &limit}}
	evaluator := &OpsAlertEvaluatorService{}
	snap := &OpsSystemMetricsSnapshot{CreatedAt: time.Now(), Capacity: capacity}
	rule := &OpsAlertRule{MetricType: "db_instance_usage_percent"}
	value, status := evaluator.computeRuleMetric(context.Background(), rule, snap, time.Now(), time.Now(), "", nil)
	require.Equal(t, alertMetricOK, status)
	require.Equal(t, float64(80), value)
	snap.CreatedAt = time.Now().Add(-4 * time.Minute)
	_, status = evaluator.computeRuleMetric(context.Background(), rule, snap, time.Now(), time.Now(), "", nil)
	require.Equal(t, alertMetricUnavailable, status)
}

type capacityAlertTestRepo struct {
	*stubOpsRepo
	snapshot *OpsSystemMetricsSnapshot
}

func (r *capacityAlertTestRepo) GetLatestSystemMetrics(context.Context, int) (*OpsSystemMetricsSnapshot, error) {
	return r.snapshot, nil
}
func TestDatabaseCapacityAlertRecoveryWorksWithoutPrometheus(t *testing.T) {
	used, limit := int64(1), int64(10)
	repo := &capacityAlertTestRepo{stubOpsRepo: &stubOpsRepo{rules: []*OpsAlertRule{{ID: 1, Enabled: true, MetricType: "db_instance_usage_percent", Operator: ">", Threshold: 80, WindowMinutes: 1, SustainedMinutes: 1}}, activeEvent: &OpsAlertEvent{ID: 7, RuleID: 1, Status: OpsAlertStatusFiring}}, snapshot: &OpsSystemMetricsSnapshot{CreatedAt: time.Now(), Capacity: &OpsDatabaseCapacity{PostgreSQL: OpsDatabaseInstanceCapacity{State: "Available", Used: &used, Limit: &limit}}}}
	evaluator := &OpsAlertEvaluatorService{opsRepo: repo}
	evaluator.evaluateOnce(time.Minute)
	require.Equal(t, 1, repo.statusCalls)
	require.Equal(t, OpsAlertDataStatusOK, repo.states[1].DataStatus)
}
