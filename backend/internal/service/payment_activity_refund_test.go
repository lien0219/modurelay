package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type paymentActivityServiceStub struct {
	revoked      int
	revokeErr    error
	refundAmount string
	orderID      int64
	txObserved   bool
}

func (s *paymentActivityServiceStub) CompleteBalancePayment(context.Context, CompleteActivityPaymentInput) (*ActivityPaymentCompletion, error) {
	return &ActivityPaymentCompletion{}, nil
}

func (s *paymentActivityServiceStub) RevokeRechargeQualificationTx(ctx context.Context, executor ActivityTxExecutor, orderID int64, refundAmount string, _ time.Time) (int, error) {
	s.orderID = orderID
	s.refundAmount = refundAmount
	s.txObserved = dbent.TxFromContext(ctx) != nil && executor != nil
	return s.revoked, s.revokeErr
}

func TestMarkRefundOkAtomicallyRevokesLotteryQualification(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createActivityRefundOrderForTest(t, ctx, client, "activity-refund-success")
	activity := &paymentActivityServiceStub{revoked: 2}
	svc := &PaymentService{entClient: client, activityService: activity}
	plan := &RefundPlan{
		OrderID:       order.ID,
		Order:         order,
		RefundAmount:  40,
		Reason:        "partial refund",
		DeductionType: payment.DeductionTypeBalance,
	}

	result, err := svc.markRefundOk(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, order.ID, activity.orderID)
	require.Equal(t, "40", activity.refundAmount)
	require.True(t, activity.txObserved)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	audit, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_SUCCESS")).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, audit.Detail, `"lotteryChancesRevoked":2`)
}

func TestMarkRefundOkRollsBackStatusWhenQualificationRevokeFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createActivityRefundOrderForTest(t, ctx, client, "activity-refund-rollback")
	activity := &paymentActivityServiceStub{revokeErr: errors.New("injected qualification failure")}
	svc := &PaymentService{entClient: client, activityService: activity}

	result, err := svc.markRefundOk(ctx, &RefundPlan{
		OrderID:      order.ID,
		Order:        order,
		RefundAmount: order.Amount,
		Reason:       "full refund",
	})
	require.Nil(t, result)
	require.ErrorContains(t, err, "injected qualification failure")

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, reloaded.Status)
	count, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func createActivityRefundOrderForTest(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.PaymentOrder {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("hash").
		SetUsername(suffix).
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-" + suffix).
		SetOutTradeNo("sub2_" + suffix).
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_" + suffix).
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRefundPending).
		SetRefundAmount(100).
		SetRefundReason("pending refund").
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	return order
}
