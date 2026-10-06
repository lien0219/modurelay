package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	PolicyQuotaReservationPending   = "pending"
	PolicyQuotaReservationFinalized = "finalized"
	PolicyQuotaReservationReleased  = "released"
)

var (
	ErrPolicyQuotaUnavailable         = errors.New("POLICY_QUOTA_UNAVAILABLE")
	ErrPolicyQuotaExceeded            = errors.New("POLICY_QUOTA_EXCEEDED")
	ErrPolicyQuotaReservationConflict = errors.New("POLICY_QUOTA_RESERVATION_CONFLICT")
	ErrPolicyQuotaReservationClosed   = errors.New("POLICY_QUOTA_RESERVATION_CLOSED")
	ErrPolicyQuotaReservationInvalid  = errors.New("POLICY_QUOTA_RESERVATION_INVALID")
	ErrPolicyQuotaTokenUsageUnknown   = errors.New("POLICY_QUOTA_TOKEN_USAGE_UNKNOWN")
)

const (
	policyQuotaMutationMaxAttempts = 3
	policyQuotaMutationBackoff     = 10 * time.Millisecond
)

// PolicyQuotaScope is an immutable policy snapshot used by admission. The
// repository stores one counter per scope, revision, metric and UTC period so
// policy changes never rewrite historical usage or in-flight reservations.
type PolicyQuotaScope struct {
	Scope               domain.PolicyScope
	ID                  int64
	Revision            int64
	DailyRequestLimit   *int64
	MonthlyRequestLimit *int64
	DailyTokenLimit     *int64
	MonthlyTokenLimit   *int64
}

type PolicyQuotaAttribution struct {
	PolicyContext domain.PolicyContext
	APIKeyID      int64
	RequestUnits  int64
}

type PolicyQuotaReservationRequest struct {
	RequestID        string
	APIKeyID         int64
	WorkspaceID      int64
	ProjectID        int64
	ServiceAccountID int64
	EstimatedTokens  int64
	RequestUnits     int64
	Scopes           []PolicyQuotaScope
	Now              time.Time
}

type PolicyQuotaReservation struct {
	ID               string
	RequestID        string
	APIKeyID         int64
	WorkspaceID      int64
	ProjectID        int64
	ServiceAccountID int64
	EstimatedTokens  int64
	RequestUnits     int64
	Status           string
}

type PolicyQuotaRepository interface {
	Reserve(context.Context, PolicyQuotaReservationRequest) (*PolicyQuotaReservation, error)
	Finalize(context.Context, string, int64) error
	Release(context.Context, string) error
}

type PolicyQuotaService struct {
	repo    PolicyQuotaRepository
	metrics policyQuotaMutationMetrics
}

type policyQuotaMutationMetrics struct {
	finalizeAttempts atomic.Uint64
	finalizeRetries  atomic.Uint64
	finalizeFailures atomic.Uint64
	releaseAttempts  atomic.Uint64
	releaseRetries   atomic.Uint64
	releaseFailures  atomic.Uint64
}

type PolicyQuotaMetricsSnapshot struct {
	FinalizeAttempts uint64
	FinalizeRetries  uint64
	FinalizeFailures uint64
	ReleaseAttempts  uint64
	ReleaseRetries   uint64
	ReleaseFailures  uint64
}

func NewPolicyQuotaService(repo PolicyQuotaRepository) *PolicyQuotaService {
	return &PolicyQuotaService{repo: repo}
}

func (s *PolicyQuotaService) SnapshotMetrics() PolicyQuotaMetricsSnapshot {
	if s == nil {
		return PolicyQuotaMetricsSnapshot{}
	}
	return PolicyQuotaMetricsSnapshot{
		FinalizeAttempts: s.metrics.finalizeAttempts.Load(),
		FinalizeRetries:  s.metrics.finalizeRetries.Load(),
		FinalizeFailures: s.metrics.finalizeFailures.Load(),
		ReleaseAttempts:  s.metrics.releaseAttempts.Load(),
		ReleaseRetries:   s.metrics.releaseRetries.Load(),
		ReleaseFailures:  s.metrics.releaseFailures.Load(),
	}
}

