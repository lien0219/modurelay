package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/google/uuid"
)

// --- Refund Flow ---

var createPaymentProviderFromInstance = provider.CreateProvider

const (
	refundPersistenceTimeout = 10 * time.Second
	// A refund can legitimately spend time in the provider, so recovery only
	// claims rows that have been untouched for several scheduler cycles. The
	// claim timestamp is persisted in PostgreSQL and acts as the lease.
	refundRecoveryStaleAfter = 5 * time.Minute
	refundRecoveryBatchSize  = 100
)

// refundOutcomeUnknownError marks a provider call whose result cannot be
// classified as a confirmed rejection. The request may have reached the
// gateway, so the refund remains pending and the held balance is retained for
// reconciliation instead of being released and submitted a second time.
type refundOutcomeUnknownError struct{ err error }

func (e *refundOutcomeUnknownError) Error() string { return e.err.Error() }
func (e *refundOutcomeUnknownError) Unwrap() error { return e.err }

func isRefundOutcomeUnknown(err error) bool {
	var unknown *refundOutcomeUnknownError
	return errors.As(err, &unknown)
}

func detachedRefundContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), refundPersistenceTimeout)
	}
	return context.WithTimeout(context.WithoutCancel(ctx), refundPersistenceTimeout)
}

// getOrderProviderInstance looks up the provider instance that processed this order.
// For legacy orders without provider_instance_id, it resolves only when the
// historical instance is uniquely identifiable from the stored order fields.
func (s *PaymentService) getOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return s.resolveUniqueLegacyOrderProviderInstance(ctx, o)
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, nil
	}
	return s.entClient.PaymentProviderInstance.Get(ctx, instID)
}

// getRefundOrderProviderInstance resolves the provider instance for refund paths.
// Refunds must be pinned to an explicit historical binding, so legacy
// "best-effort" provider guessing is intentionally not allowed here.
func (s *PaymentService) getRefundOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return nil, nil
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("order %d refund provider instance id is invalid: %s", o.ID, instIDStr)
	}
	inst, err := s.entClient.PaymentProviderInstance.Get(ctx, instID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, fmt.Errorf("order %d refund provider instance %s is missing", o.ID, instIDStr)
		}
		return nil, err
	}
	return inst, nil
}

func (s *PaymentService) resolveUniqueLegacyOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	paymentType := payment.GetBasePaymentType(strings.TrimSpace(o.PaymentType))
	providerKey := strings.TrimSpace(psStringValue(o.ProviderKey))
	if providerKey != "" {
		instances, err := s.entClient.PaymentProviderInstance.Query().
			Where(paymentproviderinstance.ProviderKeyEQ(providerKey)).
			All(ctx)
		if err != nil {
			return nil, err
		}
		matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
		if len(matched) == 1 {
			return matched[0], nil
		}
		return nil, nil
	}

	if paymentType == "" {
		return nil, nil
	}

	instances, err := s.entClient.PaymentProviderInstance.Query().
		All(ctx)
	if err != nil {
		return nil, err
	}

	matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
	if len(matched) == 1 {
		return matched[0], nil
	}
	return nil, nil
}

func psFilterLegacyOrderProviderInstances(orderPaymentType string, instances []*dbent.PaymentProviderInstance) []*dbent.PaymentProviderInstance {
	if len(instances) == 0 {
		return nil
	}
	if strings.TrimSpace(orderPaymentType) == "" {
		return instances
	}
	var matched []*dbent.PaymentProviderInstance
	for _, inst := range instances {
		if psLegacyOrderMatchesInstance(orderPaymentType, inst) {
			matched = append(matched, inst)
		}
	}
	return matched
}

func psLegacyOrderMatchesInstance(orderPaymentType string, inst *dbent.PaymentProviderInstance) bool {
	if inst == nil {
		return false
	}

	baseType := payment.GetBasePaymentType(strings.TrimSpace(orderPaymentType))
	instanceProviderKey := strings.TrimSpace(inst.ProviderKey)
	if baseType == "" {
		return false
	}

	if baseType == payment.TypeStripe {
		return instanceProviderKey == payment.TypeStripe
	}
	if instanceProviderKey == payment.TypeStripe {
		return false
	}
	if instanceProviderKey == baseType {
		return true
	}
	return payment.InstanceSupportsType(inst.SupportedTypes, baseType)
}

func (s *PaymentService) RequestRefund(ctx context.Context, oid, uid int64, reason string) error {
	o, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return err
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Balance < o.Amount {
		return infraerrors.BadRequest("BALANCE_NOT_ENOUGH", "refund amount exceeds balance")
	}
	nr := strings.TrimSpace(reason)
	now := time.Now()
	by := fmt.Sprintf("%d", uid)
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.UserIDEQ(uid), paymentorder.StatusEQ(OrderStatusCompleted), paymentorder.OrderTypeEQ(payment.OrderTypeBalance)).SetStatus(OrderStatusRefundRequested).SetRefundRequestedAt(now).SetRefundRequestReason(nr).SetRefundRequestedBy(by).SetRefundAmount(o.Amount).Save(ctx)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if c == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, oid, "REFUND_REQUESTED", fmt.Sprintf("user:%d", uid), map[string]any{"amount": o.Amount, "reason": nr})
	return nil
}

