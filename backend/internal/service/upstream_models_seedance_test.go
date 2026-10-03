package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

func TestSyncUpstreamModelCatalogSeedanceFiltersByOutputAndKeepsModelNames(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[
		{"id":"doubao-seedance-1-0-pro-fast-251015","name":"doubao-seedance-1-0-pro-fast","modalities":{"input_modalities":["text","image"],"output_modalities":["video"]}},
		{"id":"doubao-seed-mini","name":"Doubao Seed Mini","modalities":{"input_modalities":["text","video"],"output_modalities":["text"]}},
		{"id":"seedance-lookalike","output_modalities":["image"]},
		{"id":"ep-video-test"}
	]}`)),
	}}
	repo := &upstreamModelMetadataRepoStub{}
	service := &AccountTestService{cfg: upstreamModelSyncTestConfig(), httpUpstream: upstream, accountRepo: repo}
	account := &Account{ID: 12, Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://seedance.example.com/api/v3", "api_key": "test-key"}}

	catalog, err := service.SyncUpstreamModelCatalog(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []string{"doubao-seedance-1-0-pro-fast-251015", "ep-video-test"}, catalog.Models)
	require.Equal(t, "doubao-seedance-1-0-pro-fast", catalog.Metadata["doubao-seedance-1-0-pro-fast-251015"].DisplayName)
	require.Equal(t, []string{"text", "image"}, catalog.Metadata["doubao-seedance-1-0-pro-fast-251015"].InputModalities)
	snapshot := account.GetUpstreamModelMetadataSnapshot()
	require.NotNil(t, snapshot)
	require.Equal(t, account.ID, repo.accountID)
	require.Contains(t, repo.updates, UpstreamModelMetadataExtraKey)
	body, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.Equal(t, "video", gjson.GetBytes(body, "models.doubao-seedance-1-0-pro-fast-251015.output_modalities.0").String())
	require.Len(t, upstream.requests, 1, "video catalog sync must not query an unrelated provider registry")
}

func TestFetchSeedanceModelCatalogDoesNotPersistOrTreatVideoInputAsVideoOutput(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"video-input-text","modalities":{"input":["text","video"],"output":["text"]}}]}`)),
	}}
	repo := &upstreamModelMetadataRepoStub{}
	service := &AccountTestService{cfg: upstreamModelSyncTestConfig(), httpUpstream: upstream, accountRepo: repo}
	account := &Account{ID: 12, Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://seedance.example.com/api/v3", "api_key": "test-key"}}
	catalog, err := service.FetchSeedanceModelCatalog(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, catalog.Models, "the filtered response must remain an empty array")
	require.Empty(t, catalog.Models)
	require.Equal(t, []string{"text"}, catalog.Metadata["video-input-text"].OutputModalities)
	require.Nil(t, account.GetUpstreamModelMetadataSnapshot())
	require.Empty(t, repo.updates, "opening the picker must not persist account changes")
	require.Len(t, upstream.requests, 1)
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
}

func TestFetchSeedanceModelCatalogPreservesIDsWithUnrecognizedCapabilityFormats(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"ep-compatible-video","modalities":["video"]}]}`)),
	}}
	service := &AccountTestService{cfg: upstreamModelSyncTestConfig(), httpUpstream: upstream}
	catalog, err := service.FetchSeedanceModelCatalog(context.Background(), &Account{
		Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://seedance.example.com/api/v3", "api_key": "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"ep-compatible-video"}, catalog.Models)
}

func TestFetchSeedanceModelCatalogFiltersKnownNonVideoIDsWithoutCapabilities(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{"data":[
			{"id":"qwen3-8b-20250429","name":"qwen3-8b","modalities":{}},
			{"id":"deepseek-v3-241226"},
			{"id":"doubao-seed-2-1-pro-260628","name":"Doubao Seed Pro"},
			{"id":"doubao-seedream-5-0-pro-260628"},
			{"id":"doubao-seedance-2-0-260128","modalities":{"output_modalities":["video"]}},
			{"id":"qwen3-video-gateway","output_modalities":["video"]},
			{"id":"ep-custom-video"}, {"id":"48:seedance-2.0"}, {"id":"custom-video-gateway"}
		]}`)),
	}}
	service := &AccountTestService{cfg: upstreamModelSyncTestConfig(), httpUpstream: upstream}
	catalog, err := service.FetchSeedanceModelCatalog(context.Background(), &Account{
		Platform: PlatformSeedance, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://ark.cn-beijing.volces.com/api/v3", "api_key": "test-key"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"48:seedance-2.0", "custom-video-gateway", "doubao-seedance-2-0-260128", "ep-custom-video", "qwen3-video-gateway"}, catalog.Models)
	require.Equal(t, "qwen3-8b", catalog.Metadata["qwen3-8b-20250429"].DisplayName)
	require.Empty(t, catalog.Metadata["qwen3-8b-20250429"].OutputModalities, "filtering must not fabricate provider capabilities")
	require.Len(t, upstream.requests, 1)
}
