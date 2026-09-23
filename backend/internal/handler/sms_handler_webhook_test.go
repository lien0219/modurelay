package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSMSWebhookRejectsSMSPVAWithoutReadingSecretOrPayload(t *testing.T) {
	t.Setenv("SMS_SMSPVA_WEBHOOK_SECRET", "configured-but-unsupported")
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "provider", Value: "SMSPVA"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sms/webhooks/SMSPVA", nil)

	NewSMSHandler(nil).Webhook(c)

	require.Equal(t, http.StatusNotImplemented, recorder.Code)
	var response struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusNotImplemented, response.Code)
	require.Equal(t, "WEBHOOK_UNSUPPORTED", response.Reason)
}

func TestSMSWebhookKeepsLegacyFiveSIMSecretGate(t *testing.T) {
	t.Setenv("SMS_5SIM_WEBHOOK_SECRET", "expected")
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "provider", Value: "5sim"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sms/webhooks/5sim", nil)
	c.Request.Header.Set("X-SMS-Webhook-Secret", "wrong")

	NewSMSHandler(nil).Webhook(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Equal(t, "WEBHOOK_UNAUTHORIZED", response.Reason)
}
