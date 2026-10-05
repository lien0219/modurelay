package service

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	batchImageHoldRequestPrefix    = "batch_image_hold:"
	batchImageCaptureRequestPrefix = "batch_image_capture:"
	batchImageReleaseRequestPrefix = "batch_image_release:"
)

func BatchImageHoldRequestID(batchID string) string {
	return batchImageHoldRequestPrefix + strings.TrimSpace(batchID)
}

func BatchImageCaptureRequestID(batchID string) string {
	return batchImageCaptureRequestPrefix + strings.TrimSpace(batchID)
}

func BatchImageReleaseRequestID(batchID string) string {
	return batchImageReleaseRequestPrefix + strings.TrimSpace(batchID)
}

func buildBatchImageHoldCommand(job *BatchImageJob, requestID string, actualAmount float64, payloadHash string) (*BatchImageBalanceHoldCommand, error) {
	if job == nil {
		return nil, ErrBatchImageBillingHoldFailed
	}
	if job.APIKeyID == nil || *job.APIKeyID <= 0 {
		return nil, ErrBatchImageSettlementMissingAPIKeyID
	}
	holdAmount := job.EstimatedCost
	if job.HoldAmount != nil {
		holdAmount = *job.HoldAmount
	}
	if holdAmount < 0 {
		holdAmount = 0
	}
	if actualAmount < 0 {
		actualAmount = 0
	}
	return &BatchImageBalanceHoldCommand{
		RequestID:              requestID,
		APIKeyID:               *job.APIKeyID,
		UserID:                 batchImageExecutionUserID(job),
		ServiceAccountID:       valueOrZero(job.ServiceAccountID),
		WorkspaceID:            valueOrZero(job.WorkspaceID),
		ProjectID:              valueOrZero(job.ProjectID),
		BillingPrincipalUserID: valueOrZero(job.BillingPrincipalUserID),
		BudgetReservationID:    strings.TrimSpace(batchImageDerefString(job.BudgetReservationID)),
		BatchID:                job.BatchID,
		HoldAmount:             holdAmount,
		ActualAmount:           actualAmount,
		RequestPayloadHash:     strings.TrimSpace(payloadHash),
	}, nil
}

func reserveBatchImageBalanceHold(ctx context.Context, repo UsageBillingRepository, job *BatchImageJob, payloadHash string) error {
	if repo == nil {
		return ErrBatchImageBillingHoldFailed.WithCause(errors.New("batch image billing repository is not configured"))
	}
	cmd, err := buildBatchImageHoldCommand(job, BatchImageHoldRequestID(job.BatchID), 0, payloadHash)
	if err != nil {
		return err
	}
	if cmd.HoldAmount <= 0 && cmd.WorkspaceID == 0 && cmd.ProjectID == 0 {
		return nil
	}
	if _, err := repo.ReserveBatchImageBalance(ctx, cmd); err != nil {
		if errors.Is(err, ErrBatchImageInsufficientBalance) {
			return ErrBatchImageInsufficientBalance
		}
		return ErrBatchImageBillingHoldFailed.WithCause(err)
	}
	return nil
}

func captureBatchImageBalanceHold(ctx context.Context, repo UsageBillingRepository, job *BatchImageJob, actualAmount float64, payloadHash string) (bool, error) {
	if repo == nil {
		return false, ErrBatchImageSettlementBillingFailed.WithCause(errors.New("batch image billing repository is not configured"))
	}
	cmd, err := buildBatchImageHoldCommand(job, BatchImageCaptureRequestID(job.BatchID), actualAmount, payloadHash)
	if err != nil {
		return false, err
	}
	result, err := repo.CaptureBatchImageBalance(ctx, cmd)
	if err != nil {
		return false, ErrBatchImageSettlementBillingFailed.WithCause(err)
	}
	return result != nil && result.UsageLogPersisted, nil
}

func releaseBatchImageBalanceHold(ctx context.Context, repo UsageBillingRepository, job *BatchImageJob, payloadHash string) error {
	if job == nil {
		return nil
	}
	if repo == nil {
		if job.WorkspaceID != nil || job.ProjectID != nil {
			return ErrBatchImageBillingHoldFailed.WithCause(errors.New("batch image billing repository is not configured"))
		}
		return nil
	}
	cmd, err := buildBatchImageHoldCommand(job, BatchImageReleaseRequestID(job.BatchID), 0, payloadHash)
	if err != nil {
		return err
	}
	if cmd.HoldAmount <= 0 && cmd.WorkspaceID == 0 && cmd.ProjectID == 0 {
		return nil
	}
	if _, err := repo.ReleaseBatchImageBalance(ctx, cmd); err != nil {
		// 同一 release request id 出现指纹冲突，说明此前已有一次携带不同
		// payloadHash 的释放成功提交（资金已归还）。视为幂等成功，
		// 避免历史指纹不一致的 job 永远卡在释放失败的毒消息循环里。
		if errors.Is(err, ErrUsageBillingRequestConflict) && cmd.WorkspaceID == 0 && cmd.ProjectID == 0 {
			logger.L().Warn("batch_image.release_fingerprint_conflict_treated_as_released",
				zap.String("batch_id", job.BatchID),
			)
			return nil
		}
		return ErrBatchImageBillingHoldFailed.WithCause(err)
	}
	return nil
}

func batchImageBillingUserID(job *BatchImageJob) int64 {
	if job == nil {
		return 0
	}
	if payerID := valueOrZero(job.BillingPrincipalUserID); payerID > 0 {
		return payerID
	}
	return job.UserID
}

func invalidateBatchImageAuthCache(ctx context.Context, cache APIKeyAuthCacheInvalidator, job *BatchImageJob) {
	if cache == nil || job == nil {
		return
	}
	if payerID := batchImageBillingUserID(job); payerID > 0 {
		cache.InvalidateAuthCacheByUserID(ctx, payerID)
		if job.UserID > 0 && job.UserID != payerID {
			cache.InvalidateAuthCacheByUserID(ctx, job.UserID)
		}
	}
}

func batchImageExecutionUserID(job *BatchImageJob) int64 {
	if job != nil && job.ServiceAccountID != nil {
		return 0
	}
	if job == nil {
		return 0
	}
	return job.UserID
}
