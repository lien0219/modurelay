//go:build integration

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Exercises the real HTTP binding/response, service, SQL repository, PostgreSQL
// JSONB and local/Redis cache invalidation. Only the upstream provider is absent.
func TestChannelBedrockCCCompatHTTPPersistence(t *testing.T) {
	ctx := context.Background()
	var groupID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`INSERT INTO groups (name, platform) VALUES ($1, 'composite') RETURNING id`, t.Name()).Scan(&groupID))
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, groupID)
		require.NoError(t, err)
	})
	repo := NewChannelRepository(integrationDB)
	svc := service.NewChannelService(repo, nil, nil, nil, NewChannelCache(integrationRedis))
	peer := service.NewChannelService(repo, nil, nil, nil, NewChannelCache(integrationRedis))
	require.Eventually(t, func() bool {
		counts, err := integrationRedis.PubSubNumSub(ctx, channelCachePubSubKey).Result()
		return err == nil && counts[channelCachePubSubKey] >= 2
	}, 3*time.Second, 10*time.Millisecond)
	h := admin.NewChannelHandler(svc, nil, nil)
	router := gin.New()
	router.POST("/channels", h.Create)
	router.PUT("/channels/:id", h.Update)
	router.GET("/channels/:id", h.GetByID)
	router.GET("/channels", h.List)
	request := func(method, path string, body any) gjson.Result {
		t.Helper()
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return gjson.Parse(w.Body.String()).Get("data")
	}
	created := request(http.MethodPost, "/channels", map[string]any{
		"name": t.Name(), "description": "keep description", "group_ids": []int64{groupID},
		"billing_model_source": "upstream", "restrict_models": true,
		"model_mapping":   map[string]any{"anthropic": map[string]string{"alias": "claude-opus-4-6"}},
		"model_pricing":   []map[string]any{{"platform": "anthropic", "models": []string{"claude-opus-4-6"}, "billing_mode": "token", "input_price": 0.000003, "output_price": 0.000015}},
		"features_config": map[string]any{"bedrock_cc_compat": true, "other": map[string]any{"keep": "value"}},
	})
	id := created.Get("id").Int()
	t.Cleanup(func() { require.NoError(t, repo.Delete(ctx, id)) })
	path := fmt.Sprintf("/channels/%d", id)
	assertRead := func(enabled bool) {
		t.Helper()
		read := request(http.MethodGet, path, nil)
		require.Equal(t, fmt.Sprint(enabled), read.Get("features_config.bedrock_cc_compat").Raw)
		require.Equal(t, "value", read.Get("features_config.other.keep").String())
		for _, key := range []string{"description", "billing_model_source", "restrict_models", "group_ids", "model_mapping"} {
			require.JSONEq(t, created.Get(key).Raw, read.Get(key).Raw, key)
		}
		require.Equal(t, 0.000003, read.Get("model_pricing.0.input_price").Float())
		require.Equal(t, 0.000015, read.Get("model_pricing.0.output_price").Float())
		require.Equal(t, "claude-opus-4-6", read.Get("model_pricing.0.models.0").String())
		listed := request(http.MethodGet, "/channels?search="+t.Name(), nil)
		require.Len(t, listed.Get("items").Array(), 1)
		require.Equal(t, fmt.Sprint(enabled), listed.Get("items.0.features_config.bedrock_cc_compat").Raw)
		for _, cachedService := range []*service.ChannelService{svc, peer} {
			require.Eventually(t, func() bool {
				ch, err := cachedService.GetChannelForGroup(ctx, groupID)
				return err == nil && ch != nil && ch.FeaturesConfig["bedrock_cc_compat"] == enabled && ch.IsBedrockCCCompatEnabled(service.PlatformAnthropic) == enabled
			}, 3*time.Second, 10*time.Millisecond)
		}
	}
	assertStored := func(enabled bool) {
		t.Helper()
		var value string
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			`SELECT (features_config->'bedrock_cc_compat')::text FROM channels WHERE id=$1`, id).Scan(&value))
		require.Equal(t, fmt.Sprint(enabled), value)
	}
	assertRead(true)
	assertStored(true)
	// Previously rendered pages may still submit the known legacy object shape.
	for _, tc := range []struct {
		value   any
		enabled bool
	}{
		{false, false}, {true, true}, {map[string]any{"anthropic": false}, false}, {map[string]any{"anthropic": true}, true},
	} {
		updated := request(http.MethodPut, path, map[string]any{"features_config": map[string]any{
			"bedrock_cc_compat": tc.value, "other": map[string]any{"keep": "value"},
		}})
		require.Equal(t, fmt.Sprint(tc.enabled), updated.Get("features_config.bedrock_cc_compat").Raw)
		assertRead(tc.enabled)
		assertStored(tc.enabled)
	}
	// Seed historical rows without the repository write normalization. Reads must
	// work immediately, but must not rewrite the database as a read side effect.
	for _, enabled := range []bool{false, true} {
		legacy := fmt.Sprintf(`{"anthropic":%t}`, enabled)
		_, err := integrationDB.ExecContext(ctx,
			`UPDATE channels SET features_config=jsonb_set(features_config,'{bedrock_cc_compat}',$1::jsonb) WHERE id=$2`, legacy, id)
		require.NoError(t, err)
		svc.InvalidateCache()
		assertRead(enabled)
		var raw string
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			`SELECT (features_config->'bedrock_cc_compat')::text FROM channels WHERE id=$1`, id).Scan(&raw))
		require.JSONEq(t, legacy, raw)
		// An ordinary subsequent save lazily repairs only the malformed flag.
		request(http.MethodPut, path, map[string]any{"description": "keep description"})
		assertRead(enabled)
		assertStored(enabled)
	}
}
