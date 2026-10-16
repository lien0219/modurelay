package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type metricsSourceStub struct {
	calls int
	err   error
}

func (s *metricsSourceStub) GetClusterMetrics(context.Context) (*service.OpsWindowStats, *service.OpsSystemMetricsSnapshot, error) {
	s.calls++
	return &service.OpsWindowStats{SuccessCount: 100, ErrorCountTotal: 20, TokenConsumed: 1200, EndTime: time.Unix(123, 0)}, &service.OpsSystemMetricsSnapshot{InstanceCount: 3}, s.err
}

func TestMetricsEndpointAuthenticationAndClusterRates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	source := &metricsSourceStub{}
	cfg := &config.Config{}
	cfg.Ops.Metrics = config.OpsMetricsConfig{Enabled: true, Token: "scrape-token"}
	RegisterMetricsRoutes(r, cfg, source)
	for _, auth := range []string{"", "Bearer wrong", "scrape-token"} {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}
	require.Zero(t, source.calls)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer scrape-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/plain; version=0.0.4")
	require.Contains(t, rec.Body.String(), "modurelay_cluster_qps 2\n")
	require.Contains(t, rec.Body.String(), "modurelay_cluster_tps 20\n")
	require.Contains(t, rec.Body.String(), "modurelay_cluster_instances 3\n")
	// The existing PG aggregates are not multiplied by the instance count.
	require.NotContains(t, rec.Body.String(), "modurelay_cluster_qps 6\n")
	source.err = errors.New("Redis unavailable")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestMetricsEndpointDisabledByDefault(t *testing.T) {
	r := gin.New()
	RegisterMetricsRoutes(r, &config.Config{}, &metricsSourceStub{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}
