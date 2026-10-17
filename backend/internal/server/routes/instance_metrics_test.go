package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
