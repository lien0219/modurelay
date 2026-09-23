package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	// These values live in accounts.extra so PR2 does not require a schema migration.
	UpstreamBillingProbeExtraKey           = "upstream_billing_probe"
	UpstreamBillingProbeEnabledExtraKey    = "upstream_billing_probe_enabled"
	UpstreamBillingRateSyncEnabledExtraKey = "upstream_billing_rate_sync_enabled"
	// UpstreamBillingAutoUnschedulableExtraKey marks a persistent scheduling
	// pause caused by an observed exhausted upstream balance. It is cleared by
	// an explicit admin scheduling change.
	UpstreamBillingAutoUnschedulableExtraKey = "upstream_billing_auto_unschedulable"

	upstreamBillingProbeDefaultIntervalMinutes = 30
	upstreamBillingProbeMinIntervalMinutes     = 5
	upstreamBillingProbeMaxIntervalMinutes     = 24 * 60
	upstreamBillingProbeCycleInterval          = time.Minute
	upstreamBillingProbeRequestTimeout         = 10 * time.Second
	upstreamBillingProbeObservationMaxAge      = time.Hour
	upstreamBillingProbeObservationFutureSkew  = 5 * time.Minute
	upstreamBillingProbeMaxBodyBytes           = 64 * 1024
	upstreamBalanceProbeMaxBodyBytes           = 256 * 1024
	upstreamBillingProbeScheduledMaxPerCycle   = 40
	upstreamBillingProbeManualMaxBatchSize     = 20
	upstreamBillingProbeConcurrency            = 8
	upstreamBillingProbeMaxDelay               = 24 * time.Hour
	// unsupported 账号的重探间隔倍数：上游不是 sub2api 中转就不会突然长出
	// /v1/sub2api/billing，按常规 interval 重排只会持续占满每周期
	// upstreamBillingProbeScheduledMaxPerCycle 个名额。
	upstreamBillingProbeUnsupportedDelayFactor = 8
	upstreamBillingProbeAccountRateScale       = 10000.0
	upstreamBillingProbeLeaderLockKey          = "upstream:billing:probe:leader"
	upstreamBillingProbeLeaderLockTTL          = 2 * time.Minute
)

// UpstreamBillingProbeMaxBatchSize is the public limit for one manual batch.
const UpstreamBillingProbeMaxBatchSize = upstreamBillingProbeManualMaxBatchSize

// upstreamBillingRateSyncMaxMultiplier bounds the value the automatic
// write-back may push into accounts.rate_multiplier.
//
// No other code path bounds that column from above — admins may type any
// non-negative number and the only ceiling is the DECIMAL(10,4) column itself
// (999999.9999). That ceiling is meaningless as a guard: rate_multiplier
// scales the per-request account cost that feeds quota_used, so a single
// declared 999999 would exhaust any account quota on the first request and
// poison cost reporting. 100 is picked as a deliberately generous bound: it is
// two orders of magnitude above the 1.0 default and far above any plausible
// upstream resale markup, so no legitimate declaration is rejected while an
// absurd or hostile one cannot reach the quota control plane unattended.
// It only constrains the automatic path; manual edits keep their old range.
const upstreamBillingRateSyncMaxMultiplier = 100.0

var (
	errUpstreamBalanceUnsupported = errors.New("upstream balance response is unsupported")

	ErrUpstreamBillingProbeUnavailable = infraerrors.ServiceUnavailable(
		"UPSTREAM_BILLING_PROBE_UNAVAILABLE", "upstream billing probe is unavailable",
	)
	ErrUpstreamBillingProbeAccountInvalid = infraerrors.BadRequest(
		"UPSTREAM_BILLING_PROBE_ACCOUNT_INVALID", "account is not an API key account",
	)
	ErrUpstreamBillingProbeIdentityChanged = infraerrors.Conflict(
		"UPSTREAM_BILLING_PROBE_IDENTITY_CHANGED", "account identity changed during upstream billing probe; retry the probe",
	)
	ErrUpstreamBillingRateSyncBulkConflict = infraerrors.Conflict(
		"UPSTREAM_BILLING_RATE_SYNC_BULK_CONFLICT",
		"account rate multiplier cannot be changed in bulk while upstream billing rate sync is enabled",
	)
	ErrUpstreamBillingRateSyncConflict = infraerrors.Conflict(
		"UPSTREAM_BILLING_RATE_SYNC_CONFLICT",
		"account rate multiplier cannot be changed while upstream billing rate sync is enabled",
	)
)

const (
	UpstreamBillingProbeStatusOK          = "ok"
	UpstreamBillingProbeStatusUnsupported = "unsupported"
	UpstreamBillingProbeStatusFailed      = "failed"
)

