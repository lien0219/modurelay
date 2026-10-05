package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func tenantBudgetReservationTx(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (*service.BudgetReservation, error) {
	if cmd == nil || cmd.WorkspaceID <= 0 || cmd.ProjectID <= 0 || cmd.BillingPrincipalUserID <= 0 || cmd.UserID <= 0 || cmd.APIKeyID <= 0 {
		return nil, service.ErrBudgetReservationInvalid
	}
	id := strings.TrimSpace(cmd.BudgetReservationID)
	if _, err := uuid.Parse(id); err != nil {
		return nil, service.ErrBudgetReservationInvalid
	}
	res, err := loadBudgetReservationForTransition(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if !budgetAttributionMatches(res, service.BudgetAttribution{ActorUserID: cmd.UserID, APIKeyID: cmd.APIKeyID, WorkspaceID: cmd.WorkspaceID, ProjectID: cmd.ProjectID, BillingPrincipalUserID: cmd.BillingPrincipalUserID}) {
		return nil, service.ErrBudgetReservationConflict
	}
	return res, nil
}

// releaseTenantBudgetTx runs inside the wallet hold's release transaction.
// Request cleanup uses BudgetRepository.Release's harmless terminal no-op;
// financial releases require pending state so they cannot refund settled work.
func releaseTenantBudgetTx(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	if cmd == nil || (cmd.WorkspaceID == 0 && cmd.ProjectID == 0 && cmd.BillingPrincipalUserID == 0 && strings.TrimSpace(cmd.BudgetReservationID) == "") {
		return nil
	}
	res, err := tenantBudgetReservationTx(ctx, tx, cmd)
	if err != nil {
		return err
	}
	return settleBudgetReservationTx(ctx, tx, res, 0, "released", false)
}
