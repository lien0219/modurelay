package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadShutdownBudgets(t *testing.T) {
	t.Run("defaults reserve time for each shutdown phase", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 900, cfg.Server.ShutdownTimeoutSeconds)
		require.Equal(t, 5, cfg.Server.DrainDelaySeconds)
		require.Equal(t, 30, cfg.Server.CleanupTimeoutSeconds)
		require.Equal(t, 10, cfg.Server.ForceCloseTimeoutSeconds)
	})

	t.Run("yaml config", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte(`server:
  shutdown_timeout_seconds: 1200
  drain_delay_seconds: 8
  cleanup_timeout_seconds: 45
  force_close_timeout_seconds: 12
`), 0o600))
		t.Setenv("CONFIG_FILE", path)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 1200, cfg.Server.ShutdownTimeoutSeconds)
		require.Equal(t, 8, cfg.Server.DrainDelaySeconds)
		require.Equal(t, 45, cfg.Server.CleanupTimeoutSeconds)
		require.Equal(t, 12, cfg.Server.ForceCloseTimeoutSeconds)
	})

	t.Run("environment overrides yaml and permits immediate drain", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("server:\n  shutdown_timeout_seconds: 900\n  drain_delay_seconds: 5\n  cleanup_timeout_seconds: 30\n  force_close_timeout_seconds: 10\n"), 0o600))
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("SERVER_SHUTDOWN_TIMEOUT_SECONDS", "1800")
		t.Setenv("SERVER_DRAIN_DELAY_SECONDS", "0")
		t.Setenv("SERVER_CLEANUP_TIMEOUT_SECONDS", "60")
		t.Setenv("SERVER_FORCE_CLOSE_TIMEOUT_SECONDS", "15")
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, 1800, cfg.Server.ShutdownTimeoutSeconds)
		require.Zero(t, cfg.Server.DrainDelaySeconds)
		require.Equal(t, 60, cfg.Server.CleanupTimeoutSeconds)
		require.Equal(t, 15, cfg.Server.ForceCloseTimeoutSeconds)
	})
}

func TestValidateShutdownBudgetBounds(t *testing.T) {
	for _, tt := range []struct {
		key string
		min int
		max int
		set func(*Config, int)
	}{
		{"shutdown_timeout_seconds", 1, 86400, func(c *Config, n int) { c.Server.ShutdownTimeoutSeconds = n }},
		{"drain_delay_seconds", 0, 300, func(c *Config, n int) { c.Server.DrainDelaySeconds = n }},
		{"cleanup_timeout_seconds", 1, 300, func(c *Config, n int) { c.Server.CleanupTimeoutSeconds = n }},
		{"force_close_timeout_seconds", 1, 300, func(c *Config, n int) { c.Server.ForceCloseTimeoutSeconds = n }},
	} {
		t.Run(tt.key, func(t *testing.T) {
			resetViperWithJWTSecret(t)
			cfg, err := Load()
			require.NoError(t, err)
			for _, valid := range []int{tt.min, tt.max} {
				tt.set(cfg, valid)
				require.NoError(t, cfg.Validate())
			}
			for _, invalid := range []int{tt.min - 1, tt.max + 1} {
				tt.set(cfg, invalid)
				require.ErrorContains(t, cfg.Validate(), "server."+tt.key)
			}
		})
	}
}
