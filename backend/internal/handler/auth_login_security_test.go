package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type loginSecurityStoreCounter struct {
	count     int64
	expiresAt time.Time
}

type loginSecurityMemoryStore struct {
	mu       sync.Mutex
	now      time.Time
	counters map[string]loginSecurityStoreCounter
	blocks   map[string]time.Time
	audited  map[string]time.Time
	err      error
}

func newLoginSecurityMemoryStore() *loginSecurityMemoryStore {
	return &loginSecurityMemoryStore{
		now:      time.Now(),
		counters: make(map[string]loginSecurityStoreCounter),
		blocks:   make(map[string]time.Time),
		audited:  make(map[string]time.Time),
	}
}

func (s *loginSecurityMemoryStore) IncrementRequest(_ context.Context, counterKey, auditKey string, window time.Duration, limit int) (int64, time.Duration, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, 0, false, s.err
	}
	counter := s.increment(counterKey, window)
	audit := false
	if counter.count > int64(limit) && !s.now.Before(s.audited[auditKey]) {
		s.audited[auditKey] = counter.expiresAt
		audit = true
	}
	return counter.count, counter.expiresAt.Sub(s.now), audit, nil
}

func (s *loginSecurityMemoryStore) CredentialBlockTTLs(_ context.Context, accountIPKey, accountKey string) (time.Duration, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, 0, s.err
	}
	return s.blockTTL(accountIPKey), s.blockTTL(accountKey), nil
}

func (s *loginSecurityMemoryStore) IncrementFailure(_ context.Context, counterKey, blockKey string, window time.Duration, limit int, blockTTL time.Duration) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return 0, false, s.err
	}
	counter := s.increment(counterKey, window)
	blocked := counter.count >= int64(limit)
	if blocked {
		s.blocks[blockKey] = s.now.Add(blockTTL)
	}
	return counter.count, blocked, nil
}

func (s *loginSecurityMemoryStore) DeleteCounter(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	delete(s.counters, key)
	return nil
}

func (s *loginSecurityMemoryStore) increment(key string, ttl time.Duration) loginSecurityStoreCounter {
	counter := s.counters[key]
	if counter.count == 0 || !s.now.Before(counter.expiresAt) {
		counter = loginSecurityStoreCounter{expiresAt: s.now.Add(ttl)}
	}
	counter.count++
	s.counters[key] = counter
	return counter
}

func (s *loginSecurityMemoryStore) blockTTL(key string) time.Duration {
	expiresAt := s.blocks[key]
	if !s.now.Before(expiresAt) {
		delete(s.blocks, key)
		return 0
	}
	return expiresAt.Sub(s.now)
}

func (s *loginSecurityMemoryStore) setError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

type authLoginSecuritySettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *authLoginSecuritySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *authLoginSecuritySettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func (r *authLoginSecuritySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

type loginFlowUserRepository struct {
	service.UserRepository
	lookups int
	user    *service.User
}

func (r *loginFlowUserRepository) GetByEmail(_ context.Context, email string) (*service.User, error) {
	r.lookups++
	if r.user == nil || !strings.EqualFold(r.user.Email, email) {
		return nil, service.ErrUserNotFound
	}
	return r.user, nil
}

func (r *loginFlowUserRepository) GetByID(_ context.Context, id int64) (*service.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, service.ErrUserNotFound
	}
	return r.user, nil
}

type loginSecurityTotpCache struct {
	service.TotpCache
	sessions int
}

func (c *loginSecurityTotpCache) SetLoginSession(context.Context, string, *service.TotpLoginSession, time.Duration) error {
	c.sessions++
	return nil
}

type loginSecurityRefreshTokenCache struct {
	service.RefreshTokenCache
}

func (*loginSecurityRefreshTokenCache) StoreRefreshToken(context.Context, string, *service.RefreshTokenData, time.Duration) error {
	return nil
}

func (*loginSecurityRefreshTokenCache) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (*loginSecurityRefreshTokenCache) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

type loginAuditCaptureRepository struct {
	mu   sync.Mutex
	logs []*service.AuditLog
}

