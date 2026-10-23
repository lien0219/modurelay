package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpmetrics"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type monitoringMetricsSourceStub struct {
	metricsSourceStub
	capacity *service.OpsDatabaseCapacity
}

func (s *monitoringMetricsSourceStub) InstancePoolCounters() map[string]float64 {
	return map[string]float64{"modurelay_db_pool_wait_total": 7, "unexpected_secret_label": 99}
}
func (s *monitoringMetricsSourceStub) BillingObservationCounters() map[string]float64 {
	return map[string]float64{"modurelay_billing_observation_missed_total": 3, "modurelay_billing_observation_unpersisted": 1, "modurelay_billing_observation_last_failure_timestamp_seconds": 123, "unexpected_secret_label": 99}
}
func (s *monitoringMetricsSourceStub) InstanceDatabasePools() []service.OpsDatabasePoolStats {
	return []service.OpsDatabasePoolStats{
		{Role: "primary", Active: 3, Idle: 2, Limit: 20, WaitCount: 7},
		{Role: "advisory_lock", Active: 1, Limit: 3, WaitCount: 2},
		{Role: "read_replica", Active: 2, Limit: 10, WaitCount: 4},
		{Role: "unexpected_secret_label", Active: 99},
	}
}
func (s *monitoringMetricsSourceStub) DatabaseInstanceCapacity(context.Context) *service.OpsDatabaseCapacity {
	return s.capacity
}
func (s *monitoringMetricsSourceStub) MonitoringJobHeartbeats(context.Context) (map[string]float64, error) {
	return map[string]float64{"ops_metrics_collector": 123, "unexpected_secret_label": 99}, nil
}

func TestMonitoringMetricsKeepSharedCapacityOutOfInstanceCounters(t *testing.T) {
	used, limit := int64(12), int64(100)
	source := &monitoringMetricsSourceStub{capacity: &service.OpsDatabaseCapacity{
		PostgreSQL: service.OpsDatabaseInstanceCapacity{State: "Available", Used: &used, Limit: &limit},
		Redis:      service.OpsDatabaseInstanceCapacity{State: "Unknown", Used: &used, Limit: &limit},
	}}
	router := gin.New()
	cfg := &config.Config{}
	cfg.Ops.Metrics = config.OpsMetricsConfig{Enabled: true, Token: "scrape-token"}
	RegisterMetricsRoutes(router, cfg, source)
	handler := lifecycle.New().Wrap(router)
	scrape := func(path string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer scrape-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotContains(t, rec.Body.String(), "unexpected_secret_label")
		return rec.Body.String()
	}
	instance := scrape("/metrics/instance")
	require.Zero(t, source.calls, "instance scrape must not perform shared database queries")
	require.Contains(t, instance, "# TYPE modurelay_db_pool_wait_total counter\n")
	require.Contains(t, instance, "modurelay_db_pool_wait_total 7\n")
	require.Contains(t, instance, "# TYPE modurelay_billing_observation_missed_total counter\n")
	require.Contains(t, instance, "modurelay_billing_observation_unpersisted 1\n")
	require.NotContains(t, instance, "modurelay_database_instance_connections")
	require.Contains(t, instance, `modurelay_database_pool_open_connections{pool="primary"} 5`)
	require.Contains(t, instance, `modurelay_database_pool_max_open_connections{pool="advisory_lock"} 3`)
	require.Contains(t, instance, `modurelay_database_pool_wait_total{pool="read_replica"} 4`)
	shared := scrape("/metrics")
	require.Contains(t, shared, "modurelay_database_instance_connections 12\n")
	require.Contains(t, shared, "modurelay_database_instance_max_connections 100\n")
	require.NotContains(t, shared, "modurelay_redis_instance_connections", "Unknown capacity must be absent, not zero")
	require.NotContains(t, shared, "modurelay_db_pool_wait_total")
	require.Contains(t, shared, `modurelay_ops_job_last_success_timestamp_seconds{job_name="ops_metrics_collector"} 123`)
}

func TestInstanceMetricsAuthenticatedDuringDrainWithoutSharedDependencies(t *testing.T) {
	r := gin.New()
	cfg := &config.Config{}
	cfg.Ops.Metrics = config.OpsMetricsConfig{Enabled: true, Token: "scrape-token"}
	RegisterMetricsRoutes(r, cfg, nil)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	r.GET("/work", func(c *gin.Context) { close(entered); <-release; c.Status(http.StatusNoContent) })
	m := lifecycle.New()
	h := m.Wrap(r)
	go func() {
		defer close(finished)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/work", nil))
	}()
	<-entered
	defer func() { close(release); <-finished }()
	for _, auth := range []string{"", "Bearer wrong", "Bearer scrape-token"} {
		req := httptest.NewRequest(http.MethodGet, "/metrics/instance", nil)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if auth != "Bearer scrape-token" {
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			continue
		}
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "modurelay_instance_active_http_requests 1\n")
		require.Contains(t, rec.Body.String(), "modurelay_instance_draining 0\n")
		require.NotContains(t, rec.Body.String(), "modurelay_cluster_qps")
	}
	m.BeginDrain()
	m.StopAdmission()
	req := httptest.NewRequest(http.MethodGet, "/metrics/instance", nil)
	req.Header.Set("Authorization", "Bearer scrape-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "modurelay_instance_draining 1\n")
	require.Contains(t, rec.Body.String(), "modurelay_instance_active_http_requests 1\n")
}

func TestInstanceMetricsExposeHTTPSeriesWithoutChangingClusterGauges(t *testing.T) {
	httpmetrics.Reset()
	t.Cleanup(httpmetrics.Reset)
	httpmetrics.Observe("POST", 200, httpmetrics.SourceApp, 1500*time.Millisecond, 200*time.Millisecond, true)

	r := gin.New()
	cfg := &config.Config{}
	cfg.Ops.Metrics = config.OpsMetricsConfig{Enabled: true, Token: "scrape-token"}
	RegisterMetricsRoutes(r, cfg, &metricsSourceStub{})
	m := lifecycle.New()
	h := m.Wrap(r)

	req := httptest.NewRequest(http.MethodGet, "/metrics/instance", nil)
	req.Header.Set("Authorization", "Bearer scrape-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `modurelay_http_requests_total{method="POST",source="app",route="other",protocol="http",status_code="200"} 1`)
	require.Contains(t, rec.Body.String(), "modurelay_http_request_duration_seconds_bucket")
	require.Contains(t, rec.Body.String(), "modurelay_http_ttft_seconds_bucket")
	require.NotContains(t, rec.Body.String(), "modurelay_cluster_qps")

	cluster := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	cluster.Header.Set("Authorization", "Bearer scrape-token")
	clusterRec := httptest.NewRecorder()
	h.ServeHTTP(clusterRec, cluster)
	require.Equal(t, http.StatusOK, clusterRec.Code)
	require.NotContains(t, clusterRec.Body.String(), "modurelay_http_requests_total")
	require.Contains(t, clusterRec.Body.String(), "modurelay_cluster_qps")
}

func TestInstanceMetricsDoesNotFabricateZerosWithoutLifecycle(t *testing.T) {
	r := gin.New()
	cfg := &config.Config{}
	cfg.Ops.Metrics = config.OpsMetricsConfig{Enabled: true, Token: "scrape-token"}
	RegisterMetricsRoutes(r, cfg, nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics/instance", nil)
	req.Header.Set("Authorization", "Bearer scrape-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
