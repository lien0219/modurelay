//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type uncertainRefundProvider struct {
	refundProviderTestDouble
	response    *payment.RefundResponse
	err         error
	onRefund    func()
	lastRequest payment.RefundRequest
}

func (p *uncertainRefundProvider) Refund(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	p.lastRequest = req
	if p.onRefund != nil {
		p.onRefund()
	}
	return p.response, p.err
}

type sequencedRefundProvider struct {
	refundProviderTestDouble
	responses []*payment.RefundResponse
	errs      []error
	requests  []payment.RefundRequest
}

func (p *sequencedRefundProvider) Refund(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	p.requests = append(p.requests, req)
	index := len(p.requests) - 1
	return p.responses[index], p.errs[index]
}

type blockingRefundProvider struct {
	refundProviderTestDouble
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (p *blockingRefundProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	if p.calls.Add(1) == 1 {
		close(p.started)
	}
	<-p.release
	return &payment.RefundResponse{RefundID: "re_blocked", Status: payment.ProviderStatusPending}, nil
}

func TestRefundQueryUnknownOutcomeRetainsHold(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *payment.RefundResponse
	}{
		{name: "empty response"},
		{name: "unknown status", response: &payment.RefundResponse{Status: "unrecognized"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPendingRefundOrderWithDetailForTest(t, ctx, client, "query-unknown", `{"deductionHeld":true,"deductionType":"balance","balanceDeducted":60}`)
			wallet := &refundHoldWallet{balance: 40}
			svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
			restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{refundResponse: tc.response})
			defer restore()
			_, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
			require.Error(t, err)
			current, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusRefundPending, current.Status)
			require.Equal(t, 40.0, wallet.balance)
		})
	}
}

func TestRefundDispatchUnknownOutcomeRetainsHoldAndBlocksRetry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *payment.RefundResponse
		err      error
	}{
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "empty"},
		{name: "unknown", response: &payment.RefundResponse{Status: "unrecognized"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createCompletedRefundableOrderForTest(t, ctx, client, "dispatch-unknown")
			wallet := &refundHoldWallet{balance: 100}
			svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
			restore := replacePaymentProviderFactoryForTest(t, &uncertainRefundProvider{response: tc.response, err: tc.err})
			defer restore()
			plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "refund", false, true)
			require.NoError(t, err)
			require.Nil(t, early)
			result, err := svc.ExecuteRefund(ctx, plan)
			require.NoError(t, err)
			require.False(t, result.Success)
			current, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusRefundPending, current.Status)
			require.Zero(t, wallet.balance)
			pending, err := svc.latestRefundPendingDetail(ctx, order.ID)
			require.NoError(t, err)
			require.Empty(t, pending.RefundID, "an unknown provider result must not persist the local request ID as a provider refund ID")
			_, _, err = svc.PrepareRefund(ctx, order.ID, 0, "retry", true, true)
			require.Equal(t, "INVALID_STATUS", infraerrors.Reason(err))
		})
	}
}

func TestRefundConfirmedProviderFailureAllowsNewIdempotentAttempt(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "confirmed-failure-retry")
	wallet := &refundHoldWallet{balance: 100}
	provider := &sequencedRefundProvider{
		responses: []*payment.RefundResponse{
			{Status: payment.ProviderStatusFailed},
			{RefundID: "re_retry_success", Status: payment.ProviderStatusSuccess},
		},
		errs: []error{errors.New("provider confirmed refund failure"), nil},
	}
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "first attempt", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	first, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.False(t, first.Success)
	current, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, current.Status)
	require.Equal(t, 100.0, wallet.balance)

	plan, early, err = svc.PrepareRefund(ctx, order.ID, 0, "retry", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	second, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, second.Success)
	require.Len(t, provider.requests, 2)
	require.NotEmpty(t, provider.requests[0].RequestID)
	require.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, provider.requests[0].RequestID)
	require.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, provider.requests[1].RequestID)
	require.NotEqual(t, provider.requests[0].RequestID, provider.requests[1].RequestID)
	require.Zero(t, wallet.balance)
	current, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, current.Status)
}

func TestRefundClaimAndDeductionRollbackTogether(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "atomic-claim")
	_, err := client.User.UpdateOneID(order.UserID).SetBalance(100).Save(ctx)
	require.NoError(t, err)
	svc := &PaymentService{entClient: client, userRepo: &mockUserRepo{
		deductAvailableBalanceFn: func(ctx context.Context, id int64, amount float64) (float64, error) {
			writeClient := client
			if tx := dbent.TxFromContext(ctx); tx != nil {
				writeClient = tx.Client()
			}
			_, err := writeClient.User.UpdateOneID(id).AddBalance(-amount).Save(ctx)
			require.NoError(t, err)
			return 0, errors.New("injected failure after debit")
		},
	}}
	_, err = svc.ExecuteRefund(ctx, &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: 100, DeductionType: payment.DeductionTypeBalance, BalanceToDeduct: 100})
	require.ErrorContains(t, err, "injected failure after debit")
	user, err := client.User.Get(ctx, order.UserID)
	require.NoError(t, err)
	require.Equal(t, 100.0, user.Balance)
	current, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, current.Status)
}

