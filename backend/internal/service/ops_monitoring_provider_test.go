//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func monitoringTestService(t *testing.T, handler http.HandlerFunc) *OpsService {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		// Normalize for the fixture assertions below; production sends the
		// expression in the request body to avoid long URI proxy failures.
		r.URL.RawQuery = r.Form.Encode()
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return &OpsService{cfg: &config.Config{Ops: config.OpsConfig{Enabled: true, Monitoring: config.OpsMonitoringConfig{Mode: "kubernetes", PrometheusURL: server.URL, BearerToken: "test-secret", TimeoutSeconds: 2, MaxQueryRangeHours: 744}}}}
}

func TestOpsMonitoringCapabilitiesAndStandaloneFallback(t *testing.T) {
	s := &OpsService{cfg: &config.Config{Ops: config.OpsConfig{Enabled: true, Monitoring: config.OpsMonitoringConfig{Mode: "standalone"}}}}
	cap, err := s.GetMonitoringCapabilities(context.Background())
	require.NoError(t, err)
	require.Equal(t, "standalone", cap.Mode)
	require.False(t, cap.Kubernetes)
	require.False(t, cap.History)
	result, err := s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "pod_cpu_cores"})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringNotApplicable, result.State)
	result, err = s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "qps"})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringUnknown, result.State)
	require.Empty(t, result.Series)
	encoded, err := json.Marshal(cap)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "bearer_token")
	require.NotContains(t, string(encoded), "prometheus_url")
}

func TestOpsMonitoringRangePreservesGapsAndNaN(t *testing.T) {
	start := time.Now().Add(-time.Hour).UTC().Truncate(time.Minute)
	s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/query_range", r.URL.Path)
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		require.Contains(t, r.URL.Query().Get("query"), "rate(modurelay_http_requests_total")
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"service":"api","secret":"must-not-leak"},"values":[[%d,"0"],[%d,"NaN"]]}]}}`, start.Unix(), start.Add(30*time.Second).Unix())
	})
	result, err := s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "qps", Start: start, End: start.Add(45 * time.Second), StepSeconds: 15})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringAvailable, result.State)
	require.Len(t, result.Series, 1)
	points := result.Series[0].Points
	require.Len(t, points, 4)
	require.NotNil(t, points[0].Value)
	require.Zero(t, *points[0].Value)
	for _, point := range points[1:] {
		require.Nil(t, point.Value)
		require.Equal(t, OpsMonitoringUnknown, point.State)
	}
	require.NotContains(t, result.Series[0].Labels, "secret")
}

func TestOpsMonitoringLowSamplesAndInstantAlignment(t *testing.T) {
	var firstTime string
	s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
		at := r.URL.Query().Get("time")
		require.NotEmpty(t, at)
		if firstTime == "" {
			firstTime = at
		}
		require.Equal(t, firstTime, at)
		timestamp, err := time.Parse(time.RFC3339Nano, at)
		require.NoError(t, err)
		value := "1500"
		if strings.Contains(r.URL.Query().Get("query"), "_count") {
			value = "4"
		}
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"service":"api","source":"app"},"value":[%d,%q]}]}}`, timestamp.Unix(), value)
	})
	result, err := s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "http_p95_ms", MinRequests: 5})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringInsufficientData, result.State)
	require.Nil(t, result.Series[0].Points[0].Value)
}

func TestOpsMonitoringUnknownDoesNotExposeUpstreamOrZero(t *testing.T) {
	for _, body := range []string{`{"status":"error","error":"sensitive-token"}`, `{"status":"success","data":{"resultType":"vector","result":[]}}`, `{"status":"success","warnings":["partial response sensitive-token"],"data":{"resultType":"vector","result":[]}}`} {
		t.Run(body, func(t *testing.T) {
			s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			result, err := s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "qps"})
			require.NoError(t, err)
			require.Equal(t, OpsMonitoringUnknown, result.State)
			require.Empty(t, result.Series)
			encoded, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "sensitive-token")
		})
	}
}