// PolicyQuotaScopes preserves source layers and their revisions. Numeric
// effective minima are useful for explainability, but enforcement must retain
// every configured layer so an update cannot broaden a parent counter.
func PolicyQuotaScopes(policy domain.EffectivePolicy) []PolicyQuotaScope {
	entries := []struct {
		scope  domain.PolicyScope
		policy *domain.Policy
	}{
		{domain.PolicyScopeGroup, policy.Layers.Group},
		{domain.PolicyScopeWorkspace, policy.Layers.Workspace},
		{domain.PolicyScopeProject, policy.Layers.Project},
		{domain.PolicyScopeServiceAccount, policy.Layers.ServiceAccount},
		{domain.PolicyScopeCredential, policy.Layers.Credential},
	}
	scopes := make([]PolicyQuotaScope, 0, len(entries))
	for _, entry := range entries {
		p := entry.policy
		if p == nil || !policyHasQuota(p) || p.ScopeID <= 0 {
			continue
		}
		revision := p.Revision
		if revision <= 0 {
			revision = 1
		}
		scopes = append(scopes, PolicyQuotaScope{
			Scope:               entry.scope,
			ID:                  p.ScopeID,
			Revision:            revision,
			DailyRequestLimit:   clonePolicyQuotaLimit(p.DailyRequestLimit),
			MonthlyRequestLimit: clonePolicyQuotaLimit(p.MonthlyRequestLimit),
			DailyTokenLimit:     clonePolicyQuotaLimit(p.DailyTokenLimit),
			MonthlyTokenLimit:   clonePolicyQuotaLimit(p.MonthlyTokenLimit),
		})
	}
	return scopes
}

func policyHasQuota(p *domain.Policy) bool {
	return p != nil && (p.DailyRequestLimit != nil || p.MonthlyRequestLimit != nil || p.DailyTokenLimit != nil || p.MonthlyTokenLimit != nil)
}

