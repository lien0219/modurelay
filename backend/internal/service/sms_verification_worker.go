package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	smsVerificationPollInterval   = 5 * time.Second
	smsVerificationUnknownTimeout = 15 * time.Minute
)

// Reconcile polls active SMS orders and repairs purchases that were left in an
// unknown state by a process or network interruption. It is called by the
// existing leader-elected verification worker.
func (s *SMSService) Reconcile(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT o.id,o.user_id,o.status,o.product_type,o.provider_order_id,o.refund_status,p.code,p.base_url,p.credential_ref,o.expires_at,o.updated_at,o.settlement_status,o.reconciliation_action FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE (o.status IN ('active','provider_unknown') AND (((o.reconcile_after IS NULL AND o.updated_at <= NOW()-($1 * INTERVAL '1 second')) OR o.reconcile_after <= NOW()) OR o.expires_at <= NOW())) OR (o.status='reconciling' AND (o.reconcile_after IS NULL OR o.reconcile_after <= NOW())) OR (o.status IN ('failed','cancelled','expired','refunded','completed') AND o.settlement_status='held') ORDER BY COALESCE(o.reconcile_after,o.updated_at) LIMIT 100`, int(smsVerificationPollInterval.Seconds()))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var firstErr error
	for rows.Next() {
		var id, userID int64
		var status, productType, providerOrder, refundStatus, providerCode, baseURL, credential, settlementStatus, reconciliationAction string
		var expiresAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(&id, &userID, &status, &productType, &providerOrder, &refundStatus, &providerCode, &baseURL, &credential, &expiresAt, &updatedAt, &settlementStatus, &reconciliationAction); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if settlementStatus == "held" && (status == "failed" || status == "cancelled" || status == "expired" || status == "refunded") {
			if err := s.releaseSMSHold(ctx, id, userID, status, "recovered an unsettled SMS order"); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if settlementStatus == "held" && status == "completed" {
			if err := s.captureSMSSettlement(ctx, id, userID); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}

		if status == "reconciling" && reconciliationAction != "" {
			if err := s.reconcileSMSAction(ctx, id, userID, productType, providerOrder, providerCode, baseURL, credential, settlementStatus, reconciliationAction, updatedAt); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if expiresAt.Valid && !expiresAt.Time.After(time.Now()) {
			var currentStatus string
			if statusErr := s.db.QueryRowContext(ctx, `SELECT status FROM sms_orders WHERE id=$1`, id).Scan(&currentStatus); statusErr != nil {
				if firstErr == nil {
					firstErr = statusErr
				}
				continue
			}
			if currentStatus == "active" || currentStatus == "provider_unknown" {
				if err := s.expireSMSOrder(ctx, id, userID, productType, providerOrder, providerCode, baseURL, credential); err != nil && firstErr == nil {
					firstErr = err
				}
				continue
			}
		}

		if providerOrder != "" && refundStatus != "pending" {
			if err := s.pollSMSOrder(ctx, id, providerOrder, providerCode, baseURL, credential, productType); err != nil && firstErr == nil {
				firstErr = err
			}
		}

		if status == "reconciling" && providerOrder == "" {
			if time.Since(updatedAt) >= smsVerificationUnknownTimeout {
				if err := s.releaseSMSHold(ctx, id, userID, "failed", "provider result was not confirmed before reconciliation timeout"); err != nil && firstErr == nil {
					firstErr = err
				}
			}
			continue
		}
	}
	if err := rows.Err(); err != nil && firstErr == nil {
		firstErr = err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sms_quotes WHERE expires_at < NOW() - INTERVAL '1 hour'`); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *SMSService) reconcileSMSAction(ctx context.Context, id, userID int64, productType, providerOrder, providerCode, baseURL, credential, settlementStatus, action string, updatedAt time.Time) error {
	if action == smsReconciliationPurchase {
		if providerOrder != "" {
			return s.pollSMSOrder(ctx, id, providerOrder, providerCode, baseURL, credential, productType)
		}
		recovered, recoverErr := s.recoverUnknownSMSPurchase(ctx, id, userID, productType, providerCode, baseURL, credential)
		if recoverErr != nil {
			return recoverErr
		}
		if recovered {
			return nil
		}
		if time.Since(updatedAt) >= smsVerificationUnknownTimeout {
			return s.releaseSMSHold(ctx, id, userID, "failed", "provider result was not confirmed before reconciliation timeout")
		}
		return nil
	}
	if providerOrder == "" {
		if settlementStatus == "held" {
			return s.releaseSMSHold(ctx, id, userID, "failed", "provider order reference is missing")
		}
		return errors.New("provider order reference is missing during reconciliation")
	}
	key := providerAPIKey(providerCode, credential, s.encryptor)
	provider := providerFor(providerCode, baseURL, key)
	if provider == nil || strings.TrimSpace(key) == "" {
		s.deferSMSReconciliation(ctx, id, "provider credential is unavailable during reconciliation", true)
		return ErrSMSProviderCredentialMissing
	}
	var err error
	switch action {
	case smsReconciliationCancel:
		if productType == "rental" {
			if !provider.Capabilities(ctx).RentalCancel {
				return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider does not support rental cancellation")
			}
			err = provider.CancelRental(ctx, providerOrder)
			if err == nil {
				return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider rental cancellation confirmed")
			}
			break
		}
		if !provider.Capabilities(ctx).Refund {
			return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider does not support refunds; administrator review is required")
		}
		// Temporary refund-capable providers such as 5SIM use cancellation itself
		// as the refund operation. Mutate the provider exactly once.
		err = provider.RequestTemporaryRefund(ctx, providerOrder)
		if err == nil {
			s.markProviderRefund(ctx, id, "succeeded", "")
			return s.refundSMSCapture(ctx, id, userID, "cancelled", "provider cancellation and refund confirmed during reconciliation")
		}
		if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, provider, id, userID, providerOrder, "cancelled"); handled {
			return reconcileErr
		}
	case smsReconciliationRefund:
		if productType != "temporary" {
			return s.markSMSExpired(ctx, id, "rejected", "rental refunds are unavailable")
		}
		if !provider.Capabilities(ctx).Refund {
			return s.markSMSExpired(ctx, id, "rejected", "provider does not support refunds; administrator review is required")
		}
		err = provider.RequestTemporaryRefund(ctx, providerOrder)
		if err == nil {
			s.markProviderRefund(ctx, id, "succeeded", "")
			return s.refundSMSCapture(ctx, id, userID, "refunded", "provider refund confirmed during reconciliation")
		}
		if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, provider, id, userID, providerOrder, "refunded"); handled {
			return reconcileErr
		}
	case smsReconciliationExpire:
		if productType == "rental" {
			if !provider.Capabilities(ctx).RentalCancel {
				return s.markSMSExpired(ctx, id, "rejected", "provider does not support rental cancellation")
			}
			err = provider.CancelRental(ctx, providerOrder)
			if err == nil {
				return s.markSMSExpired(ctx, id, "not_requested", "rental expired")
			}
		} else {
			if !provider.Capabilities(ctx).Refund {
				return s.markSMSExpired(ctx, id, "rejected", "provider does not support refunds; administrator review is required")
			}
			// 5SIM cancellation already produces the refund. Calling Cancel and then
			// RequestTemporaryRefund would hit the same endpoint twice and can turn
			// a successful provider refund into a false platform rejection.
			err = provider.RequestTemporaryRefund(ctx, providerOrder)
			if err == nil {
				s.markProviderRefund(ctx, id, "succeeded", "")
				return s.refundSMSCapture(ctx, id, userID, "refunded", "provider expiry refund confirmed during reconciliation")
			}
			if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, provider, id, userID, providerOrder, "refunded"); handled {
				return reconcileErr
			}
		}
	default:
		return errors.New("unsupported SMS reconciliation action")
	}
	s.deferSMSReconciliation(ctx, id, sanitizeProviderError(err).Error(), false)
	return err
}

