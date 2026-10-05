package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrWorkspaceBudgetExceeded   = errors.New("WORKSPACE_BUDGET_EXCEEDED")
	ErrProjectBudgetExceeded     = errors.New("PROJECT_BUDGET_EXCEEDED")
	ErrBudgetUnpriced            = errors.New("BUDGET_UNPRICED")
	ErrBudgetUnavailable         = errors.New("BUDGET_UNAVAILABLE")
	ErrBudgetReservationConflict = errors.New("BUDGET_RESERVATION_CONFLICT")
	ErrBudgetReservationClosed   = errors.New("BUDGET_RESERVATION_CLOSED")
	ErrBudgetReservationInvalid  = errors.New("BUDGET_RESERVATION_INVALID")
	ErrBudgetCounterCorrupt      = errors.New("BUDGET_COUNTER_CORRUPT")
)

type BudgetAttribution struct {
	ServiceAccountID       int64
	WorkspaceID            int64
	ProjectID              int64
	BillingPrincipalUserID int64
	ActorUserID            int64
	APIKeyID               int64
}

// ValidExecutionAttribution enforces an exclusive human or machine actor.
func ValidExecutionAttribution(userID, serviceAccountID int64) bool {
	return (userID > 0 && serviceAccountID == 0) || (userID == 0 && serviceAccountID > 0)
}

type BudgetReservation struct {
	ServiceAccountID       int64
	ID                     string
	RequestID              string
	ActorUserID            int64
	APIKeyID               int64
	WorkspaceID            int64
	ProjectID              int64
	BillingPrincipalUserID int64
	PeriodStart            time.Time
	PeriodEnd              time.Time
	ProjectPeriodStart     time.Time
	ProjectPeriodEnd       time.Time
	Estimate               float64
	Actual                 float64
	Status                 string
}

type BudgetPolicy struct {
	WorkspaceID int64   `json:"workspace_id"`
	ProjectID   int64   `json:"project_id,omitempty"`
	Amount      float64 `json:"amount"`
	HardLimit   bool    `json:"hard_limit"`
	Enabled     bool    `json:"enabled"`
	Timezone    string  `json:"timezone"`
}

type BudgetRepository interface {
	Reserve(context.Context, BudgetAttribution, string, float64) (*BudgetReservation, error)
	Finalize(context.Context, string, float64) error
	Release(context.Context, string) error
}

type BudgetService struct{ repo BudgetRepository }

func NewBudgetService(repo BudgetRepository) *BudgetService { return &BudgetService{repo: repo} }
func (s *BudgetService) CheckEligibility(_ context.Context, _ BudgetAttribution, estimate float64, priced bool) error {
	if estimate < 0 || math.IsNaN(estimate) || math.IsInf(estimate, 0) || !priced {
		return ErrBudgetUnpriced
	}
	return nil
}
func (s *BudgetService) Reserve(ctx context.Context, a BudgetAttribution, requestID string, estimate float64) (*BudgetReservation, error) {
	if s == nil || s.repo == nil {
		return nil, ErrBudgetUnavailable
	}
	if err := s.CheckEligibility(ctx, a, estimate, true); err != nil {
		return nil, err
	}
	if a.WorkspaceID <= 0 || a.ProjectID <= 0 || a.BillingPrincipalUserID <= 0 || !ValidExecutionAttribution(a.ActorUserID, a.ServiceAccountID) || a.APIKeyID <= 0 || strings.TrimSpace(requestID) == "" {
		return nil, ErrBudgetReservationInvalid
	}
	return s.repo.Reserve(ctx, a, requestID, estimate)
}
func (s *BudgetService) Finalize(ctx context.Context, id string, actual float64) error {
	if id == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return ErrBudgetUnavailable
	}
	return s.repo.Finalize(ctx, id, actual)
}
func (s *BudgetService) Release(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return ErrBudgetUnavailable
	}
	return s.repo.Release(ctx, id)
}

type budgetReservationContextKey struct{}
type budgetServiceContextKey struct{}

func WithBudgetService(ctx context.Context, s *BudgetService) context.Context {
	return context.WithValue(ctx, budgetServiceContextKey{}, s)
}
func BudgetServiceFromContext(ctx context.Context) *BudgetService {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(budgetServiceContextKey{}).(*BudgetService)
	return s
}

