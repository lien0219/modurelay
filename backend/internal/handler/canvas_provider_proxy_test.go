package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCopyCanvasProviderResponseHeadersUsesAllowlist(t *testing.T) {
	source := http.Header{
		"Content-Type":                {"image/png"},
		"Content-Disposition":         {`attachment; filename="result.png"`},
		"X-Request-Id":                {"provider-request"},
		"Set-Cookie":                  {"provider_session=secret"},
		"Access-Control-Allow-Origin": {"*"},
	}
	destination := make(http.Header)

	copyCanvasProviderResponseHeaders(destination, source)

	require.Equal(t, "image/png", destination.Get("Content-Type"))
	require.Equal(t, `attachment; filename="result.png"`, destination.Get("Content-Disposition"))
	require.Equal(t, "provider-request", destination.Get("X-Request-ID"))
	require.Empty(t, destination.Get("Set-Cookie"))
	require.Empty(t, destination.Get("Access-Control-Allow-Origin"))
}

func TestStreamCanvasProviderResponseHonorsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Header("Content-Type", "text/event-stream")

	streamCanvasProviderResponse(context, strings.NewReader("data: first\n\ndata: second\n\n"), 12)

	require.Equal(t, "data: first\n", recorder.Body.String())
}
