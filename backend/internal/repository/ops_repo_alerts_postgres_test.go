package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func monitoringAlertTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := monitoringTestDB(t)
	// Build just the historical alert schema, then run the exact new migrations.
	base, err := migrations.FS.ReadFile("033_ops_monitoring_vnext.sql")
	require.NoError(t, err)
	start := strings.Index(string(base), "CREATE TABLE IF NOT EXISTS ops_alert_rules (")
	end := strings.Index(string(base)[start:], "-- =====================================================================")
	require.Positive(t, start)
	require.Positive(t, end)
	_, err = db.Exec(string(base)[start : start+end])
	require.NoError(t, err)
	_, err = db.Exec("ALTER TABLE ops_alert_rules ADD COLUMN notify_email BOOLEAN NOT NULL DEFAULT true")
	require.NoError(t, err)
	for _, file := range []string{"271_ops_alert_min_requests_and_rule_state.sql", "272_ops_alert_resources_and_outbox.sql", "276_ops_alert_evaluation_revision.sql"} {
		raw, err := migrations.FS.ReadFile(file)
		require.NoError(t, err)
		up := string(raw)
		_, err = db.Exec(up)
		require.NoError(t, err)
	}
	return db
}

func TestMonitoringAlertOutboxLifecyclePostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	rule, err := repo.CreateAlertRule(ctx, &service.OpsAlertRule{Name: "resource latency", MetricType: "http_p99_ms", Operator: ">", Threshold: 100, Severity: "P1", Enabled: true, WindowMinutes: 1, SustainedMinutes: 1})
	require.NoError(t, err)
	makeEvent := func(key string) *service.OpsAlertEvent {
		return &service.OpsAlertEvent{RuleID: rule.ID, Severity: "P1", Status: service.OpsAlertStatusFiring, Title: "test", Dimensions: map[string]any{"resource_key": key}, FiredAt: now, Deliveries: []*service.OpsAlertDelivery{{Channel: "webhook", Transition: "firing", Target: "https://hooks.example.com/a", Payload: []byte(`{"version":1}`)}}}
	}
	event, err := repo.CreateAlertEvent(ctx, makeEvent("uid-a"))
	require.NoError(t, err)
	require.NotNil(t, event)
	duplicate, err := repo.CreateAlertEvent(ctx, makeEvent("uid-a"))
	require.NoError(t, err)
	require.Nil(t, duplicate)
	other, err := repo.CreateAlertEvent(ctx, makeEvent("uid-b"))
	require.NoError(t, err)
	require.NotNil(t, other)
	tasks, err := repo.ClaimAlertDeliveries(ctx, now.Add(time.Second), 10)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	none, err := repo.ClaimAlertDeliveries(ctx, now.Add(time.Second), 10)
	require.NoError(t, err)
	require.Empty(t, none)
	failed := tasks[0]
	failed.Attempts = 8
	require.NoError(t, repo.CompleteAlertDelivery(ctx, failed, errors.New("webhook HTTP 500"), now.Add(time.Minute)))
	require.NoError(t, repo.RetryAlertDelivery(ctx, failed.ID))
	retried, err := repo.ClaimAlertDeliveries(ctx, time.Now().Add(time.Second), 10)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	require.Equal(t, 1, retried[0].Attempts)
	require.NotEqual(t, failed.LeaseToken, retried[0].LeaseToken)
	require.NoError(t, repo.CompleteAlertDelivery(ctx, retried[0], nil, now))
	require.Error(t, repo.RetryAlertDelivery(ctx, retried[0].ID))
	event.Status = service.OpsAlertStatusResolved
	event.ResolvedAt = &now
	resolved := []*service.OpsAlertDelivery{{Channel: "email", Transition: "resolved", Target: "ops@example.com", Payload: []byte(`{"subject":"resolved","body":"ok"}`)}}
	changed, err := repo.TransitionAlertEvent(ctx, event, resolved)
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = repo.TransitionAlertEvent(ctx, event, resolved)
	require.NoError(t, err)
	require.False(t, changed)
	rows, err := repo.ListAlertDeliveries(ctx, event.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	active, err := repo.GetResourceAlertEvent(ctx, rule.ID, "uid-a", true)
	require.NoError(t, err)
	require.Nil(t, active)
	require.NoError(t, repo.UpsertAlertResourceState(ctx, "uid-a", &service.OpsAlertRuleState{RuleID: rule.ID, ConsecutiveBreaches: 3, LastEvaluatedAt: now, DataStatus: "ok"}))
	require.NoError(t, repo.UpsertAlertResourceState(ctx, "uid-a", &service.OpsAlertRuleState{RuleID: rule.ID, ConsecutiveBreaches: 1, LastEvaluatedAt: now.Add(-time.Minute), DataStatus: "ok"}))
	state, err := repo.GetAlertResourceState(ctx, rule.ID, "uid-a")
	require.NoError(t, err)
	require.Equal(t, 3, state.ConsecutiveBreaches)
	rules, err := repo.ListAlertRules(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, rules)
}
func TestMonitoringAlertOutboxAtomicRollbackPostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	event := &service.OpsAlertEvent{RuleID: 1, Severity: "P1", Status: service.OpsAlertStatusFiring, FiredAt: time.Now(), Dimensions: map[string]any{"resource_key": "bad"}, Deliveries: []*service.OpsAlertDelivery{{Channel: "invalid", Transition: "firing", Payload: []byte(`{}`)}}}
	_, err := repo.CreateAlertEvent(ctx, event)
	require.Error(t, err)
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM ops_alert_events WHERE resource_key='bad'").Scan(&count))
	require.Zero(t, count)
}
func TestMonitoringAlertOutboxConcurrentClaimsPostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	ctx := context.Background()
	// Use separate backend connections sharing this test's isolated schema.
	var schema string
	require.NoError(t, db.QueryRow("SELECT current_schema()").Scan(&schema))
	parallelDB, err := sql.Open("postgres", os.Getenv("MODURELAY_MONITORING_TEST_DSN")+" search_path="+schema)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, parallelDB.Close()) })
	parallelDB.SetMaxOpenConns(4)
	repo := &opsRepository{db: parallelDB}
	for _, key := range []string{"a", "b", "c"} {
		_, err := repo.CreateAlertEvent(ctx, &service.OpsAlertEvent{RuleID: 1, Severity: "P1", Status: service.OpsAlertStatusFiring, FiredAt: time.Now(), Dimensions: map[string]any{"resource_key": key}, Deliveries: []*service.OpsAlertDelivery{{Channel: "email", Transition: "firing", Target: "ops@example.com", Payload: []byte(`{}`)}}})
		require.NoError(t, err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[int64]int{}
	errs := []error{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := repo.ClaimAlertDeliveries(ctx, time.Now().Add(time.Second), 1)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			}
			for _, row := range rows {
				seen[row.ID]++
			}
		}()
	}
	wg.Wait()
	require.Empty(t, errs)
	require.Len(t, seen, 3)
	for _, count := range seen {
		require.Equal(t, 1, count)
	}
}

