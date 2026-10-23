package httpmetrics

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestObserveCounterMonotonicHistogramAndTTFT(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Observe("POST", 200, SourceApp, 40*time.Millisecond, 0, false)
	Observe("POST", 200, SourceApp, 2*time.Second, 1500*time.Millisecond, true)
	Observe("GET", 401, SourceIngress, 10*time.Millisecond, 5*time.Second, true)
	Observe("POST", 500, SourceApp, 20*time.Millisecond, -1, false)

	body := Render()
	require.Contains(t, body, `modurelay_http_requests_total{method="POST",source="app",route="other",protocol="http",status_code="200"} 2`)
	require.Contains(t, body, `modurelay_http_requests_total{method="GET",source="ingress",route="other",protocol="http",status_code="401"} 1`)
	require.Contains(t, body, `modurelay_http_requests_total{method="POST",source="app",route="other",protocol="http",status_code="500"} 1`)
	require.NotContains(t, body, "user_id")
	require.NotContains(t, body, "request_id")
	require.NotContains(t, body, `user="`)
	require.Contains(t, body, "# TYPE modurelay_http_request_duration_seconds histogram")
	require.Contains(t, body, `modurelay_http_request_duration_seconds_bucket{method="POST",source="app",route="other",protocol="http",le="0.05"} 2`)
	require.Contains(t, body, `modurelay_http_request_duration_seconds_bucket{method="POST",source="app",route="other",protocol="http",le="1"} 2`)
	require.Contains(t, body, `modurelay_http_request_duration_seconds_bucket{method="POST",source="app",route="other",protocol="http",le="2.5"} 3`)
	require.Contains(t, body, `modurelay_http_request_duration_seconds_bucket{method="POST",source="app",route="other",protocol="http",le="+Inf"} 3`)
	require.Contains(t, body, `modurelay_http_request_duration_seconds_count{method="POST",source="app",route="other",protocol="http"} 3`)
	require.Contains(t, body, `modurelay_http_ttft_seconds_count{method="POST",source="app",route="other",protocol="http"} 1`)
	require.NotContains(t, body, `modurelay_http_ttft_seconds_count{method="GET",source="ingress"}`)
	require.NotContains(t, body, "modurelay_http_ttft_seconds_count{method=\"POST\",source=\"app\"} 3")

	// Ingress rejection must not enter the successful TTFT histogram.
	ttft := body[strings.Index(body, "# TYPE modurelay_http_ttft_seconds"):]
	require.NotContains(t, ttft, `source="ingress"`)
}

func TestExcludedPaths(t *testing.T) {
	for _, path := range []string{"/health", "/ready", "/metrics", "/metrics/instance", "/setup/status"} {
		require.True(t, ExcludedPath(path), path)
	}
	require.False(t, ExcludedPath("/v1/messages"))
}

func TestMetricDimensionsAreBoundedAndFailuresDoNotEnterTTFT(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ObserveDimensions("arbitrary-user-id", 500, SourceApp, "/customer/secret", "token-secret", time.Second, time.Millisecond, true)
	body := Render()
	require.Contains(t, body, `method="OTHER",source="app",route="other",protocol="http",status_code="500"`)
	require.NotContains(t, body, "secret")
	require.NotContains(t, body, `modurelay_http_ttft_seconds_count{`)
	for _, path := range []string{"/users/123", "/request-secret", "/v1/responses/request-42"} {
		require.Equal(t, "other", Route(path))
	}
}