func (r *loginAuditCaptureRepository) BatchInsert(_ context.Context, logs []*service.AuditLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}
func (*loginAuditCaptureRepository) Insert(context.Context, *service.AuditLog) error { return nil }
func (*loginAuditCaptureRepository) List(context.Context, *service.AuditLogFilter) (*service.AuditLogList, error) {
	return &service.AuditLogList{}, nil
}
func (*loginAuditCaptureRepository) GetByID(context.Context, int64) (*service.AuditLog, error) {
	return nil, service.ErrAuditLogNotFound
}
func (*loginAuditCaptureRepository) Count(context.Context) (int64, error) { return 0, nil }
func (*loginAuditCaptureRepository) TruncateAll(context.Context) error    { return nil }
func (*loginAuditCaptureRepository) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func newLoginSecurityHandler(t *testing.T, settings *service.LoginSecuritySettings) (*AuthHandler, *loginSecurityMemoryStore) {
	t.Helper()
	store := newLoginSecurityMemoryStore()

	var settingService *service.SettingService
	if settings != nil {
		repo := &authLoginSecuritySettingRepo{values: make(map[string]string)}
		settingService = service.NewSettingService(repo, nil)
		require.NoError(t, settingService.SetLoginSecuritySettings(context.Background(), settings))
	}
	h := &AuthHandler{}
	h.SetLoginAbuseProtector(service.NewLoginAbuseProtector(store, settingService))
	return h, store
}

func newLoginSecurityContext(clientIP string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"sensitive-password","turnstile_token":"sensitive-captcha"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(service.WithSessionBinding(request.Context(), &service.SessionBinding{IP: clientIP}))
	ctx.Request = request
	return ctx, recorder
}

func TestLoginCaptchaFailureDoesNotIncrementPasswordCounter(t *testing.T) {
	h, _ := newLoginSecurityHandler(t, nil)
	ctx, _ := newLoginSecurityContext("203.0.113.20")

	require.False(t, h.recordLoginFailure(ctx, "user@example.com", errors.New("CAPTCHA_REJECTED")))
	outcome, err := h.loginAbuse.RecordPasswordFailure(context.Background(), "user@example.com", "203.0.113.20")
	require.NoError(t, err)
	require.EqualValues(t, 1, outcome.AccountIPCount)
	require.EqualValues(t, 1, outcome.AccountCount)
}

func TestLoginOnlyPasswordMismatchIncrementsPasswordCounter(t *testing.T) {
	h, _ := newLoginSecurityHandler(t, nil)
	ctx, _ := newLoginSecurityContext("203.0.113.21")

	require.False(t, h.recordLoginFailure(ctx, "user@example.com", service.ErrInvalidCredentials))
	outcome, err := h.loginAbuse.RecordPasswordFailure(context.Background(), "user@example.com", "203.0.113.21")
	require.NoError(t, err)
	require.EqualValues(t, 1, outcome.AccountIPCount)
	require.EqualValues(t, 1, outcome.AccountCount)

	require.False(t, h.recordLoginFailure(ctx, "user@example.com", service.ErrPasswordMismatch))
	outcome, err = h.loginAbuse.RecordPasswordFailure(context.Background(), "user@example.com", "203.0.113.21")
	require.NoError(t, err)
	require.EqualValues(t, 3, outcome.AccountIPCount)
	require.EqualValues(t, 3, outcome.AccountCount)

	for _, ignored := range []error{errors.New("TOTP_INVALID_CODE"), errors.New("backend mode restriction"), errors.New("database unavailable")} {
		require.False(t, h.recordLoginFailure(ctx, "other@example.com", ignored))
	}
	other, err := h.loginAbuse.RecordPasswordFailure(context.Background(), "other@example.com", "203.0.113.21")
	require.NoError(t, err)
	require.EqualValues(t, 1, other.AccountIPCount)
}

