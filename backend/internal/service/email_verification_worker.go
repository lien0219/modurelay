package service

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	emailVerificationWorkerLockKey  = "email:verification:worker:leader"
	emailVerificationWorkerLockTTL  = 2 * time.Minute
	emailVerificationWorkerInterval = 2 * time.Second
)

// EmailVerificationWorker owns the server-side polling and retention loop.
// Browser requests may trigger an immediate poll, but provider calls are never
// made by the browser and the periodic worker is the normal delivery path.
type EmailVerificationWorker struct {
	service   *EmailVerificationService
	db        *sql.DB
	lockCache LeaderLockCache
	instance  string
	stopCh    chan struct{}
	stopOnce  sync.Once
	startOnce sync.Once
	wg        sync.WaitGroup
}

func NewEmailVerificationWorker(service *EmailVerificationService, db *sql.DB, lockCache LeaderLockCache) *EmailVerificationWorker {
	return &EmailVerificationWorker{
		service:   service,
		db:        db,
		lockCache: lockCache,
		instance:  uuid.NewString(),
		stopCh:    make(chan struct{}),
	}
}

func (w *EmailVerificationWorker) Start() {
	if w == nil || w.service == nil || w.db == nil {
		return
	}
	w.startOnce.Do(func() {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			ticker := time.NewTicker(emailVerificationWorkerInterval)
			defer ticker.Stop()
			w.runOnce()
			for {
				select {
				case <-ticker.C:
					w.runOnce()
				case <-w.stopCh:
					return
				}
			}
		}()
	})
}

func (w *EmailVerificationWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.wg.Wait()
}

func (w *EmailVerificationWorker) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	release, ok := tryAcquireSingletonLeaderLock(ctx, w.lockCache, w.db, emailVerificationWorkerLockKey, w.instance, emailVerificationWorkerLockTTL)
	if !ok {
		return
	}
	defer release()
	if err := w.service.PollDue(ctx); err != nil {
		slog.Warn("email verification polling cycle failed", "error", err)
	}
	if err := w.service.Reconcile(ctx); err != nil {
		slog.Warn("email verification reconciliation cycle failed", "error", err)
	}
	if err := w.service.CleanupExpiredMessages(ctx); err != nil {
		slog.Warn("email message retention cleanup failed", "error", err)
	}
}

// Reconcile repairs local states after process/DB interruptions without
// guessing that a timed-out provider request failed. Unknown generation stays
// in RECONCILING until expiry or an administrator resolves it; no duplicate
// inbox is generated automatically.
func (s *EmailVerificationService) Reconcile(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,status,expires_at,refund_policy_snapshot,sale_price_snapshot,provider_inbox_id,email_address FROM email_orders WHERE status IN ('reserved','generating_inbox','reconciling') ORDER BY updated_at LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, userID int64
		var status, policy, providerInboxID, emailAddress string
		var expires time.Time
		var price float64
		if err := rows.Scan(&id, &userID, &status, &expires, &policy, &price, &providerInboxID, &emailAddress); err != nil {
			return err
		}
		inboxDelivered := strings.TrimSpace(providerInboxID) != "" && strings.TrimSpace(emailAddress) != ""
		if !expires.IsZero() && time.Now().After(expires) && !inboxDelivered && (policy == EmailRefundIfNoMessage || policy == EmailNoRefundAfterDelivery) {
			tx, txErr := s.db.BeginTx(ctx, nil)
			if txErr != nil {
				return txErr
			}
			res, txErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='refunded',refund_status='approved',refund_reason='email inbox was not delivered during recovery',refunded_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND status IN ('reserved','generating_inbox','reconciling') AND provider_inbox_id='' AND email_address='' AND refund_status='not_requested'`, id)
			if txErr == nil {
				if n, _ := res.RowsAffected(); n == 1 {
					_, txErr = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID)
				}
			}
			if txErr != nil {
				_ = tx.Rollback()
				return txErr
			}
			if txErr = tx.Commit(); txErr != nil {
				return txErr
			}
			if n, _ := res.RowsAffected(); n == 1 {
				s.recordOrderEvent(ctx, id, "reconciled", "email_reconcile:"+strconv.FormatInt(id, 10), map[string]any{"action": "refund_expired_recovery"})
			}
			continue
		}
		if !expires.IsZero() && time.Now().After(expires) && inboxDelivered && (policy == EmailNoRefundAfterDelivery || policy == EmailRefundIfNoMessage) {
			tx, txErr := s.db.BeginTx(ctx, nil)
			if txErr != nil {
				return txErr
			}
			res, txErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='expired',refund_status='not_applicable',captured_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND status='reconciling' AND provider_inbox_id<>'' AND email_address<>'' AND captured_amount=0`, id)
			captured := false
			if txErr == nil {
				if affected, _ := res.RowsAffected(); affected == 1 {
					captured = true
					_, txErr = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID)
				}
			}
			if txErr != nil {
				_ = tx.Rollback()
				return txErr
			}
			if txErr = tx.Commit(); txErr != nil {
				return txErr
			}
			if captured {
				s.recordOrderEvent(ctx, id, "balance_captured", "email_capture:"+strconv.FormatInt(id, 10), map[string]any{"amount": price, "reason": "inbox_delivered"})
			}
			continue
		}
		if !expires.IsZero() && time.Now().After(expires) && policy == EmailManualReview && status == "reconciling" {
			if _, txErr := s.db.ExecContext(ctx, `UPDATE email_orders SET status='expired',refund_status='manual_review',refund_reason='manual review required',updated_at=NOW() WHERE id=$1 AND status='reconciling' AND refund_status='not_requested'`, id); txErr != nil {
				return txErr
			}
			s.recordOrderEvent(ctx, id, "expired", "email_expire:"+strconv.FormatInt(id, 10), map[string]any{"reason": "manual_review"})
			continue
		}
		if status == "reserved" || status == "generating_inbox" {
			// A crash before the provider result is intentionally not retried.
			// Keep the state explicit so support can reconcile it safely.
			_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET status='reconciling',error_code='EMAIL_RECOVERY_REQUIRED',error_public_message='邮箱订单正在核对中',error_admin_message='generation interrupted before provider result',updated_at=NOW() WHERE id=$1 AND status IN ('reserved','generating_inbox')`, id)
			// Normalize the user-facing recovery message even when upgrading from
			// an earlier build that wrote an incorrectly encoded literal.
			_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET error_public_message=$2 WHERE id=$1 AND status='reconciling'`, id, "邮箱订单正在核对中")
			s.recordOrderEvent(ctx, id, "recovery_started", "email_reconcile:"+strconv.FormatInt(id, 10), map[string]any{"status": status})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return s.reconcileEmailSettlements(ctx)
}

