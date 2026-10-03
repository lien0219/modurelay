package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSeedanceModelsUseConfiguredModelsWithoutInventedFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.Empty(t, defaultModelIDsForPlatform(service.PlatformSeedance))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writeModelsList(c, service.PlatformSeedance, []string{"vendor-video-model"})

	require.Equal(t, "list", gjson.GetBytes(recorder.Body.Bytes(), "object").String())
	require.Equal(t, "vendor-video-model", gjson.GetBytes(recorder.Body.Bytes(), "data.0.id").String())
}
