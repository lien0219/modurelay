package service

import (
	"context"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const StatusAPIKeyRevoked = "revoked"
const (
	SettingKeyServiceAccountsPerProject = "service_accounts_max_per_project"
	SettingKeyServiceAccountCredentials = "service_accounts_max_active_credentials"
)

var ErrServiceAccountLimit = infraerrors.Conflict("SERVICE_ACCOUNT_LIMIT", "service account or active credential limit reached")
var ErrServiceAccountSecretConsumed = infraerrors.Conflict("SERVICE_ACCOUNT_SECRET_CONSUMED", "credential already created; revoke and recreate if the one-time secret was lost")

type ServiceAccount struct {
	ID              int64      `json:"id"`
	WorkspaceID     int64      `json:"workspace_id"`
	ProjectID       int64      `json:"project_id"`
	Name            string     `json:"name"`
	Slug            string     `json:"slug"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	CreatedByUserID *int64     `json:"created_by_user_id"`
	DisabledAt      *time.Time `json:"disabled_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
type ServiceAccountInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}
type UpdateServiceAccountInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// This DTO deliberately has no field for the stored lookup digest or user.
type ServiceAccountCredential struct {
	ID               int64      `json:"id"`
	ServiceAccountID *int64     `json:"service_account_id"`
	ProjectID        *int64     `json:"project_id"`
	Name             string     `json:"name"`
	KeySuffix        *string    `json:"key_suffix"`
	Status           string     `json:"status"`
	GroupID          *int64     `json:"group_id"`
	IPWhitelist      []string   `json:"ip_whitelist"`
	IPBlacklist      []string   `json:"ip_blacklist"`
	Quota            float64    `json:"quota"`
	QuotaUsed        float64    `json:"quota_used"`
	ExpiresAt        *time.Time `json:"expires_at"`
	RateLimit5h      float64    `json:"rate_limit_5h"`
	RateLimit1d      float64    `json:"rate_limit_1d"`
	RateLimit7d      float64    `json:"rate_limit_7d"`
	Usage5h          float64    `json:"usage_5h"`
	Usage1d          float64    `json:"usage_1d"`
	Usage7d          float64    `json:"usage_7d"`
	LastUsedAt       *time.Time `json:"last_used_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func ServiceAccountCredentialFromKey(k *APIKey) *ServiceAccountCredential {
	if k == nil {
		return nil
	}
	white, black := append([]string{}, k.IPWhitelist...), append([]string{}, k.IPBlacklist...)
	return &ServiceAccountCredential{k.ID, k.ServiceAccountID, k.ProjectID, k.Name, k.KeySuffix, k.Status, k.GroupID, white, black, k.Quota, k.QuotaUsed, k.ExpiresAt, k.RateLimit5h, k.RateLimit1d, k.RateLimit7d, k.Usage5h, k.Usage1d, k.Usage7d, k.LastUsedAt, k.CreatedAt, k.UpdatedAt}
}

type ServiceAccountSecret struct {
	Credential           *ServiceAccountCredential `json:"credential"`
	Secret               string                    `json:"secret"`
	PreviousCredentialID *int64                    `json:"previous_credential_id,omitempty"`
}

type ServiceAccountTransaction interface {
	CountAccounts(context.Context) (int, error)
	CreateAccount(context.Context, *ServiceAccount) error
	SaveAccount(context.Context, *ServiceAccount) error
	Credentials(context.Context) ([]APIKey, error)
	Credential(context.Context, int64) (*APIKey, error)
	CountActiveCredentials(context.Context) (int, error)
	CreateCredential(context.Context, *APIKey) error
	SaveCredential(context.Context, *APIKey) error
	CheckCredentialBinding(context.Context, *APIKey) error
	Event(context.Context, string, *ServiceAccount, *APIKey, int64) error
}
type ServiceAccountRepository interface {
	List(context.Context, int64, int64, int64, pagination.PaginationParams) ([]ServiceAccount, int64, error)
	Get(context.Context, int64, int64, int64, int64) (*ServiceAccount, error)
	Credentials(context.Context, int64, int64, int64, int64) ([]APIKey, error)
	WithMutation(context.Context, int64, int64, int64, int64, string, bool, func(*ServiceAccount, *WorkspaceAccess, ServiceAccountTransaction) error) error
	AdminList(context.Context, int64, string, pagination.PaginationParams) ([]ServiceAccount, int64, error)
	AdminGet(context.Context, int64, int64) (*ServiceAccount, []APIKey, error)
}
type ServiceAccountService struct {
	repo     ServiceAccountRepository
	access   *WorkspaceAccessService
	keys     *APIKeyService
	settings SettingRepository
}

func NewServiceAccountService(r ServiceAccountRepository, a *WorkspaceAccessService, k *APIKeyService, settings SettingRepository) *ServiceAccountService {
	return &ServiceAccountService{r, a, k, settings}
}
func (s *ServiceAccountService) limit(ctx context.Context, key string, fallback int) int {
	if s.settings != nil {
		if raw, e := s.settings.GetValue(ctx, key); e == nil {
			if n, e := strconv.Atoi(raw); e == nil && n > 0 && n <= 10000 {
				return n
			}
		}
	}
	return fallback
}

func validServiceAccountDetails(name, description string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(name)) > 0 && utf8.RuneCountInString(name) <= 100 && utf8.RuneCountInString(description) <= 2000
}
func (s *ServiceAccountService) List(ctx context.Context, a, w, p int64, params pagination.PaginationParams) ([]ServiceAccount, int64, error) {
	if _, e := s.access.RequireProject(ctx, a, w, p, "service_account.read"); e != nil {
		return nil, 0, e
	}
	return s.repo.List(ctx, a, w, p, params)
}
func (s *ServiceAccountService) Get(ctx context.Context, a, w, p, id int64) (*ServiceAccount, error) {
	if _, e := s.access.RequireProject(ctx, a, w, p, "service_account.read"); e != nil {
		return nil, e
	}
	return s.repo.Get(ctx, a, w, p, id)
}

// Recheck human management authorization before consulting an idempotency
// response, so a previously authorized actor cannot replay after removal.
func (s *ServiceAccountService) RequireCredentialMutation(ctx context.Context, a, w, p, id int64, permission string) error {
	if _, err := s.access.RequireProject(ctx, a, w, p, permission); err != nil {
		return err
	}
	_, err := s.repo.Get(ctx, a, w, p, id)
	return err
}

func serviceAccountCredentialCountsAsActive(k *APIKey, now time.Time) bool {
	return k != nil && (k.Status == StatusActive || k.Status == StatusAPIKeyQuotaExhausted) && (k.ExpiresAt == nil || k.ExpiresAt.After(now))
}
func (s *ServiceAccountService) Create(ctx context.Context, a, w, p int64, in ServiceAccountInput) (*ServiceAccount, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.TrimSpace(in.Slug)
	if !validServiceAccountDetails(in.Name, in.Description) || !workspaceSlug.MatchString(in.Slug) {
		return nil, ErrWorkspaceInvalid
	}
	account := &ServiceAccount{WorkspaceID: w, ProjectID: p, Name: in.Name, Slug: in.Slug, Description: in.Description, Status: StatusActive, CreatedByUserID: &a}
	limit := s.limit(ctx, SettingKeyServiceAccountsPerProject, 100)
	err := s.repo.WithMutation(ctx, a, w, p, 0, "service_account.create", false, func(_ *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		n, e := tx.CountAccounts(ctx)
		if e != nil {
			return e
		}
		if n >= limit {
			return ErrServiceAccountLimit
		}
		if e = tx.CreateAccount(ctx, account); e != nil {
			return e
		}
		return tx.Event(ctx, EventServiceAccountCreated, account, nil, 0)
	})
	return account, err
}
func (s *ServiceAccountService) Update(ctx context.Context, a, w, p, id int64, in UpdateServiceAccountInput) (*ServiceAccount, error) {
	var out *ServiceAccount
	err := s.repo.WithMutation(ctx, a, w, p, id, "service_account.update", false, func(account *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		if in.Name != nil {
			account.Name = strings.TrimSpace(*in.Name)
		}
		if in.Description != nil {
			account.Description = *in.Description
		}
		if !validServiceAccountDetails(account.Name, account.Description) {
			return ErrWorkspaceInvalid
		}
		if e := tx.SaveAccount(ctx, account); e != nil {
			return e
		}
		out = account
		return tx.Event(ctx, EventServiceAccountUpdated, account, nil, 0)
	})
	return out, err
}
func (s *ServiceAccountService) SetStatus(ctx context.Context, a, w, p, id int64, enabled, admin bool) (*ServiceAccount, error) {
	var out *ServiceAccount
	var keys []APIKey
	err := s.repo.WithMutation(ctx, a, w, p, id, "service_account.disable", admin, func(account *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		status, event := "disabled", EventServiceAccountDisabled
		if enabled {
			status, event = StatusActive, EventServiceAccountEnabled
		}
		out = account
		if account.Status == status {
			return nil
		}
		account.Status = status
		account.DisabledAt = nil
		if !enabled {
			now := time.Now().UTC()
			account.DisabledAt = &now
		}
		if e := tx.SaveAccount(ctx, account); e != nil {
			return e
		}
		var e error
		keys, e = tx.Credentials(ctx)
		if e != nil {
			return e
		}
		return tx.Event(ctx, event, account, nil, 0)
	})
	if err == nil {
		s.evict(ctx, keys)
	}
	return out, err
}
func (s *ServiceAccountService) ListCredentials(ctx context.Context, a, w, p, id int64) ([]ServiceAccountCredential, error) {
	if _, e := s.access.RequireProject(ctx, a, w, p, "service_account.credential.read"); e != nil {
		return nil, e
	}
	keys, e := s.repo.Credentials(ctx, a, w, p, id)
	if e != nil {
		return nil, e
	}
	return safeServiceAccountCredentials(keys), nil
}
func safeServiceAccountCredentials(keys []APIKey) []ServiceAccountCredential {
	out := []ServiceAccountCredential{}
	for i := range keys {
		out = append(out, *ServiceAccountCredentialFromKey(&keys[i]))
	}
	return out
}
func (s *ServiceAccountService) evict(ctx context.Context, keys []APIKey) {
	for i := range keys {
		s.keys.InvalidateAuthCacheByKey(ctx, keys[i].Key)
	}
}

func validateServiceAccountCredential(key *APIKey) error {
	if key.Name == "" || utf8.RuneCountInString(key.Name) > 100 {
		return ErrWorkspaceInvalid
	}
	for _, patterns := range [][]string{key.IPWhitelist, key.IPBlacklist} {
		if len(ip.ValidateIPPatterns(patterns)) > 0 {
			return ErrInvalidIPPattern
		}
	}
	return nil
}
func (s *ServiceAccountService) CreateCredential(ctx context.Context, a, w, p, id int64, in CreateAPIKeyRequest) (*ServiceAccountSecret, error) {
	if e := validateCreateAPIKeyRequest(in); e != nil {
		return nil, e
	}
	if in.CustomKey != nil {
		return nil, ErrWorkspaceInvalid
	}
	key := &APIKey{Name: html.EscapeString(strings.TrimSpace(in.Name)), GroupID: in.GroupID, Status: StatusActive, IPWhitelist: in.IPWhitelist, IPBlacklist: in.IPBlacklist, Quota: in.Quota, RateLimit5h: in.RateLimit5h, RateLimit1d: in.RateLimit1d, RateLimit7d: in.RateLimit7d, ServiceAccountID: &id, ProjectID: &p}
	if in.ExpiresInDays != nil {
		expiry := time.Now().UTC().AddDate(0, 0, *in.ExpiresInDays)
		key.ExpiresAt = &expiry
	}
	var secret string
	limit := s.limit(ctx, SettingKeyServiceAccountCredentials, 10)
	err := s.repo.WithMutation(ctx, a, w, p, id, "service_account.credential.create", false, func(account *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		if account.Status != StatusActive {
			return ErrWorkspaceConflict
		}
		if e := validateServiceAccountCredential(key); e != nil {
			return e
		}
		if e := tx.CheckCredentialBinding(ctx, key); e != nil {
			return e
		}
		n, e := tx.CountActiveCredentials(ctx)
		if e != nil {
			return e
		}
		if n >= limit {
			return ErrServiceAccountLimit
		}
		secret, e = s.keys.GenerateKey()
		if e != nil {
			return e
		}
		suffix := secret[len(secret)-4:]
		key.Key = HashServiceAccountCredential(secret)
		key.KeySuffix = &suffix
		if e = tx.CreateCredential(ctx, key); e != nil {
			return e
		}
		return tx.Event(ctx, EventServiceAccountCredentialCreated, account, key, 0)
	})
	if err != nil {
		return nil, err
	}
	s.evict(ctx, []APIKey{*key})
	return &ServiceAccountSecret{Credential: ServiceAccountCredentialFromKey(key), Secret: secret}, nil
}
func (s *ServiceAccountService) RotateCredential(ctx context.Context, a, w, p, id, credentialID int64) (*ServiceAccountSecret, error) {
	var secret string
	var key *APIKey
	var oldKey *APIKey
	limit := s.limit(ctx, SettingKeyServiceAccountCredentials, 10)
	err := s.repo.WithMutation(ctx, a, w, p, id, "service_account.credential.rotate", false, func(account *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		if account.Status != StatusActive {
			return ErrWorkspaceConflict
		}
		old, e := tx.Credential(ctx, credentialID)
		if e != nil {
			return e
		}
		if old.Status == StatusAPIKeyRevoked {
			return ErrWorkspaceConflict
		}
		oldKey = old
		clone := *old
		key = &clone
		key.ID = 0
		key.Status = StatusActive
		key.QuotaUsed = 0
		key.Usage5h = 0
		key.Usage1d = 0
		key.Usage7d = 0
		key.Window5hStart = nil
		key.Window1dStart = nil
		key.Window7dStart = nil
		key.LastUsedAt = nil
		if key.ExpiresAt != nil && !key.ExpiresAt.After(time.Now()) {
			return ErrWorkspaceConflict
		}
		if e = validateServiceAccountCredential(key); e != nil {
			return e
		}
		if e = tx.CheckCredentialBinding(ctx, key); e != nil {
			return e
		}
		n, e := tx.CountActiveCredentials(ctx)
		if e != nil {
			return e
		}
		if n >= limit {
			return ErrServiceAccountLimit
		}
		secret, e = s.keys.GenerateKey()
		if e != nil {
			return e
		}
		suffix := secret[len(secret)-4:]
		key.Key = HashServiceAccountCredential(secret)
		key.KeySuffix = &suffix
		if e = tx.CreateCredential(ctx, key); e != nil {
			return e
		}
		return tx.Event(ctx, EventServiceAccountCredentialRotated, account, key, old.ID)
	})
	if err != nil {
		return nil, err
	}
	s.evict(ctx, []APIKey{*key, *oldKey})
	return &ServiceAccountSecret{Credential: ServiceAccountCredentialFromKey(key), Secret: secret, PreviousCredentialID: &credentialID}, nil
}
func (s *ServiceAccountService) UpdateCredential(ctx context.Context, a, w, p, id, credentialID int64, in UpdateAPIKeyRequest) (*ServiceAccountCredential, error) {
	if e := validateUpdateAPIKeyRequest(in); e != nil {
		return nil, e
	}
	if in.Status != nil && *in.Status != StatusActive && *in.Status != StatusDisabled {
		return nil, ErrWorkspaceInvalid
	}
	var out *APIKey
	limit := s.limit(ctx, SettingKeyServiceAccountCredentials, 10)
	err := s.repo.WithMutation(ctx, a, w, p, id, "service_account.credential.update", false, func(account *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		key, e := tx.Credential(ctx, credentialID)
		if e != nil {
			return e
		}
		if key.Status == StatusAPIKeyRevoked {
			return ErrWorkspaceConflict
		}
		if in.Name != nil {
			key.Name = html.EscapeString(strings.TrimSpace(*in.Name))
		}
		if in.GroupID != nil {
			key.GroupID = in.GroupID
		}
		if in.Status != nil {
			key.Status = *in.Status
		}
		if in.IPWhitelist != nil {
			key.IPWhitelist = *in.IPWhitelist
		}
		if in.IPBlacklist != nil {
			key.IPBlacklist = *in.IPBlacklist
		}
		if in.Quota != nil {
			key.Quota = *in.Quota
			if key.Status == StatusAPIKeyQuotaExhausted && (key.Quota <= 0 || key.Quota > key.QuotaUsed) {
				key.Status = StatusActive
			}
		}
		if in.ClearExpiration {
			key.ExpiresAt = nil
			if key.Status == StatusAPIKeyExpired {
				key.Status = StatusActive
			}
		} else if in.ExpiresAt != nil {
			key.ExpiresAt = in.ExpiresAt
			if key.Status == StatusAPIKeyExpired && key.ExpiresAt.After(time.Now()) {
				key.Status = StatusActive
			}
		}
		if in.RateLimit5h != nil {
			key.RateLimit5h = *in.RateLimit5h
		}
		if in.RateLimit1d != nil {
			key.RateLimit1d = *in.RateLimit1d
		}
		if in.RateLimit7d != nil {
			key.RateLimit7d = *in.RateLimit7d
		}
		if in.ResetQuota != nil && *in.ResetQuota {
			key.QuotaUsed = 0
			if key.Status == StatusAPIKeyQuotaExhausted {
				key.Status = StatusActive
			}
		}
		if in.ResetRateLimitUsage != nil && *in.ResetRateLimitUsage {
			key.Usage5h = 0
			key.Usage1d = 0
			key.Usage7d = 0
			key.Window5hStart = nil
			key.Window1dStart = nil
			key.Window7dStart = nil
		}
		if e = validateServiceAccountCredential(key); e != nil {
			return e
		}
		if e = tx.CheckCredentialBinding(ctx, key); e != nil {
			return e
		}
		if serviceAccountCredentialCountsAsActive(key, time.Now()) {
			n, e := tx.CountActiveCredentials(ctx)
			if e != nil {
				return e
			}
			if n >= limit {
				current, e := tx.Credential(ctx, credentialID)
				if e != nil {
					return e
				}
				if !serviceAccountCredentialCountsAsActive(current, time.Now()) {
					return ErrServiceAccountLimit
				}
			}
		}
		if e = tx.SaveCredential(ctx, key); e != nil {
			return e
		}
		out = key
		return tx.Event(ctx, EventServiceAccountCredentialUpdated, account, key, 0)
	})
	if err != nil {
		return nil, err
	}
	s.evict(ctx, []APIKey{*out})
	if in.ResetRateLimitUsage != nil && *in.ResetRateLimitUsage && s.keys.rateLimitCacheInvalid != nil {
		_ = s.keys.rateLimitCacheInvalid.InvalidateAPIKeyRateLimit(ctx, out.ID)
	}
	return ServiceAccountCredentialFromKey(out), nil
}
func (s *ServiceAccountService) RevokeCredential(ctx context.Context, a, w, p, id, credentialID int64, admin bool) (*ServiceAccountCredential, error) {
	var out *APIKey
	err := s.repo.WithMutation(ctx, a, w, p, id, "service_account.credential.revoke", admin, func(account *ServiceAccount, _ *WorkspaceAccess, tx ServiceAccountTransaction) error {
		key, e := tx.Credential(ctx, credentialID)
		if e != nil {
			return e
		}
		out = key
		if key.Status == StatusAPIKeyRevoked {
			return nil
		}
		key.Status = StatusAPIKeyRevoked
		if e = tx.SaveCredential(ctx, key); e != nil {
			return e
		}
		return tx.Event(ctx, EventServiceAccountCredentialRevoked, account, key, 0)
	})
	if err != nil {
		return nil, err
	}
	s.evict(ctx, []APIKey{*out})
	return ServiceAccountCredentialFromKey(out), nil
}
func (s *ServiceAccountService) AdminList(ctx context.Context, a int64, search string, params pagination.PaginationParams) ([]ServiceAccount, int64, error) {
	return s.repo.AdminList(ctx, a, strings.TrimSpace(search), params)
}
func (s *ServiceAccountService) AdminGet(ctx context.Context, a, id int64) (*ServiceAccount, []ServiceAccountCredential, error) {
	account, keys, e := s.repo.AdminGet(ctx, a, id)
	return account, safeServiceAccountCredentials(keys), e
}