// Admit binds a reservation to the authenticated key. No request-body tenant
// fields enter this boundary, and a retained key from another turn is untouched.
func (s *BudgetService) Admit(ctx context.Context, key *APIKey, requestID string, estimate float64, priced bool) (*BudgetReservationHandle, error) {
	if key == nil || key.Tenant == nil {
		return nil, nil
	}
	t := key.Tenant
	a := BudgetAttribution{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID, BillingPrincipalUserID: t.BillingPrincipalUserID, ActorUserID: key.UserID, APIKeyID: key.ID, ServiceAccountID: valueOrZero(key.ServiceAccountID)}
	if err := s.CheckEligibility(ctx, a, estimate, priced); err != nil {
		return nil, err
	}
	r, err := s.Reserve(ctx, a, requestID, estimate)
	if err != nil {
		return nil, err
	}
	if r == nil || strings.TrimSpace(r.ID) == "" {
		return nil, ErrBudgetUnavailable
	}
	if r.Status != "pending" {
		return nil, ErrBudgetReservationClosed
	}
	if r.WorkspaceID != a.WorkspaceID || r.ProjectID != a.ProjectID || r.BillingPrincipalUserID != a.BillingPrincipalUserID || r.ActorUserID != a.ActorUserID || r.ServiceAccountID != a.ServiceAccountID || r.APIKeyID != a.APIKeyID || QuantizeUsageBillingAmount(r.Estimate) != QuantizeUsageBillingAmount(estimate) {
		return nil, ErrBudgetReservationConflict
	}
	return NewBudgetReservationHandle(s, r.ID), nil
}

// BudgetReservationHandle ties request admission to the durable reservation.
// Billing marks it settled after the SQL transaction commits; request cleanup
// releases it only when no billing task took ownership.
type BudgetReservationHandle struct {
	service          *BudgetService
	id               string
	settled          atomic.Bool
	durable          atomic.Bool
	providerStarted  atomic.Bool
	providerRejected atomic.Bool
	refs             atomic.Int64
	handlerOnce      sync.Once
	releaseOnce      sync.Once
}

func NewBudgetReservationHandle(s *BudgetService, id string) *BudgetReservationHandle {
	if s == nil || id == "" {
		return nil
	}
	h := &BudgetReservationHandle{service: s, id: id}
	h.refs.Store(1)
	return h
}

func (h *BudgetReservationHandle) ID() string {
	if h == nil {
		return ""
	}
	return h.id
}

func (h *BudgetReservationHandle) MarkSettled() {
	if h != nil {
		h.settled.Store(true)
	}
}

func (h *BudgetReservationHandle) Release(ctx context.Context) error {
	if h == nil {
		return nil
	}
	var err error
	h.handlerOnce.Do(func() { err = h.dropReference(ctx) })
	return err
}

// Preserve hands an accepted asynchronous task, or an uncertain settlement,
// to durable recovery. Only a proven terminal failure may release it later.
func (h *BudgetReservationHandle) Preserve() {
	if h != nil {
		h.durable.Store(true)
	}
}

func (h *BudgetReservationHandle) MarkProviderStarted() {
	if h != nil {
		h.providerStarted.Store(true)
		h.providerRejected.Store(false)
	}
}

// MarkProviderRejected records a definitive provider-side rejection for the
// current attempt. The request was sent, but no durable async task was
// accepted, so normal handler cleanup may release the reservation.
func (h *BudgetReservationHandle) MarkProviderRejected() {
	if h != nil {
		h.providerRejected.Store(true)
	}
}

// ProviderStarted reports whether an async provider request was accepted or
// its outcome became uncertain after the request was sent.
func (h *BudgetReservationHandle) ProviderStarted() bool {
	return h != nil && h.providerStarted.Load()
}

// ProviderRejected reports that the latest provider attempt returned a
// definitive client-side rejection (4xx), so the reservation remains
// releasable by request cleanup.
func (h *BudgetReservationHandle) ProviderRejected() bool {
	return h != nil && h.providerRejected.Load()
}

func (h *BudgetReservationHandle) PreserveIfProviderStarted() {
	if h != nil && h.providerStarted.Load() && !h.providerRejected.Load() {
		h.Preserve()
	}
}

func WithoutBudgetReservation(ctx context.Context) context.Context {
	return context.WithValue(ctx, budgetReservationContextKey{}, (*BudgetReservationHandle)(nil))
}

func (h *BudgetReservationHandle) Acquire() func() {
	if h == nil {
		return func() {}
	}
	for {
		n := h.refs.Load()
		if n <= 0 {
			return func() {}
		}
		if h.refs.CompareAndSwap(n, n+1) {
			break
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = h.dropReference(ctx)
		})
	}
}
func (h *BudgetReservationHandle) dropReference(ctx context.Context) error {
	if h.refs.Add(-1) > 0 || h.settled.Load() || h.durable.Load() {
		return nil
	}
	var err error
	h.releaseOnce.Do(func() { err = h.service.Release(ctx, h.id) })
	return err
}

func WithBudgetReservation(ctx context.Context, h *BudgetReservationHandle) context.Context {
	if h == nil {
		return ctx
	}
	return context.WithValue(ctx, budgetReservationContextKey{}, h)
}

func BudgetReservationFromContext(ctx context.Context) *BudgetReservationHandle {
	if ctx == nil {
		return nil
	}
	h, _ := ctx.Value(budgetReservationContextKey{}).(*BudgetReservationHandle)
	return h
}

// BudgetReservationIDFromContext returns the immutable reservation selected
// for the current request. Admission must never write this value into the
// shared API-key authentication snapshot because that snapshot may be used by
// concurrent requests.
func BudgetReservationIDFromContext(ctx context.Context) string {
	if h := BudgetReservationFromContext(ctx); h != nil {
		return strings.TrimSpace(h.ID())
	}
	return ""
}
