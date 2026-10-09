package config

import (
	"encoding/hex"
	"fmt"
	"github.com/spf13/viper"
	"strings"
)

// Storage credentials are independently configured for tenant exports. No
// admin backup configuration or ephemeral encryption key is reused.
type DataLifecycleConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	PurgeEnabled    bool   `mapstructure:"purge_enabled"`
	EncryptionKey   string `mapstructure:"encryption_key"`
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	ForcePathStyle  bool   `mapstructure:"force_path_style"`
}

func setDataLifecycleDefaults() {
	viper.SetDefault("data_lifecycle.enabled", false)
	viper.SetDefault("data_lifecycle.purge_enabled", false)
	for _, key := range []string{"encryption_key", "endpoint", "bucket", "access_key_id", "secret_access_key"} {
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