func (s *PaymentService) validateRefundRequest(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != uid {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.OrderType != payment.OrderTypeBalance {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only balance orders can request refund")
	}
	if o.Status != OrderStatusCompleted {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only completed orders can request refund")
	}
	// Check provider instance allows user refund
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil || inst == nil {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.AllowUserRefund {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "user refund is not enabled for this provider")
	}
	return o, nil
}

func (s *PaymentService) PrepareRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	// REFUND_PENDING 只能通过 QueryAndFinalizeRefund 收敛：重新发起会重复扣减并向网关重复退款。
	ok := []string{OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed}
	if !psSliceContains(ok, o.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	// Check provider instance allows admin refund
	inst, instErr := s.getRefundOrderProviderInstance(ctx, o)
	if instErr != nil {
		slog.Warn("refund: provider instance lookup failed", "orderID", oid, "error", instErr)
		return nil, nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
	}
	if inst == nil {
		// Legacy order without provider_instance_id — block refund
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled {
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
	}
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		return nil, nil, infraerrors.BadRequest("INVALID_AMOUNT", "invalid refund amount")
	}
	if amt <= 0 {
		amt = o.Amount
	}
	orderCurrency := PaymentOrderCurrency(o)
	if amt-o.Amount > paymentAmountToleranceForCurrency(orderCurrency) {
		return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds recharge")
	}
	ga := calculateGatewayRefundAmount(o.Amount, o.PayAmount, amt, orderCurrency)
	rr := strings.TrimSpace(reason)
	if rr == "" && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	}
	if rr == "" {
		rr = fmt.Sprintf("refund order:%d", o.ID)
	}
	p := &RefundPlan{OrderID: oid, Order: o, RefundAmount: amt, GatewayAmount: ga, Reason: rr, Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
	if deduct {
		if er := s.prepDeduct(ctx, o, p, force); er != nil {
			return nil, er, nil
		}
	}
	return p, nil, nil
}

func (s *PaymentService) prepDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	if o.OrderType == payment.OrderTypeSubscription {
		p.DeductionType = payment.DeductionTypeSubscription
		if o.SubscriptionGroupID != nil && o.SubscriptionDays != nil {
			p.SubDaysToDeduct = *o.SubscriptionDays
			sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, o.UserID, *o.SubscriptionGroupID)
			if err == nil && sub != nil {
				p.SubscriptionID = sub.ID
			} else if !force {
				return &RefundResult{Success: false, Warning: "cannot find active subscription for deduction, use force", RequireForce: true}
			}
		}
		return nil
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		return nil
	}
	p.DeductionType = payment.DeductionTypeBalance
	if u.Balance < p.RefundAmount && !force {
		return &RefundResult{Success: false, Warning: "user balance is insufficient for deduction, use force", RequireForce: true}
	}
	p.BalanceToDeduct = math.Max(0, math.Min(p.RefundAmount, u.Balance))
	return nil
}

type availableBalanceDeductor interface {
	DeductAvailableBalance(ctx context.Context, id int64, amount float64) (float64, error)
}

func (s *PaymentService) deductAvailableBalance(ctx context.Context, userID int64, amount float64) (float64, error) {
	repo, ok := s.userRepo.(availableBalanceDeductor)
	if !ok {
		return 0, errors.New("user repository does not support available balance deduction")
	}
	deducted, err := repo.DeductAvailableBalance(ctx, userID, amount)
	if err == nil && deducted > 0 {
		invalidateUserBalanceCacheOnCommit(ctx, s.balanceCacheInvalidator, userID)
	}
	return deducted, err
}

// invalidateRefundBalanceCache 退款扣减或归还余额后失效计费缓存，否则网关在缓存 TTL 内仍按旧余额放行。
func (s *PaymentService) invalidateRefundBalanceCache(ctx context.Context, userID int64) {
	if s.redeemService == nil {
		return
	}
	s.redeemService.invalidateRedeemCaches(ctx, userID, &RedeemCode{Type: RedeemTypeBalance})
}

func (s *PaymentService) ExecuteRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if err := s.beginRefundAttempt(ctx, p); err != nil {
		return nil, err
	}
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		if isRefundOutcomeUnknown(err) {
			persistCtx, cancel := detachedRefundContext(ctx)
			defer cancel()
			return s.markRefundPending(persistCtx, p, &payment.RefundResponse{Status: payment.ProviderStatusPending})
		}
		return s.handleGwFail(ctx, p, err)
	}
	persistCtx, cancel := detachedRefundContext(ctx)
	defer cancel()
	return s.finishRefund(persistCtx, p, resp)
}

func (s *PaymentService) beginRefundAttempt(ctx context.Context, p *RefundPlan) (err error) {
	if p == nil || p.Order == nil {
		return errors.New("refund plan is missing order")
	}
	if p.Order != nil {
		p.OriginalStatus = p.Order.Status
	}
	if strings.TrimSpace(p.RequestID) == "" {
		p.RequestID = strings.TrimSpace(p.RefundID)
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin refund attempt: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)
	if strings.TrimSpace(p.RequestID) == "" {
		p.RequestID, err = s.nextRefundRequestID(p)
		if err != nil {
			return err
		}
	}
	claimed, err := tx.PaymentOrder.Update().
		Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed)).
		SetStatus(OrderStatusRefunding).
		Save(txCtx)
	if err != nil {
		return fmt.Errorf("claim refund attempt: %w", err)
	}
	if claimed == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}

	held, holdErr := s.refundRetryHold(txCtx, p.OrderID)
	if holdErr != nil {
		return holdErr
	}
	if held != nil {
		held.applyTo(p)
		p.deductionHeld = true
	} else {
		if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
			deducted, err := s.deductAvailableBalance(txCtx, p.Order.UserID, p.BalanceToDeduct)
			if err != nil {
				return fmt.Errorf("deduction: %w", err)
			}
			p.BalanceToDeduct = deducted
			p.deductionHeld = deducted > 0
		}
		if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
			_, err := s.subscriptionSvc.ExtendSubscription(txCtx, p.SubscriptionID, -p.SubDaysToDeduct)
			if err != nil {
				if errors.Is(err, ErrAdjustWouldExpire) {
					if revokeErr := s.subscriptionSvc.RevokeSubscription(txCtx, p.SubscriptionID); revokeErr != nil {
						return fmt.Errorf("revoke subscription: %w", revokeErr)
					}
				} else {
					return fmt.Errorf("deduct subscription days: %w", err)
				}
			}
			p.deductionHeld = true
		}
	}

	if err := createPaymentAuditLogTx(txCtx, tx.Client(), p.OrderID, "REFUND_SUBMITTING", map[string]any{
		"requestID": p.RequestID, "refundAmount": p.RefundAmount, "reason": p.Reason,
		"force": p.Force, "deductionHeld": p.deductionHeld, "deductionType": p.DeductionType,
		"balanceDeducted": p.BalanceToDeduct, "subDaysDeducted": p.SubDaysToDeduct,
		"subscriptionID": p.SubscriptionID,
	}); err != nil {
		return err
	}
	current, err := tx.PaymentOrder.Get(txCtx, p.OrderID)
	if err != nil {
		return fmt.Errorf("reload refund attempt: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit refund attempt: %w", err)
	}
	committed = true
	p.Order = current
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	}
	return nil
}

