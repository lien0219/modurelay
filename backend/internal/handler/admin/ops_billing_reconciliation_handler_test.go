package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMonitoringBillingRecheckRequiresIdentityAndBoundedNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, id, body string
		user           bool
		status         int
	}{
		{"invalid id", "0", `{"note":"checked"}`, true, 400},
		{"missing identity", "1", `{"note":"checked"}`, false, 401},
		{"empty note", "1", `{"note":" "}`, true, 400},
		{"oversize note", "1", `{"note":"` + strings.Repeat("x", 1001) + `"}`, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newIngressRejectHandlerForTest()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/monitoring/billing/operations/"+tc.id+"/recheck", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "id", Value: tc.id}}
			if tc.user {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
			}
			h.RecheckBillingOperation(c)
			require.Equal(t, tc.status, c.Writer.Status())
		})
	}
}

func TestMonitoringBillingGapAcknowledgmentRequiresIdentityAndBoundedNote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, instance, body string
		user                 bool
		status               int
	}{
		{"invalid instance", " ", `{"note":"checked"}`, true, 400},
		{"oversize instance", strings.Repeat("x", 201), `{"note":"checked"}`, true, 400},
		{"missing identity", "pod-a", `{"note":"checked"}`, false, 401},
		{"empty note", "pod-a", `{"note":" "}`, true, 400},
		{"oversize note", "pod-a", `{"note":"` + strings.Repeat("x", 1001) + `"}`, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newIngressRejectHandlerForTest()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/monitoring/billing/recording-gaps/pod-a/acknowledge", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "instanceId", Value: tc.instance}}
			if tc.user {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
			}
			h.AcknowledgeBillingRecordingGap(c)
			require.Equal(t, tc.status, c.Writer.Status())
		})
	}
}
