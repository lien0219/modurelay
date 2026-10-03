//go:build unit

package service

import (
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_SeedanceCreatesNativeTaskOnUpstream(t *testing.T) {
	account := &Account{
		ID:       12,
		Platform: PlatformSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "secret-key",
			"base_url": "https://provider.example/api/v3",
			"model_mapping": map[string]any{
				"seedance-2.0": "ep-test",
			},
		},
	}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"id":"task-1","status":"queued","model":"ep-test"}`),
	}}
	service := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	c, recorder := newTestContext()

	require.NoError(t, service.TestAccountConnection(c, account.ID, "seedance-2.0", "", AccountTestModeDefault))
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "https://provider.example/api/v3/contents/generations/tasks", req.URL.String())
	require.Equal(t, "Bearer secret-key", req.Header.Get("Authorization"))
	require.Equal(t, "application/json", req.Header.Get("Content-Type"))
	require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "ep-test", gjson.GetBytes(body, "model").String())
	require.Equal(t, "text", gjson.GetBytes(body, "content.0.type").String())
	require.NotEmpty(t, gjson.GetBytes(body, "content.0.text").String())
	require.Contains(t, recorder.Body.String(), `"type":"test_start","model":"ep-test"`)
	require.Contains(t, recorder.Body.String(), "task-1")
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_SeedanceReportsUpstreamError(t *testing.T) {
	account := seedanceAccountTestServiceFixture()
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusBadRequest, `{"error":{"code":"InvalidParameter","message":"model is invalid"}}`),
	}}
	service := &AccountTestService{accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}, httpUpstream: upstream, cfg: &config.Config{}}
	c, recorder := newTestContext()

	err := service.TestAccountConnection(c, account.ID, "seedance-2.0", "", AccountTestModeDefault)
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), "HTTP 400 (InvalidParameter): model is invalid")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_SeedanceRejectsResponseWithoutTaskID(t *testing.T) {
	account := seedanceAccountTestServiceFixture()
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"status":"queued"}`),
	}}
	service := &AccountTestService{accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}, httpUpstream: upstream, cfg: &config.Config{}}
	c, recorder := newTestContext()

	err := service.TestAccountConnection(c, account.ID, "seedance-2.0", "", AccountTestModeDefault)
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), "missing task ID")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_SeedanceReportsFailedTask(t *testing.T) {
	account := seedanceAccountTestServiceFixture()
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"id":"task-2","status":"failed","error":{"code":"QuotaExceeded","message":"quota exhausted"}}`),
	}}
	service := &AccountTestService{accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}, httpUpstream: upstream, cfg: &config.Config{}}
	c, recorder := newTestContext()

	err := service.TestAccountConnection(c, account.ID, "seedance-2.0", "", AccountTestModeDefault)
	require.Error(t, err)
	require.Contains(t, recorder.Body.String(), "task-2 returned status failed")
	require.Contains(t, recorder.Body.String(), "QuotaExceeded")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func seedanceAccountTestServiceFixture() *Account {
	return &Account{
		ID:       12,
		Platform: PlatformSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "secret-key",
			"base_url": "https://provider.example/api/v3",
			"model_mapping": map[string]any{
				"seedance-2.0": "ep-test",
			},
		},
	}
}