func (s *PaymentService) nextRefundRequestID(p *RefundPlan) (string, error) {
	return uuid.NewString(), nil
}

func (s *PaymentService) refundRetryHold(ctx context.Context, orderID int64) (*refundPendingAuditDetail, error) {
	logs, err := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10))).
		Order(paymentauditlog.ByCreatedAt(sql.OrderDesc()), paymentauditlog.ByID(sql.OrderDesc())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load refund retry evidence: %w", err)
	}
	for _, entry := range logs {
		switch entry.Action {
		case "REFUND_SUCCESS", "REFUND_GATEWAY_FAILED", "REFUND_SUBMITTING":
			return nil, nil
		case "REFUND_FAILED":
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(entry.Detail), &fields); err != nil {
				return nil, fmt.Errorf("parse refund failure evidence: %w", err)
			}
			var detail refundPendingAuditDetail
			if _, hasHoldField := fields["deductionHeld"]; hasHoldField {
				if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
					return nil, fmt.Errorf("parse refund failure evidence: %w", err)
				}
				if detail.DeductionHeld {
					return &detail, nil
				}
				return nil, nil
			}
			// Legacy REFUND_FAILED records did not carry deductionHeld. Keep
			// scanning until the preceding attempt boundary to recover a
			// legacy REFUND_ROLLBACK_FAILED hold without double-deducting.
			continue
		case "REFUND_ROLLBACK_FAILED":
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(entry.Detail), &fields); err != nil {
				return nil, fmt.Errorf("parse refund rollback evidence: %w", err)
			}
			var detail refundPendingAuditDetail
			if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
				return nil, fmt.Errorf("parse refund rollback evidence: %w", err)
			}
			detail.DeductionHeld = true
			if detail.DeductionType == "" {
				if detail.SubDaysDeducted > 0 || detail.SubscriptionID > 0 || fields["subDaysDeducted"] != nil {
					detail.DeductionType = payment.DeductionTypeSubscription
				} else {
					detail.DeductionType = payment.DeductionTypeBalance
				}
			}
			return &detail, nil
		}
	}
	return nil, nil
}

func (s *PaymentService) gwRefund(ctx context.Context, p *RefundPlan) (*payment.RefundResponse, error) {
	if p.Order.PaymentTradeNo == "" {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_NO_TRADE_NO", "admin", map[string]any{"detail": "skipped"})
		return &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, nil
	}

	// Use the exact provider instance that created this order, not a random one
	// from the registry. Each instance has its own merchant credentials.
	prov, err := s.getRefundProvider(ctx, p.Order)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	if err := validateProviderSnapshotMetadata(p.Order, prov.ProviderKey(), providerMerchantIdentityMetadata(prov)); err != nil {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_PROVIDER_METADATA_MISMATCH", "admin", map[string]any{
			"detail": err.Error(),
		})
		return nil, err
	}
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo:   p.Order.PaymentTradeNo,
		OrderID:   p.Order.OutTradeNo,
		RequestID: p.RequestID,
		Amount:    formatGatewayRefundAmount(p.GatewayAmount, p.Order),
		Reason:    p.Reason,
	})
	finishProviderCall()
	if err != nil {
		if resp != nil {
			switch strings.TrimSpace(resp.Status) {
			case payment.ProviderStatusPending, payment.ProviderStatusFailed:
				// A provider-supplied terminal failure is safe to roll back;
				// transport/unknown failures remain pending for reconciliation.
				return resp, nil
			}
		}
		return nil, &refundOutcomeUnknownError{err: err}
	}
	return resp, nil
}

func formatGatewayRefundAmount(amount float64, order *dbent.PaymentOrder) string {
	return payment.FormatAmountForCurrency(amount, PaymentOrderCurrency(order))
}

func validateRefundProviderResponse(resp *payment.RefundResponse) error {
	if resp == nil {
		return fmt.Errorf("payment refund response missing")
	}
	status := strings.TrimSpace(resp.Status)
	switch status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded, payment.ProviderStatusPending:
		return nil
	case payment.ProviderStatusFailed:
		return fmt.Errorf("payment refund failed: status %s", status)
	default:
		return fmt.Errorf("payment refund returned unknown status: %s", status)
	}
}

func (s *PaymentService) finishRefund(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	if resp == nil {
		return s.markRefundPending(ctx, p, &payment.RefundResponse{Status: payment.ProviderStatusPending})
	}
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		p.RefundID = refundResponseID(resp)
		return s.markRefundOk(ctx, p)
	case payment.ProviderStatusPending:
		return s.markRefundPending(ctx, p, resp)
	case payment.ProviderStatusFailed:
		return s.handleGwFail(ctx, p, fmt.Errorf("payment refund failed: status %s", strings.TrimSpace(resp.Status)))
	default:
		return s.markRefundPending(ctx, p, &payment.RefundResponse{RefundID: resp.RefundID, Status: payment.ProviderStatusPending})
	}
}

