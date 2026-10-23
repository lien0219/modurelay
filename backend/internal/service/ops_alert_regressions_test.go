//go:build unit

package service

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"
	"time"
)

type silenceRegressionRepo struct {
	*durableAlertTestRepo
	silenceUntil time.Time
}

func (r *silenceRegressionRepo) IsAlertResourceSilenced(_ context.Context, _ int64, _ string, now time.Time) (bool, error) {
	return now.Before(r.silenceUntil), nil
}

func TestSilenceExpiryEnqueuesFirstFiringOnceAcrossEvaluatorRestart(t *testing.T) {
	for _, scope := range []string{"rule", "resource"} {
		t.Run(scope, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 10, 23, 5, 0, 0, 0, time.UTC)
			repo := &silenceRegressionRepo{durableAlertTestRepo: &durableAlertTestRepo{stubOpsRepo: &stubOpsRepo{}}}
			value := 80.0
			rule := &OpsAlertRule{ID: 99, Name: "silenced", MetricType: "pod_cpu_usage_percent", Operator: ">", Threshold: 50, Severity: "P1", Filters: map[string]any{"webhook_urls": []string{"https://hooks.example.com/alert"}}}
			if scope == "rule" {
				rule.Filters["silenced_until"] = now.Add(time.Minute).Format(time.RFC3339)
			} else {
				repo.silenceUntil = now.Add(time.Minute)
			}
			newSvc := func() *OpsAlertEvaluatorService {
				return &OpsAlertEvaluatorService{opsRepo: repo, opsService: &OpsService{}, monitoringAlertSamples: func(context.Context, string, map[string]any, time.Time, time.Time) ([]OpsMonitoringAlertSample, error) {
					return []OpsMonitoringAlertSample{{ResourceKey: "pod-uid", Value: &value}}, nil
				}}
			}
			newSvc().evaluateMonitoringAlertRule(ctx, nil, rule, now.Add(-time.Minute), now, now, time.Minute)
			require.Len(t, repo.created, 1)
			require.Empty(t, repo.created[0].Deliveries)
			require.Empty(t, repo.deliveries)
			for i := 1; i <= 3; i++ {
				after := now.Add(time.Duration(i) * time.Minute)
				newSvc().evaluateMonitoringAlertRule(ctx, nil, rule, after.Add(-time.Minute), after, after, time.Minute)
			}
			require.Len(t, repo.created, 1)
			require.Len(t, repo.deliveries, 1, "repeated evaluation and leader restart must not duplicate notification intents")
		})
	}
}
func TestStartedMetricsRespectMinimumRequests(t *testing.T) {
	for _, metric := range []string{"qps", "qps_started", "rpm_started", "request_count_started"} {
		t.Run(metric, func(t *testing.T) {
			value := 1.0
			rule := &OpsAlertRule{MetricType: metric, MinRequests: 100}
			require.Equal(t, alertMetricInsufficient, monitoringAlertSampleStatus(rule, OpsMonitoringAlertSample{Value: &value, RequestCount: 1}))
			require.Equal(t, alertMetricOK, monitoringAlertSampleStatus(rule, OpsMonitoringAlertSample{Value: &value, RequestCount: 100}))
		})
	}
}
func TestMonitoringBusinessDimensionsRejectedForNewAndStoredRules(t *testing.T) {
	for _, filters := range []map[string]any{{"platform": "openai"}, {"group_id": float64(42)}, {"region": "us"}} {
		require.Error(t, ValidateOpsAlertFilters("http_5xx_rate", filters))
		repo := &durableAlertTestRepo{stubOpsRepo: &stubOpsRepo{}}
		called := false
		evaluator := &OpsAlertEvaluatorService{opsRepo: repo, opsService: &OpsService{}, monitoringAlertSamples: func(context.Context, string, map[string]any, time.Time, time.Time) ([]OpsMonitoringAlertSample, error) {
			called = true
			return nil, nil
		}}
		now := time.Now()
		rule := &OpsAlertRule{ID: 1, MetricType: "http_5xx_rate", Filters: filters}
		require.False(t, evaluator.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute))
		require.False(t, called, "legacy invalid rules must not issue an unfiltered query")
		require.Equal(t, OpsAlertDataStatusInvalidRule, repo.states[1].DataStatus)
		require.Empty(t, repo.transitions)
	}
}

