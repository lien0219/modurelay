package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	downstreamBillingProbeCacheTTL  = 60 * time.Second
	downstreamBillingProbeErrorTTL  = 5 * time.Second
	downstreamBillingProbeDBTimeout = 5 * time.Second
)

type DownstreamBillingProbeSettings struct {
	Enabled bool `json:"enabled"`
}

type cachedDownstreamBillingProbeSettings struct {
	enabled   bool
	expiresAt int64
}

func DefaultDownstreamBillingProbeSettings() *DownstreamBillingProbeSettings {
	return &DownstreamBillingProbeSettings{Enabled: true}
}

// GetDownstreamBillingProbeSettings reads the persisted value for admin views.
// A missing value keeps the existing public billing endpoint enabled.
func (s *SettingService) GetDownstreamBillingProbeSettings(ctx context.Context) (*DownstreamBillingProbeSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("setting repository is unavailable")
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDownstreamBillingProbeEnabled)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultDownstreamBillingProbeSettings(), nil
		}
		return nil, fmt.Errorf("get downstream billing probe setting: %w", err)
	}
	switch strings.TrimSpace(value) {
	case "true":
		return &DownstreamBillingProbeSettings{Enabled: true}, nil
	case "false":
		return &DownstreamBillingProbeSettings{Enabled: false}, nil
	default:
		return nil, fmt.Errorf("invalid downstream billing probe setting")
	}
}

// SetDownstreamBillingProbeSettings persists the switch and refreshes the
// current process immediately. Other nodes refresh within the cache TTL.
func (s *SettingService) SetDownstreamBillingProbeSettings(ctx context.Context, settings *DownstreamBillingProbeSettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("setting repository is unavailable")
	}
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	value := "false"
	if settings.Enabled {
		value = "true"
	}
	if err := s.settingRepo.Set(ctx, SettingKeyDownstreamBillingProbeEnabled, value); err != nil {
		return fmt.Errorf("set downstream billing probe setting: %w", err)
	}
	s.storeDownstreamBillingProbeCache(settings.Enabled, downstreamBillingProbeCacheTTL)
	return nil
}

// IsDownstreamBillingProbeEnabled is the fail-closed runtime read used by the
// authenticated billing endpoint. Missing configuration defaults to enabled
// for backward compatibility; repository and parse errors disable disclosure.
func (s *SettingService) IsDownstreamBillingProbeEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	if cached, ok := s.downstreamBillingProbeCache.Load().(*cachedDownstreamBillingProbeSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled
		}
	}

	result, _, _ := s.downstreamBillingProbeSF.Do(SettingKeyDownstreamBillingProbeEnabled, func() (any, error) {
		if cached, ok := s.downstreamBillingProbeCache.Load().(*cachedDownstreamBillingProbeSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.enabled, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), downstreamBillingProbeDBTimeout)
		defer cancel()

		settings, err := s.GetDownstreamBillingProbeSettings(dbCtx)
		if err != nil {
			slog.Warn("failed to get downstream billing probe setting; disabling disclosure", "error", err)
			s.storeDownstreamBillingProbeCache(false, downstreamBillingProbeErrorTTL)
			return false, nil
		}
		s.storeDownstreamBillingProbeCache(settings.Enabled, downstreamBillingProbeCacheTTL)
		return settings.Enabled, nil
	})
	enabled, _ := result.(bool)
	return enabled
}

func (s *SettingService) storeDownstreamBillingProbeCache(enabled bool, ttl time.Duration) {
	s.downstreamBillingProbeCache.Store(&cachedDownstreamBillingProbeSettings{
		enabled:   enabled,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}
