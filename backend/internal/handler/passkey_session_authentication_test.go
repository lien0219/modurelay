package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPasskeyLoginSessionRecordsMethodWithoutInferringMFA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 42, Email: "member@example.com", Role: service.RoleUser, Status: service.StatusActive, TotpEnabled: true, TokenVersion: 7, TokenVersionResolved: true}
	auth := service.NewAuthService(nil, totpUpgradeUserRepo{user: user}, nil, nil, &config.Config{JWT: config.JWTConfig{Secret: "test-passkey-session-secret", AccessTokenExpireMinutes: 60}}, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &PasskeyHandler{authService: auth}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkey/login/finish", nil).WithContext(service.WithSessionAuthentication(context.Background(), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().Add(-time.Hour), MFASatisfied: true}))
	before := time.Now()
	h.respondWithLoginSession(c, user)
	after := time.Now()
	var result struct {
		Data struct {
			AccessToken string `json:"access_token"`
		}
	}
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	claims, err := auth.ValidateToken(result.Data.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "passkey", claims.AuthMethod)
	require.False(t, claims.MFASatisfied)
	require.False(t, claims.AuthenticatedAt.Before(before))
	require.False(t, claims.AuthenticatedAt.After(after))
}