func (s *SMSService) expireSMSOrder(ctx context.Context, id, userID int64, productType, providerOrder, providerCode, baseURL, credential string) error {
	key := providerAPIKey(providerCode, credential, s.encryptor)
	if strings.TrimSpace(key) == "" {
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown')`, smsReconciliationExpire, 300, "provider credential is unavailable during expiry reconciliation", id)
		return ErrSMSProviderCredentialMissing
	}
	provider := providerFor(providerCode, baseURL, key)
	if provider == nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown')`, smsReconciliationExpire, 300, "provider adapter is unavailable during expiry reconciliation", id)
		return ErrSMSProviderUnavailable
	}
	if productType == "temporary" {
		if !provider.Capabilities(ctx).Refund {
			return s.markSMSExpired(ctx, id, "rejected", "provider does not support refunds; administrator review is required")
		}
		refundErr := provider.RequestTemporaryRefund(ctx, providerOrder)
		if refundErr == nil {
			s.markProviderRefund(ctx, id, "succeeded", "")
			return s.refundSMSCapture(ctx, id, userID, "refunded", "no SMS received before order expiry")
		}
		if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, provider, id, userID, providerOrder, "refunded"); handled {
			return reconcileErr
		}
		if isSMSProviderTimeout(refundErr) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown')`, smsReconciliationExpire, int(smsVerificationPollInterval.Seconds()), "provider refund is being confirmed", id)
			return ErrSMSRefundPending
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown')`, smsReconciliationExpire, int(smsVerificationPollInterval.Seconds()), "provider cancellation/refund requires confirmation", id)
		return ErrSMSRefundPending
	}
	if !provider.Capabilities(ctx).RentalCancel {
		return s.markSMSExpired(ctx, id, "rejected", "provider does not support rental cancellation")
	}
	cancelErr := provider.CancelRental(ctx, providerOrder)
	if cancelErr != nil {
		if isSMSProviderTimeout(cancelErr) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown')`, smsReconciliationExpire, int(smsVerificationPollInterval.Seconds()), "provider cancellation timed out", id)
			return ErrSMSProviderUnknown
		}
		return s.markSMSExpired(ctx, id, "rejected", "provider did not confirm rental cancellation")
	}
	return s.markSMSExpired(ctx, id, "not_requested", "rental expired")
}

func (s *SMSService) markSMSExpired(ctx context.Context, id int64, refundStatus, reason string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sms_orders SET status='expired',refund_status=$1,refund_reason=$2,reconciliation_action='',reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$3 AND status IN ('active','provider_unknown','reconciling')`, refundStatus, reason, id)
	return err
}

func (s *SMSService) captureSMSSettlement(ctx context.Context, id, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var amount float64
	if err = tx.QueryRowContext(ctx, `UPDATE sms_orders SET captured_amount=reserved_amount,settlement_status='captured',reconciliation_action='',reconciliation_attempts=0,reconcile_after=NULL,updated_at=NOW() WHERE id=$1 AND settlement_status='held' RETURNING reserved_amount`, id).Scan(&amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, amount, userID); err != nil {
		return err
	}
	return tx.Commit()
}