func clonePolicyQuotaLimit(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func (s *PolicyQuotaService) Admit(ctx context.Context, policy domain.EffectivePolicy, attribution PolicyQuotaAttribution, requestID string, estimatedTokens int64) (*PolicyQuotaReservationHandle, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || attribution.APIKeyID <= 0 || estimatedTokens < 0 || attribution.RequestUnits < 0 {
		return nil, ErrPolicyQuotaReservationInvalid
	}
	scopes := PolicyQuotaScopes(policy)
	if len(scopes) == 0 {
		return nil, nil
	}
	if s == nil || s.repo == nil {
		return nil, ErrPolicyQuotaUnavailable
	}
	if attribution.PolicyContext.WorkspaceID <= 0 || attribution.PolicyContext.ProjectID <= 0 {
		return nil, ErrPolicyQuotaReservationInvalid
	}
	requestUnits := attribution.RequestUnits
	if requestUnits == 0 {
		requestUnits = 1
	}
	if estimatedTokens == 0 && policyHasTokenQuota(scopes) {
		// A zero usage report must never bypass a hard token quota. The gateway
		// may not have a reliable tokenizer for every protocol, so reserve one
		// conservative token and let the actual usage replace it on finalize.
		estimatedTokens = 1
	}
	reservation, err := s.repo.Reserve(ctx, PolicyQuotaReservationRequest{
		RequestID:        requestID,
		APIKeyID:         attribution.APIKeyID,
		WorkspaceID:      attribution.PolicyContext.WorkspaceID,
		ProjectID:        attribution.PolicyContext.ProjectID,
		ServiceAccountID: attribution.PolicyContext.ServiceAccountID,
		EstimatedTokens:  estimatedTokens,
		RequestUnits:     requestUnits,
		Scopes:           scopes,
		Now:              time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if reservation == nil || strings.TrimSpace(reservation.ID) == "" {
		return nil, ErrPolicyQuotaUnavailable
	}
	if reservation.Status != "" && reservation.Status != PolicyQuotaReservationPending {
		return nil, ErrPolicyQuotaReservationClosed
	}
	if reservation.APIKeyID != 0 && reservation.APIKeyID != attribution.APIKeyID {
		return nil, ErrPolicyQuotaReservationConflict
	}
	return NewPolicyQuotaReservationHandleWithUnits(s, reservation.ID, maxInt64(reservation.EstimatedTokens, estimatedTokens), maxInt64(reservation.RequestUnits, requestUnits)), nil
}

func policyHasTokenQuota(scopes []PolicyQuotaScope) bool {
	for _, scope := range scopes {
		if scope.DailyTokenLimit != nil || scope.MonthlyTokenLimit != nil {
			return true
		}
	}
	return false
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *PolicyQuotaService) Finalize(ctx context.Context, reservationID string, actualTokens int64) error {
	if strings.TrimSpace(reservationID) == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return ErrPolicyQuotaUnavailable
	}
	if actualTokens < 0 {
		return ErrPolicyQuotaTokenUsageUnknown
	}
	return s.mutateWithRetry(ctx, "finalize", reservationID, func() error {
		return s.repo.Finalize(ctx, reservationID, actualTokens)
	})
}

func (s *PolicyQuotaService) Release(ctx context.Context, reservationID string) error {
	if strings.TrimSpace(reservationID) == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return ErrPolicyQuotaUnavailable
	}
	return s.mutateWithRetry(ctx, "release", reservationID, func() error {
		return s.repo.Release(ctx, reservationID)
	})
}

func (s *PolicyQuotaService) mutateWithRetry(ctx context.Context, operation, reservationID string, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for attempt := 1; attempt <= policyQuotaMutationMaxAttempts; attempt++ {
		s.recordMutationAttempt(operation)
		err := fn()
		if err == nil {
			return nil
		}
		if !policyQuotaMutationRetryable(err) || attempt == policyQuotaMutationMaxAttempts {
			s.recordMutationFailure(operation)
			logger.FromContext(ctx).Error("policy quota mutation failed", zap.String("operation", operation), zap.String("reservation_id", strings.TrimSpace(reservationID)), zap.Int("attempts", attempt), zap.String("failure_code", policyQuotaFailureCode(err)))
			return err
		}
		s.recordMutationRetry(operation)
		logger.FromContext(ctx).Warn("policy quota mutation retry scheduled", zap.String("operation", operation), zap.String("reservation_id", strings.TrimSpace(reservationID)), zap.Int("attempt", attempt), zap.String("failure_code", policyQuotaFailureCode(err)))
		delay := policyQuotaMutationBackoff * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			s.recordMutationFailure(operation)
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ErrPolicyQuotaUnavailable
}

func (s *PolicyQuotaService) recordMutationAttempt(operation string) {
	if operation == "release" {
		s.metrics.releaseAttempts.Add(1)
		return
	}
	s.metrics.finalizeAttempts.Add(1)
}

func (s *PolicyQuotaService) recordMutationRetry(operation string) {
	if operation == "release" {
		s.metrics.releaseRetries.Add(1)
		return
	}
	s.metrics.finalizeRetries.Add(1)
}

func (s *PolicyQuotaService) recordMutationFailure(operation string) {
	if operation == "release" {
		s.metrics.releaseFailures.Add(1)
		return
	}
	s.metrics.finalizeFailures.Add(1)
}

func policyQuotaMutationRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	for _, permanent := range []error{ErrPolicyQuotaUnavailable, ErrPolicyQuotaExceeded, ErrPolicyQuotaReservationConflict, ErrPolicyQuotaReservationClosed, ErrPolicyQuotaReservationInvalid, ErrPolicyQuotaTokenUsageUnknown} {
		if errors.Is(err, permanent) {
			return false
		}
	}
	return true
}

func policyQuotaFailureCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline"
	case errors.Is(err, ErrPolicyQuotaUnavailable):
		return "unavailable"
	case errors.Is(err, ErrPolicyQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(err, ErrPolicyQuotaReservationConflict):
		return "reservation_conflict"
	case errors.Is(err, ErrPolicyQuotaReservationClosed):
		return "reservation_closed"
	case errors.Is(err, ErrPolicyQuotaReservationInvalid):
		return "reservation_invalid"
	case errors.Is(err, ErrPolicyQuotaTokenUsageUnknown):
		return "token_usage_unknown"
	default:
		return "repository_error"
	}
}

type policyQuotaReservationContextKey struct{}
type policyQuotaServiceContextKey struct{}

func WithPolicyQuotaService(ctx context.Context, service *PolicyQuotaService) context.Context {
	return context.WithValue(ctx, policyQuotaServiceContextKey{}, service)
}

func PolicyQuotaServiceFromContext(ctx context.Context) *PolicyQuotaService {
	if ctx == nil {
		return nil
	}
	service, _ := ctx.Value(policyQuotaServiceContextKey{}).(*PolicyQuotaService)
	return service
}

func WithPolicyQuotaReservation(ctx context.Context, handle *PolicyQuotaReservationHandle) context.Context {
	if handle == nil {
		return ctx
	}
	return context.WithValue(ctx, policyQuotaReservationContextKey{}, handle)
}

func PolicyQuotaReservationFromContext(ctx context.Context) *PolicyQuotaReservationHandle {
	if ctx == nil {
		return nil
	}
	handle, _ := ctx.Value(policyQuotaReservationContextKey{}).(*PolicyQuotaReservationHandle)
	return handle
}

func PolicyQuotaReservationIDFromContext(ctx context.Context) string {
	if handle := PolicyQuotaReservationFromContext(ctx); handle != nil {
		return handle.ID()
	}
	return ""
}

