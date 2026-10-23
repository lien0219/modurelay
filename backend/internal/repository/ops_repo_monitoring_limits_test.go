package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMonitoringKubernetesResourceContractPostgres(t *testing.T) {
	db := monitoringTestDB(t)
	for _, name := range []string{"274_ops_kubernetes_events.sql", "277_ops_kubernetes_resource_contract.sql"} {
		sql, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = db.Exec(string(sql))
		require.NoError(t, err)
	}
	repo, ok := NewOpsRepository(db).(*opsRepository)
	require.True(t, ok)
	ctx := context.Background()
	now := time.Now().UTC()
	value := 2.5
	rows := []service.OpsMonitoringPodLimit{{Cluster: "test", Namespace: "n", Pod: "same", UID: "old", Resource: "cpu", Limit: &value, Bounded: true, ObservedAt: now}, {Cluster: "test", Namespace: "n", Pod: "same", UID: "old", Resource: "memory", Bounded: false, ObservedAt: now}}
	require.NoError(t, repo.ReplaceMonitoringPodLimits(ctx, "test", "n", rows))
	got, err := repo.ListMonitoringPodLimits(ctx, "test", now.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.NotNil(t, got[0].Limit)
	require.Nil(t, got[1].Limit)
	rows[0].UID = "new"
	rows = rows[:1]
	require.NoError(t, repo.ReplaceMonitoringPodLimits(ctx, "test", "n", rows))
	got, err = repo.ListMonitoringPodLimits(ctx, "test", now.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "new", got[0].UID)
	bad := append(rows, service.OpsMonitoringPodLimit{Cluster: "other"})
	require.Error(t, repo.ReplaceMonitoringPodLimits(ctx, "test", "n", bad))
	got, err = repo.ListMonitoringPodLimits(ctx, "test", now.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, got, 1, "snapshot must roll back atomically")
	got, err = repo.ListMonitoringPodLimits(ctx, "test", now.Add(time.Minute))
	require.NoError(t, err)
	require.Empty(t, got, "stale resources must be absent")
	for _, kind := range []string{"HorizontalPodAutoscaler", "Deployment", "ReplicaSet"} {
		event := service.OpsKubernetesEvent{Cluster: "test", Namespace: "n", EventUID: "e-" + kind, ObjectKind: kind, ObjectName: "same", ObjectUID: "uid-" + kind, Reason: "SuccessfulRescale", Count: 1, FirstSeenAt: now, LastSeenAt: now}
		require.NoError(t, repo.UpsertMonitoringEvents(ctx, []service.OpsKubernetesEvent{event}))
	}
	events, err := repo.ListMonitoringEvents(ctx, service.OpsMonitoringEventsFilter{Cluster: "test", ObjectKind: "HorizontalPodAutoscaler", Start: now.Add(-time.Minute), End: now.Add(time.Minute)})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Empty(t, events[0].Pod)
	require.Equal(t, "uid-HorizontalPodAutoscaler", events[0].ObjectUID)
}
