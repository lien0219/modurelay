//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type recoveryRefundProvider struct {
	refundProviderTestDouble
	mu            sync.Mutex
	queryResponse *payment.RefundResponse
	queryErr      error
	lastQuery     payment.RefundQueryRequest
	queryCalls    atomic.Int32
	refundCalls   atomic.Int32
}

func (p *recoveryRefundProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	p.refundCalls.Add(1)
	return nil, errors.New("recovery must never submit a second refund")
}

func (p *recoveryRefundProvider) QueryRefund(_ context.Context, req payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	p.queryCalls.Add(1)
	p.mu.Lock()
	p.lastQuery = req
	p.mu.Unlock()
	return p.queryResponse, p.queryErr
}

func TestRecoverStaleRefundFinalizesSuccessAfterCrash(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createStaleRefundingOrderForTest(t, ctx, client, "recover-success")
	provider := &recoveryRefundProvider{
		queryResponse: &payment.RefundResponse{RefundID: "re_provider_success", Status: payment.ProviderStatusSuccess},
	}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	wallet := &refundHoldWallet{balance: 40}
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}

	recovered, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.EqualValues(t, 1, provider.queryCalls.Load())
	require.Zero(t, provider.refundCalls.Load())

	provider.mu.Lock()
	require.Equal(t, "recover-request", provider.lastQuery.RequestID)
	provider.mu.Unlock()
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 40.0, wallet.balance)
	require.Equal(t, 1, countRefundAuditsForTest(t, ctx, client, order.ID, "REFUND_SUCCESS"))
}

func TestRecoverStaleRefundFailureReturnsHeldBalanceOnce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createStaleRefundingOrderForTest(t, ctx, client, "recover-failed")
	provider := &recoveryRefundProvider{
		queryResponse: &payment.RefundResponse{RefundID: "re_provider_failed", Status: payment.ProviderStatusFailed},
	}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	wallet := &refundHoldWallet{balance: 40}
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}

	recovered, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.Equal(t, 100.0, wallet.balance)
	require.Equal(t, 1, wallet.restoreCalls)
	require.Equal(t, OrderStatusRefundFailed, mustPaymentOrderStatus(t, ctx, client, order.ID))

	second, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Zero(t, second)
	require.Equal(t, 1, wallet.restoreCalls)
	require.EqualValues(t, 1, provider.queryCalls.Load())
}

func TestRecoverStaleRefundPendingTransitionsWithoutSubmittingAgain(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createStaleRefundingOrderForTest(t, ctx, client, "recover-pending")
	provider := &recoveryRefundProvider{
		queryResponse: &payment.RefundResponse{RefundID: "re_provider_pending", Status: payment.ProviderStatusPending},
	}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	wallet := &refundHoldWallet{balance: 40}
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}

	recovered, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.Equal(t, OrderStatusRefundPending, mustPaymentOrderStatus(t, ctx, client, order.ID))
	require.Zero(t, provider.refundCalls.Load())
	require.Zero(t, wallet.restoreCalls)
	require.Equal(t, 1, countRefundAuditsForTest(t, ctx, client, order.ID, "REFUND_PENDING"))
}

func TestRecoverStaleRefundUnknownOutcomeStaysRecoverable(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createStaleRefundingOrderForTest(t, ctx, client, "recover-unknown")
	provider := &recoveryRefundProvider{queryErr: context.DeadlineExceeded}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}
	recovered, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.Equal(t, OrderStatusRefunding, mustPaymentOrderStatus(t, ctx, client, order.ID))
	require.EqualValues(t, 1, provider.queryCalls.Load())
	require.Equal(t, 1, countRefundAuditsForTest(t, ctx, client, order.ID, "REFUND_RECOVERY_UNKNOWN"))

	// The database claim lease prevents an immediate duplicate query, and no
	// new upstream refund is ever submitted for the ambiguous result.
	second, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Zero(t, second)
	require.EqualValues(t, 1, provider.queryCalls.Load())
	require.Zero(t, provider.refundCalls.Load())

	// Once the persisted lease expires, the same request may be queried again;
	// it still must never be submitted as a second refund.
	_, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetUpdatedAt(time.Now().Add(-refundRecoveryStaleAfter - time.Minute)).
		Save(ctx)
	require.NoError(t, err)
	third, err := svc.RecoverStaleRefunds(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, third)
	require.EqualValues(t, 2, provider.queryCalls.Load())
	require.Zero(t, provider.refundCalls.Load())
}

func TestRecoverStaleRefundConditionalClaimAllowsOneWorker(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createStaleRefundingOrderForTest(t, ctx, client, "recover-claim")
	provider := &recoveryRefundProvider{
		queryResponse: &payment.RefundResponse{RefundID: "re_provider_claim", Status: payment.ProviderStatusPending},
	}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	first := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}
	second := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}
	start := make(chan struct{})
	results := make(chan int, 2)
	errorsCh := make(chan error, 2)
	for _, svc := range []*PaymentService{first, second} {
		go func(svc *PaymentService) {
			<-start
			n, err := svc.RecoverStaleRefunds(ctx)
			results <- n
			errorsCh <- err
		}(svc)
	}
	close(start)
	for i := 0; i < 2; i++ {
		require.NoError(t, <-errorsCh)
	}
	require.Equal(t, 1, (<-results)+(<-results))
	require.EqualValues(t, 1, provider.queryCalls.Load())
	require.Equal(t, OrderStatusRefundPending, mustPaymentOrderStatus(t, ctx, client, order.ID))
}

func createStaleRefundingOrderForTest(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.PaymentOrder {
	t.Helper()
	order := createCompletedRefundableOrderForTest(t, ctx, client, suffix)
	old := time.Now().Add(-refundRecoveryStaleAfter - time.Minute)
	order, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusRefunding).
		SetRefundAmount(60).
		SetRefundReason("recovery test").
		SetUpdatedAt(old).
		Save(ctx)
	require.NoError(t, err)
	require.NoError(t, createPaymentAuditLogTx(ctx, client, order.ID, "REFUND_SUBMITTING", map[string]any{
		"requestID":       "recover-request",
		"refundAmount":    60,
		"reason":          "recovery test",
		"force":           true,
		"deductionHeld":   true,
		"deductionType":   payment.DeductionTypeBalance,
		"balanceDeducted": 60,
	}))
	return order
}

func mustPaymentOrderStatus(t *testing.T, ctx context.Context, client *dbent.Client, orderID int64) string {
	t.Helper()
	order, err := client.PaymentOrder.Get(ctx, orderID)
	require.NoError(t, err)
	return order.Status
}
