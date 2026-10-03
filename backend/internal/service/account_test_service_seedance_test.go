//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_SeedanceChecksConfigurationWithoutUpstreamRequest(t *testing.T) {
	account := &Account{
		ID:       12,
		Platform: PlatformSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "secret-key",
			"base_url": "https://provider.example/api/v3",
		},
	}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &queuedHTTPUpstream{}
	service := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	c, recorder := newTestContext()

	require.NoError(t, service.TestAccountConnection(c, account.ID, "vendor-video", "", AccountTestModeDefault))
	require.Empty(t, upstream.requests)
	require.Contains(t, recorder.Body.String(), "Seedance configuration is valid. No upstream request was sent.")
	require.Contains(t, recorder.Body.String(), `"success":true`)
}
