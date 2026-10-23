package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpmetrics"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHTTPArrivalsPrecedeCompletionAndSSEUsesOwnHistogram(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initMiddlewareTestLogger(t)
	httpmetrics.Reset()
	t.Cleanup(httpmetrics.Reset)
	r := gin.New()
	r.Use(Logger())
	r.POST("/v1/responses", func(c *gin.Context) {
		during := httpmetrics.Render()
		require.Contains(t, during, `modurelay_http_requests_started_total{method="POST",source="app",route="responses"} 1`)
		require.NotContains(t, during, `modurelay_http_requests_total{`)
		service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, 30)
		c.Data(200, "text/event-stream", []byte("data: test\n\n"))
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	body := httpmetrics.Render()
	require.Contains(t, body, `modurelay_http_requests_total{method="POST",source="app",route="responses",protocol="sse",status_code="200"} 1`)
	require.Contains(t, body, `modurelay_http_ttft_seconds_count{method="POST",source="app",route="responses",protocol="sse"} 1`)
}

func TestLoggerRecordsInstanceHTTPMetricsAndSkipsProbes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initMiddlewareTestLogger(t)
	httpmetrics.Reset()
	t.Cleanup(httpmetrics.Reset)

	r := gin.New()
	r.Use(Logger())
	r.POST("/v1/messages", func(c *gin.Context) {
		service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, 25)
		c.Status(http.StatusOK)
	})
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/v1/messages", nil),
		httptest.NewRequest(http.MethodGet, "/health", nil),
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	} {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	body := httpmetrics.Render()
	require.Contains(t, body, `modurelay_http_requests_total{method="POST",source="app",route="messages",protocol="http",status_code="200"} 1`)
	require.Contains(t, body, `modurelay_http_ttft_seconds_count{method="POST",source="app",route="messages",protocol="http"} 1`)
	require.NotContains(t, body, `method="GET"`)
	require.Less(t, time.Duration(0), time.Second)
}

func TestLoggerMarksIngressRejectSeparately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initMiddlewareTestLogger(t)
	httpmetrics.Reset()
	t.Cleanup(httpmetrics.Reset)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		MarkIngressRejected(c, IngressRejectReason("invalid_api_key"))
		c.Next()
	})
	r.Use(Logger())
	r.GET("/v1/responses", func(c *gin.Context) {
		service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, 10)
		c.Status(http.StatusUnauthorized)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/responses", nil))

	body := httpmetrics.Render()
	require.Contains(t, body, `modurelay_http_requests_total{method="GET",source="ingress",route="responses",protocol="http",status_code="401"} 1`)
	require.NotContains(t, body, "modurelay_http_ttft_seconds_count")
}