func (s *PaymentService) QueryAndFinalizeRefund(ctx context.Context, oid int64) (*RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status != OrderStatusRefundPending {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only refund pending orders can be finalized")
	}

	prov, err := s.getRefundProvider(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	queryProvider, ok := prov.(payment.RefundQueryProvider)
	if !ok {
		return nil, infraerrors.BadRequest("REFUND_QUERY_UNSUPPORTED", "this payment provider does not support refund status query; please verify manually")
	}

	pendingDetail, err := s.latestRefundPendingDetail(ctx, oid)
	if err != nil {
		return nil, err
	}
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := queryProvider.QueryRefund(ctx, payment.RefundQueryRequest{
		TradeNo:     o.PaymentTradeNo,
		OrderID:     o.OutTradeNo,
		RequestID:   pendingDetail.RequestID,
		RefundID:    pendingDetail.RefundID,
		Amount:      formatGatewayRefundAmount(o.RefundAmount, o),
		RequestedAt: pendingDetail.requestedAt,
	})
	finishProviderCall()
	if err != nil {
		return nil, fmt.Errorf("query refund: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("query refund returned no status; reconciliation required")
	}
	persistCtx, persistCancel := detachedRefundContext(ctx)
	defer persistCancel()
	plan := s.refundFinalizePlan(o)
	switch {
	case pendingDetail.DeductionHeld:
		pendingDetail.applyTo(plan)
	case !pendingDetail.DeductionRollbackOK:
		plan.BalanceToDeduct = 0
		plan.SubDaysToDeduct = 0
	case o.OrderType == payment.OrderTypeSubscription:
		if early := s.prepDeduct(ctx, o, plan, true); early != nil {
			return early, nil
		}
	}
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.finalizePendingRefundSuccess(persistCtx, plan)
	case payment.ProviderStatusPending:
		s.writeAuditLog(persistCtx, oid, "REFUND_QUERY_PENDING", "admin", map[string]any{"refundID": resp.RefundID})
		return &RefundResult{Success: false, Warning: "gateway refund is still pending confirmation"}, nil
	case payment.ProviderStatusFailed:
		return s.finalizeRefundFailed(persistCtx, plan, fmt.Errorf("payment refund failed: status %s", strings.TrimSpace(resp.Status)))
	default:
		return nil, fmt.Errorf("query refund returned unknown status %q; reconciliation required", strings.TrimSpace(resp.Status))
	}
}

type refundRecoveryClaim struct {
	order     *dbent.PaymentOrder
	detail    refundPendingAuditDetail
	detailErr error
}

// RecoverStaleRefunds reconciles refund attempts that were durably marked as
// REFUNDING before a process stopped. The order row and audit trail are the
// durable state machine; this method never creates a new provider refund.
func (s *PaymentService) RecoverStaleRefunds(ctx context.Context) (int, error) {
	if s == nil || s.entClient == nil {
		return 0, nil
	}
	cutoff := time.Now().Add(-refundRecoveryStaleAfter)
	orders, err := s.entClient.PaymentOrder.Query().
		Where(paymentorder.StatusEQ(OrderStatusRefunding), paymentorder.UpdatedAtLT(cutoff)).
		Order(paymentorder.ByUpdatedAt()).
		Limit(refundRecoveryBatchSize).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query stale refunds: %w", err)
	}

	recovered := 0
	var firstErr error
	for _, candidate := range orders {
		claim, claimed, claimErr := s.claimStaleRefundWithRetry(ctx, candidate)
		if claimErr != nil {
			if firstErr == nil {
				firstErr = claimErr
			}
			continue
		}
		if !claimed {
			continue
		}
		recovered++
		if claim.detailErr != nil {
			s.recordRefundRecoveryUnknown(ctx, claim.order.ID, "", claim.detailErr)
			continue
		}
		if err := s.reconcileClaimedRefund(ctx, claim); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return recovered, firstErr
}

func (s *PaymentService) claimStaleRefundWithRetry(ctx context.Context, candidate *dbent.PaymentOrder) (*refundRecoveryClaim, bool, error) {
	const maxAttempts = 4
	for attempt := 0; ; attempt++ {
		claim, claimed, err := s.claimStaleRefund(ctx, candidate)
		if err == nil || attempt >= maxAttempts-1 || !isRetryableRefundRecoveryClaimError(err) {
			return claim, claimed, err
		}
		delay := time.Duration(attempt+1) * 10 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, false, ctx.Err()
		case <-timer.C:
		}
	}
}

func isRetryableRefundRecoveryClaimError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database is deadlocked") ||
		strings.Contains(msg, "deadlock detected") ||
		strings.Contains(msg, "serialization failure")
}

func (s *PaymentService) claimStaleRefund(ctx context.Context, candidate *dbent.PaymentOrder) (*refundRecoveryClaim, bool, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin refund recovery claim: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)
	query := tx.PaymentOrder.Query().Where(paymentorder.IDEQ(candidate.ID))
	if tx.Client().Driver().Dialect() != dialect.SQLite {
		query.ForUpdate()
	}
	current, err := query.Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load refund recovery claim: %w", err)
	}
	if current.Status != OrderStatusRefunding || !current.UpdatedAt.Equal(candidate.UpdatedAt) || !current.UpdatedAt.Before(time.Now().Add(-refundRecoveryStaleAfter)) {
		return nil, false, nil
	}
	claimedAt := time.Now()
	updated, err := tx.PaymentOrder.Update().
		Where(paymentorder.IDEQ(candidate.ID), paymentorder.StatusEQ(OrderStatusRefunding)).
		SetUpdatedAt(claimedAt).
		Save(txCtx)
	if err != nil {
		return nil, false, fmt.Errorf("claim stale refund: %w", err)
	}
	if updated == 0 {
		return nil, false, nil
	}

	detail, detailErr := loadRefundSubmittingDetailTx(txCtx, tx.Client(), candidate.ID)
	worker := strings.TrimSpace(s.refundRecoveryOwner)
	if worker == "" {
		worker = "refund-recovery"
	}
	claimDetail := map[string]any{
		"worker":    worker,
		"claimedAt": claimedAt,
		"requestID": detail.RequestID,
	}
	if detailErr != nil {
		claimDetail["error"] = detailErr.Error()
	}
	if err := createPaymentAuditLogTx(txCtx, tx.Client(), candidate.ID, "REFUND_RECOVERY_CLAIMED", claimDetail); err != nil {
		return nil, false, err
	}
	current, err = tx.PaymentOrder.Get(txCtx, candidate.ID)
	if err != nil {
		return nil, false, fmt.Errorf("reload refund recovery claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit refund recovery claim: %w", err)
	}
	committed = true
	return &refundRecoveryClaim{order: current, detail: detail, detailErr: detailErr}, true, nil
}

