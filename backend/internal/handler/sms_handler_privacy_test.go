package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSMSPrivacyContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	return c, recorder
}

func assertOpaqueSMSChannelRejection(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusNotFound, recorder.Code)
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, http.StatusNotFound, payload.Code)
	require.Equal(t, "CHANNEL_UNAVAILABLE", payload.Reason)
	require.NotContains(t, strings.ToLower(payload.Message), "smspva")
	require.NotContains(t, strings.ToLower(recorder.Body.String()), "5sim")
}

func TestSMSUserCatalogRejectsRawProviderCodes(t *testing.T) {
	h := NewSMSHandler(nil)
	cases := []struct {
		name   string
		invoke func(*gin.Context)
	}{
		{
			name: "services",
			invoke: func(c *gin.Context) {
				c.Params = gin.Params{{Key: "provider", Value: "smspva"}}
				h.ProviderServices(c)
			},
		},
		{
			name: "countries",
			invoke: func(c *gin.Context) {
				c.Params = gin.Params{{Key: "provider", Value: "5sim"}, {Key: "service", Value: "google"}}
				h.ServiceCountries(c)
			},
		},
		{
			name: "operators",
			invoke: func(c *gin.Context) {
				c.Params = gin.Params{{Key: "provider", Value: "smspva"}, {Key: "service", Value: "google"}, {Key: "country", Value: "US"}}
				h.ProviderOperators(c)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newSMSPrivacyContext(http.MethodGet, "/api/v1/sms/providers/smspva/services")
			tc.invoke(c)
			assertOpaqueSMSChannelRejection(t, recorder)
		})
	}
}

func TestSMSQuoteRejectsRawProviderQuery(t *testing.T) {
	h := NewSMSHandler(nil)
	c, recorder := newSMSPrivacyContext(http.MethodGet, "/api/v1/sms/quotes?provider=smspva&service=google&country=US")
	h.Quotes(c)
	assertOpaqueSMSChannelRejection(t, recorder)
}

func TestSMSPublicChannelValidationOnlyAcceptsOpaqueChannels(t *testing.T) {
	for _, value := range []string{"channel_1", "CHANNEL_2"} {
		require.True(t, isPublicSMSChannel(value), value)
	}
	for _, value := range []string{"", "smspva", "5sim", "channel_3", "channel_1/smspva"} {
		require.False(t, isPublicSMSChannel(value), value)
	}
}