func TestOpsMonitoringRejectsRedirectAndUnboundedQueries(t *testing.T) {
	targetReached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetReached = true }))
	defer target.Close()
	s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	result, err := s.QueryMonitoring(context.Background(), OpsMonitoringQuery{Metric: "qps"})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringUnknown, result.State)
	require.False(t, targetReached)
	now := time.Now().UTC()
	for _, q := range []OpsMonitoringQuery{{Metric: "up"}, {Metric: "qps", Window: "[1m]) or vector(1)"}, {Metric: "qps", Source: "all"}, {Metric: "qps", Namespace: "foo\nbar"}, {Metric: "qps", Start: now.Add(-32 * 24 * time.Hour), End: now}, {Metric: "qps", Start: now.Add(-30 * 24 * time.Hour), End: now, StepSeconds: 15}, {Metric: "hpa_at_max", Service: "api"}, {Metric: "deployment_ready_replicas", Pod: "api-1"}} {
		_, err := s.QueryMonitoring(context.Background(), q)
		require.ErrorIs(t, err, ErrOpsMonitoringQuery)
	}
}

func TestOpsMonitoringPromQLContracts(t *testing.T) {
	q := OpsMonitoringQuery{Window: "5m", Source: "app", Namespace: `x"} or vector(1) #`, Service: "api"}
	for _, metric := range opsMonitoringMetrics {
		q.Metric = metric.ID
		expr, _, err := monitoringPromQL(q)
		require.NoError(t, err, metric.ID)
		require.NotEmpty(t, expr)
		require.NotContains(t, expr, "modurelay_cluster_", metric.ID)
		require.Contains(t, expr, "namespace="+strconv.Quote(q.Namespace), metric.ID)
	}
	q.Metric = "http_p99_ms"
	expr, count, err := monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "histogram_quantile(0.99, (sum by (cluster,namespace,service,source,le)")
	require.Contains(t, expr, "max by (cluster,namespace,service,pod,uid,instance,source,method,route,protocol,le)")
	require.Contains(t, count, "_duration_seconds_count")
	q.Metric = "pod_cpu_usage_percent"
	expr, _, err = monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "group_left (uid)")
	require.Contains(t, expr, "kube_pod_created")
	require.Contains(t, expr, "kube_pod_container_info")
	require.Contains(t, expr, "kube_pod_container_resource_limits")
	q.Metric = "http_4xx_rate"
	q.Exclude429 = true
	expr, _, err = monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, `status_code!="429"`)
	require.Contains(t, expr, "0 * (sum by")
}

func TestOpsMonitoringSourceHealthIndependentOfMissingSamples(t *testing.T) {
	for _, healthy := range []bool{false, true} {
		t.Run(strconv.FormatBool(healthy), func(t *testing.T) {
			s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "vector(1)", r.URL.Query().Get("query"))
				if !healthy {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[%d,"1"]}]}}`, time.Now().Unix())
			})
			samples, err := s.EvaluateMonitoringAlertMetric(context.Background(), "monitoring_source_unavailable", nil, time.Now().Add(-time.Minute), time.Now())
			require.NoError(t, err)
			require.Len(t, samples, 1)
			expected := float64(1)
			if healthy {
				expected = 0
			}
			require.Equal(t, expected, *samples[0].Value)
		})
	}
}

func TestOpsMonitoringAlertResourceIdentityAndErrorCount(t *testing.T) {
	s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		value := "100"
		if strings.HasPrefix(q, "100 *") {
			value = "5"
		} else if strings.Contains(q, "status_code=~") {
			value = "5"
		}
		at, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("time"))
		require.NoError(t, err)
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"cluster":"c","namespace":"n","service":"api","source":"app"},"value":[%d,%q]}]}}`, at.Unix(), value)
	})
	samples, err := s.EvaluateMonitoringAlertMetric(context.Background(), "http_5xx_rate", nil, time.Now().Add(-5*time.Minute), time.Now())
	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, int64(100), samples[0].RequestCount)
	require.Equal(t, float64(5), *samples[0].ErrorCount)
	require.Equal(t, float64(5), *samples[0].Value)
	require.Contains(t, samples[0].ResourceKey, `"service":"api"`)
}