func loadRefundSubmittingDetailTx(ctx context.Context, client *dbent.Client, oid int64) (refundPendingAuditDetail, error) {
	entry, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10)), paymentauditlog.ActionEQ("REFUND_SUBMITTING")).
		Order(paymentauditlog.ByCreatedAt(sql.OrderDesc()), paymentauditlog.ByID(sql.OrderDesc())).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return refundPendingAuditDetail{}, fmt.Errorf("refund submitting audit is missing")
		}
		return refundPendingAuditDetail{}, fmt.Errorf("load refund submitting audit: %w", err)
	}
	var detail refundPendingAuditDetail
	if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
		return refundPendingAuditDetail{}, fmt.Errorf("parse refund submitting audit: %w", err)
	}
	detail.RefundID = strings.TrimSpace(detail.RefundID)
	detail.RequestID = strings.TrimSpace(detail.RequestID)
	if detail.RequestID == "" {
		return detail, fmt.Errorf("refund submitting audit has no request ID")
	}
	detail.requestedAt = entry.CreatedAt
	return detail, nil
}

func (s *PaymentService) reconcileClaimedRefund(ctx context.Context, claim *refundRecoveryClaim) error {
	if claim == nil || claim.order == nil {
		return errors.New("refund recovery claim is empty")
	}
	p := s.refundFinalizePlan(claim.order)
	p.Order = claim.order
	p.OrderID = claim.order.ID
	p.RefundAmount = claim.detail.RefundAmount
	if p.RefundAmount <= 0 {
		p.RefundAmount = claim.order.RefundAmount
	}
	p.GatewayAmount = calculateGatewayRefundAmount(claim.order.Amount, claim.order.PayAmount, p.RefundAmount, PaymentOrderCurrency(claim.order))
	if strings.TrimSpace(claim.detail.Reason) != "" {
		p.Reason = strings.TrimSpace(claim.detail.Reason)
	}
	p.Force = claim.detail.Force
	claim.detail.applySubmittingTo(p)
	if strings.TrimSpace(claim.order.PaymentTradeNo) == "" {
		persistCtx, cancel := detachedRefundContext(ctx)
		defer cancel()
		_, err := s.finalizeRecoveredRefundSuccess(persistCtx, p)
		return err
	}

	prov, err := s.getRefundProvider(ctx, claim.order)
	if err != nil {
		s.recordRefundRecoveryUnknown(ctx, claim.order.ID, p.RequestID, err)
		return nil
	}
	if err := validateProviderSnapshotMetadata(claim.order, prov.ProviderKey(), providerMerchantIdentityMetadata(prov)); err != nil {
		s.recordRefundRecoveryUnknown(ctx, claim.order.ID, p.RequestID, err)
		return nil
	}
	queryProvider, ok := prov.(payment.RefundQueryProvider)
	if !ok {
		s.recordRefundRecoveryUnknown(ctx, claim.order.ID, p.RequestID, fmt.Errorf("provider %s does not support refund status query", prov.ProviderKey()))
		return nil
	}
	finishProviderCall := servertiming.ObserveDependency(ctx, "payment")
	resp, err := queryProvider.QueryRefund(ctx, payment.RefundQueryRequest{
		TradeNo:     claim.order.PaymentTradeNo,
		OrderID:     claim.order.OutTradeNo,
		RequestID:   p.RequestID,
		RefundID:    claim.detail.RefundID,
		Amount:      formatGatewayRefundAmount(p.GatewayAmount, claim.order),
		RequestedAt: claim.detail.requestedAt,
	})
	finishProviderCall()
	if err != nil || resp == nil {
		if err == nil {
			err = errors.New("provider returned no refund status")
		}
		s.recordRefundRecoveryUnknown(ctx, claim.order.ID, p.RequestID, err)
		return nil
	}

	persistCtx, cancel := detachedRefundContext(ctx)
	defer cancel()
	switch strings.TrimSpace(resp.Status) {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		p.RefundID = strings.TrimSpace(resp.RefundID)
		_, err = s.finalizeRecoveredRefundSuccess(persistCtx, p)
	case payment.ProviderStatusPending:
		_, err = s.markRefundPending(persistCtx, p, resp)
	case payment.ProviderStatusFailed:
		_, err = s.finalizeRecoveredRefundFailure(persistCtx, p, fmt.Errorf("payment refund failed: status %s", strings.TrimSpace(resp.Status)))
	default:
		s.recordRefundRecoveryUnknown(persistCtx, claim.order.ID, p.RequestID, fmt.Errorf("provider returned unknown refund status %q", strings.TrimSpace(resp.Status)))
		return nil
	}
	return err
}

func (s *PaymentService) recordRefundRecoveryUnknown(ctx context.Context, orderID int64, requestID string, reason error) {
	persistCtx, cancel := detachedRefundContext(ctx)
	defer cancel()
	detail := map[string]any{"requestID": strings.TrimSpace(requestID), "reason": psErrMsg(reason), "status": OrderStatusRefunding}
	s.writeAuditLog(persistCtx, orderID, "REFUND_RECOVERY_UNKNOWN", "system", detail)
}

func (s *PaymentService) finalizeRecoveredRefundSuccess(ctx context.Context, p *RefundPlan) (_ *RefundResult, err error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin recovered refund finalization: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)
	claimed, err := claimRefundOrderTransition(txCtx, tx, p.OrderID, OrderStatusRefunding, p.Order.UpdatedAt, OrderStatusRefunding)
	if err != nil {
		return nil, fmt.Errorf("claim recovered refund success: %w", err)
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "refund recovery claim changed")
	}
	if err := s.applyRefundFinalDeduction(txCtx, p); err != nil {
		return nil, err
	}
	result, err := s.markRefundOkTx(txCtx, tx.Client(), p)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit recovered refund success: %w", err)
	}
	if !p.deductionHeld && p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	}
	return result, nil
}

