package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegionRestrictedPageChineseByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/region-restricted", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Equal(t, "zh-CN", rec.Header().Get("Content-Language"))
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	require.Contains(t, rec.Body.String(), "暂不对中国大陆地区开放")
	require.Contains(t, rec.Body.String(), "建议开启全局代理模式后重新访问")
	require.Contains(t, rec.Body.String(), "/region-restricted?lang=en")
}

func TestRegionRestrictedPageEnglishSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/region-restricted?lang=en", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "en", rec.Header().Get("Content-Language"))
	require.Contains(t, rec.Body.String(), "Temporarily unavailable in Mainland China")
	require.Contains(t, rec.Body.String(), "Global Proxy mode")
	require.Contains(t, rec.Body.String(), "/region-restricted?lang=zh")
}

func TestRegionRestrictedPageHeadHasNoBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r)

	req := httptest.NewRequest(http.MethodHead, "/region-restricted", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
}
