package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
)

var ErrAdminMFARequired = infraerrors.Forbidden(
	"ADMIN_MFA_REQUIRED",
	"administrator password sign-in requires TOTP; configure TOTP or use Passkey sign-in",
)

const loginSecuritySettingsCacheTTL = 15 * time.Second

// LoginAbuseStore owns the distributed atomic operations used by login
// protection. The service keeps identity normalization and policy decisions
// independent from the Redis implementation.
type LoginAbuseStore interface {
	IncrementRequest(ctx context.Context, counterKey, auditKey string, window time.Duration, limit int) (count int64, ttl time.Duration, audit bool, err error)
	CredentialBlockTTLs(ctx context.Context, accountIPKey, accountKey string) (accountIPTTL, accountTTL time.Duration, err error)
	IncrementFailure(ctx context.Context, counterKey, blockKey string, window time.Duration, limit int, blockTTL time.Duration) (count int64, blocked bool, err error)
	DeleteCounter(ctx context.Context, key string) error
}

type LoginAbuseDecision struct {
	Blocked    bool
	Audit      bool
	RetryAfter time.Duration
	Scope      string
}

type LoginFailureOutcome struct {
	AccountIPCount   int64
	AccountCount     int64
	AccountIPBlocked bool
	AccountBlocked   bool
}

type cachedLoginSecuritySettings struct {
	settings  LoginSecuritySettings
	expiresAt time.Time
	valid     bool
	loadErr   error
}

// LoginAbuseProtector stores distributed login-abuse counters in Redis.
// It is intentionally separate from generic panel rate limiting so password
// failures can be counted only after CAPTCHA succeeds and password validation
// actually fails.
type LoginAbuseProtector struct {
	store          LoginAbuseStore
	settingService *SettingService

	loadMu  sync.Mutex
	cacheMu sync.RWMutex
	cache   cachedLoginSecuritySettings
}

func NewLoginAbuseProtector(store LoginAbuseStore, settingService *SettingService) *LoginAbuseProtector {
	return &LoginAbuseProtector{
		store:          store,
		settingService: settingService,
	}
}

func (p *LoginAbuseProtector) Settings(ctx context.Context) (LoginSecuritySettings, error) {
	if p == nil {
		return *DefaultLoginSecuritySettings(), nil
	}
	now := time.Now()
	p.cacheMu.RLock()
	if !p.cache.expiresAt.IsZero() && now.Before(p.cache.expiresAt) {
		settings, loadErr, valid := p.cache.settings, p.cache.loadErr, p.cache.valid
		p.cacheMu.RUnlock()
		if !valid {
			return LoginSecuritySettings{}, loadErr
		}
		return settings, nil
	}
	p.cacheMu.RUnlock()

	p.loadMu.Lock()
	defer p.loadMu.Unlock()

	// Another request may have refreshed the setting while this request waited.
	now = time.Now()
	p.cacheMu.RLock()
	if !p.cache.expiresAt.IsZero() && now.Before(p.cache.expiresAt) {
		settings, loadErr, valid := p.cache.settings, p.cache.loadErr, p.cache.valid
		p.cacheMu.RUnlock()
		if !valid {
			return LoginSecuritySettings{}, loadErr
		}
		return settings, nil
	}
	p.cacheMu.RUnlock()

	settings := DefaultLoginSecuritySettings()
	var err error
	if p.settingService != nil {
		settings, err = p.settingService.GetLoginSecuritySettings(ctx)
	}
	if err != nil {
		p.cacheMu.Lock()
		p.cache = cachedLoginSecuritySettings{
			expiresAt: time.Now().Add(loginSecuritySettingsCacheTTL),
			loadErr:   err,
		}
		p.cacheMu.Unlock()
		return LoginSecuritySettings{}, err
	}

	p.cacheMu.Lock()
	p.cache = cachedLoginSecuritySettings{
		settings:  *settings,
		expiresAt: time.Now().Add(loginSecuritySettingsCacheTTL),
		valid:     true,
	}
	p.cacheMu.Unlock()
	return *settings, nil
}

func normalizeLoginEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func loginIdentityHash(email string) string {
	sum := sha256.Sum256([]byte(normalizeLoginEmail(email)))
	return hex.EncodeToString(sum[:])
}