func TestMissingSamplesBreakContinuousEvidenceWithoutResolving(t *testing.T) {
	repo := &durableAlertTestRepo{stubOpsRepo: &stubOpsRepo{}}
	value := 80.0
	rule := &OpsAlertRule{ID: 8, Name: "continuous", MetricType: "pod_cpu_usage_percent", Operator: ">", Threshold: 50, Severity: "P1", SustainedMinutes: 2}
	now := time.Date(2026, 10, 23, 5, 0, 0, 0, time.UTC)
	samples := []OpsMonitoringAlertSample{{ResourceKey: "uid", Value: &value}}
	newSvc := func() *OpsAlertEvaluatorService {
		return &OpsAlertEvaluatorService{opsRepo: repo, opsService: &OpsService{}, monitoringAlertSamples: func(context.Context, string, map[string]any, time.Time, time.Time) ([]OpsMonitoringAlertSample, error) {
			return samples, nil
		}}
	}
	evaluate := func(at time.Time) {
		newSvc().evaluateMonitoringAlertRule(context.Background(), nil, rule, at.Add(-time.Minute), at, at, time.Minute)
	}
	evaluate(now)
	evaluate(now.Add(time.Minute))
	require.Equal(t, 2, repo.resources["uid"].ConsecutiveBreaches)
	// A new leader after a gap must not count the unobserved period as breaching.
	evaluate(now.Add(3 * time.Minute))
	require.Equal(t, 1, repo.resources["uid"].ConsecutiveBreaches)
	require.Empty(t, repo.created)
	evaluate(now.Add(4 * time.Minute))
	evaluate(now.Add(5 * time.Minute))
	require.Len(t, repo.created, 1)
	samples = nil
	evaluate(now.Add(6 * time.Minute))
	require.Equal(t, 0, repo.resources["uid"].ConsecutiveBreaches)
	require.Equal(t, OpsAlertDataStatusUnavailable, repo.resources["uid"].DataStatus)
	require.Equal(t, OpsAlertStatusFiring, repo.created[0].Status)
	require.Empty(t, repo.transitions)
}

func TestRuleRevisionRequiresFreshSustainedEvidenceAndFingerprint(t *testing.T) {
	repo := &durableAlertTestRepo{stubOpsRepo: &stubOpsRepo{}}
	value := 80.0
	rule := &OpsAlertRule{ID: 8, EvaluationRevision: 1, Name: "continuous", MetricType: "pod_cpu_usage_percent", Operator: ">", Threshold: 50, Severity: "P1", SustainedMinutes: 1, WindowMinutes: 1}
	now := time.Date(2026, 10, 23, 5, 0, 0, 0, time.UTC)
	svc := &OpsAlertEvaluatorService{opsRepo: repo, opsService: &OpsService{}, monitoringAlertSamples: func(context.Context, string, map[string]any, time.Time, time.Time) ([]OpsMonitoringAlertSample, error) {
		return []OpsMonitoringAlertSample{{ResourceKey: "uid", Value: &value}}, nil
	}}
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-time.Minute), now, now, time.Minute)
	rule.EvaluationRevision = 2
	rule.WindowMinutes = 5
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-5*time.Minute), now, now, time.Minute)
	require.Empty(t, repo.created, "the previous window's sample must not satisfy the new window's duration")
	require.Equal(t, 1, repo.resources["uid"].ConsecutiveBreaches)
	now = now.Add(time.Minute)
	svc.evaluateMonitoringAlertRule(context.Background(), nil, rule, now.Add(-5*time.Minute), now, now, time.Minute)
	require.Len(t, repo.created, 1)
	require.EqualValues(t, 2, repo.created[0].Dimensions["rule_revision"])
	require.Equal(t, 5, repo.created[0].Dimensions["window_minutes"])
	require.NotEmpty(t, repo.created[0].Dimensions["fingerprint"])
}

