package service

import (
	"context"
	"errors"
)

// Frozen accounting does not need a live credential, account, membership or
// price lookup. These identity-only objects carry the admitted snapshot.
func (s *OpenAIGatewayService) replayFrozenVideoSettlement(ctx context.Context, pending *GrokVideoPendingBilling) (bool, error) {
	settlement := pending.Settlement
	if settlement == nil || settlement.UsageLog == nil {
		return false, errors.New("frozen video usage is missing")
	}
	log := settlement.UsageLog
	if log.APIKeyID != pending.APIKeyID || log.AccountID != pending.AccountID || log.UserID != pending.UserID || valueOrZero(log.ServiceAccountID) != pending.ServiceAccountID ||
		log.RequestID != StableGrokVideoBillingRequestID(pending.RequestID) {
		return false, errors.New("frozen video ownership mismatch")
	}
	claimed, err := s.ClaimGrokVideoBilling(ctx, pending.RequestID, pending.OwnershipID(), pending.APIKeyID)
	if err != nil || !claimed {
		return false, err
	}
	key := &APIKey{ID: pending.APIKeyID, UserID: pending.UserID, ServiceAccountID: log.ServiceAccountID, GroupID: log.GroupID,
		BillingPrincipal: &User{ID: pending.BillingUserID()}, Tenant: &TenantContext{WorkspaceID: pending.WorkspaceID, ProjectID: pending.ProjectID, BillingPrincipalUserID: pending.BillingPrincipalUserID, BudgetReservationID: pending.BudgetReservationID}}
	account := &Account{ID: pending.AccountID}
	if settlement.Command != nil {
		account.Type = settlement.Command.AccountType
	}
	input := &OpenAIRecordUsageInput{User: &User{ID: pending.UserID}, APIKey: key, Account: account, VideoTaskID: pending.RequestID,
		Result: &OpenAIForwardResult{RequestID: pending.RequestID, ResponseID: pending.RequestID, VideoCount: log.VideoCount}}
	if err := s.recordVideoUsageSettlement(ctx, input, settlement); err != nil {
		_ = s.ReleaseGrokVideoBilling(ctx, pending.RequestID, pending.OwnershipID(), pending.APIKeyID)
		return false, err
	}
	return true, nil
}
