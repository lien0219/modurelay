package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type totpUpgradeUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r totpUpgradeUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}

type totpUpgradeCache struct {
	service.TotpCache
	grant bool
}

func (c *totpUpgradeCache) GetVerifyAttempts(context.Context, int64) (int, error) { return 0, nil }
func (c *totpUpgradeCache) ClearVerifyAttempts(context.Context, int64) error      { return nil }
func (c *totpUpgradeCache) SetStepUpGrant(_ context.Context, _ int64, _ string, _ time.Duration) error {
	c.grant = true
	return nil
}

type totpUpgradeCipher struct{}

func (totpUpgradeCipher) Decrypt(string) (string, error) { return "JBSWY3DPEHPK3PXP", nil }
func (totpUpgradeCipher) Encrypt(string) (string, error) { return "fixture-ciphertext", nil }

type totpUpgradeRefreshCache struct {
	service.RefreshTokenCache
	data     *service.RefreshTokenData
	consumed bool
}

func (c *totpUpgradeRefreshCache) StoreRefreshToken(_ context.Context, _ string, data *service.RefreshTokenData, _ time.Duration) error {
	copy := *data
	c.data = &copy
	return nil
}
func (c *totpUpgradeRefreshCache) GetRefreshToken(context.Context, string) (*service.RefreshTokenData, error) {
	copy := *c.data
	return &copy, nil
}
func (c *totpUpgradeRefreshCache) ConsumeRefreshToken(context.Context, string) (bool, error) {
	if c.consumed {
		return false, nil
	}
	c.consumed = true
	return true, nil
}
func (c *totpUpgradeRefreshCache) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}
func (c *totpUpgradeRefreshCache) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func TestTOTPStepUpAddsVerifiedSessionTokensAndKeepsGrantExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secretCipher := "fixture-ciphertext"
	user := &service.User{ID: 42, Email: "member@example.com", Status: service.StatusActive, Role: service.RoleUser, TotpEnabled: true, TotpSecretEncrypted: &secretCipher, TokenVersion: 7, TokenVersionResolved: true}
	users := totpUpgradeUserRepo{user: user}
	cache := &totpUpgradeCache{}
	factors := service.NewTotpService(users, totpUpgradeCipher{}, cache, nil, nil, nil)
	refreshCache := &totpUpgradeRefreshCache{}
	auth := service.NewAuthService(nil, users, nil, refreshCache, &config.Config{JWT: config.JWTConfig{Secret: "test-session-upgrade-secret", AccessTokenExpireMinutes: 60, RefreshTokenExpireDays: 7}}, nil, nil, nil, nil, nil, nil, nil, nil)
	auth.SetSessionMFAVerifier(factors)
	original := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	initial, err := auth.GenerateTokenPair(service.WithSessionAuthentication(context.Background(), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: original}), user, "")
	require.NoError(t, err)
	claims, err := auth.ValidateToken(initial.AccessToken)
	require.NoError(t, err)
	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	require.NoError(t, err)
	body, err := json.Marshal(map[string]string{"code": code, "refresh_token": initial.RefreshToken})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/totp/step-up", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Bearer "+initial.AccessToken)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, PrincipalType: service.PrincipalHuman})
	c.Set(middleware.ContextKeySessionID, claims.SessionID)
	h := NewTotpHandler(factors)
	h.SetAuthService(auth)
	h.StepUp(c)
	require.Equal(t, http.StatusOK, w.Code)
	var result struct {
		Data struct {
			Verified       bool   `json:"verified"`
			ExpiresIn      int64  `json:"expires_in"`
			AccessToken    string `json:"access_token"`
			RefreshToken   string `json:"refresh_token"`
			TokenType      string `json:"token_type"`
			TokenExpiresIn int    `json:"token_expires_in"`
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.True(t, result.Data.Verified)
	require.EqualValues(t, 900, result.Data.ExpiresIn)
	require.Equal(t, "Bearer", result.Data.TokenType)
	require.Equal(t, 3600, result.Data.TokenExpiresIn)
	require.NotEmpty(t, result.Data.RefreshToken)
	upgraded, err := auth.ValidateToken(result.Data.AccessToken)
	require.NoError(t, err)
	require.True(t, upgraded.MFASatisfied)
	require.Equal(t, original, upgraded.AuthenticatedAt)
	require.Equal(t, claims.SessionID, upgraded.SessionID)
	require.True(t, cache.grant)
}
