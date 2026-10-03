package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSeedanceAccountConfig(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		credentials map[string]any
		wantErr     bool
	}{
		{
			name:        "valid api key account",
			accountType: AccountTypeAPIKey,
			credentials: map[string]any{"api_key": "secret", "base_url": "https://provider.example/v3"},
		},
		{
			name:        "missing api key",
			accountType: AccountTypeAPIKey,
			credentials: map[string]any{"base_url": "https://provider.example/v3"},
			wantErr:     true,
		},
		{
			name:        "missing base url",
			accountType: AccountTypeAPIKey,
			credentials: map[string]any{"api_key": "secret"},
			wantErr:     true,
		},
		{
			name:        "relative base url",
			accountType: AccountTypeAPIKey,
			credentials: map[string]any{"api_key": "secret", "base_url": "/v3"},
			wantErr:     true,
		},
		{
			name:        "unsupported scheme",
			accountType: AccountTypeAPIKey,
			credentials: map[string]any{"api_key": "secret", "base_url": "ftp://provider.example"},
			wantErr:     true,
		},
		{
			name:        "base url credentials rejected",
			accountType: AccountTypeAPIKey,
			credentials: map[string]any{"api_key": "secret", "base_url": "https://user:pass@provider.example"},
			wantErr:     true,
		},
		{
			name:        "oauth rejected",
			accountType: AccountTypeOAuth,
			credentials: map[string]any{"api_key": "secret", "base_url": "https://provider.example"},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSeedanceAccountConfig(PlatformSeedance, tt.accountType, tt.credentials)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateSeedanceAccountConfigLeavesOtherPlatformsUnchanged(t *testing.T) {
	require.NoError(t, validateSeedanceAccountConfig(PlatformOpenAI, AccountTypeOAuth, nil))
}