// reconcileEmailSettlements repairs terminal rows left with an unsettled hold
// by an older build or an interrupted non-transactional deployment. The amount
// predicates make the adjustment idempotent and avoid touching unrelated SMS
// or subscription holds in the user's aggregate frozen balance.
func (s *EmailVerificationService) reconcileEmailSettlements(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,status,refund_status,sale_price_snapshot,reserved_amount,captured_amount,released_amount,refunded_amount FROM email_orders WHERE status IN ('completed','failed','cancelled','expired','refunded') AND reserved_amount > captured_amount+released_amount+refunded_amount ORDER BY updated_at LIMIT 100`)
	if err != nil {
		return err
	}
	type unsettledOrder struct {
		id, userID                                    int64
		status, refundStatus                          string
		price, reserved, captured, released, refunded float64
	}
	orders := []unsettledOrder{}
	for rows.Next() {
		var order unsettledOrder
		if err = rows.Scan(&order.id, &order.userID, &order.status, &order.refundStatus, &order.price, &order.reserved, &order.captured, &order.released, &order.refunded); err != nil {
			_ = rows.Close()
			return err
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, order := range orders {
		remaining := order.reserved - order.captured - order.released - order.refunded
		if remaining <= 0 {
			continue
		}
		returnFunds := order.status == "failed" || order.status == "refunded" || order.refundStatus == "released" || order.refundStatus == "approved"
		captureFunds := order.status == "completed" || order.refundStatus == "not_applicable"
		if !returnFunds && !captureFunds {
			continue
		}
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return txErr
		}
		var result sql.Result
		if returnFunds {
			column := "released_amount"
			if order.status == "refunded" || order.refundStatus == "approved" {
				column = "refunded_amount"
			}
			query := `UPDATE email_orders SET ` + column + `=` + column + `+$1,updated_at=NOW() WHERE id=$2 AND reserved_amount > captured_amount+released_amount+refunded_amount`
			result, txErr = tx.ExecContext(ctx, query, remaining, order.id)
			if txErr == nil {
				if affected, _ := result.RowsAffected(); affected == 1 {
					_, txErr = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, remaining, order.userID)
				}
			}
		} else {
			result, txErr = tx.ExecContext(ctx, `UPDATE email_orders SET captured_amount=captured_amount+$1,updated_at=NOW() WHERE id=$2 AND reserved_amount > captured_amount+released_amount+refunded_amount`, remaining, order.id)
			if txErr == nil {
				if affected, _ := result.RowsAffected(); affected == 1 {
					_, txErr = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, remaining, order.userID)
				}
			}
		}
		if txErr != nil {
			_ = tx.Rollback()
			return txErr
		}
		if txErr = tx.Commit(); txErr != nil {
			return txErr
		}
		s.recordOrderEvent(ctx, order.id, "settlement_reconciled", "email_settlement_reconcile:"+strconv.FormatInt(order.id, 10), map[string]any{"amount": remaining, "returned": returnFunds})
	}
	return nil
}

func (s *EmailVerificationService) CleanupExpiredMessages(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	retentionDays := 7
	secretHours := 24
	if s.settings != nil && s.settings.settingRepo != nil {
		if raw, err := s.settings.settingRepo.GetValue(ctx, "email_message_retention_days"); err == nil {
			if n, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil && n > 0 && n <= 3650 {
				retentionDays = n
			}
		}
		if raw, err := s.settings.settingRepo.GetValue(ctx, "email_verification_secret_retention_hours"); err == nil {
			if n, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil && n > 0 && n <= 8760 {
				secretHours = n
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE email_messages SET verification_code='',verification_url='',updated_at=NOW() WHERE received_at < NOW() - ($1 * INTERVAL '1 hour') AND (verification_code <> '' OR verification_url <> '')`, secretHours); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE email_messages SET text_body='',html_body='',raw_payload='{}'::jsonb,updated_at=NOW() WHERE received_at < NOW() - ($1 * INTERVAL '1 day') AND (text_body <> '' OR html_body <> '' OR raw_payload <> '{}'::jsonb)`, retentionDays)
	return err
}