func TestRefundConcurrentServiceInstancesSubmitOnce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "concurrent-submit")
	wallet := &refundHoldWallet{balance: 100}
	provider := &blockingRefundProvider{started: make(chan struct{}), release: make(chan struct{})}
	restore := replacePaymentProviderFactoryForTest(t, provider)
	defer restore()

	svcA := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	svcB := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	planA, early, err := svcA.PrepareRefund(ctx, order.ID, 0, "instance A", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	planB, early, err := svcB.PrepareRefund(ctx, order.ID, 0, "instance B", false, true)
	require.NoError(t, err)
	require.Nil(t, early)

	type execution struct {
		result *RefundResult
		err    error
	}
	first := make(chan execution, 1)
	go func() {
		result, err := svcA.ExecuteRefund(ctx, planA)
		first <- execution{result: result, err: err}
	}()
	<-provider.started
	secondResult, secondErr := svcB.ExecuteRefund(ctx, planB)
	close(provider.release)
	firstResult := <-first

	require.NoError(t, firstResult.err)
	require.False(t, firstResult.result.Success)
	require.Nil(t, secondResult)
	require.Equal(t, "CONFLICT", infraerrors.Reason(secondErr))
	require.EqualValues(t, 1, provider.calls.Load())
	require.Equal(t, 1, wallet.deductCalls)
	require.Zero(t, wallet.balance)
	current, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, current.Status)
}

func TestRefundStaleQueryCannotReleaseANewerAttempt(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	oldOrder := createPendingRefundOrderWithDetailForTest(t, ctx, client, "stale-attempt", `{"deductionHeld":true,"deductionType":"balance","balanceDeducted":60}`)
	wallet := &refundHoldWallet{balance: 40}
	svc := &PaymentService{entClient: client, userRepo: wallet.repo(oldOrder.UserID)}
	oldPlan := svc.refundFinalizePlan(oldOrder)
	detail, err := svc.latestRefundPendingDetail(ctx, oldOrder.ID)
	require.NoError(t, err)
	detail.applyTo(oldPlan)
	_, err = client.PaymentOrder.UpdateOneID(oldOrder.ID).
		SetRefundReason("new attempt").
		SetUpdatedAt(oldOrder.UpdatedAt.Add(time.Second)).
		Save(ctx)
	require.NoError(t, err)
	_, err = svc.finalizeRefundFailed(ctx, oldPlan, errors.New("old query failed"))
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
	current, err := client.PaymentOrder.Get(ctx, oldOrder.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, current.Status)
	require.Equal(t, 40.0, wallet.balance)
}

func TestRefundCanceledCallerStillPersistsProviderOutcome(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "canceled-after-dispatch")
	wallet := &refundHoldWallet{balance: 100}
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	restore := replacePaymentProviderFactoryForTest(t, &uncertainRefundProvider{response: &payment.RefundResponse{Status: payment.ProviderStatusPending}, onRefund: cancel})
	defer restore()
	plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "refund", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	_, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	current, err := client.PaymentOrder.Get(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, current.Status)
}

func TestRefundRetryPreservesOutstandingHoldForLaterFailure(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderWithDetailForTest(t, ctx, client, "retry-held-evidence", `{"deductionHeld":true,"deductionType":"balance","balanceDeducted":60}`)
	wallet := &refundHoldWallet{balance: 40}
	repo := wallet.repo(order.UserID)
	repo.updateBalanceFn = func(context.Context, int64, float64) error { return errors.New("return temporarily failed") }
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: repo}
	prov := &pendingRefundProviderTestDouble{queryStatus: payment.ProviderStatusFailed}
	restore := replacePaymentProviderFactoryForTest(t, prov)
	defer restore()
	_, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	svc.userRepo = wallet.repo(order.UserID)
	plan, early, err := svc.PrepareRefund(ctx, order.ID, 0, "retry", true, true)
	require.NoError(t, err)
	require.Nil(t, early)
	_, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	_, err = svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 100.0, wallet.balance, "a failed retry must return the original 60 held by the prior attempt")
	require.Zero(t, wallet.deductCalls)
	// Once returned, an old rollback-failed audit must not suppress a later deduction.
	svc.userRepo = wallet.repo(order.UserID)
	plan, early, err = svc.PrepareRefund(ctx, order.ID, 0, "fresh retry", false, true)
	require.NoError(t, err)
	require.Nil(t, early)
	require.Equal(t, 100.0, plan.BalanceToDeduct)
	_, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.Zero(t, wallet.balance)
}
