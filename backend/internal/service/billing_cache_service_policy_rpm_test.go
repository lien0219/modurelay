package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type policyRPMCacheStub struct {
	counters []RPMCounter
	result   RPMAdmissionResult
	err      error
	calls    int
}

func (s *policyRPMCacheStub) AdmitMultiScopeRPM(_ context.Context, counters []RPMCounter) (RPMAdmissionResult, error) {
	s.calls++
	s.counters = append([]RPMCounter(nil), counters...)
	return s.result, s.err
}

func (*policyRPMCacheStub) IncrementUserGroupRPM(context.Context, int64, int64) (int, error) {
	return 0, nil
}
func (*policyRPMCacheStub) IncrementUserRPM(context.Context, int64) (int, error) { return 0, nil }
func (*policyRPMCacheStub) GetUserGroupRPM(context.Context, int64, int64) (int, error) {
	return 0, nil
}
func (*policyRPMCacheStub) GetUserRPM(context.Context, int64) (int, error) { return 0, nil }

func TestCheckRPMPolicyLimitsUseOneAtomicMultiScopeAdmission(t *testing.T) {
	cache := &policyRPMCacheStub{result: RPMAdmissionResult{Allowed: true}}
	svc := NewBillingCacheService(nil, nil, nil, nil, cache, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	ctx := WithEffectivePolicy(context.Background(), domain.EffectivePolicy{Layers: domain.PolicyLayers{
		Workspace:      &domain.Policy{Scope: domain.PolicyScopeWorkspace, ScopeID: 11, RPMLimit: int64Pointer(100)},
		Project:        &domain.Policy{Scope: domain.PolicyScopeProject, ScopeID: 22, RPMLimit: int64Pointer(50)},
		ServiceAccount: &domain.Policy{Scope: domain.PolicyScopeServiceAccount, ScopeID: 33, RPMLimit: int64Pointer(20)},
	}})
	key := &APIKey{ID: 44, Tenant: &TenantContext{WorkspaceID: 11, ProjectID: 22}, ServiceAccountID: int64Pointer(33)}
	user := &User{ID: 55, RPMLimit: 200}
	group := &Group{ID: 66, RPMLimit: 150}

	require.NoError(t, svc.checkRPMForAPIKey(ctx, user, group, key))
	require.Equal(t, 1, cache.calls)
	require.Equal(t, []RPMCounter{
		{Key: "policy:workspace:11", Scope: "workspace", Limit: 100},
		{Key: "policy:project:22", Scope: "project", Limit: 50},
		{Key: "policy:service_account:33", Scope: "service_account", Limit: 20},
		{Key: "ug:55:66", Scope: "group", Limit: 150},
		{Key: "u:55", Scope: "user", Limit: 200},
	}, cache.counters)
}

func TestCheckRPMPolicyDenialReportsScopeAndFailsClosedOnCacheError(t *testing.T) {
	cache := &policyRPMCacheStub{result: RPMAdmissionResult{Allowed: false, Scope: "project", Limit: 7, Count: 8}}
	svc := NewBillingCacheService(nil, nil, nil, nil, cache, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	ctx := WithEffectivePolicy(context.Background(), domain.EffectivePolicy{Layers: domain.PolicyLayers{
		Project: &domain.Policy{Scope: domain.PolicyScopeProject, ScopeID: 22, RPMLimit: int64Pointer(7)},
	}})

	err := svc.checkRPMForAPIKey(ctx, &User{ID: 55}, nil, &APIKey{ID: 44, Tenant: &TenantContext{ProjectID: 22}})
	require.ErrorIs(t, err, ErrPolicyRPMExceeded)
	require.ErrorContains(t, err, "project")

	cache.err = errors.New("redis unavailable")
	err = svc.checkRPMForAPIKey(ctx, &User{ID: 55}, nil, &APIKey{ID: 44, Tenant: &TenantContext{ProjectID: 22}})
	require.ErrorIs(t, err, ErrBillingServiceUnavailable)
}

func int64Pointer(value int64) *int64 { return &value }
