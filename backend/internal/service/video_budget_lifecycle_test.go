//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGrokVideoPendingBillingTenantRejectsPartialSnapshot(t *testing.T) {
	tests := []struct {
		name    string
		pending GrokVideoPendingBilling
	}{
		{
			name:    "workspace without project",
			pending: GrokVideoPendingBilling{WorkspaceID: 1, BillingPrincipalUserID: 3, BudgetReservationID: "reservation"},
		},
		{
			name:    "tenant without reservation",
			pending: GrokVideoPendingBilling{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3},
		},
		{
			name:    "reservation without tenant",
			pending: GrokVideoPendingBilling{BudgetReservationID: "reservation"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, validateGrokVideoPendingBillingTenant(&tt.pending), ErrBudgetReservationInvalid)
		})
	}
}

func TestValidateGrokVideoPendingBillingTenantAllowsLegacyUnscopedSnapshot(t *testing.T) {
	require.NoError(t, validateGrokVideoPendingBillingTenant(&GrokVideoPendingBilling{}))
}

func TestVideoRecoveryReleasesReservationForTerminalFailure(t *testing.T) {
	gateway, cache, _, _, _, pending := videoRecoveryEdgeFixture(t)
	pending.WorkspaceID = 1
	pending.ProjectID = 2
	pending.BillingPrincipalUserID = 10
	pending.BudgetReservationID = "reservation-terminal-failure"
	payload, err := json.Marshal(pending)
	require.NoError(t, err)
	cache.payload = payload
	budgetRepo := &budgetRepoFake{}
	keys := &APIKeyService{}
	keys.SetBudgetService(NewBudgetService(budgetRepo))
	gateway.SetVideoRecoveryAPIKeyService(keys)
	gateway.httpUpstream = &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"status":"failed"}`)}

	terminal, err := gateway.recoverVideoBillingKey(context.Background(), cache.key)
	require.NoError(t, err)
	require.True(t, terminal)
	require.Equal(t, 1, budgetRepo.released)
}