func MarkPolicyQuotaProviderStarted(ctx context.Context) {
	if handle := PolicyQuotaReservationFromContext(ctx); handle != nil {
		handle.MarkProviderStarted()
	}
}

func MarkPolicyQuotaProviderRejected(ctx context.Context) {
	if handle := PolicyQuotaReservationFromContext(ctx); handle != nil {
		handle.MarkProviderRejected()
	}
}

// PolicyQuotaReservationHandle owns exactly one request reservation. It is
// safe for a handler and its asynchronous usage worker to race cleanup.
type PolicyQuotaReservationHandle struct {
	service      *PolicyQuotaService
	id           string
	estimate     int64
	requestUnits int64
	mu           sync.Mutex
	finalized    bool
	released     bool
	durable      atomic.Bool
	started      atomic.Bool
	rejected     atomic.Bool
}

func NewPolicyQuotaReservationHandle(service *PolicyQuotaService, id string, estimate int64) *PolicyQuotaReservationHandle {
	return NewPolicyQuotaReservationHandleWithUnits(service, id, estimate, 1)
}

func NewPolicyQuotaReservationHandleWithUnits(service *PolicyQuotaService, id string, estimate, requestUnits int64) *PolicyQuotaReservationHandle {
	if service == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	return &PolicyQuotaReservationHandle{service: service, id: strings.TrimSpace(id), estimate: maxInt64(estimate, 0), requestUnits: maxInt64(requestUnits, 1)}
}

func (h *PolicyQuotaReservationHandle) ID() string {
	if h == nil {
		return ""
	}
	return h.id
}

func (h *PolicyQuotaReservationHandle) EstimatedTokens() int64 {
	if h == nil {
		return 0
	}
	return h.estimate
}

func (h *PolicyQuotaReservationHandle) RequestUnits() int64 {
	if h == nil || h.requestUnits <= 0 {
		return 1
	}
	return h.requestUnits
}

func (h *PolicyQuotaReservationHandle) MarkProviderStarted() {
	if h != nil {
		h.started.Store(true)
		h.rejected.Store(false)
	}
}

func (h *PolicyQuotaReservationHandle) MarkProviderRejected() {
	if h != nil {
		h.rejected.Store(true)
	}
}

func (h *PolicyQuotaReservationHandle) ProviderStarted() bool {
	return h != nil && h.started.Load()
}

func (h *PolicyQuotaReservationHandle) ProviderRejected() bool {
	return h != nil && h.rejected.Load()
}

func (h *PolicyQuotaReservationHandle) Preserve() {
	if h != nil {
		h.durable.Store(true)
	}
}

// PreserveIfProviderStarted keeps an async reservation when the provider
// request was sent and its outcome is not a definitive client rejection.
func (h *PolicyQuotaReservationHandle) PreserveIfProviderStarted() {
	if h != nil && h.started.Load() && !h.rejected.Load() {
		h.Preserve()
	}
}

func (h *PolicyQuotaReservationHandle) Durable() bool {
	return h != nil && h.durable.Load()
}

func (h *PolicyQuotaReservationHandle) Finalize(ctx context.Context, actualTokens int64) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.finalized {
		return nil
	}
	if h.released {
		return ErrPolicyQuotaReservationClosed
	}
	if actualTokens == 0 {
		actualTokens = h.estimate
	}
	if actualTokens < 0 {
		return ErrPolicyQuotaTokenUsageUnknown
	}
	if err := h.service.Finalize(ctx, h.id, actualTokens); err != nil {
		return err
	}
	h.finalized = true
	return nil
}

// FinalizeRequestOnly settles a provider request that was sent but explicitly
// rejected before any billable token usage was produced. Request units remain
// consumed while token usage is finalized at zero.
func (h *PolicyQuotaReservationHandle) FinalizeRequestOnly(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.finalized {
		return nil
	}
	if h.released {
		return ErrPolicyQuotaReservationClosed
	}
	if err := h.service.Finalize(ctx, h.id, 0); err != nil {
		return err
	}
	h.finalized = true
	return nil
}

func (h *PolicyQuotaReservationHandle) Release(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.finalized || h.released {
		return nil
	}
	if err := h.service.Release(ctx, h.id); err != nil {
		return err
	}
	h.released = true
	return nil
}

func (h *PolicyQuotaReservationHandle) Finalized() bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.finalized
}

func PolicyQuotaPeriodStarts(now time.Time) (day, month time.Time) {
	utc := now.UTC()
	day = time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	month = time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return day, month
}