func TestLoginFlowChecksCaptchaBeforeCountingAndBlocksAfterFifthPasswordFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newLoginSecurityMemoryStore()

	settingRepo := &authLoginSecuritySettingRepo{values: map[string]string{
		service.SettingKeyTencentCaptchaEnabled: "true",
	}}
	settingService := service.NewSettingService(settingRepo, nil)
	require.NoError(t, settingService.SetLoginSecuritySettings(context.Background(), service.DefaultLoginSecuritySettings()))
	users := &loginFlowUserRepository{}
	authService := service.NewAuthService(nil, users, nil, nil, nil, settingService, nil, nil, nil, nil, nil, nil, nil)
	passwordHash, err := authService.HashPassword("correct-password")
	require.NoError(t, err)
	h := NewAuthHandler(nil, authService, nil, settingService, nil, nil, nil, nil)
	h.SetLoginAbuseProtector(service.NewLoginAbuseProtector(store, settingService))
	router := gin.New()
	router.POST("/api/v1/auth/login", h.LoginRequestRateLimit, h.Login)

	request := func(email, clientIP string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"wrong-password"}`, email)))
		req.RemoteAddr = clientIP + ":1234"
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		return recorder
	}

	captchaRejected := request("user@example.com", "203.0.113.24")
	require.NotEqual(t, http.StatusTooManyRequests, captchaRejected.Code)
	require.Equal(t, 0, users.lookups)

	settingRepo.values[service.SettingKeyTencentCaptchaEnabled] = "false"
	unknownAccount := request("user@example.com", "203.0.113.24")
	require.Equal(t, http.StatusUnauthorized, unknownAccount.Code)
	require.Equal(t, 1, users.lookups)
	users.user = &service.User{Email: "user@example.com", PasswordHash: passwordHash, Role: service.RoleUser}

	var passwordFailureResponse string
	for attempt := 0; attempt < 5; attempt++ {
		recorder := request("user@example.com", "203.0.113.25")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		if attempt == 0 {
			passwordFailureResponse = recorder.Body.String()
		}
	}
	require.Equal(t, unknownAccount.Body.String(), passwordFailureResponse)

	blocked := request("user@example.com", "203.0.113.25")
	require.Equal(t, http.StatusTooManyRequests, blocked.Code)
	require.Contains(t, blocked.Body.String(), "LOGIN_RATE_LIMITED")
	require.Equal(t, 6, users.lookups)

	for attempt := 0; attempt < 5; attempt++ {
		recorder := request("missing@example.com", "203.0.113.26")
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	}
	unknownBlocked := request("missing@example.com", "203.0.113.26")
	require.Equal(t, http.StatusTooManyRequests, unknownBlocked.Code)
	require.Equal(t, blocked.Body.String(), unknownBlocked.Body.String())
	require.Equal(t, 11, users.lookups)
}

func TestLoginInputAndStoredHashErrorsDoNotIncrementPasswordCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name         string
		passwordHash string
		password     string
		wantStatus   int
	}{
		{name: "malformed stored hash", passwordHash: "not-a-bcrypt-hash", password: "password", wantStatus: http.StatusUnauthorized},
		{name: "password too long", password: strings.Repeat("x", 73), wantStatus: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newLoginSecurityMemoryStore()

			settingRepo := &authLoginSecuritySettingRepo{}
			settingService := service.NewSettingService(settingRepo, nil)
			require.NoError(t, settingService.SetLoginSecuritySettings(context.Background(), service.DefaultLoginSecuritySettings()))
			passwordHash := test.passwordHash
			if passwordHash == "" {
				hashService := service.NewAuthService(nil, nil, nil, nil, nil, settingService, nil, nil, nil, nil, nil, nil, nil)
				var err error
				passwordHash, err = hashService.HashPassword("correct-password")
				require.NoError(t, err)
			}
			users := &loginFlowUserRepository{user: &service.User{
				Email:        "user@example.com",
				PasswordHash: passwordHash,
				Role:         service.RoleUser,
			}}
			authService := service.NewAuthService(nil, users, nil, nil, nil, settingService, nil, nil, nil, nil, nil, nil, nil)
			h := NewAuthHandler(nil, authService, nil, settingService, nil, nil, nil, nil)
			h.SetLoginAbuseProtector(service.NewLoginAbuseProtector(store, settingService))
			router := gin.New()
			router.POST("/api/v1/auth/login", h.LoginRequestRateLimit, h.Login)

			body := fmt.Sprintf(`{"email":"user@example.com","password":%q}`, test.password)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
			request.RemoteAddr = "203.0.113.25:1234"
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, test.wantStatus, recorder.Code)

			outcome, err := h.loginAbuse.RecordPasswordFailure(context.Background(), "user@example.com", "203.0.113.25")
			require.NoError(t, err)
			require.EqualValues(t, 1, outcome.AccountIPCount)
			require.EqualValues(t, 1, outcome.AccountCount)
		})
	}
}

func TestAdminPasswordLoginRequiresExistingTotpChallengeWhenMFAIsRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := *service.DefaultLoginSecuritySettings()
	settings.AdminMFARequired = true
	repo := &authLoginSecuritySettingRepo{values: map[string]string{
		service.SettingKeyTencentCaptchaEnabled: "false",
		"totp_enabled":                          "true",
	}}
	settingService := service.NewSettingService(repo, nil)
	require.NoError(t, settingService.SetLoginSecuritySettings(context.Background(), &settings))
	user := &service.User{
		ID:          1,
		Email:       "admin@example.com",
		Role:        service.RoleAdmin,
		Status:      service.StatusActive,
		TotpEnabled: true,
	}
	passwordHash, err := (&service.AuthService{}).HashPassword("correct-password")
	require.NoError(t, err)
	user.PasswordHash = passwordHash
	users := &loginFlowUserRepository{user: user}
	jwtConfig := &config.Config{}
	jwtConfig.JWT.Secret = "test-secret"
	jwtConfig.JWT.AccessTokenExpireMinutes = 15
	authService := service.NewAuthService(nil, users, nil, nil, jwtConfig, settingService, nil, nil, nil, nil, nil, nil, nil)
	totpCache := &loginSecurityTotpCache{}
	totpService := service.NewTotpService(users, nil, totpCache, settingService, nil, nil)
	h := NewAuthHandler(jwtConfig, authService, nil, settingService, nil, nil, totpService, nil)
	h.SetLoginAbuseProtector(service.NewLoginAbuseProtector(newLoginSecurityMemoryStore(), settingService))
	router := gin.New()
	router.POST("/api/v1/auth/login", h.LoginRequestRateLimit, h.Login)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"correct-password"}`))
	request.RemoteAddr = "203.0.113.26:1234"
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"requires_2fa":true`)
	require.NotContains(t, recorder.Body.String(), `"access_token"`)
	require.Equal(t, 1, totpCache.sessions)
}

func TestNormalPasswordLoginRecoversAfterOccasionalFailuresWithoutClearingGlobalHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The reserved fixture domain skips Ent-backed identity backfill.
	const loginEmail = "test@dingtalk-connect.invalid"
	settingRepo := &authLoginSecuritySettingRepo{values: map[string]string{
		service.SettingKeyTencentCaptchaEnabled: "false",
	}}
	settingService := service.NewSettingService(settingRepo, nil)
	require.NoError(t, settingService.SetLoginSecuritySettings(context.Background(), service.DefaultLoginSecuritySettings()))
	user := &service.User{
		ID:     2,
		Email:  loginEmail,
		Role:   service.RoleUser,
		Status: service.StatusActive,
	}
	passwordHash, err := (&service.AuthService{}).HashPassword("correct-password")
	require.NoError(t, err)
	user.PasswordHash = passwordHash
	users := &loginFlowUserRepository{user: user}
	jwtConfig := &config.Config{}
	jwtConfig.JWT.Secret = "test-secret"
	jwtConfig.JWT.AccessTokenExpireMinutes = 15
	jwtConfig.JWT.RefreshTokenExpireDays = 1
	tokenCache := &loginSecurityRefreshTokenCache{}
	authService := service.NewAuthService(nil, users, nil, tokenCache, jwtConfig, settingService, nil, nil, nil, nil, nil, nil, nil)
	h := NewAuthHandler(jwtConfig, authService, nil, settingService, nil, nil, nil, nil)
	store := newLoginSecurityMemoryStore()
	protector := service.NewLoginAbuseProtector(store, settingService)
	h.SetLoginAbuseProtector(protector)
	router := gin.New()
	router.POST("/api/v1/auth/login", h.LoginRequestRateLimit, h.Login)

	login := func(password string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(fmt.Sprintf(`{"email":%q,"password":%q}`, loginEmail, password)))
		request.RemoteAddr = "203.0.113.27:1234"
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		return recorder
	}

	firstSuccess := login("correct-password")
	require.Equal(t, http.StatusOK, firstSuccess.Code)
	require.Contains(t, firstSuccess.Body.String(), `"access_token"`)
	require.Equal(t, http.StatusUnauthorized, login("wrong-password").Code)
	require.Equal(t, http.StatusUnauthorized, login("wrong-password").Code)
	secondSuccess := login("correct-password")
	require.Equal(t, http.StatusOK, secondSuccess.Code)
	require.Contains(t, secondSuccess.Body.String(), `"access_token"`)

	outcome, err := protector.RecordPasswordFailure(context.Background(), loginEmail, "203.0.113.27")
	require.NoError(t, err)
	require.EqualValues(t, 1, outcome.AccountIPCount)
	require.EqualValues(t, 3, outcome.AccountCount)
}

func TestLoginRequestRateLimitFailsClosedAndSamplesUnavailableAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, store := newLoginSecurityHandler(t, nil)
	store.setError(errors.New("redis unavailable"))
	nextLoginProtectionUnavailableAudit.Store(0)

	first, firstRecorder := newLoginSecurityContext("203.0.113.22")
	h.LoginRequestRateLimit(first)
	require.Equal(t, http.StatusServiceUnavailable, firstRecorder.Code)
	require.Contains(t, firstRecorder.Body.String(), "Login protection is temporarily unavailable")
	action, exists := first.Get("audit_action")
	require.True(t, exists)
	require.Equal(t, "security.login.protection_unavailable", action)

	second, secondRecorder := newLoginSecurityContext("203.0.113.22")
	h.LoginRequestRateLimit(second)
	require.Equal(t, http.StatusServiceUnavailable, secondRecorder.Code)
	skip, exists := second.Get("audit_skip")
	require.True(t, exists)
	require.Equal(t, true, skip)
}

func TestLoginRequestThrottleWritesOneRepresentativeAuditEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := *service.DefaultLoginSecuritySettings()
	settings.RequestLimitPerMinute = 20
	h, _ := newLoginSecurityHandler(t, &settings)

	repository := &loginAuditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()
	defer auditService.Stop()

	router := gin.New()
	router.POST("/api/v1/auth/login",
		gin.HandlerFunc(middleware.NewAuditLogMiddleware(auditService)),
		h.LoginRequestRateLimit,
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	for attempt := range 22 {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(fmt.Sprintf(`{"email":"user@example.com","password":"password-%d","turnstile_token":"captcha-%d"}`, attempt, attempt)))
		request.RemoteAddr = "203.0.113.23:1234"
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if attempt < 20 {
			require.Equal(t, http.StatusOK, recorder.Code)
		} else {
			require.Equal(t, http.StatusTooManyRequests, recorder.Code)
			require.NotEmpty(t, recorder.Header().Get("Retry-After"))
		}
	}
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, 21)
	throttled := 0
	for _, entry := range logs {
		if entry.Action == "security.login.throttled" {
			throttled++
			require.Equal(t, "ip", entry.Extra["limit_scope"])
			require.Equal(t, "LOGIN_RATE_LIMITED", entry.Extra["error_code"])
		}
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
		require.NotContains(t, entry.RequestBody, "sensitive-password")
		require.NotContains(t, entry.RequestBody, "sensitive-captcha")
	}
	require.Equal(t, 1, throttled)
}
