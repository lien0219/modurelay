package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

const settingKeyLoginSecuritySettings = "login_security_settings"

// LoginSecuritySettings controls password-login abuse protection.
// CAPTCHA failures are deliberately excluded from password-failure counters.
type LoginSecuritySettings struct {
	Enabled                  bool `json:"enabled"`
	RequestLimitPerMinute    int  `json:"request_limit_per_minute"`
	GroupIPv6By64            bool `json:"group_ipv6_by_64"`
	AccountIPFailureLimit    int  `json:"account_ip_failure_limit"`
	AccountIPWindowMinutes   int  `json:"account_ip_window_minutes"`
	AccountIPBlockMinutes    int  `json:"account_ip_block_minutes"`
	AccountFailureLimit      int  `json:"account_failure_limit"`
	AccountWindowMinutes     int  `json:"account_window_minutes"`
	AccountBlockMinutes      int  `json:"account_block_minutes"`
	AdminMFARequired         bool `json:"admin_mfa_required"`
}

func DefaultLoginSecuritySettings() *LoginSecuritySettings {
	return &LoginSecuritySettings{
		Enabled:                true,
		RequestLimitPerMinute:  20,
		GroupIPv6By64:          true,
		AccountIPFailureLimit:  5,
		AccountIPWindowMinutes: 30,
		AccountIPBlockMinutes:  30,
		AccountFailureLimit:    20,
		AccountWindowMinutes:   30,
		AccountBlockMinutes:    30,
		AdminMFARequired:       false,
	}
}

func validateLoginSecuritySettings(s *LoginSecuritySettings) error {
	if s == nil {
		return fmt.Errorf("login security settings cannot be nil")
	}
	if s.RequestLimitPerMinute < 5 || s.RequestLimitPerMinute > 300 {
		return fmt.Errorf("request_limit_per_minute must be between 5 and 300")
	}
	if s.AccountIPFailureLimit < 3 || s.AccountIPFailureLimit > 20 {
		return fmt.Errorf("account_ip_failure_limit must be between 3 and 20")
	}
	if s.AccountIPWindowMinutes < 5 || s.AccountIPWindowMinutes > 120 {
		return fmt.Errorf("account_ip_window_minutes must be between 5 and 120")
	}
	if s.AccountIPBlockMinutes < 5 || s.AccountIPBlockMinutes > 1440 {
		return fmt.Errorf("account_ip_block_minutes must be between 5 and 1440")
	}
	if s.AccountFailureLimit < s.AccountIPFailureLimit || s.AccountFailureLimit > 200 {
		return fmt.Errorf("account_failure_limit must be between account_ip_failure_limit and 200")
	}
	if s.AccountWindowMinutes < 5 || s.AccountWindowMinutes > 240 {
		return fmt.Errorf("account_window_minutes must be between 5 and 240")
	}
	if s.AccountBlockMinutes < 5 || s.AccountBlockMinutes > 1440 {
		return fmt.Errorf("account_block_minutes must be between 5 and 1440")
	}
	return nil
}

func (s *SettingService) GetLoginSecuritySettings(ctx context.Context) (*LoginSecuritySettings, error) {
	defaults := DefaultLoginSecuritySettings()
	if s == nil || s.settingRepo == nil {
		return defaults, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, settingKeyLoginSecuritySettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return defaults, nil
		}
		return nil, fmt.Errorf("get login security settings: %w", err)
	}
	if raw == "" {
		return defaults, nil
	}

	settings := &LoginSecuritySettings{}
	if err := json.Unmarshal([]byte(raw), settings); err != nil {
		slog.Warn("invalid persisted login security settings; using secure defaults", "error", err)
		return defaults, nil
	}
	if err := validateLoginSecuritySettings(settings); err != nil {
		slog.Warn("unsafe persisted login security settings; using secure defaults", "error", err)
		return defaults, nil
	}
	return settings, nil
}

func (s *SettingService) SetLoginSecuritySettings(ctx context.Context, settings *LoginSecuritySettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("setting repository is not configured")
	}
	if err := validateLoginSecuritySettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal login security settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, settingKeyLoginSecuritySettings, string(data)); err != nil {
		return fmt.Errorf("save login security settings: %w", err)
	}
	return nil
}