func (s *PaymentService) finalizeRecoveredRefundFailure(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	warning := "gateway refund failed: " + psErrMsg(gErr)
	returnErr, err := s.finalizeRefundFailedFromStatus(ctx, p, gErr, nil, OrderStatusRefunding)
	if err != nil {
		return nil, err
	}
	if returnErr == nil {
		if p.deductionHeld && p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
			s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
		}
		return &RefundResult{Success: false, Warning: warning}, nil
	}

	// The first transaction rolled back when the held deduction could not be
	// returned. Reload the pre-claim version before recording the durable
	// rollback failure, otherwise the version guard would reject the second
	// transaction and leave the order stranded in REFUNDING.
	fresh, loadErr := s.entClient.PaymentOrder.Get(ctx, p.OrderID)
	if loadErr != nil {
		return nil, fmt.Errorf("reload refund after rollback failure: %w", loadErr)
	}
	p.Order = fresh
	if _, err := s.finalizeRefundFailedFromStatus(ctx, p, gErr, returnErr, OrderStatusRefunding); err != nil {
		return nil, err
	}
	return &RefundResult{Success: false, Warning: warning + "; refund deduction rollback failed"}, nil
}

func (s *PaymentService) finalizePendingRefundSuccess(ctx context.Context, p *RefundPlan) (_ *RefundResult, err error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refund finalization: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)

	claimed, err := claimRefundOrderTransition(txCtx, tx, p.OrderID, OrderStatusRefundPending, p.Order.UpdatedAt, OrderStatusRefunding)
	if err != nil {
		return nil, fmt.Errorf("claim pending refund: %w", err)
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}

	if err := s.applyRefundFinalDeduction(txCtx, p); err != nil {
		return nil, err
	}
	result, err := s.markRefundOkTx(txCtx, tx.Client(), p)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit refund finalization: %w", err)
	}
	if !p.deductionHeld && p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	}
	return result, nil
}

func claimRefundOrderTransition(ctx context.Context, tx *dbent.Tx, orderID int64, expectedStatus string, expectedUpdatedAt time.Time, nextStatus string) (int, error) {
	query := tx.PaymentOrder.Query().Where(paymentorder.IDEQ(orderID))
	if tx.Client().Driver().Dialect() != dialect.SQLite {
		query.ForUpdate()
	}
	current, err := query.Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	if current.Status != expectedStatus || !current.UpdatedAt.Equal(expectedUpdatedAt) {
		return 0, nil
	}
	return tx.PaymentOrder.Update().
		Where(paymentorder.IDEQ(orderID), paymentorder.StatusEQ(expectedStatus)).
		SetStatus(nextStatus).
		Save(ctx)
}

func (s *PaymentService) refundFinalizePlan(o *dbent.PaymentOrder) *RefundPlan {
	refundAmount := o.RefundAmount
	reason := strings.TrimSpace(psStringValue(o.RefundReason))
	if reason == "" {
		reason = fmt.Sprintf("refund order:%d", o.ID)
	}
	return &RefundPlan{
		OrderID:       o.ID,
		Order:         o,
		RefundAmount:  refundAmount,
		GatewayAmount: calculateGatewayRefundAmount(o.Amount, o.PayAmount, refundAmount, PaymentOrderCurrency(o)),
		Reason:        reason,
		Force:         o.ForceRefund,
		DeductBalance: true,
		DeductionType: payment.DeductionTypeBalance,
		BalanceToDeduct: func() float64 {
			if o.OrderType == payment.OrderTypeBalance {
				return refundAmount
			}
			return 0
		}(),
	}
}

func (s *PaymentService) applyRefundFinalDeduction(ctx context.Context, p *RefundPlan) error {
	if p.deductionHeld {
		// 进入 pending 时扣减已保留，终态成功无需再扣。
		return nil
	}
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		deducted, err := s.deductAvailableBalance(ctx, p.Order.UserID, p.BalanceToDeduct)
		if err != nil {
			return fmt.Errorf("deduction: %w", err)
		}
		p.BalanceToDeduct = deducted
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, -p.SubDaysToDeduct); err != nil {
			if errors.Is(err, ErrAdjustWouldExpire) {
				if revokeErr := s.subscriptionSvc.RevokeSubscription(ctx, p.SubscriptionID); revokeErr != nil {
					return fmt.Errorf("revoke subscription: %w", revokeErr)
				}
			} else {
				return fmt.Errorf("deduct subscription days: %w", err)
			}
		}
	}
	return nil
}

// finalizeRefundFailed 将 REFUND_PENDING 收敛为 REFUND_FAILED，并归还进入 pending 时保留的扣减。
// 认领状态、归还扣减与审计在同一事务内完成：两者分开提交时，进程在中间退出会留下
// 已失败但扣减未归还、且没有 REFUND_ROLLBACK_FAILED 标记的订单，重试退款会再扣一次。
func (s *PaymentService) finalizeRefundFailed(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	warning := "gateway refund failed: " + psErrMsg(gErr)
	returnErr, err := s.finalizeRefundFailedFromStatus(ctx, p, gErr, nil, OrderStatusRefundPending)
	if err != nil {
		return nil, err
	}
	if returnErr == nil {
		if p.deductionHeld && p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
			s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
		}
		return &RefundResult{Success: false, Warning: warning}, nil
	}

	// 归还失败（整个事务已回滚，订单仍为 REFUND_PENDING）：仍收敛为 REFUND_FAILED，
	// 并同事务写入 REFUND_ROLLBACK_FAILED，使重试退款跳过扣减，避免二次扣减。
	slog.Error("[CRITICAL] refund failed but held deduction could not be returned", "orderID", p.OrderID, "error", returnErr)
	if _, err := s.finalizeRefundFailedFromStatus(ctx, p, gErr, returnErr, OrderStatusRefundPending); err != nil {
		return nil, err
	}
	return &RefundResult{Success: false, Warning: warning + "; refund deduction rollback failed"}, nil
}