func normalizedLoginIP(clientIP string, settings LoginSecuritySettings) string {
	normalized := ippkg.NormalizeAbuseIP(clientIP, settings.GroupIPv6By64)
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func loginRequestKey(clientIP string, settings LoginSecuritySettings) string {
	return "login_security:req:" + normalizedLoginIP(clientIP, settings)
}

func loginAccountIPBase(email, clientIP string, settings LoginSecuritySettings) string {
	return "login_security:account_ip:" + loginIdentityHash(email) + ":" + normalizedLoginIP(clientIP, settings)
}

func loginAccountBase(email string) string {
	return "login_security:account:" + loginIdentityHash(email)
}

func (p *LoginAbuseProtector) CheckRequestRate(ctx context.Context, clientIP string) (LoginAbuseDecision, error) {
	settings, err := p.Settings(ctx)
	if err != nil {
		return LoginAbuseDecision{}, err
	}
	if !settings.Enabled {
		return LoginAbuseDecision{}, nil
	}
	if p.store == nil {
		return LoginAbuseDecision{}, fmt.Errorf("login abuse store is not configured")
	}

	window := time.Minute
	count, ttl, audit, err := p.store.IncrementRequest(
		ctx,
		loginRequestKey(clientIP, settings),
		loginRequestKey(clientIP, settings)+":audit",
		window,
		settings.RequestLimitPerMinute,
	)
	if err != nil {
		return LoginAbuseDecision{}, fmt.Errorf("check login request rate: %w", err)
	}
	if count <= int64(settings.RequestLimitPerMinute) {
		return LoginAbuseDecision{}, nil
	}
	return LoginAbuseDecision{
		Blocked:    true,
		Audit:      audit,
		RetryAfter: positiveDuration(ttl, window),
		Scope:      "ip",
	}, nil
}

func (p *LoginAbuseProtector) CheckCredentialsAllowed(ctx context.Context, email, clientIP string) (LoginAbuseDecision, error) {
	settings, err := p.Settings(ctx)
	if err != nil {
		return LoginAbuseDecision{}, err
	}
	if !settings.Enabled {
		return LoginAbuseDecision{}, nil
	}
	if p.store == nil {
		return LoginAbuseDecision{}, fmt.Errorf("login abuse store is not configured")
	}

	accountIPBlock := loginAccountIPBase(email, clientIP, settings) + ":block"
	accountBlock := loginAccountBase(email) + ":block"
	accountIPTTL, accountTTL, err := p.store.CredentialBlockTTLs(ctx, accountIPBlock, accountBlock)
	if err != nil {
		return LoginAbuseDecision{}, fmt.Errorf("check login credential blocks: %w", err)
	}

	if accountTTL > 0 {
		return LoginAbuseDecision{Blocked: true, RetryAfter: accountTTL, Scope: "account"}, nil
	}
	if accountIPTTL > 0 {
		return LoginAbuseDecision{Blocked: true, RetryAfter: accountIPTTL, Scope: "account_ip"}, nil
	}
	return LoginAbuseDecision{}, nil
}

func (p *LoginAbuseProtector) RecordPasswordFailure(ctx context.Context, email, clientIP string) (LoginFailureOutcome, error) {
	settings, err := p.Settings(ctx)
	if err != nil {
		return LoginFailureOutcome{}, err
	}
	if !settings.Enabled {
		return LoginFailureOutcome{}, nil
	}
	if p.store == nil {
		return LoginFailureOutcome{}, fmt.Errorf("login abuse store is not configured")
	}

	accountIPBase := loginAccountIPBase(email, clientIP, settings)
	accountBase := loginAccountBase(email)

	accountIPCount, accountIPBlocked, err := p.store.IncrementFailure(
		ctx,
		accountIPBase+":fail",
		accountIPBase+":block",
		time.Duration(settings.AccountIPWindowMinutes)*time.Minute,
		settings.AccountIPFailureLimit,
		time.Duration(settings.AccountIPBlockMinutes)*time.Minute,
	)
	if err != nil {
		return LoginFailureOutcome{}, fmt.Errorf("record account-ip login failure: %w", err)
	}

	accountCount, accountBlocked, err := p.store.IncrementFailure(
		ctx,
		accountBase+":fail",
		accountBase+":block",
		time.Duration(settings.AccountWindowMinutes)*time.Minute,
		settings.AccountFailureLimit,
		time.Duration(settings.AccountBlockMinutes)*time.Minute,
	)
	if err != nil {
		return LoginFailureOutcome{}, fmt.Errorf("record account login failure: %w", err)
	}
	return LoginFailureOutcome{
		AccountIPCount:   accountIPCount,
		AccountCount:     accountCount,
		AccountIPBlocked: accountIPBlocked,
		AccountBlocked:   accountBlocked,
	}, nil
}

func (p *LoginAbuseProtector) RecordSuccess(ctx context.Context, email, clientIP string) error {
	settings, err := p.Settings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled || p.store == nil {
		return nil
	}
	// Successful authentication clears only the source-specific failure counter.
	// The global account counter intentionally remains so a distributed attack
	// cannot erase history with one successful login.
	base := loginAccountIPBase(email, clientIP, settings)
	if err := p.store.DeleteCounter(ctx, base+":fail"); err != nil {
		return fmt.Errorf("clear account-ip login failures: %w", err)
	}
	return nil
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
