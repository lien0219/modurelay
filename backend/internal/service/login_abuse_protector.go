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

	"github.com/redis/go-redis/v9"
)

var ErrAdminMFARequired = infraerrors.Forbidden(
	"ADMIN_MFA_REQUIRED",
	"administrator password sign-in requires TOTP; configure TOTP or use Passkey sign-in",
)

const loginSecuritySettingsCacheTTL = 15 * time.Second

var loginRequestRateScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
if current == 1 or ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {current, ttl}
`)

var loginFailureScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
if current == 1 or ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
local blocked = 0
if current >= tonumber(ARGV[2]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
  blocked = 1
end
return {current, ttl, blocked}
`)

type LoginAbuseDecision struct {
	Blocked    bool
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
}

// LoginAbuseProtector stores distributed login-abuse counters in Redis.
// It is intentionally separate from generic panel rate limiting so password
// failures can be counted only after CAPTCHA succeeds and password validation
// actually fails.
type LoginAbuseProtector struct {
	redis          *redis.Client
	settingService *SettingService

	cacheMu sync.RWMutex
	cache   cachedLoginSecuritySettings
}

func NewLoginAbuseProtector(redisClient *redis.Client, settingService *SettingService) *LoginAbuseProtector {
	return &LoginAbuseProtector{
		redis:          redisClient,
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
		settings := p.cache.settings
		p.cacheMu.RUnlock()
		return settings, nil
	}
	p.cacheMu.RUnlock()

	settings := DefaultLoginSecuritySettings()
	var err error
	if p.settingService != nil {
		settings, err = p.settingService.GetLoginSecuritySettings(ctx)
	}
	if err != nil {
		p.cacheMu.RLock()
		if !p.cache.expiresAt.IsZero() {
			stale := p.cache.settings
			p.cacheMu.RUnlock()
			return stale, nil
		}
		p.cacheMu.RUnlock()
		return LoginSecuritySettings{}, err
	}

	p.cacheMu.Lock()
	p.cache = cachedLoginSecuritySettings{
		settings:  *settings,
		expiresAt: now.Add(loginSecuritySettingsCacheTTL),
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
	if p.redis == nil {
		return LoginAbuseDecision{}, fmt.Errorf("login security redis is not configured")
	}

	window := time.Minute
	values, err := loginRequestRateScript.Run(
		ctx,
		p.redis,
		[]string{loginRequestKey(clientIP, settings)},
		window.Milliseconds(),
	).Slice()
	if err != nil {
		return LoginAbuseDecision{}, fmt.Errorf("check login request rate: %w", err)
	}
	if len(values) < 2 {
		return LoginAbuseDecision{}, fmt.Errorf("invalid login request rate response")
	}
	count, err := loginScriptInt64(values[0])
	if err != nil {
		return LoginAbuseDecision{}, err
	}
	ttlMs, err := loginScriptInt64(values[1])
	if err != nil {
		return LoginAbuseDecision{}, err
	}
	if count <= int64(settings.RequestLimitPerMinute) {
		return LoginAbuseDecision{}, nil
	}
	return LoginAbuseDecision{
		Blocked:    true,
		RetryAfter: positiveDuration(time.Duration(ttlMs)*time.Millisecond, window),
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
	if p.redis == nil {
		return LoginAbuseDecision{}, fmt.Errorf("login security redis is not configured")
	}

	accountIPBlock := loginAccountIPBase(email, clientIP, settings) + ":block"
	accountBlock := loginAccountBase(email) + ":block"
	pipe := p.redis.Pipeline()
	accountIPTTL := pipe.PTTL(ctx, accountIPBlock)
	accountTTL := pipe.PTTL(ctx, accountBlock)
	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return LoginAbuseDecision{}, fmt.Errorf("check login credential blocks: %w", err)
	}

	if ttl := accountTTL.Val(); ttl > 0 {
		return LoginAbuseDecision{Blocked: true, RetryAfter: ttl, Scope: "account"}, nil
	}
	if ttl := accountIPTTL.Val(); ttl > 0 {
		return LoginAbuseDecision{Blocked: true, RetryAfter: ttl, Scope: "account_ip"}, nil
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
	if p.redis == nil {
		return LoginFailureOutcome{}, fmt.Errorf("login security redis is not configured")
	}

	accountIPBase := loginAccountIPBase(email, clientIP, settings)
	accountBase := loginAccountBase(email)

	accountIPValues, err := loginFailureScript.Run(
		ctx,
		p.redis,
		[]string{accountIPBase + ":fail", accountIPBase + ":block"},
		(time.Duration(settings.AccountIPWindowMinutes)*time.Minute).Milliseconds(),
		settings.AccountIPFailureLimit,
		(time.Duration(settings.AccountIPBlockMinutes)*time.Minute).Milliseconds(),
	).Slice()
	if err != nil {
		return LoginFailureOutcome{}, fmt.Errorf("record account-ip login failure: %w", err)
	}

	accountValues, err := loginFailureScript.Run(
		ctx,
		p.redis,
		[]string{accountBase + ":fail", accountBase + ":block"},
		(time.Duration(settings.AccountWindowMinutes)*time.Minute).Milliseconds(),
		settings.AccountFailureLimit,
		(time.Duration(settings.AccountBlockMinutes)*time.Minute).Milliseconds(),
	).Slice()
	if err != nil {
		return LoginFailureOutcome{}, fmt.Errorf("record account login failure: %w", err)
	}

	accountIPCount, accountIPBlocked, err := loginFailureResult(accountIPValues)
	if err != nil {
		return LoginFailureOutcome{}, err
	}
	accountCount, accountBlocked, err := loginFailureResult(accountValues)
	if err != nil {
		return LoginFailureOutcome{}, err
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
	if !settings.Enabled || p.redis == nil {
		return nil
	}
	// Successful authentication clears only the source-specific failure counter.
	// The global account counter intentionally remains so a distributed attack
	// cannot erase history with one successful login.
	base := loginAccountIPBase(email, clientIP, settings)
	if err := p.redis.Del(ctx, base+":fail").Err(); err != nil {
		return fmt.Errorf("clear account-ip login failures: %w", err)
	}
	return nil
}

func loginFailureResult(values []any) (int64, bool, error) {
	if len(values) < 3 {
		return 0, false, fmt.Errorf("invalid login failure script response")
	}
	count, err := loginScriptInt64(values[0])
	if err != nil {
		return 0, false, err
	}
	blocked, err := loginScriptInt64(values[2])
	if err != nil {
		return 0, false, err
	}
	return count, blocked == 1, nil
}

func loginScriptInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		var parsed int64
		if _, err := fmt.Sscan(v, &parsed); err != nil {
			return 0, fmt.Errorf("parse login script integer: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected login script value type %T", value)
	}
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
