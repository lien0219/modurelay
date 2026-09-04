package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newDownstreamBillingProbeSettingsRouter(repo service.SettingRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := &SettingHandler{
		settingService: service.NewSettingService(repo, &config.Config{}),
	}
	router := gin.New()
	router.GET("/admin/settings/downstream-billing-probe", handler.GetDownstreamBillingProbeSettings)
	router.PUT("/admin/settings/downstream-billing-probe", handler.UpdateDownstreamBillingProbeSettings)
	return router
}

func TestDownstreamBillingProbeSettingsHandlers(t *testing.T) {
	repo := newTestSettingRepo()
	router := newDownstreamBillingProbeSettingsRouter(repo)

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/admin/settings/downstream-billing-probe", nil))
	require.Equal(t, http.StatusOK, getRecorder.Code)
	var getResponse struct {
		Data service.DownstreamBillingProbeSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &getResponse))
	require.True(t, getResponse.Data.Enabled)

	putRecorder := httptest.NewRecorder()
	putRequest := httptest.NewRequest(
		http.MethodPut,
		"/admin/settings/downstream-billing-probe",
		bytes.NewBufferString(`{"enabled":false}`),
	)
	putRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(putRecorder, putRequest)
	require.Equal(t, http.StatusOK, putRecorder.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyDownstreamBillingProbeEnabled])

	getRecorder = httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/admin/settings/downstream-billing-probe", nil))
	require.Equal(t, http.StatusOK, getRecorder.Code)
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &getResponse))
	require.False(t, getResponse.Data.Enabled)
}

func TestUpdateDownstreamBillingProbeSettingsRequiresEnabled(t *testing.T) {
	router := newDownstreamBillingProbeSettingsRouter(newTestSettingRepo())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/admin/settings/downstream-billing-probe",
		bytes.NewBufferString(`{}`),
	)
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
