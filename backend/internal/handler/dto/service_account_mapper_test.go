package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestServiceAccountLegacyDTOsMaskCredentialAndRetainUsageIdentity(t *testing.T) {
	sa := int64(9)
	suffix := "abcd"
	key := &service.APIKey{ID: 5, ServiceAccountID: &sa, Key: "sha256:private-machine-digest", KeySuffix: &suffix}
	usage := &service.UsageLog{APIKeyID: key.ID, ServiceAccountID: &sa, APIKey: key}
	for _, value := range []any{APIKeyFromService(key), UsageLogFromService(usage), UsageLogFromServiceAdmin(usage)} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(raw), key.Key)
		require.NotContains(t, string(raw), "private-machine-digest")
		require.Contains(t, string(raw), "abcd")
	}
	for _, value := range []any{UsageLogFromService(usage), UsageLogFromServiceAdmin(usage)} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"service_account_id":9`)
	}
	key.ServiceAccountID, key.UserID, key.Key = nil, 2, "legacy-human-key"
	require.Equal(t, key.Key, APIKeyFromService(key).Key, "preserve human DTO behavior")
}
