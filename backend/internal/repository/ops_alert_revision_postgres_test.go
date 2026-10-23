package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonitoringSilenceExpiryOutboxSupplementIsDurablePostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	event, err := repo.CreateAlertEvent(ctx, &service.OpsAlertEvent{RuleID: 1, Status: service.OpsAlertStatusFiring, Severity: "P1", FiredAt: now, Dimensions: map[string]any{"resource_key": "silenced-pod"}})
	require.NoError(t, err)
	require.NotNil(t, event)
	makeTasks := func() []*service.OpsAlertDelivery {
		return []*service.OpsAlertDelivery{{Channel: "webhook", Transition: "firing", Target: "https://hooks.example.com/alert", Payload: []byte(`{"version":1}`)}}
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); failures <- repo.EnsureFiringAlertDeliveries(ctx, event.ID, makeTasks()) }()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	deliveries, err := repo.ListAlertDeliveries(ctx, event.ID)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	claimed, err := repo.ClaimAlertDeliveries(ctx, now.Add(time.Minute), 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, repo.CompleteAlertDelivery(ctx, claimed[0], nil, now))
	// A different repository instance simulates the next leader after restart.
	restarted := &opsRepository{db: db}
	require.NoError(t, restarted.EnsureFiringAlertDeliveries(ctx, event.ID, makeTasks()))
	deliveries, err = restarted.ListAlertDeliveries(ctx, event.ID)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, "sent", deliveries[0].Status)
	event.Status = service.OpsAlertStatusResolved
	event.ResolvedAt = &now
	changed, err := repo.TransitionAlertEvent(ctx, event, nil)
	require.NoError(t, err)
	require.True(t, changed)
	later := makeTasks()
	later[0].Target = "https://hooks.example.com/new"
	require.NoError(t, restarted.EnsureFiringAlertDeliveries(ctx, event.ID, later))
	deliveries, err = restarted.ListAlertDeliveries(ctx, event.ID)
	require.NoError(t, err)
	require.Len(t, deliveries, 1, "a concurrent recovery must not gain a late firing intent")
}

func TestMonitoringRuleRevisionRetiresOldWindowAndFencesStaleEvaluatorPostgres(t *testing.T) {
	db := monitoringAlertTestDB(t)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	rule, err := repo.CreateAlertRule(ctx, &service.OpsAlertRule{Name: "window revision", MetricType: "http_p99_ms", Operator: ">", Threshold: 100, Severity: "P1", Enabled: true, WindowMinutes: 1, SustainedMinutes: 5, Filters: map[string]any{"webhook_urls": []string{"https://hooks.example.com/a"}}})
	require.NoError(t, err)
	require.EqualValues(t, 1, rule.EvaluationRevision)
	state := &service.OpsAlertRuleState{RuleID: rule.ID, RuleRevision: rule.EvaluationRevision, ConsecutiveBreaches: 4, LastEvaluatedAt: now, DataStatus: "ok"}
	require.NoError(t, repo.UpsertAlertRuleState(ctx, state))
	require.NoError(t, repo.UpsertAlertResourceState(ctx, "uid-a", state))
	event, err := repo.CreateAlertEvent(ctx, &service.OpsAlertEvent{RuleID: rule.ID, RuleRevision: rule.EvaluationRevision, Status: service.OpsAlertStatusFiring, Severity: "P1", FiredAt: now, Dimensions: map[string]any{"resource_key": "uid-a"}})
	require.NoError(t, err)
	rule.Filters["silenced_until"] = now.Add(time.Hour).Format(time.RFC3339)
	rule.Filters["webhook_urls"] = []string{"https://hooks.example.com/b"}
	rule, err = repo.UpdateAlertRule(ctx, rule)
	require.NoError(t, err)
	require.EqualValues(t, 1, rule.EvaluationRevision, "silence/targets are delivery-only changes")
	kept, err := repo.GetAlertResourceState(ctx, rule.ID, "uid-a")
	require.NoError(t, err)
	require.Equal(t, 4, kept.ConsecutiveBreaches)
	rule.WindowMinutes = 5
	rule, err = repo.UpdateAlertRule(ctx, rule)
	require.NoError(t, err)
	require.EqualValues(t, 2, rule.EvaluationRevision)
	cleared, err := repo.GetAlertResourceState(ctx, rule.ID, "uid-a")
	require.NoError(t, err)
	require.Nil(t, cleared)
	old, err := repo.GetAlertEventByID(ctx, event.ID)
	require.NoError(t, err)
	require.Equal(t, service.OpsAlertStatusManualResolved, old.Status)
	require.Equal(t, "rule_reconfigured", old.Dimensions["resolution_reason"])
	tasks, err := repo.ListAlertDeliveries(ctx, event.ID)
	require.NoError(t, err)
	require.Empty(t, tasks, "a rule edit must not fabricate a healthy recovery")
	// An already-running old evaluator cannot resurrect old progress or events.
	state.LastEvaluatedAt = now.Add(time.Minute)
	require.NoError(t, repo.UpsertAlertRuleState(ctx, state))
	require.NoError(t, repo.UpsertAlertResourceState(ctx, "uid-a", state))
	cleared, err = repo.GetAlertResourceState(ctx, rule.ID, "uid-a")
	require.NoError(t, err)
	require.Nil(t, cleared)
	global, err := repo.GetAlertRuleState(ctx, rule.ID)
	require.NoError(t, err)
	require.Nil(t, global)
	_, err = repo.CreateAlertEvent(ctx, &service.OpsAlertEvent{RuleID: rule.ID, RuleRevision: 1, Status: service.OpsAlertStatusFiring, Severity: "P1", FiredAt: now, Dimensions: map[string]any{"resource_key": "uid-a"}})
	require.ErrorContains(t, err, "changed")
	state.RuleRevision = 2
	require.NoError(t, repo.UpsertAlertResourceState(ctx, "uid-a", state))
	missing, err := repo.MarkMissingAlertResources(ctx, rule.ID, 1, nil, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.False(t, missing)
	kept, err = repo.GetAlertResourceState(ctx, rule.ID, "uid-a")
	require.NoError(t, err)
	require.Equal(t, 4, kept.ConsecutiveBreaches)
	fresh, err := repo.CreateAlertEvent(ctx, &service.OpsAlertEvent{RuleID: rule.ID, RuleRevision: 2, Status: service.OpsAlertStatusFiring, Severity: "P1", FiredAt: now, Dimensions: map[string]any{"resource_key": "uid-a"}})
	require.NoError(t, err)
	require.NotEqual(t, event.ID, fresh.ID)
	require.NotEqual(t, event.Dimensions["incident_id"], fresh.Dimensions["incident_id"])
}
