package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Settlement alerts describe an accepted tenant obligation, not an upstream
// account. Their dedup state outlives event retention and survives worker replay.
func settlementReason(err error) string {
	switch {
	case errors.Is(err, service.ErrProjectBudgetExceeded):
		return "project_budget_exceeded"
	case errors.Is(err, service.ErrWorkspaceBudgetExceeded):
		return "workspace_budget_exceeded"
	case errors.Is(err, service.ErrInsufficientBalance):
		return "insufficient_balance"
	default:
		return "billing_write_pending"
	}
}

func (r *usageBillingRepository) recordPendingVideoSettlement(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog, cause error) error {
	if r == nil || r.db == nil {
		return service.ErrBudgetUnavailable
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tenantBudgetReservationTx(ctx, tx, cmd)
	if err != nil {
		return err
	}
	if res.Status != "pending" {
		// Another worker may already have completed or cancelled this work.
		return tx.Commit()
	}
	reason := settlementReason(cause)
	data := service.DomainEventData{
		"request_id": cmd.RequestID, "task_id": strings.TrimPrefix(cmd.RequestID, "grok-video:"),
		"model": log.Model, "platform": cmd.ResolvedPlatform,
		"estimated_amount": res.Estimate, "actual_amount": cmd.BudgetActualCost, "reason_code": reason,
	}
	event, err := service.NewDomainEvent(service.EventBillingPending, res.WorkspaceID, res.ProjectID, 0, "billing_settlement", res.ID, data)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO billing_settlement_alerts(reservation_id,request_id,api_key_id,data) VALUES($1,$2,$3,$4) ON CONFLICT(reservation_id) DO NOTHING`, res.ID, cmd.RequestID, res.APIKeyID, payload)
	if err != nil {
		return err
	}
	created, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if created == 0 {
		return tx.Commit()
	}
	if err = appendWorkspaceAudit(ctx, tx, res.WorkspaceID, res.ActorUserID, &res.ProjectID, service.EventBillingPending, "api_key", res.APIKeyID, map[string]any{"request_id": cmd.RequestID, "reservation_id": res.ID, "reason_code": reason}); err != nil {
		return err
	}
	if err = insertDomainEventTx(ctx, tx, event, "billing.pending:"+res.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// This runs after wallet/quota, usage and budget finalization on their original
// transaction. A failed commit removes the recovered state and event together.
func recordRecoveredVideoSettlementTx(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	var payload []byte
	err := tx.QueryRowContext(ctx, `UPDATE billing_settlement_alerts SET state='recovered',recovered_at=now() WHERE reservation_id=$1 AND request_id=$2 AND api_key_id=$3 AND state='pending' RETURNING data`, cmd.BudgetReservationID, cmd.RequestID, cmd.APIKeyID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var data service.DomainEventData
	if err = json.Unmarshal(payload, &data); err != nil {
		return err
	}
	data["reason_code"] = "settled"
	event, err := service.NewDomainEvent(service.EventBillingRecovered, cmd.WorkspaceID, cmd.ProjectID, 0, "billing_settlement", cmd.BudgetReservationID, data)
	if err != nil {
		return err
	}
	if err = appendWorkspaceAudit(ctx, tx, cmd.WorkspaceID, cmd.UserID, &cmd.ProjectID, service.EventBillingRecovered, "api_key", cmd.APIKeyID, map[string]any{"request_id": cmd.RequestID, "reservation_id": cmd.BudgetReservationID}); err != nil {
		return err
	}
	return insertDomainEventTx(ctx, tx, event, "billing.recovered:"+cmd.BudgetReservationID)
}