// UpstreamBillingProbeSettings controls the periodic probe runner.
type UpstreamBillingProbeSettings struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"interval_minutes"`
}

// UpstreamBillingProbeSnapshot is persisted in accounts.extra. Data is kept as
// a sanitized map so future response fields do not require a database change.
type UpstreamBillingProbeSnapshot struct {
	Status        string                        `json:"status"`
	Data          map[string]any                `json:"data,omitempty"`
	Balance       *UpstreamBalanceProbeSnapshot `json:"balance,omitempty"`
	ReceivedAt    *time.Time                    `json:"received_at,omitempty"`
	FreshUntil    *time.Time                    `json:"fresh_until,omitempty"`
	LastAttemptAt time.Time                     `json:"last_attempt_at"`
	NextProbeAt   time.Time                     `json:"next_probe_at"`
	FailureCount  int                           `json:"failure_count,omitempty"`
	HTTPStatus    int                           `json:"http_status,omitempty"`
	LastError     string                        `json:"last_error,omitempty"`
	// AutoUnschedulable tells the admin UI that this result exhausted the
	// observed balance and caused scheduling to be disabled.
	AutoUnschedulable bool `json:"auto_unschedulable,omitempty"`
	// SyncedRateMultiplier records the value this probe wrote into
	// accounts.rate_multiplier. It is only set when the account opted into rate
	// sync and the declared value passed the write-back range check, so the
	// stored snapshot always answers "did this probe move the account rate, and
	// to what" without a separate history table.
	SyncedRateMultiplier *float64 `json:"synced_rate_multiplier,omitempty"`
}

// NewAPIUpstreamGroup is a sanitized New API group option discovered from
// /api/pricing. The group controls billing rate only; it never represents a
// separate balance.
type NewAPIUpstreamGroup struct {
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// UpstreamBalanceProbeSnapshot is independent from the declared-rate status:
// an older ModuRelay upstream may expose /v1/usage without exposing the
// billing declaration endpoint, and a rate probe may succeed while balance
// lookup is unsupported.
type UpstreamBalanceProbeSnapshot struct {
	Status        string         `json:"status"`
	Data          map[string]any `json:"data,omitempty"`
	Source        string         `json:"source,omitempty"`
	ReceivedAt    *time.Time     `json:"received_at,omitempty"`
	FreshUntil    *time.Time     `json:"fresh_until,omitempty"`
	LastAttemptAt time.Time      `json:"last_attempt_at"`
	NextProbeAt   time.Time      `json:"next_probe_at"`
	FailureCount  int            `json:"failure_count,omitempty"`
	HTTPStatus    int            `json:"http_status,omitempty"`
	LastError     string         `json:"last_error,omitempty"`
}

// UpstreamBillingBalanceExhausted reports whether a successful, validated
// balance observation is at or below zero. Failed, unsupported, stale, invalid,
// and unlimited observations never change scheduling.
func UpstreamBillingBalanceExhausted(snapshot *UpstreamBillingProbeSnapshot) bool {
	if snapshot == nil || snapshot.Balance == nil || snapshot.Balance.Status != UpstreamBillingProbeStatusOK {
		return false
	}
	// A failed top-level probe may retain the last known balance for display.
	// Only a balance observed during this probe may change scheduling.
	if snapshot.LastAttemptAt.IsZero() || snapshot.Balance.LastAttemptAt.IsZero() ||
		!snapshot.Balance.LastAttemptAt.Equal(snapshot.LastAttemptAt) {
		return false
	}
	data := snapshot.Balance.Data
	if data == nil {
		return false
	}
	if unlimited, ok := data["unlimited"].(bool); ok && unlimited {
		return false
	}
	if valid, ok := data["is_valid"].(bool); ok && !valid {
		return false
	}
	if valid, ok := data["isValid"].(bool); ok && !valid {
		return false
	}

	mode, _ := data["mode"].(string)
	keys := []string{"remaining"}
	if mode == "wallet" {
		keys = []string{"balance", "remaining"}
	}
	for _, key := range keys {
		value, ok := upstreamBillingNumber(data[key])
		if ok {
			return value <= 0
		}
	}
	return false
}

func upstreamBillingNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, !math.IsNaN(number) && !math.IsInf(number, 0)
	case float32:
		converted := float64(number)
		return converted, !math.IsNaN(converted) && !math.IsInf(converted, 0)
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		converted, err := number.Float64()
		return converted, err == nil && !math.IsNaN(converted) && !math.IsInf(converted, 0)
	default:
		return 0, false
	}
}

// UpstreamBillingProbeResult is returned by manual probe endpoints.
type UpstreamBillingProbeResult struct {
	AccountID int64                         `json:"account_id"`
	Snapshot  *UpstreamBillingProbeSnapshot `json:"snapshot,omitempty"`
	Error     string                        `json:"error,omitempty"`
}

// UpstreamBillingRateSnapshotItem is the compact representation used by the
// account table's background refresh. It intentionally excludes credentials,
// runtime counters, and usage data from the response.
type UpstreamBillingRateSnapshotItem struct {
	AccountID         int64                         `json:"account_id"`
	Schedulable       bool                          `json:"schedulable"`
	RateMultiplier    float64                       `json:"rate_multiplier"`
	AutoUnschedulable bool                          `json:"auto_unschedulable"`
	Snapshot          *UpstreamBillingProbeSnapshot `json:"snapshot"`
}

// BuildUpstreamBillingRateSnapshotItems projects account rows into the
// read-only payload used by the rate refresh endpoint. Decode snapshots here
// so malformed or legacy extra data is handled consistently with probe logic.
func BuildUpstreamBillingRateSnapshotItems(accounts []Account) []UpstreamBillingRateSnapshotItem {
	items := make([]UpstreamBillingRateSnapshotItem, 0, len(accounts))
	for _, account := range accounts {
		var snapshot *UpstreamBillingProbeSnapshot
		// The billing endpoint is supported by every eligible API-key identity;
		// include legacy Antigravity upstream rows so refresh never erases their
		// persisted snapshots from the table.
		if IsUpstreamBillingProbeIdentity(account.Platform, account.Type) {
			snapshot = decodeUpstreamBillingProbeSnapshot(account.Extra)
			if snapshot != nil {
				// The top-level marker is the durable scheduling fact. Normalize
				// legacy snapshots so a manual scheduling change cannot be undone
				// by the table's compact background refresh.
				snapshot.AutoUnschedulable = upstreamBillingAutoUnschedulable(account.Extra)
			}
		}
		items = append(items, UpstreamBillingRateSnapshotItem{
			AccountID:         account.ID,
			Schedulable:       account.Schedulable,
			RateMultiplier:    account.BillingRateMultiplier(),
			AutoUnschedulable: upstreamBillingAutoUnschedulable(account.Extra),
			Snapshot:          snapshot,
		})
	}
	return items
}

type upstreamBillingProbeResponse struct {
	Object                  string          `json:"object"`
	SchemaVersion           int             `json:"schema_version"`
	BillingScope            string          `json:"billing_scope"`
	GroupRateMultiplier     *float64        `json:"group_rate_multiplier"`
	UserRateMultiplier      *float64        `json:"user_rate_multiplier"`
	ResolvedRateMultiplier  *float64        `json:"resolved_rate_multiplier"`
	PeakRateEnabled         *bool           `json:"peak_rate_enabled"`
	PeakStart               *string         `json:"peak_start"`
	PeakEnd                 *string         `json:"peak_end"`
	PeakRateMultiplier      *float64        `json:"peak_rate_multiplier"`
	AppliedPeakMultiplier   *float64        `json:"applied_peak_multiplier"`
	EffectiveRateMultiplier *float64        `json:"effective_rate_multiplier"`
	Timezone                *string         `json:"timezone"`
	ObservedAt              string          `json:"observed_at"`
	Funding                 json.RawMessage `json:"funding"`
}

type upstreamBillingFundingResponse struct {
	Mode      string   `json:"mode"`
	Unit      string   `json:"unit"`
	Balance   *float64 `json:"balance"`
	Remaining *float64 `json:"remaining"`
	Limit     *float64 `json:"limit"`
	Used      *float64 `json:"used"`
	Unlimited bool     `json:"unlimited"`
}

type upstreamUsageBalanceResponse struct {
	Mode      string   `json:"mode"`
	IsValid   *bool    `json:"isValid"`
	Unit      string   `json:"unit"`
	Balance   *float64 `json:"balance"`
	Remaining *float64 `json:"remaining"`
	Quota     *struct {
		Limit     *float64 `json:"limit"`
		Used      *float64 `json:"used"`
		Remaining *float64 `json:"remaining"`
		Unit      string   `json:"unit"`
	} `json:"quota"`
	Subscription json.RawMessage `json:"subscription"`
}

// GetUpstreamBillingProbeSettings returns defaults when the setting is absent.
func (s *SettingService) GetUpstreamBillingProbeSettings(ctx context.Context) (*UpstreamBillingProbeSettings, error) {
	defaults := defaultUpstreamBillingProbeSettings()
	if s == nil || s.settingRepo == nil {
		return defaults, nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyUpstreamBillingProbeSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return defaults, nil
		}
		return nil, fmt.Errorf("get upstream billing probe settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return defaults, nil
	}
	settings := *defaults
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("parse upstream billing probe settings: %w", err)
	}
	if settings.IntervalMinutes == 0 {
		settings.IntervalMinutes = defaults.IntervalMinutes
	}
	normalizeUpstreamBillingProbeSettings(&settings)
	return &settings, nil
}

// SetUpstreamBillingProbeSettings validates and persists the runner settings.
func (s *SettingService) SetUpstreamBillingProbeSettings(ctx context.Context, settings *UpstreamBillingProbeSettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("setting repository is unavailable")
	}
	if settings == nil {
		return infraerrors.BadRequest("INVALID_UPSTREAM_BILLING_PROBE_SETTINGS", "settings cannot be nil")
	}
	if settings.IntervalMinutes < upstreamBillingProbeMinIntervalMinutes || settings.IntervalMinutes > upstreamBillingProbeMaxIntervalMinutes {
		return infraerrors.BadRequest(
			"INVALID_UPSTREAM_BILLING_PROBE_INTERVAL",
			fmt.Sprintf("interval_minutes must be between %d and %d", upstreamBillingProbeMinIntervalMinutes, upstreamBillingProbeMaxIntervalMinutes),
		)
	}
	normalizeUpstreamBillingProbeSettings(settings)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal upstream billing probe settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyUpstreamBillingProbeSettings, string(data))
}

func defaultUpstreamBillingProbeSettings() *UpstreamBillingProbeSettings {
	return &UpstreamBillingProbeSettings{Enabled: true, IntervalMinutes: upstreamBillingProbeDefaultIntervalMinutes}
}

func normalizeUpstreamBillingProbeSettings(settings *UpstreamBillingProbeSettings) {
	if settings.IntervalMinutes < upstreamBillingProbeMinIntervalMinutes {
		settings.IntervalMinutes = upstreamBillingProbeMinIntervalMinutes
	}
	if settings.IntervalMinutes > upstreamBillingProbeMaxIntervalMinutes {
		settings.IntervalMinutes = upstreamBillingProbeMaxIntervalMinutes
	}
}

// UpstreamBillingProbeService discovers a remote Sub2API billing snapshot.
type UpstreamBillingProbeService struct {
	accountRepo         AccountRepository
	accountTestService  *AccountTestService
	settingService      *SettingService
	secretEncryptor     SecretEncryptor
	secretKeyConfigured bool

	parentCtx    context.Context
	parentCancel context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	started      bool
	stopped      bool
	cycleMu      sync.Mutex
	probeGroup   singleflight.Group
	probeSlots   chan struct{}
	now          func() time.Time
	lockCache    LeaderLockCache
	db           *sql.DB
	instanceID   string
}

type upstreamBillingProbeExecutionResult struct {
	snapshot      *UpstreamBillingProbeSnapshot
	balanceForced bool
}

type upstreamBillingProbeSnapshotWriter interface {
	UpdateUpstreamBillingProbeSnapshot(context.Context, *Account, *UpstreamBillingProbeSnapshot, *float64) error
}

type upstreamBillingProbeDueAccountLister interface {
	ListDueUpstreamBillingProbeAccounts(context.Context, time.Time, int) ([]Account, error)
}

func NewUpstreamBillingProbeService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	settingService *SettingService,
) *UpstreamBillingProbeService {
	ctx, cancel := context.WithCancel(context.Background())
	return &UpstreamBillingProbeService{
		accountRepo:        accountRepo,
		accountTestService: accountTestService,
		settingService:     settingService,
		parentCtx:          ctx,
		parentCancel:       cancel,
		probeSlots:         make(chan struct{}, upstreamBillingProbeConcurrency),
		now:                time.Now,
		instanceID:         uuid.NewString(),
	}
}

func (s *UpstreamBillingProbeService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// ProvideUpstreamBillingProbeService starts the process-wide periodic runner.
func ProvideUpstreamBillingProbeService(
	accountRepo AccountRepository,
	accountTestService *AccountTestService,
	settingService *SettingService,
	secretEncryptor SecretEncryptor,
	cfg *config.Config,
	lockCache LeaderLockCache,
	db *sql.DB,
) *UpstreamBillingProbeService {
	svc := NewUpstreamBillingProbeService(accountRepo, accountTestService, settingService)
	svc.secretEncryptor = secretEncryptor
	svc.secretKeyConfigured = cfg != nil && cfg.Totp.EncryptionKeyConfigured
	svc.SetLeaderLock(lockCache, db)
	svc.Start()
	return svc
}

func (s *UpstreamBillingProbeService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.wg.Add(1)
	s.mu.Unlock()
	go s.runLoop()
}

func (s *UpstreamBillingProbeService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.parentCancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *UpstreamBillingProbeService) runLoop() {
	defer s.wg.Done()
	if err := s.RunDue(s.parentCtx); err != nil {
		logger.LegacyPrintf("service.upstream_billing_probe", "initial_run_due_failed: err=%v", err)
	}
	ticker := time.NewTicker(upstreamBillingProbeCycleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.parentCtx.Done():
			return
		case <-ticker.C:
			if err := s.RunDue(s.parentCtx); err != nil {
				logger.LegacyPrintf("service.upstream_billing_probe", "run_due_failed: err=%v", err)
			}
		}
	}
}

// RunDue executes at most one bounded batch of due accounts.
func (s *UpstreamBillingProbeService) RunDue(ctx context.Context) error {
	if s == nil || s.accountRepo == nil {
		return nil
	}
	s.cycleMu.Lock()
	defer s.cycleMu.Unlock()

	settings, err := s.getSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	runRelease, acquired, lockErr := s.tryAcquireLeaderLock(ctx, upstreamBillingProbeLeaderLockKey)
	if lockErr != nil {
		return fmt.Errorf("acquire upstream billing probe leader lock: %w", lockErr)
	}
	if !acquired {
		return nil
	}
	defer runRelease()

	lockNow := time.Now()
	cadenceRelease, acquired, lockErr := s.tryAcquireLeaderLock(ctx, upstreamBillingProbeLeaderLockKeyAt(lockNow))
	if lockErr != nil {
		return fmt.Errorf("acquire upstream billing probe cadence lock: %w", lockErr)
	}
	if !acquired {
		return nil
	}
	defer releaseUpstreamBillingProbeLeaderLock(cadenceRelease, lockNow.Truncate(upstreamBillingProbeCycleInterval).Add(upstreamBillingProbeCycleInterval))

	now := s.currentTime()
	cycleStartedAt := time.Now()
	accounts, err := s.listDueAccounts(ctx, now)
	if err != nil {
		return fmt.Errorf("list enabled upstream billing probes: %w", err)
	}
	due := make([]Account, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		if !isUpstreamBillingProbeAccount(&account) || !account.IsActive() || !upstreamBillingProbeEnabled(&account) {
			continue
		}
		snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra)
		if snapshot != nil && !snapshot.NextProbeAt.IsZero() && now.Before(snapshot.NextProbeAt) {
			continue
		}
		due = append(due, account)
	}
	sort.SliceStable(due, func(i, j int) bool {
		left := decodeUpstreamBillingProbeSnapshot(due[i].Extra)
		right := decodeUpstreamBillingProbeSnapshot(due[j].Extra)
		leftUnset := left == nil || left.NextProbeAt.IsZero()
		rightUnset := right == nil || right.NextProbeAt.IsZero()
		if leftUnset && rightUnset {
			return due[i].ID < due[j].ID
		}
		if leftUnset {
			return true
		}
		if rightUnset {
			return false
		}
		return left.NextProbeAt.Before(right.NextProbeAt)
	})
	backlogDetected := len(accounts) > upstreamBillingProbeScheduledMaxPerCycle
	if len(due) > upstreamBillingProbeScheduledMaxPerCycle {
		due = due[:upstreamBillingProbeScheduledMaxPerCycle]
	}

	var group errgroup.Group
	for i := range due {
		accountID := due[i].ID
		group.Go(func() error {
			if _, probeErr := s.probeScheduledAccount(ctx, accountID, settings.IntervalMinutes); probeErr != nil {
				logger.LegacyPrintf("service.upstream_billing_probe", "probe_due_failed: account_id=%d err=%v", accountID, probeErr)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	if len(due) > 0 {
		slog.Info("upstream_billing_probe_cycle_completed",
			"selected_accounts", len(due),
			"candidate_accounts", len(accounts),
			"backlog_detected", backlogDetected,
			"duration_ms", time.Since(cycleStartedAt).Milliseconds(),
		)
	}
	if backlogDetected {
		slog.Warn("upstream_billing_probe_backlog_detected",
			"scheduled_limit", upstreamBillingProbeScheduledMaxPerCycle,
			"candidate_accounts_lower_bound", len(accounts),
		)
	}
	return nil
}

func (s *UpstreamBillingProbeService) listDueAccounts(ctx context.Context, now time.Time) ([]Account, error) {
	if lister, ok := s.accountRepo.(upstreamBillingProbeDueAccountLister); ok {
		return lister.ListDueUpstreamBillingProbeAccounts(ctx, now, upstreamBillingProbeScheduledMaxPerCycle+1)
	}
	// Non-production repositories and older adapters keep the generic path. The
	// runner still truncates before issuing network requests.
	return s.accountRepo.FindByExtraField(ctx, UpstreamBillingProbeEnabledExtraKey, true)
}

func (s *UpstreamBillingProbeService) getSettings(ctx context.Context) (*UpstreamBillingProbeSettings, error) {
	if s.settingService == nil {
		return defaultUpstreamBillingProbeSettings(), nil
	}
	return s.settingService.GetUpstreamBillingProbeSettings(ctx)
}

func (s *UpstreamBillingProbeService) GetSettings(ctx context.Context) (*UpstreamBillingProbeSettings, error) {
	return s.getSettings(ctx)
}

func (s *UpstreamBillingProbeService) UpdateSettings(ctx context.Context, settings *UpstreamBillingProbeSettings) error {
	if s == nil || s.settingService == nil {
		return ErrUpstreamBillingProbeUnavailable
	}
	return s.settingService.SetUpstreamBillingProbeSettings(ctx, settings)
}

// ProbeAccount performs one manual or scheduled probe. Manual calls ignore both switches.
func (s *UpstreamBillingProbeService) ProbeAccount(ctx context.Context, accountID int64) (*UpstreamBillingProbeSnapshot, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrUpstreamBillingProbeUnavailable
	}
	settings, err := s.getSettings(ctx)
	if err != nil {
		return nil, err
	}
	return s.probeAccount(ctx, accountID, settings.IntervalMinutes)
}

func (s *UpstreamBillingProbeService) probeAccount(ctx context.Context, accountID int64, intervalMinutes int) (*UpstreamBillingProbeSnapshot, error) {
	return s.probeAccountWithMode(ctx, accountID, intervalMinutes, false)
}

func (s *UpstreamBillingProbeService) probeScheduledAccount(ctx context.Context, accountID int64, intervalMinutes int) (*UpstreamBillingProbeSnapshot, error) {
	return s.probeAccountWithMode(ctx, accountID, intervalMinutes, true)
}

func (s *UpstreamBillingProbeService) probeAccountWithMode(ctx context.Context, accountID int64, intervalMinutes int, requireEnabled bool) (*UpstreamBillingProbeSnapshot, error) {
	key := strconv.FormatInt(accountID, 10)
	for {
		value, err, shared := s.probeGroup.Do(key, func() (any, error) {
			result := &upstreamBillingProbeExecutionResult{balanceForced: !requireEnabled}
			select {
			case s.probeSlots <- struct{}{}:
				defer func() { <-s.probeSlots }()
			case <-ctx.Done():
				return result, ctx.Err()
			}
			account, loadErr := s.accountRepo.GetByID(ctx, accountID)
			if loadErr != nil {
				return result, loadErr
			}
			if !isUpstreamBillingProbeAccount(account) {
				return result, ErrUpstreamBillingProbeAccountInvalid
			}
			if requireEnabled {
				if !account.IsActive() || !upstreamBillingProbeEnabled(account) {
					return result, nil
				}
				if snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra); snapshot != nil &&
					!snapshot.NextProbeAt.IsZero() && s.currentTime().Before(snapshot.NextProbeAt) {
					return result, nil
				}
			}
			snapshot, probeErr := s.probeLoadedAccount(ctx, account, intervalMinutes, !requireEnabled)
			result.snapshot = snapshot
			return result, probeErr
		})
		result, ok := value.(*upstreamBillingProbeExecutionResult)
		if !ok || result == nil {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("invalid upstream billing probe result")
		}
		// A manual refresh must not inherit a scheduled run that was allowed to
		// reuse a cached balance. The shared run has completed, so retrying here
		// stays serialized behind the same per-account singleflight key.
		if !requireEnabled && shared && !result.balanceForced {
			continue
		}
		if err != nil {
			return nil, err
		}
		return result.snapshot, nil
	}
}

// ProbeAccounts performs a bounded manual batch with the same concurrency limit as the runner.
func (s *UpstreamBillingProbeService) ProbeAccounts(ctx context.Context, accountIDs []int64) []UpstreamBillingProbeResult {
	if len(accountIDs) > upstreamBillingProbeManualMaxBatchSize {
		accountIDs = accountIDs[:upstreamBillingProbeManualMaxBatchSize]
	}
	results := make([]UpstreamBillingProbeResult, len(accountIDs))
	if s == nil || s.accountRepo == nil {
		for i, accountID := range accountIDs {
			results[i] = UpstreamBillingProbeResult{AccountID: accountID, Error: ErrUpstreamBillingProbeUnavailable.Error()}
		}
		return results
	}
	settings, settingsErr := s.getSettings(ctx)
	if settingsErr != nil {
		for i, accountID := range accountIDs {
			results[i] = UpstreamBillingProbeResult{AccountID: accountID, Error: safeProbeError(settingsErr)}
		}
		return results
	}
	var group errgroup.Group
	for i, accountID := range accountIDs {
		i, accountID := i, accountID
		results[i].AccountID = accountID
		group.Go(func() error {
			snapshot, err := s.probeAccount(ctx, accountID, settings.IntervalMinutes)
			if err != nil {
				results[i].Error = safeProbeError(err)
				return nil
			}
			results[i].Snapshot = snapshot
			return nil
		})
	}
	_ = group.Wait()
	return results
}

func upstreamBillingProbeLeaderLockKeyAt(now time.Time) string {
	return fmt.Sprintf("%s:%d", upstreamBillingProbeLeaderLockKey, now.Unix()/int64(upstreamBillingProbeCycleInterval/time.Second))
}

func (s *UpstreamBillingProbeService) tryAcquireLeaderLock(ctx context.Context, key string) (func(), bool, error) {
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if s.lockCache != nil {
		acquired, err := s.lockCache.TryAcquireLeaderLock(lockCtx, key, s.instanceID, upstreamBillingProbeLeaderLockTTL)
		if err != nil {
			return nil, false, err
		}
		if !acquired {
			return nil, false, nil
		}
		return func() {
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer releaseCancel()
			_ = s.lockCache.ReleaseLeaderLock(releaseCtx, key, s.instanceID)
		}, true, nil
	}
	if s.db != nil {
		return tryAcquireDBAdvisoryLockWithError(lockCtx, s.db, hashAdvisoryLockID(key))
	}
	return func() {}, true, nil
}

func releaseUpstreamBillingProbeLeaderLock(release func(), releaseAt time.Time) {
	delay := time.Until(releaseAt)
	if delay <= 0 {
		release()
		return
	}
	time.AfterFunc(delay, release)
}

func (s *UpstreamBillingProbeService) SetAccountEnabled(ctx context.Context, accountID int64, enabled bool) error {
	if s == nil || s.accountRepo == nil {
		return ErrUpstreamBillingProbeUnavailable
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if !isUpstreamBillingProbeAccount(account) {
		return ErrUpstreamBillingProbeAccountInvalid
	}
	updates := map[string]any{UpstreamBillingProbeEnabledExtraKey: enabled}
	if !enabled {
		updates[UpstreamBillingRateSyncEnabledExtraKey] = false
	}
	return s.accountRepo.UpdateExtra(ctx, accountID, updates)
}

func (s *UpstreamBillingProbeService) probeLoadedAccount(ctx context.Context, account *Account, intervalMinutes int, forceBalance bool) (*UpstreamBillingProbeSnapshot, error) {
	now := s.currentTime().UTC()
	if s.accountTestService == nil || s.accountTestService.httpUpstream == nil {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "transport_unavailable", 0)
	}
	// 平台放宽后取数直读 credentials：所有 API-key 平台的密钥与自定义上游
	// 统一存放在 credentials.api_key / credentials.base_url。
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "missing_api_key", 0)
	}
	baseURL := account.GetCredential("base_url")
	if account.IsCNProvider() && account.IsAdaptiveAPIProtocol() {
		baseURL = account.GetCNProtocolBaseURL(APIProtocolChatCompletions)
	}
	if upstreamBillingProbeTargetIsOfficialAPI(baseURL) {
		// base_url 为空或指向官方 API 根域（前端创建时会把空值填成
		// 官方默认域）时必无 /v1/sub2api/billing；不发请求，直接记
		// unsupported，避免拿账号 Key 周期性请求官方域的不存在路径。
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "unsupported", 0)
	}
	normalizedBaseURL, err := s.accountTestService.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "invalid_base_url", 0)
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil {
			return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "proxy_unavailable", 0)
		}
		if account.Proxy.ID != *account.ProxyID {
			return nil, ErrUpstreamBillingProbeIdentityChanged
		}
		proxyURL = account.Proxy.URL()
	}
	probeURL := buildOpenAIEndpointURL(normalizedBaseURL, "/v1/sub2api/billing")
	probeCtx, cancel := context.WithTimeout(ctx, upstreamBillingProbeRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, probeURL, bytes.NewReader(nil))
	if err != nil {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "request_build_failed", 0)
	}
	// OpenAI 账号保持官方 openai 传输画像；其他平台探测走默认画像。
	profile := HTTPUpstreamProfileDefault
	if account.Platform == PlatformOpenAI {
		profile = HTTPUpstreamProfileOpenAI
	}
	reqCtx := WithHTTPUpstreamProfile(req.Context(), profile)
	reqCtx = WithHTTPUpstreamResolvedIPPinning(reqCtx)
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(reqCtx))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	account.ApplyHeaderOverrides(req.Header)
	var tlsProfile *tlsfingerprint.Profile
	if s.accountTestService.tlsFPProfileService != nil {
		tlsProfile = s.accountTestService.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := s.accountTestService.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		reason := upstreamBillingProbeRequestFailureReason(err)
		logUpstreamBillingProbeRequestFailure(account.ID, req, proxyURL != "", reason, err)
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, reason, 0)
	}
	if resp == nil || resp.Body == nil {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, 0, "empty_response", 0)
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamBillingProbeMaxBodyBytes+1))
	if readErr != nil {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, resp.StatusCode, "response_read_failed", retryAfter(resp.Header, now))
	}
	if len(body) > upstreamBillingProbeMaxBodyBytes {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, resp.StatusCode, "response_too_large", retryAfter(resp.Header, now))
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		_ = resp.Body.Close()
		if newAPISnapshot, identified := s.probeNewAPIUpstream(
			ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile,
			intervalMinutes, now, forceBalance,
		); identified {
			return s.persistSuccessfulUpstreamBillingProbe(ctx, account, newAPISnapshot)
		}
		balanceSnapshot := s.probeUpstreamUsageBalance(
			ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile,
			intervalMinutes, now, forceBalance,
		)
		return s.persistProbeFailureWithBalance(
			ctx, account, intervalMinutes, now, resp.StatusCode, "unsupported",
			retryAfter(resp.Header, now), balanceSnapshot,
		)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, resp.StatusCode, "http_error", retryAfter(resp.Header, now))
	}
	data, err := parseUpstreamBillingProbeResponse(body)
	if err != nil {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, resp.StatusCode, "invalid_response", retryAfter(resp.Header, now))
	}
	if !upstreamBillingObservationIsFresh(data, now) {
		return s.persistProbeFailure(ctx, account, intervalMinutes, now, resp.StatusCode, "stale_response", retryAfter(resp.Header, now))
	}
	balanceSnapshot := upstreamBalanceSnapshotFromBillingData(data, intervalMinutes, now, resp.StatusCode)
	delete(data, "funding")
	if balanceSnapshot == nil {
		_ = resp.Body.Close()
		balanceSnapshot = s.probeUpstreamUsageBalance(
			ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile,
			intervalMinutes, now, forceBalance,
		)
	}
	snapshot := &UpstreamBillingProbeSnapshot{
		Status:        UpstreamBillingProbeStatusOK,
		Data:          data,
		Balance:       balanceSnapshot,
		ReceivedAt:    probeTimePtr(now),
		FreshUntil:    probeTimePtr(now.Add(2 * time.Duration(intervalMinutes) * time.Minute)),
		LastAttemptAt: now,
		NextProbeAt:   now.Add(nextProbeDelay(intervalMinutes, 0)),
		HTTPStatus:    resp.StatusCode,
	}
	return s.persistSuccessfulUpstreamBillingProbe(ctx, account, snapshot)
}

func (s *UpstreamBillingProbeService) persistSuccessfulUpstreamBillingProbe(
	ctx context.Context,
	account *Account,
	snapshot *UpstreamBillingProbeSnapshot,
) (*UpstreamBillingProbeSnapshot, error) {
	// 账号级值域与精度只在真要写回时才有影响：只观察上游声明、未开启同步的
	// 账号不因声明值不适配 accounts.rate_multiplier 而被记成探测失败并进入
	// 指数退避——探测本身成功了，原始声明照常存进快照供展示。
	var syncRate *float64
	previousRate := account.BillingRateMultiplier()
	if upstreamBillingRateSyncEnabled(account) {
		if value, valid := upstreamBillingProbeSyncRate(snapshot.Data); valid {
			syncRate = &value
			snapshot.SyncedRateMultiplier = &value
		} else {
			declared, _ := resolveAccountExtraNumber(snapshot.Data, "resolved_rate_multiplier")
			slog.Warn("upstream_billing_rate_sync_rejected",
				"source", "upstream_billing_probe",
				"account_id", account.ID,
				"declared_resolved_rate_multiplier", declared,
				"max_rate_multiplier", upstreamBillingRateSyncMaxMultiplier,
				"current_rate_multiplier", previousRate,
			)
		}
	}
	if err := s.updateSnapshot(ctx, account, snapshot, syncRate); err != nil {
		return nil, err
	}
	if syncRate != nil {
		// 写回是后台任务的裸 SQL，不经过管理端路由，因此不会产生 audit_logs 行。
		// old_rate_multiplier 是本次探测开始时读到的值（写回的 CAS 不比对该列）。
		slog.Info("upstream_billing_rate_sync_applied",
			"source", "upstream_billing_probe",
			"account_id", account.ID,
			"old_rate_multiplier", previousRate,
			"new_rate_multiplier", *syncRate,
		)
	}
	return snapshot, nil
}

func (s *UpstreamBillingProbeService) probeUpstreamUsageBalance(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	apiKey string,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	intervalMinutes int,
	now time.Time,
	force bool,
) *UpstreamBalanceProbeSnapshot {
	var previous *UpstreamBalanceProbeSnapshot
	if snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra); snapshot != nil {
		previous = snapshot.Balance
	}
	if upstreamBillingProbeTargetIsOfficialAPI(normalizedBaseURL) {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, 0, "unsupported", 0)
	}
	if !force && previous != nil && !previous.NextProbeAt.IsZero() && now.Before(previous.NextProbeAt) {
		return previous
	}

	probeURL := buildOpenAIEndpointURL(normalizedBaseURL, "/v1/usage")
	if parsed, err := url.Parse(probeURL); err == nil {
		query := parsed.Query()
		query.Set("days", "1")
		query.Set("start_date", now.Format("2006-01-02"))
		query.Set("end_date", now.Format("2006-01-02"))
		parsed.RawQuery = query.Encode()
		probeURL = parsed.String()
	}
	probeCtx, cancel := context.WithTimeout(ctx, upstreamBillingProbeRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, probeURL, bytes.NewReader(nil))
	if err != nil {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, 0, "request_build_failed", 0)
	}
	reqCtx := WithHTTPUpstreamProfile(req.Context(), profile)
	reqCtx = WithHTTPUpstreamResolvedIPPinning(reqCtx)
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(reqCtx))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	account.ApplyHeaderOverrides(req.Header)

	resp, err := s.accountTestService.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		reason := upstreamBillingProbeRequestFailureReason(err)
		logUpstreamBillingProbeRequestFailure(account.ID, req, proxyURL != "", reason, err)
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, 0, reason, 0)
	}
	if resp == nil || resp.Body == nil {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, 0, "empty_response", 0)
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamBalanceProbeMaxBodyBytes+1))
	if readErr != nil {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, resp.StatusCode, "response_read_failed", retryAfter(resp.Header, now))
	}
	if len(body) > upstreamBalanceProbeMaxBodyBytes {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, resp.StatusCode, "response_too_large", retryAfter(resp.Header, now))
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, resp.StatusCode, "unsupported", retryAfter(resp.Header, now))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, resp.StatusCode, "unauthorized", retryAfter(resp.Header, now))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, resp.StatusCode, "http_error", retryAfter(resp.Header, now))
	}
	data, err := parseUpstreamUsageBalanceResponse(body)
	if err != nil {
		reason := "invalid_response"
		if errors.Is(err, errUpstreamBalanceUnsupported) {
			reason = "unsupported"
		}
		return newUpstreamBalanceProbeFailure(previous, intervalMinutes, now, resp.StatusCode, reason, retryAfter(resp.Header, now))
	}
	return newUpstreamBalanceProbeSuccess(data, "usage", intervalMinutes, now, resp.StatusCode)
}

func upstreamBalanceSnapshotFromBillingData(data map[string]any, intervalMinutes int, now time.Time, statusCode int) *UpstreamBalanceProbeSnapshot {
	funding, ok := data["funding"].(map[string]any)
	if !ok || len(funding) == 0 {
		return nil
	}
	return newUpstreamBalanceProbeSuccess(funding, "billing", intervalMinutes, now, statusCode)
}

func newUpstreamBalanceProbeSuccess(data map[string]any, source string, intervalMinutes int, now time.Time, statusCode int) *UpstreamBalanceProbeSnapshot {
	return &UpstreamBalanceProbeSnapshot{
		Status:        UpstreamBillingProbeStatusOK,
		Data:          data,
		Source:        source,
		ReceivedAt:    probeTimePtr(now),
		FreshUntil:    probeTimePtr(now.Add(2 * time.Duration(intervalMinutes) * time.Minute)),
		LastAttemptAt: now,
		NextProbeAt:   now.Add(nextProbeDelay(intervalMinutes, 0)),
		HTTPStatus:    statusCode,
	}
}

func newUpstreamBalanceProbeFailure(
	previous *UpstreamBalanceProbeSnapshot,
	intervalMinutes int,
	now time.Time,
	statusCode int,
	reason string,
	retryAfterDuration time.Duration,
) *UpstreamBalanceProbeSnapshot {
	status := UpstreamBillingProbeStatusFailed
	delay := nextProbeDelay(intervalMinutes, retryAfterDuration)
	if reason == "unsupported" {
		status = UpstreamBillingProbeStatusUnsupported
		delay = unsupportedProbeDelay(intervalMinutes, retryAfterDuration)
	}
	failureCount := 1
	if previous != nil {
		failureCount = previous.FailureCount + 1
	}
	source := "usage"
	if previous != nil && len(previous.Data) > 0 && previous.Source != "" {
		source = previous.Source
	}
	snapshot := &UpstreamBalanceProbeSnapshot{
		Status:        status,
		Source:        source,
		LastAttemptAt: now,
		NextProbeAt:   now.Add(delay),
		FailureCount:  failureCount,
		HTTPStatus:    statusCode,
		LastError:     reason,
	}
	if previous != nil {
		snapshot.Data = previous.Data
		snapshot.ReceivedAt = previous.ReceivedAt
		snapshot.FreshUntil = previous.FreshUntil
		if snapshot.FreshUntil == nil && previous.Status == UpstreamBillingProbeStatusOK && previous.ReceivedAt != nil {
			snapshot.FreshUntil = probeTimePtr(previous.ReceivedAt.Add(2 * time.Duration(intervalMinutes) * time.Minute))
		}
	}
	return snapshot
}

func (s *UpstreamBillingProbeService) persistProbeFailure(
	ctx context.Context,
	account *Account,
	intervalMinutes int,
	now time.Time,
	statusCode int,
	reason string,
	retryAfterDuration time.Duration,
) (*UpstreamBillingProbeSnapshot, error) {
	return s.persistProbeFailureWithBalance(
		ctx, account, intervalMinutes, now, statusCode, reason, retryAfterDuration, nil,
	)
}

func (s *UpstreamBillingProbeService) persistProbeFailureWithBalance(
	ctx context.Context,
	account *Account,
	intervalMinutes int,
	now time.Time,
	statusCode int,
	reason string,
	retryAfterDuration time.Duration,
	balance *UpstreamBalanceProbeSnapshot,
) (*UpstreamBillingProbeSnapshot, error) {
	previous := decodeUpstreamBillingProbeSnapshot(account.Extra)
	failureCount := 1
	if previous != nil {
		failureCount = previous.FailureCount + 1
	}
	status := UpstreamBillingProbeStatusFailed
	delay := nextProbeDelay(intervalMinutes, retryAfterDuration)
	if reason == "unsupported" {
		status = UpstreamBillingProbeStatusUnsupported
		delay = unsupportedProbeDelay(intervalMinutes, retryAfterDuration)
		if balance != nil && balance.Status == UpstreamBillingProbeStatusOK {
			delay = nextProbeDelay(intervalMinutes, retryAfterDuration)
		}
	}
	snapshot := &UpstreamBillingProbeSnapshot{
		Status:        status,
		LastAttemptAt: now,
		NextProbeAt:   now.Add(delay),
		FailureCount:  failureCount,
		HTTPStatus:    statusCode,
		LastError:     reason,
		Balance:       balance,
	}
	if previous != nil {
		snapshot.Data = previous.Data
		snapshot.ReceivedAt = previous.ReceivedAt
		snapshot.FreshUntil = previous.FreshUntil
		if snapshot.FreshUntil == nil && previous.Status == UpstreamBillingProbeStatusOK && previous.ReceivedAt != nil {
			snapshot.FreshUntil = probeTimePtr(previous.ReceivedAt.Add(2 * time.Duration(intervalMinutes) * time.Minute))
		}
		if snapshot.Balance == nil {
			snapshot.Balance = previous.Balance
		}
	}
	if err := s.updateSnapshot(ctx, account, snapshot, nil); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *UpstreamBillingProbeService) updateSnapshot(
	ctx context.Context,
	account *Account,
	snapshot *UpstreamBillingProbeSnapshot,
	rateMultiplier *float64,
) error {
	if snapshot != nil {
		alreadyAutoPaused := account != nil && upstreamBillingAutoUnschedulable(account.Extra)
		snapshot.AutoUnschedulable = alreadyAutoPaused ||
			(UpstreamBillingBalanceExhausted(snapshot) && account != nil && account.Schedulable)
	}
	writer, ok := s.accountRepo.(upstreamBillingProbeSnapshotWriter)
	if !ok {
		return ErrUpstreamBillingProbeUnavailable
	}
	return writer.UpdateUpstreamBillingProbeSnapshot(ctx, account, snapshot, rateMultiplier)
}

func upstreamBillingAutoUnschedulable(extra map[string]any) bool {
	if extra == nil {
		return false
	}
	marked, ok := extra[UpstreamBillingAutoUnschedulableExtraKey].(bool)
	return ok && marked
}

func parseUpstreamBillingProbeResponse(body []byte) (map[string]any, error) {
	var response upstreamBillingProbeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Object != "sub2api.key_billing" || response.SchemaVersion != 1 || response.BillingScope != "token" {
		return nil, fmt.Errorf("unexpected billing response schema")
	}
	if response.GroupRateMultiplier == nil || response.ResolvedRateMultiplier == nil ||
		response.PeakRateEnabled == nil || response.EffectiveRateMultiplier == nil {
		return nil, fmt.Errorf("incomplete billing response")
	}
	for _, value := range []float64{
		*response.GroupRateMultiplier,
		*response.ResolvedRateMultiplier,
		*response.EffectiveRateMultiplier,
	} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid billing multiplier")
		}
	}
	if response.UserRateMultiplier != nil && (*response.UserRateMultiplier < 0 || math.IsNaN(*response.UserRateMultiplier) || math.IsInf(*response.UserRateMultiplier, 0)) {
		return nil, fmt.Errorf("invalid user billing multiplier")
	}
	expectedResolved := *response.GroupRateMultiplier
	if response.UserRateMultiplier != nil {
		expectedResolved = *response.UserRateMultiplier
	}
	if !equalBillingMultiplier(*response.ResolvedRateMultiplier, expectedResolved) {
		return nil, fmt.Errorf("inconsistent resolved billing multiplier")
	}
	observedAt, err := time.Parse(time.RFC3339Nano, response.ObservedAt)
	if err != nil || observedAt.IsZero() {
		return nil, fmt.Errorf("invalid observed_at")
	}
	data := map[string]any{
		"object":                    response.Object,
		"schema_version":            response.SchemaVersion,
		"billing_scope":             response.BillingScope,
		"group_rate_multiplier":     *response.GroupRateMultiplier,
		"resolved_rate_multiplier":  *response.ResolvedRateMultiplier,
		"peak_rate_enabled":         *response.PeakRateEnabled,
		"effective_rate_multiplier": *response.EffectiveRateMultiplier,
		"observed_at":               observedAt.UTC().Format(time.RFC3339Nano),
	}
	if response.UserRateMultiplier != nil {
		data["user_rate_multiplier"] = *response.UserRateMultiplier
	}
	if len(response.Funding) > 0 {
		var declaredFunding upstreamBillingFundingResponse
		if fundingErr := json.Unmarshal(response.Funding, &declaredFunding); fundingErr != nil || bytes.Equal(bytes.TrimSpace(response.Funding), []byte("null")) {
			return nil, fmt.Errorf("invalid funding response")
		}
		funding, fundingErr := sanitizeUpstreamBillingFunding(&declaredFunding)
		if fundingErr != nil {
			return nil, fmt.Errorf("invalid funding response: %w", fundingErr)
		}
		data["funding"] = funding
	}
	if *response.PeakRateEnabled {
		if response.PeakStart == nil || response.PeakEnd == nil || response.Timezone == nil ||
			response.PeakRateMultiplier == nil || response.AppliedPeakMultiplier == nil ||
			*response.PeakStart == "" || *response.PeakEnd == "" || *response.Timezone == "" ||
			*response.PeakRateMultiplier < 0 || *response.AppliedPeakMultiplier < 0 ||
			math.IsNaN(*response.PeakRateMultiplier) || math.IsInf(*response.PeakRateMultiplier, 0) ||
			math.IsNaN(*response.AppliedPeakMultiplier) || math.IsInf(*response.AppliedPeakMultiplier, 0) {
			return nil, fmt.Errorf("incomplete peak billing response")
		}
		data["peak_start"] = *response.PeakStart
		data["peak_end"] = *response.PeakEnd
		data["peak_rate_multiplier"] = *response.PeakRateMultiplier
		data["applied_peak_multiplier"] = *response.AppliedPeakMultiplier
		data["timezone"] = *response.Timezone
	}
	appliedPeak, ok := upstreamBillingPeakMultiplierAt(data, observedAt)
	if !ok {
		return nil, fmt.Errorf("invalid peak billing response")
	}
	if response.PeakRateEnabled != nil && *response.PeakRateEnabled {
		if !equalBillingMultiplier(*response.AppliedPeakMultiplier, appliedPeak) {
			return nil, fmt.Errorf("inconsistent applied peak multiplier")
		}
	} else if response.AppliedPeakMultiplier != nil && !equalBillingMultiplier(*response.AppliedPeakMultiplier, 1) {
		return nil, fmt.Errorf("inconsistent applied peak multiplier")
	}
	if !equalBillingMultiplier(*response.EffectiveRateMultiplier, *response.ResolvedRateMultiplier*appliedPeak) {
		return nil, fmt.Errorf("inconsistent effective billing multiplier")
	}
	return data, nil
}

func upstreamBillingObservationIsFresh(data map[string]any, now time.Time) bool {
	value, ok := data["observed_at"].(string)
	if !ok {
		return false
	}
	observedAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || observedAt.IsZero() {
		return false
	}
	delta := observedAt.Sub(now)
	return delta >= -upstreamBillingProbeObservationMaxAge && delta <= upstreamBillingProbeObservationFutureSkew
}

func sanitizeUpstreamBillingFunding(funding *upstreamBillingFundingResponse) (map[string]any, error) {
	if funding == nil {
		return nil, errUpstreamBalanceUnsupported
	}
	unit, ok := normalizeUpstreamBalanceUnit(funding.Unit)
	if !ok {
		return nil, fmt.Errorf("invalid funding unit")
	}
	for _, value := range []*float64{funding.Balance, funding.Remaining, funding.Limit, funding.Used} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return nil, fmt.Errorf("invalid funding amount")
		}
	}

	data := map[string]any{"mode": funding.Mode, "unit": unit}
	switch funding.Mode {
	case "wallet":
		if funding.Balance == nil {
			return nil, fmt.Errorf("wallet balance is missing")
		}
		data["balance"] = *funding.Balance
		remaining := funding.Balance
		if funding.Remaining != nil {
			remaining = funding.Remaining
		}
		data["remaining"] = *remaining
	case "key_quota":
		if funding.Remaining == nil {
			return nil, fmt.Errorf("key quota remaining is invalid")
		}
		data["remaining"] = *funding.Remaining
		if funding.Limit != nil {
			if *funding.Limit < 0 {
				return nil, fmt.Errorf("key quota limit is invalid")
			}
			data["limit"] = *funding.Limit
		}
		if funding.Used != nil {
			if *funding.Used < 0 {
				return nil, fmt.Errorf("key quota usage is invalid")
			}
			data["used"] = *funding.Used
		}
	case "subscription":
		if funding.Unlimited {
			data["unlimited"] = true
		} else {
			if funding.Remaining == nil {
				return nil, fmt.Errorf("subscription remaining is invalid")
			}
			data["remaining"] = *funding.Remaining
		}
	default:
		return nil, errUpstreamBalanceUnsupported
	}
	return data, nil
}

func parseUpstreamUsageBalanceResponse(body []byte) (map[string]any, error) {
	var response upstreamUsageBalanceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	var funding upstreamBillingFundingResponse
	switch response.Mode {
	case "quota_limited":
		if response.Quota == nil && response.Remaining == nil {
			return nil, errUpstreamBalanceUnsupported
		}
		funding.Mode = "key_quota"
		funding.Unit = response.Unit
		funding.Remaining = response.Remaining
		if response.Quota != nil {
			if funding.Unit == "" {
				funding.Unit = response.Quota.Unit
			}
			if funding.Remaining == nil {
				funding.Remaining = response.Quota.Remaining
			}
			funding.Limit = response.Quota.Limit
			funding.Used = response.Quota.Used
		}
	case "unrestricted":
		funding.Unit = response.Unit
		if response.Balance != nil {
			funding.Mode = "wallet"
			funding.Balance = response.Balance
			funding.Remaining = response.Remaining
		} else if hasJSONObject(response.Subscription) {
			funding.Mode = "subscription"
			if response.Remaining != nil && *response.Remaining < 0 {
				funding.Unlimited = true
			} else {
				funding.Remaining = response.Remaining
			}
		} else {
			return nil, errUpstreamBalanceUnsupported
		}
	default:
		return nil, errUpstreamBalanceUnsupported
	}

	data, err := sanitizeUpstreamBillingFunding(&funding)
	if err != nil {
		return nil, err
	}
	if response.IsValid != nil {
		data["is_valid"] = *response.IsValid
	}
	return data, nil
}

func hasJSONObject(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

func upstreamBillingProbeRequestFailureReason(err error) string {
	if err == nil {
		return "request_failed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "request_timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "request_timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns_failed"
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "resolved ip") && strings.Contains(lower, "not allowed") {
		return "resolved_address_blocked"
	}
	if strings.Contains(lower, "proxy") || strings.Contains(lower, "socks") {
		return "proxy_failed"
	}
	if strings.Contains(lower, "http/1.x transport connection broken") ||
		strings.Contains(lower, "malformed http response") ||
		strings.Contains(lower, "http2: frame too large") {
		return "protocol_failed"
	}
	var recordHeaderErr tls.RecordHeaderError
	var unknownAuthorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	if errors.As(err, &recordHeaderErr) || errors.As(err, &unknownAuthorityErr) || errors.As(err, &hostnameErr) ||
		strings.Contains(lower, "tls") || strings.Contains(lower, "x509") || strings.Contains(lower, "certificate") {
		return "tls_failed"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "connection_failed"
	}
	return "request_failed"
}

func logUpstreamBillingProbeRequestFailure(accountID int64, req *http.Request, proxyConfigured bool, reason string, err error) {
	targetHost := ""
	if req != nil && req.URL != nil {
		targetHost = req.URL.Hostname()
	}
	slog.Warn("upstream_billing_probe_request_failed",
		"account_id", accountID,
		"target_host", targetHost,
		"proxy_configured", proxyConfigured,
		"reason", reason,
		"error", err,
	)
}

func normalizeUpstreamBalanceUnit(value string) (string, bool) {
	unit := strings.ToUpper(strings.TrimSpace(value))
	if unit == "" || len(unit) > 12 {
		return "", false
	}
	for _, char := range unit {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return "", false
		}
	}
	return unit, true
}

func upstreamBillingRateAt(data map[string]any, now time.Time) (float64, bool) {
	if scope, _ := data["billing_scope"].(string); scope != "token" {
		return 0, false
	}
	base, ok := resolveAccountExtraNumber(data, "resolved_rate_multiplier")
	if !ok || base < 0 || math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, false
	}
	appliedPeak, ok := upstreamBillingPeakMultiplierAt(data, now)
	if !ok {
		return 0, false
	}
	base *= appliedPeak
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, false
	}
	return base, true
}

// upstreamBillingProbeSyncRate converts the declared multiplier into the value
// the automatic write-back may store in accounts.rate_multiplier, at the
// precision that column supports (DECIMAL(10,4)).
//
// It reads resolved_rate_multiplier, not effective_rate_multiplier: the
// effective value folds in the peak coefficient that happened to apply at the
// instant of the probe, so writing it would freeze one probe cycle's peak (or
// off-peak) factor into a static column, while display and scheduling
// recompute the peak factor for the current time through upstreamBillingRateAt.
//
// The accepted range is deliberately narrower than the column:
//   - 0 is rejected. accountCost multiplies the request cost by this value, so
//     an upstream-declared 0 would stop quota_used from ever growing and every
//     admin-configured account quota and cost alert would silently stop
//     working. Admins may still set 0 by hand; only the automatic path refuses.
//   - anything above upstreamBillingRateSyncMaxMultiplier is rejected.
//
// A rejected declaration leaves the current multiplier untouched; the probe
// still records an OK snapshot carrying the raw declaration for display.
func upstreamBillingProbeSyncRate(data map[string]any) (float64, bool) {
	value, ok := resolveAccountExtraNumber(data, "resolved_rate_multiplier")
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	rounded := math.Round(value*upstreamBillingProbeAccountRateScale) / upstreamBillingProbeAccountRateScale
	if rounded <= 0 || rounded > upstreamBillingRateSyncMaxMultiplier {
		return 0, false
	}
	return rounded, true
}

func upstreamBillingPeakMultiplierAt(data map[string]any, now time.Time) (float64, bool) {
	peakEnabled, ok := data["peak_rate_enabled"].(bool)
	if !ok {
		return 0, false
	}
	if !peakEnabled {
		return 1, true
	}

	start, startOK := data["peak_start"].(string)
	end, endOK := data["peak_end"].(string)
	timezoneName, timezoneOK := data["timezone"].(string)
	peakMultiplier, multiplierOK := resolveAccountExtraNumber(data, "peak_rate_multiplier")
	startMinute, validStart := parseMinutes(start)
	endMinute, validEnd := parseMinutes(end)
	if !startOK || !endOK || !timezoneOK || !multiplierOK || !validStart || !validEnd ||
		startMinute >= endMinute || peakMultiplier < 0 || math.IsNaN(peakMultiplier) || math.IsInf(peakMultiplier, 0) {
		return 0, false
	}
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return 0, false
	}

	local := now.In(location)
	minute := local.Hour()*60 + local.Minute()
	if minute >= startMinute && minute < endMinute {
		return peakMultiplier, true
	}
	return 1, true
}

func equalBillingMultiplier(left, right float64) bool {
	if math.IsNaN(left) || math.IsNaN(right) || math.IsInf(left, 0) || math.IsInf(right, 0) {
		return false
	}
	scale := math.Max(1, math.Max(math.Abs(left), math.Abs(right)))
	return math.Abs(left-right) <= 1e-9*scale
}

func decodeUpstreamBillingProbeSnapshot(extra map[string]any) *UpstreamBillingProbeSnapshot {
	if extra == nil {
		return nil
	}
	value, ok := extra[UpstreamBillingProbeExtraKey]
	if !ok {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var snapshot UpstreamBillingProbeSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil || snapshot.Status == "" {
		return nil
	}
	if snapshot.Status != UpstreamBillingProbeStatusOK &&
		snapshot.Status != UpstreamBillingProbeStatusUnsupported &&
		snapshot.Status != UpstreamBillingProbeStatusFailed {
		return nil
	}
	return &snapshot
}

// IsUpstreamBillingProbeIdentity reports whether an account identity may opt
// in to the upstream billing probe. `/v1/sub2api/billing` is a key-scoped
// sub2api convention shared by the supported API-key platforms (including the
// CN providers, whose official-domain accounts are short-circuited to
// "unsupported" by upstreamBillingProbeTargetIsOfficialAPI).
// Non-sub2api upstreams return 404 and the snapshot records "unsupported".
// OAuth/Bedrock hold no static API key to present at all. Antigravity's legacy
// AccountTypeUpstream rows do carry the same base_url + api_key identity as
// API-key rows, so they share the probe path for backwards compatibility.
func IsUpstreamBillingProbeIdentity(platform, accountType string) bool {
	if accountType != AccountTypeAPIKey && (accountType != AccountTypeUpstream || platform != PlatformAntigravity) {
		return false
	}
	switch platform {
	case PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformGrok,
		PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo:
		return true
	default:
		return false
	}
}

func isUpstreamBillingProbeAccount(account *Account) bool {
	return account != nil && IsUpstreamBillingProbeIdentity(account.Platform, account.Type)
}

// upstreamBillingProbeOfficialAPIDomains lists the root domains of official
// provider APIs. The create form fills empty base_url values with official
// defaults (and offers official regional presets like us-east-1.api.x.ai),
// so probing them would send the account key to an official API path that
// cannot exist. Matching is by registrable root domain — exact host or any
// subdomain, after stripping the port and a trailing DNS dot — because no
// third-party sub2api relay can live under these domains, while custom
// relays (the only targets that can answer /v1/sub2api/billing) always do
// probe.
// ollama.com is a first-class configuration here (Ollama Cloud accounts are
// platform openai/anthropic with base_url https://ollama.com/v1), and it is
// an official provider API just like the rest, so it belongs on this list.
// CN provider domains (moonshot.cn / kimi.com / bigmodel.cn / deepseek.com)
// serve the same role: official APIs that can never host /v1/sub2api/billing,
// so their accounts short-circuit to "unsupported" without a request.
var upstreamBillingProbeOfficialAPIDomains = []string{
	"anthropic.com",
	"googleapis.com",
	"x.ai",
	"grok.com",
	"openai.com",
	"openai.azure.com",
	"ollama.com",
	"moonshot.ai",
	"moonshot.cn",
	"kimi.com",
	"bigmodel.cn",
	"z.ai",
	"deepseek.com",
	"opencode.ai",
}

func upstreamBillingProbeTargetIsOfficialAPI(baseURL string) bool {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return true
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" {
		return true
	}
	for _, domain := range upstreamBillingProbeOfficialAPIDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func upstreamBillingProbeEnabled(account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	enabled, ok := account.Extra[UpstreamBillingProbeEnabledExtraKey].(bool)
	return ok && enabled
}

// upstreamBillingRateSyncEnabled is the probe-side pre-filter deciding whether
// a rate is even proposed for write-back. It is a necessary condition, not the
// authority: the repository CAS re-checks both switches against the row it
// updates, so a switch flipped between load and write can never sneak a rate in.
func upstreamBillingRateSyncEnabled(account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	enabled, ok := account.Extra[UpstreamBillingRateSyncEnabledExtraKey].(bool)
	return ok && enabled && upstreamBillingProbeEnabled(account)
}

func (s *UpstreamBillingProbeService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func nextProbeDelay(intervalMinutes int, retryAfterDuration time.Duration) time.Duration {
	interval := time.Duration(intervalMinutes) * time.Minute
	if interval < upstreamBillingProbeMinIntervalMinutes*time.Minute {
		interval = upstreamBillingProbeMinIntervalMinutes * time.Minute
	}
	if interval > upstreamBillingProbeMaxDelay {
		interval = upstreamBillingProbeMaxDelay
	}
	jitterRange := interval / 5
	if jitterRange > 5*time.Minute {
		jitterRange = 5 * time.Minute
	}
	if jitterRange > 0 {
		interval += time.Duration(rand.Int64N(int64(jitterRange)*2+1)) - jitterRange
	}
	if retryAfterDuration > interval {
		// Retry-After is an explicit upstream instruction; do not shorten it
		// with the local maximum delay.
		return retryAfterDuration
	}
	if interval > upstreamBillingProbeMaxDelay {
		return upstreamBillingProbeMaxDelay
	}
	return interval
}

// unsupportedProbeDelay 拉长 unsupported 账号的重探间隔，让无效候选自然退出
// 热队列，不再和真正接入 sub2api 的中转账号抢每周期的探测名额。
// 仍按 upstreamBillingProbeMaxDelay 封顶，保证上游后来接入 sub2api 时最迟一天
// 内会被重新发现；base 本身已达上限（例如 Retry-After 明确要求更久）时原样返回，
// 不缩短上游指令。
func unsupportedProbeDelay(intervalMinutes int, retryAfterDuration time.Duration) time.Duration {
	base := nextProbeDelay(intervalMinutes, retryAfterDuration)
	if base >= upstreamBillingProbeMaxDelay {
		return base
	}
	stretched := base * upstreamBillingProbeUnsupportedDelayFactor
	if stretched > upstreamBillingProbeMaxDelay {
		return upstreamBillingProbeMaxDelay
	}
	return stretched
}

func retryAfter(header http.Header, now time.Time) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		if delay := at.Sub(now); delay > 0 {
			return delay
		}
	}
	return 0
}

func probeTimePtr(value time.Time) *time.Time {
	return &value
}

func safeProbeError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrUpstreamBillingProbeAccountInvalid) {
		return ErrUpstreamBillingProbeAccountInvalid.Error()
	}
	if errors.Is(err, ErrUpstreamBillingProbeUnavailable) {
		return ErrUpstreamBillingProbeUnavailable.Error()
	}
	return "probe_failed"
}
