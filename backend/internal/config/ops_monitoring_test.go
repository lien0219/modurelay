//go:build unit

package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOpsMonitoringConfigEnvironment(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("OPS_MONITORING_MODE", "kubernetes")
	t.Setenv("OPS_MONITORING_PROMETHEUS_URL", "http://prometheus.monitoring.svc:9090")
	t.Setenv("OPS_MONITORING_BEARER_TOKEN", "secret")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "kubernetes", cfg.Ops.Monitoring.Mode)
	require.Equal(t, "secret", cfg.Ops.Monitoring.BearerToken)
	require.Equal(t, 8, cfg.Ops.Monitoring.TimeoutSeconds)
}

func TestOpsMonitoringConfigRejectsUnsafeDestinations(t *testing.T) {
	for _, value := range []string{"file:///etc/passwd", "http://user:secret@prometheus:9090", "http://169.254.169.254/latest/meta-data", "http://[fe80::1]", "http://0.0.0.0:9090", "http://224.0.0.1:9090", "http://prometheus:9090?url=other"} {
		require.Error(t, (OpsMonitoringConfig{PrometheusURL: value}).Validate(), value)
	}
	for _, value := range []string{"http://localhost:9090", "http://127.0.0.1:9090", "http://prometheus.monitoring.svc:9090", "https://metrics.example/prometheus", "http://10.0.0.8:9090"} {
		require.NoError(t, (OpsMonitoringConfig{PrometheusURL: value}).Validate(), value)
	}
}
