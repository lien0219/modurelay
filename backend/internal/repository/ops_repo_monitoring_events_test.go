package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// Uses the shared disposable database helper and an isolated temporary schema.
func TestMonitoringEventsPersistencePostgres(t *testing.T) {
	db := monitoringTestDB(t)
	migration, err := migrations.FS.ReadFile("274_ops_kubernetes_events.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	// Migrations are forward-only and idempotent under the production runner.
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	extension, err := migrations.FS.ReadFile("277_ops_kubernetes_resource_contract.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(extension))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo := NewOpsRepository(db).(*opsRepository)
	now := time.Now().UTC().Truncate(time.Second)
	cluster := "events-test-" + time.Now().UTC().Format("20060102T150405.000000000")
	defer func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM ops_kubernetes_events WHERE cluster=$1", cluster)
	}()
	event := service.OpsKubernetesEvent{Cluster: cluster, Namespace: "ns", EventUID: "e1", Pod: "same-name", PodUID: "old-uid", Reason: "OOMKilled", Type: "Warning", Message: "first", FirstSeenAt: now.Add(-48 * time.Hour), LastSeenAt: now.Add(-24 * time.Hour), Count: 3}
	require.NoError(t, repo.UpsertMonitoringEvents(ctx, []service.OpsKubernetesEvent{event}))
	event.Count = 10
	event.Message = "latest"
	require.NoError(t, repo.UpsertMonitoringEvents(ctx, []service.OpsKubernetesEvent{event}))
	event.Count = 1
	event.LastSeenAt = now.Add(-25 * time.Hour)
	event.Message = "out of order"
	require.NoError(t, repo.UpsertMonitoringEvents(ctx, []service.OpsKubernetesEvent{event}))
	event.EventUID = "e2"
	event.PodUID = "new-uid"
	event.FirstSeenAt = now.Add(-time.Minute)
	event.LastSeenAt = now
	event.Message = "new pod"
	require.NoError(t, repo.UpsertMonitoringEvents(ctx, []service.OpsKubernetesEvent{event}))
	filter := service.OpsMonitoringEventsFilter{Cluster: cluster, Namespace: "ns", Pod: "same-name", Start: now.Add(-72 * time.Hour), End: now, Limit: 100}
	events, err := repo.ListMonitoringEvents(ctx, filter)
	require.NoError(t, err)
	require.Len(t, events, 2)
	filter.UID = "old-uid"
	events, err = repo.ListMonitoringEvents(ctx, filter)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, int64(10), events[0].Count)
	require.Equal(t, "latest", events[0].Message)
	require.NoError(t, repo.DeleteExpiredMonitoringEvents(ctx, cluster, now.Add(-time.Hour)))
	filter.UID = ""
	events, err = repo.ListMonitoringEvents(ctx, filter)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "new-uid", events[0].PodUID)
}
