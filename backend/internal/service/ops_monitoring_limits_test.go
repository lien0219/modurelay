//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpsMonitoringEffectivePodLimits(t *testing.T) {
	cases := []struct {
		name, spec, status, resource string
		value                        float64
		bounded, unknown             bool
	}{
		{"regular cpu", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"250m"}}},{"name":"b","resources":{"limits":{"cpu":"750m"}}}]}`, `{}`, "cpu", 1, true, false},
		{"sequential init maximum", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"1"}}}],"initContainers":[{"name":"i","resources":{"limits":{"cpu":"3"}}}],"overhead":{"cpu":"100m"}}`, `{}`, "cpu", 3.1, true, false},
		{"ordered restartable sidecars", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"1"}}}],"initContainers":[{"name":"s1","restartPolicy":"Always","resources":{"limits":{"cpu":"500m"}}},{"name":"i","resources":{"limits":{"cpu":"3"}}},{"name":"s2","restartPolicy":"Always","resources":{"limits":{"cpu":"1"}}}]}`, `{}`, "cpu", 3.5, true, false},
		{"pod budget overrides unlimited containers", `{"resources":{"limits":{"memory":"2Gi"}},"containers":[{"name":"a"}],"overhead":{"memory":"64Mi"}}`, `{}`, "memory", 2*(1<<30) + 64*(1<<20), true, false},
		{"unlimited regular container", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"1"}}},{"name":"b"}]}`, `{}`, "cpu", 0, false, false},
		{"unlimited init", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"1"}}}],"initContainers":[{"name":"i"}]}`, `{}`, "cpu", 0, false, false},
		{"actuated container limit", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"3"}}}]}`, `{"resize":"InProgress","containerStatuses":[{"name":"a","resources":{"limits":{"cpu":"1"}}}]}`, "cpu", 1, true, false},
		{"resize without status is unknown", `{"containers":[{"name":"a","resources":{"limits":{"cpu":"3"}}}]}`, `{"resize":"InProgress"}`, "cpu", 0, false, true},
		{"actuated pod budget already includes overhead", `{"resources":{"limits":{"memory":"2Gi"}},"containers":[{"name":"a"}],"overhead":{"memory":"64Mi"}}`, `{"resources":{"limits":{"memory":"1Gi"}}}`, "memory", 1 << 30, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var pod monitoringResourcePod
			require.NoError(t, json.Unmarshal([]byte(`{"spec":`+tc.spec+`,"status":`+tc.status+`}`), &pod))
			v, b, err := effectiveMonitoringPodLimit(pod, tc.resource)
			if tc.unknown {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.bounded, b)
			if b {
				require.NotNil(t, v)
				require.InDelta(t, tc.value, *v, 1e-6)
			} else {
				require.Nil(t, v)
			}
		})
	}
}
func TestOpsMonitoringResourceQuantities(t *testing.T) {
	for input, want := range map[string]float64{"500m": .5, "250000u": .25, "100000000n": .1, "1.5Gi": 1.5 * (1 << 30), "2e3": 2000, "1E-3": .001, "2M": 2e6} {
		got, err := monitoringResourceQuantity(input)
		require.NoError(t, err)
		require.InDelta(t, want, got, 1e-8)
	}
	for _, input := range []string{"-1", "NaN", "1invalid", "1e999", ""} {
		_, err := monitoringResourceQuantity(input)
		require.Error(t, err)
	}
}
func TestOpsMonitoringPodResourcePagination(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		if pages == 1 {
			fmt.Fprint(w, `{"metadata":{"continue":"next"},"items":[{"metadata":{"name":"a","namespace":"n","uid":"u1"},"spec":{"resources":{"limits":{"cpu":"2"}},"containers":[{"name":"main"}]}}]}`)
		} else {
			require.Equal(t, "next", r.URL.Query().Get("continue"))
			fmt.Fprint(w, `{"metadata":{},"items":[]}`)
		}
	}))
	defer server.Close()
	rows, err := fetchMonitoringPodLimits(context.Background(), server.Client(), server.URL, "secret", "c")
	require.NoError(t, err)
	require.Equal(t, 2, pages)
	require.Len(t, rows, 2)
	require.True(t, rows[0].Bounded)
	require.Equal(t, 2.0, *rows[0].Limit)
	require.False(t, rows[1].Bounded)
	require.Equal(t, "u1", rows[1].UID)
}
func TestOpsMonitoringUnlimitedDiffersFromUnknown(t *testing.T) {
	s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"pod":"a","uid":"u","limit_state":"unlimited"},"value":[%d,"0"]}]}}`, time.Now().Unix())
	})
	s.cfg.Ops.Monitoring.KubernetesResourcesEnabled = true
	result, err := s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "pod_cpu_usage_percent"})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringNotApplicable, result.State)
	require.Nil(t, result.Series[0].Points[0].Value)
	require.Equal(t, OpsMonitoringNotApplicable, result.Series[0].Points[0].State)
}
func TestOpsMonitoringExternalIngressValidationAndIsolation(t *testing.T) {
	cfg := config.OpsMonitoringConfig{Mode: "kubernetes", MaxQueryRangeHours: 744, ExternalIngressAdapter: "nginx", ExternalIngressJob: "edge"}
	q := OpsMonitoringQuery{Metric: "http_5xx_rate", Source: "external_ingress", StatusCode: "502"}
	_, err := normalizeMonitoringQuery(&q, cfg)
	require.NoError(t, err)
	expr, count, err := monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "nginx_ingress_controller_requests")
	require.Contains(t, expr, `status="502"`)
	require.NotContains(t, count, `status="502"`)
	require.NotContains(t, expr, "modurelay_http_requests_total")
	require.Contains(t, expr, `"external_ingress"`)
	for _, metric := range []string{"ttft_p95_ms", "qps_started", "pod_cpu_cores"} {
		q := OpsMonitoringQuery{Metric: metric, Source: "external_ingress"}
		_, err := normalizeMonitoringQuery(&q, cfg)
		require.Error(t, err)
	}
	q = OpsMonitoringQuery{Metric: "qps", Source: "external_ingress", Route: "messages"}
	_, err = normalizeMonitoringQuery(&q, cfg)
	require.Error(t, err)
	cfg.ExternalIngressAdapter = "normalized"
	_, err = normalizeMonitoringQuery(&q, cfg)
	require.NoError(t, err)
	expr, _, err = monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "modurelay_external_http_requests_total")
	require.Contains(t, expr, `route="messages"`)
}

type monitoringLimitSnapshotRepo struct {
	OpsRepository
	rows       []OpsMonitoringPodLimit
	heartbeats []*OpsJobHeartbeat
}

func (r *monitoringLimitSnapshotRepo) ReplaceMonitoringPodLimits(context.Context, string, string, []OpsMonitoringPodLimit) error {
	return nil
}
func (r *monitoringLimitSnapshotRepo) ListMonitoringPodLimits(_ context.Context, _ string, since time.Time) ([]OpsMonitoringPodLimit, error) {
	out := []OpsMonitoringPodLimit{}
	for _, row := range r.rows {
		if !row.ObservedAt.Before(since) {
			out = append(out, row)
		}
	}
	return out, nil
}
func (r *monitoringLimitSnapshotRepo) ListJobHeartbeats(context.Context) ([]*OpsJobHeartbeat, error) {
	return r.heartbeats, nil
}
func TestOpsMonitoringLimitMetricsRejectFailedAndStaleSnapshots(t *testing.T) {
	now := time.Now().UTC()
	value := 2.0
	repo := &monitoringLimitSnapshotRepo{rows: []OpsMonitoringPodLimit{{Cluster: "c", Namespace: "n", Pod: "api", UID: "u", Resource: "cpu", Limit: &value, Bounded: true, ObservedAt: now}}}
	s := &OpsService{cfg: &config.Config{Ops: config.OpsConfig{Enabled: true, Monitoring: config.OpsMonitoringConfig{Mode: "kubernetes", ClusterName: "c", KubernetesResourcesEnabled: true}}}, opsRepo: repo}
	repo.heartbeats = []*OpsJobHeartbeat{{JobName: monitoringLimitsJobName(s.monitoringConfig()), LastSuccessAt: &now}}
	rendered := s.MonitoringPodLimitMetrics(context.Background())
	require.Contains(t, rendered, "modurelay_kubernetes_pod_effective_limit")
	require.Contains(t, rendered, `uid="u"`)
	failed := now.Add(time.Second)
	repo.heartbeats[0].LastErrorAt = &failed
	require.Empty(t, s.MonitoringPodLimitMetrics(context.Background()), "failed collector may not re-expose a prior healthy snapshot")
	repo.heartbeats[0].LastErrorAt = nil
	old := now.Add(-3 * time.Minute)
	repo.heartbeats[0].LastSuccessAt = &old
	require.Empty(t, s.MonitoringPodLimitMetrics(context.Background()))
	repo.heartbeats[0].LastSuccessAt = &now
	repo.rows[0].ObservedAt = old
	require.Empty(t, s.MonitoringPodLimitMetrics(context.Background()))
}
