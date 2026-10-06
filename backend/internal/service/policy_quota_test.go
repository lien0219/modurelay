package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type policyQuotaRepositoryFake struct {
	mu           sync.Mutex
	request      PolicyQuotaReservationRequest
	reservation  *PolicyQuotaReservation
	finalize     []int64
	releases     int
	finalizeErrs []error
	releaseErrs  []error
}

func (r *policyQuotaRepositoryFake) Reserve(_ context.Context, request PolicyQuotaReservationRequest) (*PolicyQuotaReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.request = request
	if r.reservation == nil {
		r.reservation = &PolicyQuotaReservation{ID: "quota-reservation-1", RequestID: request.RequestID, EstimatedTokens: request.EstimatedTokens, Status: PolicyQuotaReservationPending}
	}
	copy := *r.reservation
	return &copy, nil
}

func (r *policyQuotaRepositoryFake) Finalize(_ context.Context, _ string, actualTokens int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.finalizeErrs) > 0 {
		err := r.finalizeErrs[0]
		r.finalizeErrs = r.finalizeErrs[1:]
		if err != nil {
			return err
		}
	}
	r.finalize = append(r.finalize, actualTokens)
	return nil
}

func (r *policyQuotaRepositoryFake) Release(_ context.Context, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.releaseErrs) > 0 {
		err := r.releaseErrs[0]
		r.releaseErrs = r.releaseErrs[1:]
		if err != nil {
			return err
		}
	}
	r.releases++
	return nil
}

func int64ptr(v int64) *int64 { return &v }

func TestPolicyQuotaScopesRetainEachConfiguredRevision(t *testing.T) {
	workspace := &domain.Policy{Scope: domain.PolicyScopeWorkspace, ScopeID: 10, Revision: 3, DailyRequestLimit: int64ptr(100), DailyTokenLimit: int64ptr(1000)}
	project := &domain.Policy{Scope: domain.PolicyScopeProject, ScopeID: 20, Revision: 7, MonthlyRequestLimit: int64ptr(20), MonthlyTokenLimit: int64ptr(500)}
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: workspace, Project: project})
	require.NoError(t, err)

	scopes := PolicyQuotaScopes(policy)
	require.Len(t, scopes, 2)
	require.Equal(t, domain.PolicyScopeWorkspace, scopes[0].Scope)
	require.Equal(t, int64(3), scopes[0].Revision)
	require.Equal(t, int64(100), *scopes[0].DailyRequestLimit)
	require.Equal(t, domain.PolicyScopeProject, scopes[1].Scope)
	require.Equal(t, int64(7), scopes[1].Revision)
	require.Equal(t, int64(500), *scopes[1].MonthlyTokenLimit)
}

func TestPolicyQuotaServiceAdmitIsIdempotentAndPreservesAttribution(t *testing.T) {
	repo := &policyQuotaRepositoryFake{}
	svc := NewPolicyQuotaService(repo)
	workspace := &domain.Policy{Scope: domain.PolicyScopeWorkspace, ScopeID: 10, Revision: 3, DailyRequestLimit: int64ptr(100)}
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: workspace})
	require.NoError(t, err)
	attr := PolicyQuotaAttribution{PolicyContext: domain.PolicyContext{WorkspaceID: 10, ProjectID: 20, ServiceAccountID: 30}, APIKeyID: 40, RequestUnits: 3}

	h, err := svc.Admit(context.Background(), policy, attr, "request-1", 123)
	require.NoError(t, err)
	require.NotNil(t, h)
	require.Equal(t, "quota-reservation-1", h.ID())
	require.Equal(t, "request-1", repo.request.RequestID)
	require.Equal(t, int64(123), repo.request.EstimatedTokens)
	require.Equal(t, int64(10), repo.request.Scopes[0].ID)
	require.Equal(t, int64(40), repo.request.APIKeyID)
	require.Equal(t, int64(3), repo.request.RequestUnits)

	second, err := svc.Admit(context.Background(), policy, attr, "request-1", 123)
	require.NoError(t, err)
	require.Equal(t, h.ID(), second.ID())
}

func TestPolicyQuotaHandleFinalizesZeroWithEstimateExactlyOnce(t *testing.T) {
	repo := &policyQuotaRepositoryFake{reservation: &PolicyQuotaReservation{ID: "r-1", EstimatedTokens: 321, Status: PolicyQuotaReservationPending}}
	svc := NewPolicyQuotaService(repo)
	h := NewPolicyQuotaReservationHandle(svc, "r-1", 321)
	require.NoError(t, h.Finalize(context.Background(), 0))
	require.NoError(t, h.Finalize(context.Background(), 999))
	require.NoError(t, h.Release(context.Background()))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, []int64{321}, repo.finalize)
	require.Zero(t, repo.releases)
}

func TestPolicyQuotaHandleFinalizesProviderRejectionWithoutTokenUsage(t *testing.T) {
	repo := &policyQuotaRepositoryFake{reservation: &PolicyQuotaReservation{ID: "r-rejected", Status: PolicyQuotaReservationPending}}
	svc := NewPolicyQuotaService(repo)
	h := NewPolicyQuotaReservationHandle(svc, "r-rejected", 321)
	h.MarkProviderStarted()
	h.MarkProviderRejected()

	require.NoError(t, h.FinalizeRequestOnly(context.Background()))
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, []int64{0}, repo.finalize)
	require.Zero(t, repo.releases)
}

func TestPolicyQuotaHandleReleasesWhenProviderNeverStarts(t *testing.T) {
	repo := &policyQuotaRepositoryFake{}
	svc := NewPolicyQuotaService(repo)
	h := NewPolicyQuotaReservationHandle(svc, "r-2", 10)
	require.NoError(t, h.Release(context.Background()))
	require.NoError(t, h.Release(context.Background()))
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.releases)
}

func TestPolicyQuotaServiceRetriesTransientFinalizeFailure(t *testing.T) {
	repo := &policyQuotaRepositoryFake{finalizeErrs: []error{errors.New("temporary database failure"), nil}}
	svc := NewPolicyQuotaService(repo)

	require.NoError(t, svc.Finalize(context.Background(), "r-retry", 17))
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, []int64{17}, repo.finalize)
}

func TestPolicyQuotaServiceRejectsInvalidAdmission(t *testing.T) {
	svc := NewPolicyQuotaService(&policyQuotaRepositoryFake{})
	_, err := svc.Admit(context.Background(), domain.EffectivePolicy{}, PolicyQuotaAttribution{}, "", 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPolicyQuotaReservationInvalid))
}

func TestPolicyQuotaPeriodStartsUseUTC(t *testing.T) {
	day, month := PolicyQuotaPeriodStarts(time.Date(2026, 10, 6, 23, 59, 59, 0, time.FixedZone("CST", 8*60*60)))
	require.Equal(t, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), day)
	require.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), month)
}
