package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateAdminMFAUserRequiresConfiguredAdminTOTP(t *testing.T) {
	tests := []struct {
		name string
		user *service.User
		want bool
	}{
		{name: "missing user", user: nil, want: false},
		{name: "ordinary user", user: &service.User{Role: service.RoleUser, TotpEnabled: true}, want: false},
		{name: "admin without totp", user: &service.User{Role: service.RoleAdmin}, want: false},
		{name: "admin with totp", user: &service.User{Role: service.RoleAdmin, TotpEnabled: true}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAdminMFAUser(test.user)
			if test.want {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestUpdateLoginSecuritySettingsRejectsAdminMFAWhenSystemTOTPIsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &loginSecuritySettingsRepoForAdminTest{values: map[string]string{"totp_enabled": "false"}}
	settings := service.NewSettingService(repo, nil)
	h := NewSettingHandler(settings, nil, nil, nil, nil, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/login-security", strings.NewReader(`{"enabled":true,"request_limit_per_minute":20,"group_ipv6_by_64":true,"account_ip_failure_limit":5,"account_ip_window_minutes":30,"account_ip_block_minutes":30,"account_failure_limit":20,"account_window_minutes":30,"account_block_minutes":30,"admin_mfa_required":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
	h.UpdateLoginSecuritySettings(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Enable system TOTP")
	require.Contains(t, recorder.Body.String(), "ADMIN_MFA_TOTP_FEATURE_REQUIRED")
	_, err := settings.GetLoginSecuritySettings(context.Background())
	require.NoError(t, err)
	require.False(t, repo.hasSetting("login_security_settings"))
}

func TestUpdateLoginSecuritySettingsRejectsAdminWithoutTotp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &loginSecuritySettingsRepoForAdminTest{values: map[string]string{"totp_enabled": "true"}}
	settings := service.NewSettingService(repo, nil)
	users := service.NewUserService(&loginSecurityAdminUserRepo{user: &service.User{ID: 1, Role: service.RoleAdmin}}, nil, nil, nil)
	h := NewSettingHandler(settings, nil, nil, nil, nil, nil, nil)
	h.userService = users

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/login-security", strings.NewReader(`{"enabled":true,"request_limit_per_minute":20,"group_ipv6_by_64":true,"account_ip_failure_limit":5,"account_ip_window_minutes":30,"account_ip_block_minutes":30,"account_failure_limit":20,"account_window_minutes":30,"account_block_minutes":30,"admin_mfa_required":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 1})
	h.UpdateLoginSecuritySettings(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "ADMIN_MFA_TOTP_REQUIRED")
	require.Contains(t, recorder.Body.String(), "Configure TOTP for the current administrator")
	require.False(t, repo.hasSetting("login_security_settings"))
}

type loginSecurityAdminUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *loginSecurityAdminUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, service.ErrUserNotFound
	}
	return r.user, nil
}

func (*loginSecurityAdminUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type loginSecuritySettingsRepoForAdminTest struct {
	service.SettingRepository
	values map[string]string
}

func (r *loginSecuritySettingsRepoForAdminTest) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *loginSecuritySettingsRepoForAdminTest) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *loginSecuritySettingsRepoForAdminTest) hasSetting(key string) bool {
	_, ok := r.values[key]
	return ok
}
