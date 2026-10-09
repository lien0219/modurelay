package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func lifecycleKeyConfig(t *testing.T, overrides map[string]any) DataLifecycleConfig {
	t.Helper()
	v := viper.New()
	values := map[string]any{"enabled": true, "encryption_key": strings.Repeat("07", 32), "bucket": "fixture", "access_key_id": "fixture", "secret_access_key": "fixture"}
	for k, value := range overrides {
		values[k] = value
	}
	v.Set("lifecycle", values)
	var c DataLifecycleConfig
	require.NoError(t, v.Sub("lifecycle").Unmarshal(&c))
	return c
}

func TestDataLifecycleKeyConfigurationRejectsUnsafePromotion(t *testing.T) {
	active := map[string]any{"id": "key-2026", "version": 2, "status": "active", "key": strings.Repeat("09", 32)}
	for _, tc := range []struct {
		name   string
		values map[string]any
	}{
		{"duplicate ID", map[string]any{"active_key_id": "key-2026", "keys": []any{active, active}}},
		{"missing active key", map[string]any{"active_key_id": "missing", "keys": []any{active}}},
		{"zero version", map[string]any{"active_key_id": "zero", "keys": []any{map[string]any{"id": "zero", "version": 0, "status": "active", "key": strings.Repeat("09", 32)}}}},
		{"invalid status", map[string]any{"active_key_id": "key-2026", "keys": []any{map[string]any{"id": "key-2026", "version": 2, "status": "destroyed", "key": strings.Repeat("09", 32)}}}},
		{"wrong length", map[string]any{"active_key_id": "key-2026", "keys": []any{map[string]any{"id": "key-2026", "version": 2, "status": "active", "key": "09"}}}},
		{"V2 without inventory", map[string]any{"active_key_id": "key-2026", "keys": []any{active}, "v2_write_enabled": true}},
		{"V2 without rollback", map[string]any{"active_key_id": "key-2026", "keys": []any{active}, "v2_write_enabled": true, "instance_id": "one", "expected_instance_ids": []string{"one"}}},
		{"duplicate instance", map[string]any{"active_key_id": "key-2026", "keys": []any{active}, "instance_id": "one", "expected_instance_ids": []string{"one", "one"}, "v2_rollback_compatible": true}},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Error(t, lifecycleKeyConfig(t, tc.values).Validate()) })
	}
	valid := lifecycleKeyConfig(t, map[string]any{"active_key_id": "key-2026", "keys": []any{active}, "v2_write_enabled": true, "instance_id": "one", "expected_instance_ids": []string{"one", "two"}, "v2_rollback_compatible": true})
	require.NoError(t, valid.Validate())
	// Config-only reader rollout does not promote writing.
	valid = lifecycleKeyConfig(t, map[string]any{"active_key_id": "key-2026", "keys": []any{active}})
	require.NoError(t, valid.Validate())
}
