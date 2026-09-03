//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type redeemActivityQualifierStub struct {
	calls    int
	inTx     bool
	input    BalanceRedeemQualificationInput
	granted  int
	grantErr error
}

func (s *redeemActivityQualifierStub) GrantBalanceRedeemQualificationTx(ctx context.Context, executor ActivityTxExecutor, input BalanceRedeemQualificationInput) (int, error) {
	s.calls++
	s.inTx = executor != nil
	s.input = input
	return s.granted, s.grantErr
}

func TestRedeemPositiveBalanceGrantsActivityQualification(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	redeemRepo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{
		"BALANCE-100": {
			ID:     201,
			Code:   "BALANCE-100",
			Type:   RedeemTypeBalance,
			Value:  100,
			Status: StatusUnused,
		},
	}}
	userRepo := &mockUserRepo{getByIDUser: &User{ID: 7}}
	qualifier := &redeemActivityQualifierStub{granted: 2}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil, qualifier)

	result, err := svc.Redeem(ctx, 7, "BALANCE-100")

	require.NoError(t, err)
	require.Equal(t, StatusUsed, result.Status)
	require.Equal(t, 1, qualifier.calls)
	require.True(t, qualifier.inTx)
	require.Equal(t, int64(201), qualifier.input.RedeemCodeID)
	require.Equal(t, int64(7), qualifier.input.UserID)
	require.Equal(t, "100", qualifier.input.RechargeAmount)
	require.Equal(t, "CNY", qualifier.input.Currency)
	require.False(t, qualifier.input.RedeemedAt.IsZero())
}

func TestRedeemPaymentCodeCanSkipActivityQualification(t *testing.T) {
	ctx := ContextSkipRedeemActivityQualification(context.Background())
	client := newPaymentOrderLifecycleTestClient(t)
	redeemRepo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{
		"PAYMENT-BALANCE-100": {
			ID:     202,
			Code:   "PAYMENT-BALANCE-100",
			Type:   RedeemTypeBalance,
			Value:  100,
			Status: StatusUnused,
		},
	}}
	userRepo := &mockUserRepo{getByIDUser: &User{ID: 7}}
	qualifier := &redeemActivityQualifierStub{}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil, qualifier)

	result, err := svc.Redeem(ctx, 7, "PAYMENT-BALANCE-100")

	require.NoError(t, err)
	require.Equal(t, StatusUsed, result.Status)
	require.Zero(t, qualifier.calls)
}

func TestRedeemConcurrencyCodeDoesNotGrantActivityQualification(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	redeemRepo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{
		"CONCURRENCY-5": {
			ID:     203,
			Code:   "CONCURRENCY-5",
			Type:   RedeemTypeConcurrency,
			Value:  5,
			Status: StatusUnused,
		},
	}}
	userRepo := &mockUserRepo{getByIDUser: &User{ID: 7}}
	qualifier := &redeemActivityQualifierStub{}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil, qualifier)

	result, err := svc.Redeem(ctx, 7, "CONCURRENCY-5")

	require.NoError(t, err)
	require.Equal(t, StatusUsed, result.Status)
	require.Zero(t, qualifier.calls)
}
