//go:build unit

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOpsMetricsFromEnvironment(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("OPS_METRICS_ENABLED", "true")
	t.Setenv("OPS_METRICS_TOKEN", "prometheus-secret")
	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.Ops.Metrics.Enabled)
	require.Equal(t, "prometheus-secret", cfg.Ops.Metrics.Token)
}

func TestOpsMetricsRequiresToken(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("OPS_METRICS_ENABLED", "true")
	t.Setenv("OPS_METRICS_TOKEN", "")
	_, err := Load()
	require.ErrorContains(t, err, "ops.metrics.token is required")
}