func TestOpsMonitoringTTFTAlertUsesSuccessfulTTFTSampleCount(t *testing.T) {
	s := monitoringTestService(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		// High HTTP traffic is unrelated to how many requests produced a
		// successful first output. The adapter must not query that counter.
		require.NotContains(t, query, "modurelay_http_requests_total")
		value := "2500"
		if strings.Contains(query, "modurelay_http_ttft_seconds_count") {
			value = "2"
		}
		at, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("time"))
		require.NoError(t, err)
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"service":"api","source":"app"},"value":[%d,%q]}]}}`, at.Unix(), value)
	})
	samples, err := s.EvaluateMonitoringAlertMetric(context.Background(), "ttft_p99_ms", nil, time.Now().Add(-5*time.Minute), time.Now())
	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, int64(2), samples[0].RequestCount)
	require.Equal(t, float64(2500), *samples[0].Value)
}

func TestOpsMonitoringPodHTTPQueriesKeepLifecycleIdentity(t *testing.T) {
	q := OpsMonitoringQuery{Metric: "http_p99_ms", Window: "5m", Source: "app", Pod: "api-0"}
	expr, count, err := monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "sum by (cluster,namespace,service,source,pod,uid,le)")
	require.Contains(t, count, "sum by (cluster,namespace,service,source,pod,uid)")
	q.Metric = "http_5xx_rate"
	expr, _, err = monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "or on (cluster,namespace,service,source,pod,uid)")
	q.Metric = "pod_cpu_throttling_percent"
	expr, _, err = monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, expr, "container_start_time_seconds")
	require.Contains(t, expr, "kube_pod_created")
	require.Contains(t, expr, ">= 300")
}

func TestOpsMonitoringHTTPDimensionValidation(t *testing.T) {
	cfg := config.OpsMonitoringConfig{Mode: "kubernetes", MaxQueryRangeHours: 744}
	for _, q := range []OpsMonitoringQuery{
		{Metric: "http_p95_ms", Protocol: "sse", Route: "messages"},
		{Metric: "http_5xx_rate", StatusCode: "503"},
		{Metric: "request_count_started", Route: "messages"},
	} {
		_, err := normalizeMonitoringQuery(&q, cfg)
		require.NoError(t, err)
	}
	for _, q := range []OpsMonitoringQuery{
		{Metric: "qps", Protocol: "ftp"}, {Metric: "qps", Route: "/users/123"},
		{Metric: "qps", StatusCode: "5.."}, {Metric: "http_p99_ms", StatusCode: "200"},
		{Metric: "qps_started", Protocol: "sse"}, {Metric: "pod_info", Route: "messages"},
	} {
		_, err := normalizeMonitoringQuery(&q, cfg)
		require.ErrorIs(t, err, ErrOpsMonitoringQuery)
	}
	q := OpsMonitoringQuery{Metric: "http_p99_ms", Window: "5m", Source: "app", Protocol: "sse", Route: "messages"}
	value, count, err := monitoringPromQL(q)
	require.NoError(t, err)
	for _, expr := range []string{value, count} {
		require.Contains(t, expr, `protocol="sse"`)
		require.Contains(t, expr, `route="messages"`)
	}
	q = OpsMonitoringQuery{Metric: "request_count_started", Window: "15m", Source: "app", Route: "messages"}
	value, _, err = monitoringPromQL(q)
	require.NoError(t, err)
	require.Contains(t, value, "increase(modurelay_http_requests_started_total")
	require.Contains(t, value, "[15m]")
}
