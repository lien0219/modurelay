package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type totpLoginSecuritySettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *totpLoginSecuritySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func TestAdminCannotDisableTOTPWhileAdminMFAIsRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &totpLoginSecuritySettingRepo{values: map[string]string{
		"login_security_settings": `{"enabled":true,"request_limit_per_minute":20,"group_ipv6_by_64":true,"account_ip_failure_limit":5,"account_ip_window_minutes":30,"account_ip_block_minutes":30,"account_failure_limit":20,"account_window_minutes":30,"account_block_minutes":30,"admin_mfa_required":true}`,
	}}
	settings := service.NewSettingService(repo, nil)
	h := NewTotpHandler(nil)
	h.SetSettingService(settings)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/totp/disable", nil)
	ctx.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
	ctx.Set(string(servermiddleware.ContextKeyUserRole), string(service.RoleAdmin))
	h.Disable(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Disable administrator MFA requirement")
}

func TestAdminPasswordMFARequirementDoesNotBlockOrdinaryUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &totpLoginSecuritySettingRepo{values: map[string]string{
		"totp_enabled":            "true",
		"login_security_settings": `{"enabled":true,"request_limit_per_minute":20,"group_ipv6_by_64":true,"account_ip_failure_limit":5,"account_ip_window_minutes":30,"account_ip_block_minutes":30,"account_failure_limit":20,"account_window_minutes":30,"account_block_minutes":30,"admin_mfa_required":true}`,
	}}
	settings := service.NewSettingService(repo, nil)
	auth := &AuthHandler{settingSvc: settings, totpService: &service.TotpService{}}
	auth.SetLoginAbuseProtector(service.NewLoginAbuseProtector(nil, settings))

	ordinaryRecorder := httptest.NewRecorder()
	ordinaryCtx, _ := gin.CreateTestContext(ordinaryRecorder)
	ordinaryCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	require.True(t, auth.enforceAdminPasswordMFA(ordinaryCtx, &service.User{Role: service.RoleUser}))

	adminRecorder := httptest.NewRecorder()
	adminCtx, _ := gin.CreateTestContext(adminRecorder)
	adminCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	require.True(t, auth.enforceAdminPasswordMFA(adminCtx, &service.User{Role: service.RoleAdmin, TotpEnabled: true}))

	unenrolledRecorder := httptest.NewRecorder()
	unenrolledCtx, _ := gin.CreateTestContext(unenrolledRecorder)
	unenrolledCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	require.False(t, auth.enforceAdminPasswordMFA(unenrolledCtx, &service.User{Role: service.RoleAdmin}))
	require.Equal(t, http.StatusForbidden, unenrolledRecorder.Code)
}
