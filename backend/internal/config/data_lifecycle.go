package config

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// Storage credentials are independently configured for tenant exports. No
// admin backup configuration or ephemeral encryption key is reused.
type DataLifecycleConfig struct {
	Enabled              bool                     `mapstructure:"enabled"`
	PurgeEnabled         bool                     `mapstructure:"purge_enabled"`
	EncryptionKey        string                   `mapstructure:"encryption_key"`
	ActiveKeyID          string                   `mapstructure:"active_key_id"`
	Keys                 []LifecycleEncryptionKey `mapstructure:"keys"`
	V2WriteEnabled       bool                     `mapstructure:"v2_write_enabled"`
	InstanceID           string                   `mapstructure:"instance_id"`
	ExpectedInstanceIDs  []string                 `mapstructure:"expected_instance_ids"`
	V2RollbackCompatible bool                     `mapstructure:"v2_rollback_compatible"`
	Endpoint             string                   `mapstructure:"endpoint"`
	Region               string                   `mapstructure:"region"`
	Bucket               string                   `mapstructure:"bucket"`
	AccessKeyID          string                   `mapstructure:"access_key_id"`
	SecretAccessKey      string                   `mapstructure:"secret_access_key"`
	ForcePathStyle       bool                     `mapstructure:"force_path_style"`
}

// Keys are injected by trusted secret configuration, never stored in SQL.
type LifecycleEncryptionKey struct {
	ID      string `mapstructure:"id"`
	Version uint32 `mapstructure:"version"`
	Status  string `mapstructure:"status"`
	Key     string `mapstructure:"key"`
}

var lifecycleKeyIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func ValidLifecycleKeyIdentifier(id string) bool { return lifecycleKeyIdentifier.MatchString(id) }

func setDataLifecycleDefaults() {
	viper.SetDefault("data_lifecycle.enabled", false)
	viper.SetDefault("data_lifecycle.purge_enabled", false)
	viper.SetDefault("data_lifecycle.v2_write_enabled", false)
	viper.SetDefault("data_lifecycle.v2_rollback_compatible", false)
	viper.SetDefault("data_lifecycle.keys", []LifecycleEncryptionKey{})
	viper.SetDefault("data_lifecycle.expected_instance_ids", []string{})
	for _, key := range []string{"encryption_key", "active_key_id", "instance_id", "endpoint", "bucket", "access_key_id", "secret_access_key"} {
		viper.SetDefault("data_lifecycle."+key, "")
	}
	viper.SetDefault("data_lifecycle.region", "auto")
	viper.SetDefault("data_lifecycle.force_path_style", true)
}
func (c DataLifecycleConfig) Validate() error {
	if c.PurgeEnabled && !c.Enabled {
		return fmt.Errorf("data_lifecycle.purge_enabled requires enabled")
	}
	if !c.Enabled {
		return nil
	}
	key, err := hex.DecodeString(c.EncryptionKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("data_lifecycle.encryption_key requires a stable 32-byte hex key")
	}
	if err = c.validateKeyRing(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Bucket) == "" || c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return fmt.Errorf("data_lifecycle requires independently configured tenant export storage")
	}
	if c.Endpoint != "" {
		if err = ValidateAbsoluteHTTPURL(c.Endpoint); err != nil {
			return fmt.Errorf("data_lifecycle.endpoint: %w", err)
		}
	}
	return nil
}

func (c DataLifecycleConfig) validateKeyRing() error {
	if len(c.Keys) > 64 || len(c.ExpectedInstanceIDs) > 128 {
		return fmt.Errorf("data_lifecycle key or instance inventory exceeds limit")
	}
	ids := make(map[string]bool, len(c.Keys))
	active := 0
	for _, entry := range c.Keys {
		key, err := hex.DecodeString(entry.Key)
		if !ValidLifecycleKeyIdentifier(entry.ID) || ids[entry.ID] || entry.Version == 0 || err != nil || len(key) != 32 {
			return fmt.Errorf("data_lifecycle keys require unique bounded IDs, positive versions and 32-byte hex keys")
		}
		ids[entry.ID] = true
		switch entry.Status {
		case "active":
			active++
			if entry.ID != c.ActiveKeyID {
				return fmt.Errorf("data_lifecycle active_key_id must select the active key")
			}
		case "decrypt_only":
		default:
			return fmt.Errorf("data_lifecycle key status must be active or decrypt_only")
		}
	}
	if len(c.Keys) > 0 && (active != 1 || !ids[c.ActiveKeyID]) {
		return fmt.Errorf("data_lifecycle requires exactly one active key")
	}
	if len(c.Keys) == 0 && (c.ActiveKeyID != "" || c.V2WriteEnabled) {
		return fmt.Errorf("data_lifecycle V2 requires a complete key ring")
	}
	instances := make(map[string]bool, len(c.ExpectedInstanceIDs))
	for _, id := range c.ExpectedInstanceIDs {
		if !ValidLifecycleKeyIdentifier(id) || instances[id] {
			return fmt.Errorf("data_lifecycle expected_instance_ids must be unique bounded IDs")
		}
		instances[id] = true
	}
	if c.InstanceID != "" && !ValidLifecycleKeyIdentifier(c.InstanceID) {
		return fmt.Errorf("data_lifecycle instance_id is invalid")
	}
	if len(instances) > 0 && !instances[c.InstanceID] {
		return fmt.Errorf("data_lifecycle reader inventory must include this instance")
	}
	if c.V2WriteEnabled && (len(instances) == 0 || !c.V2RollbackCompatible) {
		return fmt.Errorf("data_lifecycle V2 writes require an approved complete instance inventory and compatible rollback")
	}
	return nil
}
