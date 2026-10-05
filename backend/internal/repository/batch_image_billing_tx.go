package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func batchImageHoldPayerID(cmd *service.BatchImageBalanceHoldCommand) int64 {
	if cmd.BillingPrincipalUserID > 0 {
		return cmd.BillingPrincipalUserID
	}
	return cmd.UserID
}

// The job is the frozen attribution authority. Never consult current key,
// membership or workspace billing ownership when completing admitted work.
func validateBatchImageHoldSnapshotTx(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand, operation string) (*service.BatchImageJob, *service.UsageBillingCommand, error) {
	cmd.BatchID = strings.TrimSpace(cmd.BatchID)
	cmd.RequestID = strings.TrimSpace(cmd.RequestID)
	if !validBudgetAmount(cmd.HoldAmount) || !validBudgetAmount(cmd.ActualAmount) {
		return nil, nil, service.ErrBudgetReservationInvalid
	}
	expectedRequestID := ""
	switch operation {
	case "reserve":
		expectedRequestID = service.BatchImageHoldRequestID(cmd.BatchID)
	case "capture":
		expectedRequestID = service.BatchImageCaptureRequestID(cmd.BatchID)
	case "release":
		expectedRequestID = service.BatchImageReleaseRequestID(cmd.BatchID)
	}
	if cmd.BatchID == "" || cmd.RequestID != expectedRequestID {
		return nil, nil, service.ErrUsageBillingRequestConflict
	}
	job, err := scanBatchImageJob(tx.QueryRowContext(ctx, batchImageJobSelectSQL+` WHERE batch_id=$1 FOR UPDATE`, cmd.BatchID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, service.ErrBatchImageJobNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	hold := job.EstimatedCost
	if job.HoldAmount != nil {
		hold = *job.HoldAmount
	}
	if job.UserID != cmd.UserID || batchImageSnapshotID(job.APIKeyID) != cmd.APIKeyID || batchImageSnapshotID(job.WorkspaceID) != cmd.WorkspaceID || batchImageSnapshotID(job.ProjectID) != cmd.ProjectID || batchImageSnapshotID(job.BillingPrincipalUserID) != cmd.BillingPrincipalUserID || service.QuantizeUsageBillingAmount(hold) != service.QuantizeUsageBillingAmount(cmd.HoldAmount) {
		return nil, nil, service.ErrBudgetReservationConflict
	}
	if operation == "release" && job.ProviderCreateStartedAt != nil && (job.ProviderJobName == nil || strings.TrimSpace(*job.ProviderJobName) == "") && (job.Status == service.BatchImageJobStatusUploading || job.Status == service.BatchImageJobStatusCreated || (job.LastErrorCode != nil && *job.LastErrorCode == "SUBMIT_OUTCOME_UNKNOWN")) {
		return nil, nil, service.ErrBatchImageProviderSubmitUncertain
	}
	if cmd.WorkspaceID == 0 && cmd.ProjectID == 0 && cmd.BillingPrincipalUserID == 0 {
		if strings.TrimSpace(cmd.BudgetReservationID) != "" {
			return nil, nil, service.ErrBudgetReservationConflict
		}
		return job, nil, nil
	}
	if cmd.WorkspaceID <= 0 || cmd.ProjectID <= 0 || cmd.BillingPrincipalUserID <= 0 {
		return nil, nil, service.ErrBudgetReservationInvalid
	}
	// Lock wallet rows in ID order before reservation locks. NO KEY UPDATE is
	// compatible with the usage row's foreign-key checks for the actor.
	if _, err := tx.ExecContext(ctx, `SELECT id FROM users WHERE id IN ($1,$2) ORDER BY id FOR NO KEY UPDATE`, cmd.UserID, cmd.BillingPrincipalUserID); err != nil {
		return nil, nil, err
	}
	reservationID := ""
	if job.BudgetReservationID != nil {
		reservationID = strings.TrimSpace(*job.BudgetReservationID)
	}
	if operation == "release" && reservationID == "" {
		// Reserve can commit immediately before a process dies without saving
		// its returned ID. The hold request ID plus the complete frozen owner
		// safely recovers that reservation without relying on live membership.
		err = tx.QueryRowContext(ctx, `SELECT id::text FROM budget_reservations
			WHERE request_id=$1 AND api_key_id=$2 AND actor_user_id=$3
				AND workspace_id=$4 AND project_id=$5 AND billing_principal_user_id=$6`,
			service.BatchImageHoldRequestID(job.BatchID), cmd.APIKeyID, cmd.UserID, cmd.WorkspaceID, cmd.ProjectID, cmd.BillingPrincipalUserID).Scan(&reservationID)
		if errors.Is(err, sql.ErrNoRows) {
			if strings.TrimSpace(cmd.BudgetReservationID) != "" {
				return nil, nil, service.ErrBudgetReservationConflict
			}
			return job, nil, nil
		}
		if err != nil {
			return nil, nil, err
		}
	}
	if reservationID == "" || (strings.TrimSpace(cmd.BudgetReservationID) != "" && strings.TrimSpace(cmd.BudgetReservationID) != reservationID) {
		return nil, nil, service.ErrBudgetReservationConflict
	}
	if operation != "release" && strings.TrimSpace(cmd.BudgetReservationID) != reservationID {
		return nil, nil, service.ErrBudgetReservationConflict
	}
	cmd.BudgetReservationID = reservationID
	budgetCmd := &service.UsageBillingCommand{
		RequestID: cmd.RequestID, UserID: cmd.UserID, APIKeyID: cmd.APIKeyID,
		WorkspaceID: cmd.WorkspaceID, ProjectID: cmd.ProjectID,
		BillingPrincipalUserID: cmd.BillingPrincipalUserID, BudgetReservationID: reservationID,
		BudgetActualCost: service.QuantizeUsageBillingAmount(cmd.ActualAmount), ResolvedPlatform: service.PlatformGemini,
	}
	return job, budgetCmd, nil
}

func validateBatchImageHoldOperationTx(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand, budgetCmd *service.UsageBillingCommand, operation string) error {
	if operation == "capture" || operation == "release" {
		oppositeID := service.BatchImageCaptureRequestID(cmd.BatchID)
		if operation == "capture" {
			oppositeID = service.BatchImageReleaseRequestID(cmd.BatchID)
		}
		closed, err := batchImageHoldClaimExists(ctx, tx, oppositeID, cmd.APIKeyID)
		if err != nil {
			return err
		}
		if closed {
			return service.ErrBudgetReservationClosed
		}
	}
	if budgetCmd != nil {
		reservation, err := tenantBudgetReservationTx(ctx, tx, budgetCmd)
		if err != nil {
			return err
		}
		if reservation.RequestID != service.BatchImageHoldRequestID(cmd.BatchID) {
			return service.ErrBudgetReservationConflict
		}
		if reservation.Status != "pending" {
			return service.ErrBudgetReservationClosed
		}
	}
	if operation == "capture" || (operation == "release" && cmd.WorkspaceID > 0 && budgetCmd == nil) {
		held, err := batchImageHoldClaimExists(ctx, tx, service.BatchImageHoldRequestID(cmd.BatchID), cmd.APIKeyID)
		if err != nil {
			return err
		}
		if !held && operation == "capture" && (cmd.HoldAmount > 0 || budgetCmd != nil) {
			return service.ErrBatchImageBillingHoldFailed
		}
		if held && operation == "release" && budgetCmd == nil {
			return service.ErrBudgetReservationInvalid
		}
	}
	return nil
}

func batchImageSnapshotID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

func persistBatchImageSummaryTx(ctx context.Context, tx *sql.Tx, job *service.BatchImageJob, cmd *service.BatchImageBalanceHoldCommand) error {
	if job.APIKeyID == nil || job.AccountID == nil || *job.AccountID <= 0 {
		return service.ErrBatchImageSettlementMissingAccountID
	}
	if job.SuccessCount < 0 || job.FailCount < 0 || job.SuccessCount+job.FailCount > job.ItemCount {
		return service.ErrBatchImageSettlementInvalidCounts
	}
	if job.PricingSnapshotVersion >= 1 && service.QuantizeUsageBillingAmount(float64(job.SuccessCount)*job.BillableUnitPrice) != cmd.ActualAmount {
		return service.ErrBudgetReservationConflict
	}
	inbound, upstream, platform, mode, size := "/v1/images/batches", "vertex:batchPredictionJobs", service.PlatformGemini, string(service.BillingModeImage), "1K"
	if job.Provider == service.BatchImageProviderGeminiAPI {
		upstream = "/v1beta/models/" + job.Model + ":batchGenerateContent"
	}
	reservationID := cmd.BudgetReservationID
	log := &service.UsageLog{
		UserID: job.UserID, APIKeyID: *job.APIKeyID, AccountID: *job.AccountID,
		WorkspaceID: job.WorkspaceID, ProjectID: job.ProjectID, BillingPrincipalUserID: job.BillingPrincipalUserID,
		BudgetReservationID: &reservationID, ResolvedPlatform: &platform,
		RequestID: cmd.RequestID, Model: job.Model, RequestedModel: job.Model,
		InboundEndpoint: &inbound, UpstreamEndpoint: &upstream,
		ImageCount: job.SuccessCount, ImageOutputCost: cmd.ActualAmount, TotalCost: cmd.ActualAmount, ActualCost: cmd.ActualAmount,
		RateMultiplier: job.GroupRateMultiplier * job.BatchDiscountMultiplier, AccountRateMultiplier: &job.AccountRateMultiplier,
		BillingType: service.BillingTypeBalance, RequestType: service.RequestTypeSync, BillingMode: &mode, ImageSize: &size,
		SessionID: job.SessionID, CreatedAt: time.Now(),
	}
	query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(log)}, `ON CONFLICT (request_id, api_key_id) DO NOTHING`)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	var owned bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM usage_logs
		WHERE request_id=$1 AND api_key_id=$2 AND user_id=$3 AND account_id=$4
			AND workspace_id=$5 AND project_id=$6 AND billing_principal_user_id=$7
			AND budget_reservation_id=$8 AND image_count=$9 AND actual_cost=$10::numeric AND model=$11)`,
		cmd.RequestID, cmd.APIKeyID, cmd.UserID, *job.AccountID, cmd.WorkspaceID, cmd.ProjectID,
		cmd.BillingPrincipalUserID, cmd.BudgetReservationID, job.SuccessCount, cmd.ActualAmount, job.Model).Scan(&owned)
	if err != nil {
		return err
	}
	if !owned {
		return service.ErrBudgetReservationConflict
	}
	return nil
}
