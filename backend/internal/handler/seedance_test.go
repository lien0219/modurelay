//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedanceHandlerLifecycleAndOwnership(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	var owner int64
	upstream.call = func(req *http.Request, id int64) (*http.Response, error) {
		body := `{"id":"task-ark","status":"queued"}`
		if req.Method == http.MethodPost {
			owner = id
		} else {
			require.Equal(t, owner, id)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	newContext := func(method string) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), method == http.MethodPost)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.Group.Platform = service.PlatformOpenAI
		body := ""
		if method == http.MethodPost {
			body = `{"model":"doubao-seedance","content":[{"type":"text","text":"waves"}]}`
		}
		c.Request = httptest.NewRequest(method, "/api/v3/contents/generations/tasks", strings.NewReader(body))
		c.Params = gin.Params{{Key: "task_id", Value: "task-ark"}}
		return c, w
	}
	c, w := newContext(http.MethodPost)
	h.SeedanceTasks(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Positive(t, owner)
	require.Len(t, bindings.pending, 1)
	slots.assertReleased(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		c, w = newContext(method)
		h.SeedanceTasks(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		slots.assertReleased(t)
	}
	for _, other := range []string{"user", "key", "group", "task", "provider"} {
		c, w = newContext(http.MethodGet)
		key, _ := middleware.GetAPIKeyFromContext(c)
		switch other {
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		case "key":
			key.ID = 21
		case "group":
			group := int64(25)
			key.GroupID = &group
		case "task":
			c.Params = gin.Params{{Key: "task_id", Value: "other"}}
		case "provider":
			c.Params = gin.Params{{Key: "request_id", Value: "task-ark"}}
		}
		before := upstream.calls
		if other == "provider" {
			h.GrokVideoStatus(c)
		} else {
			h.SeedanceTasks(c)
		}
		require.Equal(t, 404, w.Code, other+": "+w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
	c, _ = newContext(http.MethodGet)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	result := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 12345}, ResponseID: "seedance:task-ark", VideoCount: 1}
	for i := range 20 {
		billed := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, result.ResponseID, result)
		if i == 0 {
			require.NotNil(t, billed)
			require.Equal(t, "doubao-seedance", billed.BillingModel)
			require.Equal(t, 12345, billed.Usage.OutputTokens)
			require.Equal(t, 1, billed.VideoCount)
			require.True(t, billed.ForceTokenBilling)
		} else {
			require.Nil(t, billed)
		}
	}
	require.Len(t, bindings.billed, 1)
}

func TestFirstClassSeedanceLookupsKeepOriginalAccount(test *testing.T) {
	for _, endpoint := range []string{"native status", "native delete", "compatible status", "compatible content"} {
		test.Run(endpoint, func(test *testing.T) {
			handler, slots, bindings, upstream := newGrokMediaSlotHandler(test, false, false, service.PlatformSeedance)
			groupID := int64(24)
			require.NoError(test, handler.gatewayService.BindGrokMediaVideoRequestAccount(context.Background(), &groupID, "seedance:task-ark", 10, 20, 1))
			bindings.writes = 0
			upstream.call = func(request *http.Request, accountID int64) (*http.Response, error) {
				require.Equal(test, int64(1), accountID)
				require.Equal(test, "/api/v3/contents/generations/tasks/task-ark", request.URL.Path)
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"id":"task-ark","status":"succeeded","content":{"video_url":"https://videos.example.com/task.mp4"}}`)),
				}, nil
			}
			for range 20 {
				requestContext, recorder := grokMediaSlotContext(context.Background(), false)
				key, _ := middleware.GetAPIKeyFromContext(requestContext)
				key.Group.Platform = service.PlatformSeedance
				requestContext.Params = gin.Params{{Key: "request_id", Value: "seedance:task-ark"}, {Key: "task_id", Value: "task-ark"}}
				switch endpoint {
				case "native status":
					handler.SeedanceTasks(requestContext)
				case "native delete":
					requestContext.Request.Method = http.MethodDelete
					handler.SeedanceTasks(requestContext)
				case "compatible status":
					handler.GrokVideoStatus(requestContext)
				case "compatible content":
					handler.GrokVideoContent(requestContext)
				}
				requestContext.Writer.WriteHeaderNow()
				if endpoint == "compatible content" {
					require.Equal(test, http.StatusFound, recorder.Code, recorder.Body.String())
					require.Equal(test, "https://videos.example.com/task.mp4", recorder.Header().Get("Location"))
				} else {
					require.Equal(test, http.StatusOK, recorder.Code, recorder.Body.String())
				}
				slots.assertReleased(test)
			}
			require.Equal(test, 20, upstream.calls)
			require.Zero(test, bindings.writes)
			require.Empty(test, bindings.billed)
		})
	}
}