// finalizeRefundFailedTx 在单个事务内认领 REFUND_PENDING → REFUND_FAILED。rollbackErr 为 nil 时同事务
// 归还保留的扣减，归还失败则回滚整个事务并通过 returnErr 返回；rollbackErr 非 nil 时不再归还，
// 改为同事务记录 REFUND_ROLLBACK_FAILED。
func (s *PaymentService) finalizeRefundFailedFromStatus(ctx context.Context, p *RefundPlan, gErr, rollbackErr error, expectedStatus string) (returnErr error, err error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refund failure finalization: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)

	claimed, err := claimRefundOrderTransition(txCtx, tx, p.OrderID, expectedStatus, p.Order.UpdatedAt, OrderStatusRefundFailed)
	if err != nil {
		return nil, fmt.Errorf("mark refund failed: %w", err)
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	if _, err := tx.PaymentOrder.UpdateOneID(p.OrderID).
		SetFailedAt(time.Now()).
		SetFailedReason(psErrMsg(gErr)).
		Save(txCtx); err != nil {
		return nil, fmt.Errorf("mark refund failure details: %w", err)
	}

	if p.deductionHeld && rollbackErr == nil {
		if err := s.returnHeldRefundDeduction(txCtx, p); err != nil {
			return err, nil
		}
	}
	if rollbackErr != nil {
		detail := refundFailureAuditDetail(p, gErr, rollbackErr)
		detail["rollbackError"] = psErrMsg(rollbackErr)
		if err := createPaymentAuditLogTx(txCtx, tx.Client(), p.OrderID, "REFUND_ROLLBACK_FAILED", detail); err != nil {
			return nil, err
		}
	}
	if err := createPaymentAuditLogTx(txCtx, tx.Client(), p.OrderID, "REFUND_FAILED", refundFailureAuditDetail(p, gErr, rollbackErr)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit refund failure finalization: %w", err)
	}
	committed = true
	return nil, nil
}

// returnHeldRefundDeduction 归还进入 REFUND_PENDING 时保留的扣减；ctx 携带事务时随事务提交或回滚。
func (s *PaymentService) returnHeldRefundDeduction(ctx context.Context, p *RefundPlan) error {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.UpdateBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			return fmt.Errorf("return held balance deduction: %w", err)
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, p.SubDaysToDeduct); err != nil {
			return fmt.Errorf("return held subscription deduction: %w", err)
		}
	}
	return nil
}

func createPaymentAuditLogTx(ctx context.Context, client *dbent.Client, oid int64, action string, detail map[string]any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal %s audit: %w", action, err)
	}
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(oid, 10)).
		SetAction(action).
		SetDetail(string(raw)).
		SetOperator("admin").
		Save(ctx); err != nil {
		return fmt.Errorf("write %s audit: %w", action, err)
	}
	return nil
}

// refundPendingAuditDetail 记录在 REFUND_PENDING 审计日志中。DeductionHeld 为 true 表示
// 进入 pending 时扣减被保留；旧记录没有该字段，扣减已在 pending 时归还（DeductionRollbackOK）。
type refundPendingAuditDetail struct {
	RefundID            string  `json:"refundID"`
	RequestID           string  `json:"requestID,omitempty"`
	RefundAmount        float64 `json:"refundAmount"`
	Reason              string  `json:"reason"`
	Force               bool    `json:"force"`
	DeductionRollbackOK bool    `json:"deductionRollbackOK,omitempty"`
	DeductionHeld       bool    `json:"deductionHeld,omitempty"`
	DeductionType       string  `json:"deductionType,omitempty"`
	BalanceDeducted     float64 `json:"balanceDeducted"`
	SubDaysDeducted     int     `json:"subDaysDeducted"`
	SubscriptionID      int64   `json:"subscriptionID,omitempty"`
	// requestedAt 取自审计记录的创建时间，不参与序列化。
	requestedAt time.Time
}

func (d refundPendingAuditDetail) applyTo(p *RefundPlan) {
	p.deductionHeld = d.DeductionHeld
	if refundID := strings.TrimSpace(d.RefundID); refundID != "" {
		p.RefundID = refundID
	}
	if strings.TrimSpace(p.RequestID) == "" {
		p.RequestID = d.RequestID
		if p.RequestID == "" {
			p.RequestID = d.RefundID
		}
	}
	p.DeductionType = d.DeductionType
	if p.DeductionType == "" {
		p.DeductionType = payment.DeductionTypeNone
	}
	p.BalanceToDeduct = d.BalanceDeducted
	p.SubDaysToDeduct = d.SubDaysDeducted
	p.SubscriptionID = d.SubscriptionID
}

// applySubmittingTo restores the durable hold made before the provider call.
// Older REFUND_SUBMITTING records did not persist deductionHeld, so a positive
// recorded deduction is treated as held for crash recovery.
func (d refundPendingAuditDetail) applySubmittingTo(p *RefundPlan) {
	d.applyTo(p)
	if p.DeductionType == payment.DeductionTypeNone {
		switch {
		case d.SubDaysDeducted > 0 || d.SubscriptionID > 0:
			p.DeductionType = payment.DeductionTypeSubscription
		case d.BalanceDeducted > 0:
			p.DeductionType = payment.DeductionTypeBalance
		}
	}
	if !p.deductionHeld && (d.BalanceDeducted > 0 || d.SubDaysDeducted > 0 || d.SubscriptionID > 0) {
		p.deductionHeld = true
	}
}

// latestRefundPendingDetail 读取最近一次 REFUND_PENDING 审计。没有记录视为旧订单（扣减已在
// pending 时归还）；读取或解析失败必须返回错误：误判为旧记录会让保留的扣减被重复扣减或不再归还。
func (s *PaymentService) latestRefundPendingDetail(ctx context.Context, oid int64) (refundPendingAuditDetail, error) {
	detail := refundPendingAuditDetail{DeductionRollbackOK: true}
	logEntry, err := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10)), paymentauditlog.ActionEQ("REFUND_PENDING")).
		Order(paymentauditlog.ByCreatedAt(sql.OrderDesc()), paymentauditlog.ByID(sql.OrderDesc())).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return detail, nil
		}
		return refundPendingAuditDetail{}, fmt.Errorf("load refund pending audit: %w", err)
	}
	if err := json.Unmarshal([]byte(logEntry.Detail), &detail); err != nil {
		return refundPendingAuditDetail{}, fmt.Errorf("parse refund pending audit: %w", err)
	}
	detail.RefundID = strings.TrimSpace(detail.RefundID)
	detail.requestedAt = logEntry.CreatedAt
	return detail, nil
}

