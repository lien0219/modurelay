package config

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestDataLifecycleConfigurationFailsClosed(t *testing.T) {
	require.NoError(t, (DataLifecycleConfig{}).Validate())
	require.Error(t, (DataLifecycleConfig{PurgeEnabled: true}).Validate())
	require.Error(t, (DataLifecycleConfig{Enabled: true}).Validate())
	valid := DataLifecycleConfig{Enabled: true, EncryptionKey: strings.Repeat("01", 32), Bucket: "tenant-exports", AccessKeyID: "test", SecretAccessKey: "test"}
	require.NoError(t, valid.Validate())
	valid.EncryptionKey = "unstable"
	require.Error(t, valid.Validate())
}