func TestStartedAlertSamplesUseArrivalCounterAndMatchingWindow(t *testing.T) {
	for _, metric := range []string{"qps_started", "rpm_started", "request_count_started"} {
		t.Run(metric, func(t *testing.T) {
			countQueries := 0
			svc := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query().Get("query")
				require.Contains(t, q, "modurelay_http_requests_started_total")
				require.NotContains(t, q, "modurelay_http_requests_total[")
				value := "2"
				if strings.Contains(q, "increase(") && strings.Contains(q, "[5m]") {
					countQueries++
					value = "7"
				}
				at, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("time"))
				require.NoError(t, err)
				fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"cluster":"local","namespace":"default","service":"api"},"value":[%d,"%s"]}]}}`, at.Unix(), value)
			})
			end := time.Now().UTC().Truncate(time.Second)
			samples, err := svc.EvaluateMonitoringAlertMetric(context.Background(), metric, nil, end.Add(-5*time.Minute), end)
			require.NoError(t, err)
			require.Len(t, samples, 1)
			require.EqualValues(t, 7, samples[0].RequestCount)
			require.Greater(t, countQueries, 0)
		})
	}
}

func TestOperationalBillingAlertsRequireFreshAvailableCounts(t *testing.T) {
	for _, metric := range []string{"billing_failed_settlements", "billing_compensation_backlog", "billing_ledger_mismatches"} {
		t.Run(metric, func(t *testing.T) {
			now := time.Now()
			count := int64(3)
			state := &OpsBillingReconciliation{CheckedAt: &now, OperationalState: "Available", OperationalCheckedAt: &now, FailedSettlements: &count, CompensationBacklog: &count, LedgerMismatches: &count}
			evaluator := &OpsAlertEvaluatorService{opsService: &OpsService{opsRepo: &billingReconciliationRepoStub{snapshot: state}}}
			rule := &OpsAlertRule{MetricType: metric}
			value, status := evaluator.computeRuleMetric(context.Background(), rule, nil, now.Add(-time.Minute), now, "", nil)
			require.Equal(t, alertMetricOK, status)
			require.Equal(t, 3.0, value)
			state.OperationalState = "Unknown"
			_, status = evaluator.computeRuleMetric(context.Background(), rule, nil, now.Add(-time.Minute), now, "", nil)
			require.Equal(t, alertMetricUnavailable, status)
		})
	}
}

func TestDatabaseRolePoolAlertsKeepBudgetsSeparate(t *testing.T) {
	pools := []OpsDatabasePoolStats{{Role: "primary", Active: 1, Limit: 100}, {Role: "advisory_lock", Active: 8, Idle: 2, Limit: 10}, {Role: "advisory_lock", Active: 10, Limit: 10}, {Role: "read_replica", Active: 3, Idle: 2, Limit: 20}}
	value, status := databaseRolePoolUsage(pools, "advisory_lock")
	require.Equal(t, alertMetricOK, status)
	require.Equal(t, 100.0, value)
	value, status = databaseRolePoolUsage(pools, "read_replica")
	require.Equal(t, alertMetricOK, status)
	require.Equal(t, 25.0, value)
	_, status = databaseRolePoolUsage(pools, "missing")
	require.Equal(t, alertMetricUnavailable, status)
	pools[1].Limit = 0
	_, status = databaseRolePoolUsage(pools, "advisory_lock")
	require.Equal(t, alertMetricUnavailable, status)
}