// getRefundProvider creates a provider using the order's original instance config.
// Delegates to getOrderProvider which handles instance lookup and fallback.
func (s *PaymentService) getRefundProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("refund provider instance is unavailable for order %d", o.ID)
	}
	return s.createProviderFromInstance(ctx, inst)
}

func (s *PaymentService) handleGwFail(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	persistCtx, cancel := detachedRefundContext(ctx)
	defer cancel()
	ctx = persistCtx
	if s.RollbackRefund(ctx, p, gErr) {
		s.restoreStatus(ctx, p)
		s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
		return &RefundResult{Success: false, Warning: "gateway failed: " + psErrMsg(gErr) + ", rolled back"}, nil
	}
	now := time.Now()
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	s.writeAuditLog(ctx, p.OrderID, "REFUND_FAILED", "admin", refundFailureAuditDetail(p, gErr, errors.New("refund deduction rollback failed")))
	return nil, infraerrors.InternalServer("REFUND_FAILED", psErrMsg(gErr))
}

func refundFailureAuditDetail(p *RefundPlan, gErr, rollbackErr error) map[string]any {
	held := p.deductionHeld && rollbackErr != nil
	return map[string]any{
		"detail":              psErrMsg(gErr),
		"requestID":           p.RequestID,
		"deductionHeld":       held,
		"deductionRollbackOK": !held,
		"deductionType":       p.DeductionType,
		"balanceDeducted":     p.BalanceToDeduct,
		"subDaysDeducted":     p.SubDaysToDeduct,
		"subscriptionID":      p.SubscriptionID,
	}
}

func (s *PaymentService) markRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	persistCtx, cancel := detachedRefundContext(ctx)
	defer cancel()
	ctx = persistCtx
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin refund completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	result, err := s.markRefundOkTx(txCtx, tx.Client(), p)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit refund completion: %w", err)
	}
	return result, nil
}

func (s *PaymentService) markRefundOkTx(ctx context.Context, client *dbent.Client, p *RefundPlan) (*RefundResult, error) {
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	revokedDraws := 0
	if s.activityService != nil && p.Order != nil && p.Order.OrderType == payment.OrderTypeBalance {
		var err error
		revokedDraws, err = s.activityService.RevokeRechargeQualificationTx(
			ctx,
			client,
			p.OrderID,
			strconv.FormatFloat(p.RefundAmount, 'f', -1, 64),
			now,
		)
		if err != nil {
			return nil, fmt.Errorf("revoke refunded lottery qualification: %w", err)
		}
	}
	_, err := client.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(fs).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetRefundAt(now).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	detail, err := json.Marshal(map[string]any{"refundID": p.RefundID, "refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "lotteryChancesRevoked": revokedDraws, "force": p.Force})
	if err != nil {
		return nil, fmt.Errorf("marshal refund audit: %w", err)
	}
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(p.OrderID, 10)).
		SetAction("REFUND_SUCCESS").
		SetDetail(string(detail)).
		SetOperator("admin").
		Save(ctx); err != nil {
		return nil, fmt.Errorf("write refund audit: %w", err)
	}
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct}, nil
}

// markRefundPending 在网关未给出终态时保留已执行的扣减：若此时归还，用户可在 pending 期间
// 消费这部分余额，终态成功时只能扣到剩余余额。网关确认失败后由 finalizeRefundFailed 归还。
// 订单状态与审计记录同事务写入，确保终态收敛时能读到保留的扣减。
func (s *PaymentService) markRefundPending(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (_ *RefundResult, err error) {
	detailValue := refundPendingAuditDetail{
		RefundID:        refundResponseID(resp),
		RequestID:       p.RequestID,
		RefundAmount:    p.RefundAmount,
		Reason:          p.Reason,
		Force:           p.Force,
		DeductionHeld:   true,
		DeductionType:   p.DeductionType,
		BalanceDeducted: p.BalanceToDeduct,
		SubDaysDeducted: p.SubDaysToDeduct,
		SubscriptionID:  p.SubscriptionID,
	}
	detail, err := json.Marshal(detailValue)
	if err != nil {
		return nil, fmt.Errorf("marshal refund pending audit: %w", err)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin mark refund pending: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			slog.Error("[CRITICAL] gateway refund pending but order not marked", "orderID", p.OrderID, "error", err)
		}
	}()
	txCtx := dbent.NewTxContext(ctx, tx)
	updated, err := claimRefundOrderTransition(txCtx, tx, p.OrderID, OrderStatusRefunding, p.Order.UpdatedAt, OrderStatusRefundPending)
	if err != nil {
		return nil, fmt.Errorf("mark refund pending: %w", err)
	}
	if updated == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	if _, err = tx.PaymentOrder.UpdateOneID(p.OrderID).
		SetRefundAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		ClearRefundAt().
		SetForceRefund(p.Force).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx); err != nil {
		return nil, fmt.Errorf("update pending refund details: %w", err)
	}
	if _, err = tx.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(p.OrderID, 10)).
		SetAction("REFUND_PENDING").
		SetDetail(string(detail)).
		SetOperator("admin").
		Save(txCtx); err != nil {
		return nil, fmt.Errorf("write refund pending audit: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit mark refund pending: %w", err)
	}
	return &RefundResult{Success: false, Warning: "gateway refund is pending confirmation"}, nil
}

func refundResponseID(resp *payment.RefundResponse) string {
	if resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.RefundID)
}

func (s *PaymentService) RollbackRefund(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.UpdateBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			slog.Error("[CRITICAL] rollback failed", "orderID", p.OrderID, "amount", p.BalanceToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "balanceDeducted": p.BalanceToDeduct})
			return false
		}
		invalidateUserBalanceCacheOnCommit(ctx, s.balanceCacheInvalidator, p.Order.UserID)
		s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, p.SubDaysToDeduct); err != nil {
			slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
			return false
		}
	}
	return true
}

func (s *PaymentService) restoreStatus(ctx context.Context, p *RefundPlan) {
	rs := OrderStatusCompleted
	if p.OriginalStatus == OrderStatusRefundRequested || (p.OriginalStatus == "" && p.Order.Status == OrderStatusRefundRequested) {
		rs = OrderStatusRefundRequested
	}
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(rs).Save(ctx)
}
