package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsMonitoringHandlerRejectsInvalidQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newIngressRejectHandlerForTest()
	for _, query := range []string{"metric=up", "metric=qps&step=-1", "metric=qps&min_requests=no", "metric=qps&window=0m", "metric=qps&start=no", "metric=qps&end=2026-01-01T00:00:00Z", "metric=hpa_at_max&service=api", "metric=qps&exclude_429=no"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/query?"+query, nil)
			h.QueryMonitoring(c)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
	for _, query := range []string{"limit=501", "limit=0", "start=invalid", "service=api", "deployment=api"} {
		t.Run("events "+query, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/events?"+query, nil)
			h.ListMonitoringEvents(c)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestOpsMonitoringHandlerStandaloneUnavailableIsExplicit(t *testing.T) {
	h := newIngressRejectHandlerForTest()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/query?metric=qps", nil)
	h.QueryMonitoring(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"state":"Unknown"`)
	require.Contains(t, w.Body.String(), `"series":[]`)
}