func TestMonitoringDatabaseCapacityPersistencePostgres(t *testing.T) {
	db := monitoringTestDB(t)
	base, err := migrations.FS.ReadFile("033_ops_monitoring_vnext.sql")
	require.NoError(t, err)
	start := strings.Index(string(base), "CREATE TABLE IF NOT EXISTS ops_system_metrics (")
	end := strings.Index(string(base)[start:], "CREATE TABLE IF NOT EXISTS ops_job_heartbeats (")
	_, err = db.Exec(string(base)[start : start+end])
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE ops_system_metrics ADD COLUMN account_switch_count BIGINT NOT NULL DEFAULT 0, ADD COLUMN redis_conn_total INT, ADD COLUMN redis_conn_idle INT`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("275_ops_database_capacity.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	repo := &opsRepository{db: db}
	used, limit := int64(20), int64(100)
	pool := 10
	capacity := &service.OpsDatabaseCapacity{PostgreSQL: service.OpsDatabaseInstanceCapacity{State: "Available", Source: "standard PostgreSQL", Used: &used, Limit: &limit}, Redis: service.OpsDatabaseInstanceCapacity{State: "Unknown"}, PostgreSQLPoolLimit: &pool}
	require.NoError(t, repo.InsertSystemMetrics(context.Background(), &service.OpsInsertSystemMetricsInput{CreatedAt: time.Now(), WindowMinutes: 1, Capacity: capacity}))
	snap, err := repo.GetLatestSystemMetrics(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, snap.Capacity)
	require.EqualValues(t, 100, *snap.Capacity.PostgreSQL.Limit)
	require.Equal(t, 10, *snap.Capacity.PostgreSQLPoolLimit)
	require.Nil(t, snap.Capacity.Redis.Limit)
}

func TestMonitoringResourceSilenceAndHistoryIsolationPostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	now := time.Now()
	silence, err := repo.CreateAlertSilence(ctx, &service.OpsAlertSilence{RuleID: 1, ResourceKey: "uid-a", Until: now.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, "uid-a", silence.ResourceKey)
	yes, err := repo.IsAlertResourceSilenced(ctx, 1, "uid-a", now)
	require.NoError(t, err)
	require.True(t, yes)
	no, err := repo.IsAlertResourceSilenced(ctx, 1, "uid-b", now)
	require.NoError(t, err)
	require.False(t, no)
	for _, key := range []string{"uid-a", "uid-b"} {
		_, err := repo.CreateAlertEvent(ctx, &service.OpsAlertEvent{RuleID: 1, Severity: "P1", Status: service.OpsAlertStatusFiring, FiredAt: now, Dimensions: map[string]any{"resource_key": key}})
		require.NoError(t, err)
	}
	rows, err := repo.ListAlertEvents(ctx, &service.OpsAlertEventFilter{RuleID: 1, ResourceKey: "uid-a"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "uid-a", rows[0].Dimensions["resource_key"])
}

func TestMonitoringRelatedRulesShareIncidentAndCloseOncePostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	now := time.Now()
	otherRule, err := repo.CreateAlertRule(ctx, &service.OpsAlertRule{Name: "second incident rule", MetricType: "pod_memory_usage_percent", Operator: ">", Threshold: 90, Severity: "P0", Enabled: true, WindowMinutes: 1})
	require.NoError(t, err)
	event := func(rule int64, severity string) *service.OpsAlertEvent {
		return &service.OpsAlertEvent{RuleID: rule, Severity: severity, Status: service.OpsAlertStatusFiring, FiredAt: now, Dimensions: map[string]any{"resource_key": "same-pod-uid"}}
	}
	first, err := repo.CreateAlertEvent(ctx, event(1, "P1"))
	require.NoError(t, err)
	second, err := repo.CreateAlertEvent(ctx, event(otherRule.ID, "P0"))
	require.NoError(t, err)
	require.Equal(t, first.Dimensions["incident_id"], second.Dimensions["incident_id"])
	var status, severity string
	require.NoError(t, db.QueryRow("SELECT status,severity FROM ops_alert_incidents WHERE id=$1", first.Dimensions["incident_id"]).Scan(&status, &severity))
	require.Equal(t, "P0", severity)
	require.NoError(t, repo.UpdateAlertEventStatus(ctx, first.ID, service.OpsAlertStatusResolved, &now))
	require.NoError(t, db.QueryRow("SELECT status FROM ops_alert_incidents WHERE id=$1", first.Dimensions["incident_id"]).Scan(&status))
	require.Equal(t, "firing", status)
	require.NoError(t, repo.UpdateAlertEventStatus(ctx, second.ID, service.OpsAlertStatusResolved, &now))
	require.NoError(t, db.QueryRow("SELECT status FROM ops_alert_incidents WHERE id=$1", first.Dimensions["incident_id"]).Scan(&status))
	require.Equal(t, "resolved", status)
	next, err := repo.CreateAlertEvent(ctx, event(1, "P1"))
	require.NoError(t, err)
	require.NotEqual(t, first.Dimensions["incident_id"], next.Dimensions["incident_id"])
}
