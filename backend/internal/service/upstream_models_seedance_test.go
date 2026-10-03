package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildUpstreamModelsRequestSeedance(t *testing.T) {
	for _, testCase := range []struct {
		baseURL string
		wantURL string
	}{
		{"https://seedance.example.com", "https://seedance.example.com/v1/models"},
		{"https://seedance.example.com/v1/", "https://seedance.example.com/v1/models"},
		{"https://seedance.example.com/api/v3/", "https://seedance.example.com/api/v3/models"},
		{"https://seedance.example.com/v3", "https://seedance.example.com/v3/models"},
	} {
		t.Run(testCase.baseURL, func(t *testing.T) {
			service := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
			request, err := service.buildUpstreamModelsRequest(context.Background(), &Account{
				Platform: PlatformSeedance, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"base_url": testCase.baseURL, "api_key": "test-seedance-key"},
			})
			require.NoError(t, err)
			require.Equal(t, http.MethodGet, request.Method)
			require.Equal(t, testCase.wantURL, request.URL.String())
			require.Equal(t, "Bearer test-seedance-key", request.Header.Get("Authorization"))
			require.Empty(t, request.Header.Get("x-api-key"))
			require.Empty(t, request.Header.Get("anthropic-version"))
		})
	}
}

func TestBuildUpstreamModelsRequestSeedanceRejectsInvalidCredentials(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		accountType string
		credentials map[string]any
	}{
		{"missing URL", AccountTypeAPIKey, map[string]any{"api_key": "test-key"}},
		{"missing key", AccountTypeAPIKey, map[string]any{"base_url": "https://seedance.example.com"}},
		{"invalid URL", AccountTypeAPIKey, map[string]any{"base_url": "/api/v3", "api_key": "test-key"}},
		{"unsupported account type", AccountTypeOAuth, map[string]any{"base_url": "https://seedance.example.com", "api_key": "test-key"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := &AccountTestService{cfg: upstreamModelSyncTestConfig()}
			request, err := service.buildUpstreamModelsRequest(context.Background(), &Account{
				Platform: PlatformSeedance, Type: testCase.accountType, Credentials: testCase.credentials,
			})
			require.Error(t, err)
			require.Nil(t, request)
		})
	}
}

func TestBuildUpstreamModelsRequestSeedanceEnforcesURLAllowlist(t *testing.T) {
	configuration := upstreamModelSyncTestConfig()
	configuration.Security.URLAllowlist.Enabled = true
	configuration.Security.URLAllowlist.UpstreamHosts = []string{"allowed.example.com"}
	service := &AccountTestService{cfg: configuration}
	request, err := service.buildUpstreamModelsRequest(context.Background(), &Account{
		Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://blocked.example.com/api/v3", "api_key": "test-key"},
	})
	require.Error(t, err)
	require.Nil(t, request)
}

func TestSyncUpstreamModelCatalogSeedanceUsesOnlyLiveList(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"ep-video-test"},{"id":"48:seedance-2.0"}]}`)),
	}}
	service := &AccountTestService{cfg: upstreamModelSyncTestConfig(), httpUpstream: upstream}
	catalog, err := service.SyncUpstreamModelCatalog(context.Background(), &Account{
		Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://seedance.example.com/api/v3", "api_key": "test-seedance-key"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"48:seedance-2.0", "ep-video-test"}, catalog.Models)
	require.Empty(t, catalog.Metadata)
	require.Empty(t, catalog.Warnings)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://seedance.example.com/api/v3/models", upstream.lastReq.URL.String())
}

func TestSyncUpstreamModelCatalogSeedanceDoesNotPretendConfiguredModelsAreLive(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound, Header: http.Header{},
		Body: io.NopCloser(strings.NewReader(`{"error":"model listing is not supported"}`)),
	}}
	service := &AccountTestService{cfg: upstreamModelSyncTestConfig(), httpUpstream: upstream}
	catalog, err := service.SyncUpstreamModelCatalog(context.Background(), &Account{
		Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://seedance.example.com/api/v3", "api_key": "test-seedance-key",
			"model_mapping": map[string]any{"seedance-video": "ep-existing-video"},
		},
	})
	require.Error(t, err)
	require.Nil(t, catalog)
	var syncError *UpstreamModelSyncError
	require.ErrorAs(t, err, &syncError)
	require.Equal(t, http.StatusNotFound, syncError.StatusCode)
	require.NotContains(t, syncError.SafeMessage(), "test-seedance-key")
	require.Len(t, upstream.requests, 1)
}
