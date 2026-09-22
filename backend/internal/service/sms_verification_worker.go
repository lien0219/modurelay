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

// smsProviderPollDelay is deliberately conservative.  SMSPVA does not
// publish a rate-limit contract that we can safely turn into a platform
// promise, so the worker starts with the existing five-second cadence and
// backs off as an order ages.  5SIM keeps the historical cadence exactly.
func smsProviderPollDelay(providerCode string, createdAt, now time.Time) time.Duration {
	if !strings.EqualFold(strings.TrimSpace(providerCode), "smspva") {
		return smsVerificationPollInterval
	}
	age := now.Sub(createdAt)
	if age < time.Minute {
		return 5 * time.Second
	}
	if age < 5*time.Minute {
		return 10 * time.Second
	}
	return 25 * time.Second
}

// Reconcile polls active SMS orders and repairs purchases that were left in an
// unknown state by a process or network interruption. It is called by the
// existing leader-elected verification worker.
func (s *SMSService) Reconcile(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT o.id,o.user_id,o.status,o.product_type,o.provider_order_id,o.refund_status,p.code,p.base_url,p.credential_ref,o.expires_at,o.updated_at,o.created_at,o.settlement_status,o.reconciliation_action FROM sms_orders o JOIN sms_providers p ON p.id=o.provider_id WHERE (o.status IN ('active','provider_unknown') AND (((o.reconcile_after IS NULL AND o.updated_at <= NOW()-($1 * INTERVAL '1 second')) OR o.reconcile_after <= NOW()) OR o.expires_at <= NOW())) OR (o.status='reconciling' AND (o.reconcile_after IS NULL OR o.reconcile_after <= NOW())) OR (o.status='pending' AND o.settlement_status='held' AND o.provider_order_id='') OR (o.status IN ('failed','cancelled','expired','refunded','completed') AND o.settlement_status='held') ORDER BY COALESCE(o.reconcile_after,o.updated_at) LIMIT 100`, int(smsVerificationPollInterval.Seconds()))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var firstErr error
	for rows.Next() {
		var id, userID int64
		var status, productType, providerOrder, refundStatus, providerCode, baseURL, credential, settlementStatus, reconciliationAction string
		var expiresAt sql.NullTime
		var updatedAt, createdAt time.Time
		if err := rows.Scan(&id, &userID, &status, &productType, &providerOrder, &refundStatus, &providerCode, &baseURL, &credential, &expiresAt, &updatedAt, &createdAt, &settlementStatus, &reconciliationAction); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if settlementStatus == "held" && (status == "failed" || status == "cancelled" || status == "expired" || status == "refunded") {
			if strings.EqualFold(providerCode, "smspva") && strings.EqualFold(productType, "temporary") {
				var firstSMS sql.NullTime
				if queryErr := s.db.QueryRowContext(ctx, `SELECT first_sms_received_at FROM sms_orders WHERE id=$1`, id).Scan(&firstSMS); queryErr != nil {
					if firstErr == nil {
						firstErr = queryErr
					}
					continue
				}
				if firstSMS.Valid {
					if err := s.captureSMSSettlement(ctx, id, userID); err != nil && firstErr == nil {
						firstErr = err
					}
					continue
				}
			}
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

		if status == "pending" && settlementStatus == "held" && providerOrder == "" {
			_, promoteErr := s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW(),updated_at=NOW() WHERE id=$2 AND status='pending' AND settlement_status='held'`, smsReconciliationPurchase, id)
			if promoteErr != nil {
				if firstErr == nil {
					firstErr = promoteErr
				}
				continue
			}
			status = "reconciling"
			reconciliationAction = smsReconciliationPurchase
		}
		// A provider allocation that is still settling must not bypass the
		// platform expiry policy merely because the local row is reconciling.
		// The normal reconciliation branch polls first, which could leave an
		// already-expired temporary order active until another cycle (or keep
		// extending the apparent lifetime after a delayed purchase response).
		// Once the provider order id is known, run the same automatic
		// cancellation/refund path used by active orders.
		if status == "reconciling" && reconciliationAction == smsReconciliationPurchase && providerOrder != "" && expiresAt.Valid && !expiresAt.Time.After(time.Now()) {
			if err := s.expireSMSOrder(ctx, id, userID, productType, providerOrder, providerCode, baseURL, credential); err != nil && firstErr == nil {
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
			// Only SMSPVA uses the age-based schedule.  The 5SIM path remains
			// exactly five seconds as the frozen production baseline.
			if (status == "active" || status == "provider_unknown") &&
				time.Since(updatedAt) < smsProviderPollDelay(providerCode, createdAt, time.Now()) &&
				(!expiresAt.Valid || expiresAt.Time.After(time.Now())) {
				continue
			}
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
	s.cleanupExpiredRentalQuotes(ctx)
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
			if strings.EqualFold(providerCode, "smspva") && productType == "rental" {
				var restoreHistoryID string
				if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(metadata->>'provider_history_order_id','') FROM sms_orders WHERE id=$1`, id).Scan(&restoreHistoryID); err != nil {
					return err
				}
				if strings.TrimSpace(restoreHistoryID) != "" {
					// Restore is a mutating provider operation. Without positive
					// evidence that it failed, releasing the hold could create a
					// free restored rental when the provider committed the write
					// but our response was lost. Keep the amount frozen and
					// continue deterministic before/after reconciliation.
					_, err := s.db.ExecContext(ctx, `UPDATE sms_orders
						SET status='reconciling',
						    reconciliation_action=$1,
						    reconciliation_attempts=reconciliation_attempts+1,
						    reconcile_after=NOW()+INTERVAL '30 seconds',
						    last_provider_error='restore outcome remains unconfirmed; automatic release is blocked',
						    updated_at=updated_at
						WHERE id=$2 AND settlement_status='held'`,
						smsReconciliationPurchase, id)
					return err
				}
			}
			reason := "provider did not create a recoverable order before reconciliation timeout"
			if strings.EqualFold(providerCode, "smspva") {
				reason = "provider purchase outcome cannot be deterministically recovered before reconciliation timeout"
			}
			return s.failSMSPurchase(ctx, id, userID, reason)
		}
		return nil
	}
	if providerOrder == "" {
		if settlementStatus == "held" {
			return s.failSMSPurchase(ctx, id, userID, "provider order reference is missing and no allocation was confirmed")
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
		capabilities := provider.Capabilities(ctx)
		if !capabilities.Refund && !capabilities.Cancel {
			return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider does not support cancellation or refunds; administrator review is required")
		}
		// Refund-capable temporary providers such as 5SIM use cancellation itself
		// as the refund operation. Providers that expose cancellation without a
		// refund endpoint still use their cancellation operation here.
		if capabilities.Refund {
			err = provider.RequestTemporaryRefund(ctx, providerOrder)
		} else {
			err = provider.CancelTemporary(ctx, providerOrder)
		}
		if err == nil {
			if capabilities.Refund {
				s.markProviderRefund(ctx, id, "succeeded", "")
				return s.settleSMSExpiry(ctx, id, userID, "cancelled", "approved", "provider cancellation and refund confirmed during reconciliation")
			}
			if strings.EqualFold(providerCode, "smspva") {
				if handled, settleErr := s.settleSMSPVANoDelivery(ctx, id, userID, "cancelled", "SMSPVA cancellation confirmed before SMS delivery; reserved balance released"); handled {
					return settleErr
				}
				return s.markSMSCancellationPendingRefund(ctx, id, "provider cancellation accepted after SMS delivery; upstream refund confirmation is unavailable")
			}
			return s.markSMSClosed(ctx, id, "cancelled", "rejected", "provider cancellation confirmed during reconciliation; refund unsupported")
		}
		if capabilities.Refund {
			if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, provider, id, userID, providerOrder, "cancelled"); handled {
				return reconcileErr
			}
		}
		if strings.EqualFold(providerCode, "smspva") {
			// SMSPVA has no cancellation/refund status endpoint. Once the
			// cancellation call itself is ambiguous or rejected, stop retrying the
			// mutating endpoint and leave the captured balance for manual review.
			return s.markSMSCancellationPendingRefund(ctx, id, "provider cancellation outcome is ambiguous; upstream refund confirmation is unavailable")
		}
	case smsReconciliationRefund:
		if productType != "temporary" {
			return s.markSMSExpired(ctx, id, "rejected", "rental refunds are unavailable")
		}
		if !provider.Capabilities(ctx).Refund {
			if strings.EqualFold(providerCode, "smspva") {
				stateResult, stateErr := provider.GetTemporaryStatus(ctx, providerOrder)
				if stateErr == nil && stateResult != nil {
					state := smsStatusFromProvider(stateResult)
					if state == "cancelled" || state == "expired" {
						if handled, settleErr := s.settleSMSPVANoDelivery(ctx, id, userID, state, "SMSPVA terminal state confirmed before SMS delivery; reserved balance released"); handled {
							return settleErr
						}
					}
					if state == "completed" {
						return s.markSMSExpiryPendingRefund(ctx, id, "SMS delivery already occurred; automatic refund is unavailable")
					}
				}
				s.deferSMSReconciliation(ctx, id, "SMSPVA refund reconciliation is waiting for a terminal provider state", false)
				return ErrSMSRefundPending
			}
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
				if strings.EqualFold(providerCode, "smspva") {
					err = provider.CancelTemporary(ctx, providerOrder)
					if err == nil {
						if handled, settleErr := s.settleSMSPVANoDelivery(ctx, id, userID, "expired", "SMSPVA cancellation confirmed before SMS delivery; reserved balance released"); handled {
							return settleErr
						}
						return s.markSMSExpiryPendingRefund(ctx, id, "provider cancellation accepted after SMS delivery; upstream refund confirmation is unavailable")
					}
					if isSMSProviderTimeout(err) {
						s.deferSMSReconciliation(ctx, id, "provider cancellation timeout during expiry reconciliation", false)
						return ErrSMSProviderUnknown
					}
					return s.markSMSExpiryPendingRefund(ctx, id, "provider cancellation outcome is ambiguous or rejected; upstream refund confirmation is unavailable")
				}
				return s.markSMSExpired(ctx, id, "rejected", "provider does not support refunds; administrator review is required")
			}
			// 5SIM cancellation already produces the refund. Calling Cancel and then
			// RequestTemporaryRefund would hit the same endpoint twice and can turn
			// a successful provider refund into a false platform rejection.
			err = provider.RequestTemporaryRefund(ctx, providerOrder)
			if err == nil {
				s.markProviderRefund(ctx, id, "succeeded", "")
				return s.settleSMSExpiry(ctx, id, userID, "refunded", "approved", "provider expiry refund confirmed during reconciliation")
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
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown','reconciling')`, smsReconciliationExpire, 300, "provider credential is unavailable during expiry reconciliation", id)
		return ErrSMSProviderCredentialMissing
	}
	provider := providerFor(providerCode, baseURL, key)
	if provider == nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown','reconciling')`, smsReconciliationExpire, 300, "provider adapter is unavailable during expiry reconciliation", id)
		return ErrSMSProviderUnavailable
	}
	if productType == "temporary" {
		if !provider.Capabilities(ctx).Refund {
			if strings.EqualFold(providerCode, "smspva") {
				cancelErr := provider.CancelTemporary(ctx, providerOrder)
				if cancelErr == nil {
					if handled, settleErr := s.settleSMSPVANoDelivery(ctx, id, userID, "expired", "SMSPVA cancellation confirmed before SMS delivery; reserved balance released"); handled {
						return settleErr
					}
					return s.markSMSExpiryPendingRefund(ctx, id, "provider cancellation accepted after SMS delivery; upstream refund confirmation is unavailable")
				}
				return s.markSMSExpiryPendingRefund(ctx, id, "provider cancellation outcome is ambiguous; upstream refund confirmation is unavailable")
			}
			return s.markSMSExpired(ctx, id, "rejected", "provider does not support refunds; administrator review is required")
		}
		refundErr := provider.RequestTemporaryRefund(ctx, providerOrder)
		if refundErr == nil {
			s.markProviderRefund(ctx, id, "succeeded", "")
			return s.settleSMSExpiry(ctx, id, userID, "refunded", "approved", "no SMS received before order expiry")
		}
		if handled, reconcileErr := s.reconcileTemporaryRefundState(ctx, provider, id, userID, providerOrder, "refunded"); handled {
			return reconcileErr
		}
		if isSMSProviderTimeout(refundErr) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown','reconciling')`, smsReconciliationExpire, int(smsVerificationPollInterval.Seconds()), "provider refund is being confirmed", id)
			return ErrSMSRefundPending
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',refund_status='pending',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),refund_reason=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown','reconciling')`, smsReconciliationExpire, int(smsVerificationPollInterval.Seconds()), "provider cancellation/refund requires confirmation", id)
		return ErrSMSRefundPending
	}
	if !provider.Capabilities(ctx).RentalCancel {
		return s.markSMSExpired(ctx, id, "rejected", "provider does not support rental cancellation")
	}
	cancelErr := provider.CancelRental(ctx, providerOrder)
	if cancelErr != nil {
		if isSMSProviderTimeout(cancelErr) {
			_, _ = s.db.ExecContext(ctx, `UPDATE sms_orders SET status='reconciling',reconciliation_action=$1,reconcile_after=NOW()+($2 * INTERVAL '1 second'),last_provider_error=$3,updated_at=NOW() WHERE id=$4 AND status IN ('active','provider_unknown','reconciling')`, smsReconciliationExpire, int(smsVerificationPollInterval.Seconds()), "provider cancellation timed out", id)
			return ErrSMSProviderUnknown
		}
		return s.markSMSExpired(ctx, id, "rejected", "provider did not confirm rental cancellation")
	}
	return s.markSMSExpired(ctx, id, "not_requested", "rental expired")
}

// settleSMSExpiry returns a still-held reservation or refunds a captured
// settlement, depending on which phase the order reached before expiry. A
// provider allocation can be known while local activation is still settling;
// treating that row as captured would leave the user's balance frozen and a
// successful provider cancellation unable to close the order.
func (s *SMSService) settleSMSExpiry(ctx context.Context, id, userID int64, terminalStatus, refundStatus, reason string) error {
	var settlementStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT settlement_status FROM sms_orders WHERE id=$1`, id).Scan(&settlementStatus); err != nil {
		return err
	}
	if settlementStatus == "held" {
		return s.returnSMSBalance(ctx, id, userID, true, terminalStatus, refundStatus, reason)
	}
	if settlementStatus == "captured" {
		return s.refundSMSCapture(ctx, id, userID, terminalStatus, reason)
	}
	return nil
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
