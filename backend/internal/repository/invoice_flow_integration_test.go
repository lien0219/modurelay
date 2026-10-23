//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func invoiceFlowService(t *testing.T) *service.InvoiceService {
	t.Helper()
	settings := NewSettingRepository(testEntClient(t))
	previous, err := settings.GetValue(context.Background(), service.SettingKeyInvoiceEnabled)
	require.NoError(t, err)
	require.NoError(t, settings.Set(context.Background(), service.SettingKeyInvoiceEnabled, "true"))
	t.Cleanup(func() {
		require.NoError(t, settings.Set(context.Background(), service.SettingKeyInvoiceEnabled, previous))
	})
	return service.NewInvoiceService(NewInvoiceRepository(integrationDB), settings)
}

func invoiceFlowUser(t *testing.T) int64 {
	t.Helper()
	var id int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users (email,password_hash,balance,total_recharged)
		VALUES ($1,'integration-test-only',999999,999999) RETURNING id`,
		fmt.Sprintf("invoice-%d@example.test", time.Now().UnixNano())).Scan(&id))
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM invoice_applications WHERE user_id=$1`, id)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`DELETE FROM payment_orders WHERE user_id=$1`, id)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`DELETE FROM users WHERE id=$1`, id)
		require.NoError(t, err)
	})
	return id
}

func invoiceFlowPayment(t *testing.T, userID int64, paid, wallet, refund, currency, status, orderType string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO payment_orders
		(user_id,amount,pay_amount,refund_amount,provider_snapshot,status,order_type,expires_at,paid_at,completed_at)
		VALUES ($1,$2,$3,$4,jsonb_build_object('currency',$5::text),$6,$7,NOW(),NOW(),NOW()) RETURNING id`,
		userID, wallet, paid, refund, currency, status, orderType).Scan(&id))
	return id
}

func invoiceFlowInput(amount int64) service.InvoiceInput {
	return service.InvoiceInput{AmountCents: amount, InvoiceType: "vat_normal", TitleType: "enterprise",
		Title: "发票集成测试企业", TaxNumber: "913100001234567890", Email: "finance@example.test"}
}

func TestInvoiceFlowThresholdOwnershipAndDisabledHistory(t *testing.T) {
	svc := invoiceFlowService(t)
	ctx := context.Background()
	userID := invoiceFlowUser(t)
	otherID := invoiceFlowUser(t)

	// A large wallet/admin-adjusted total is not proof of CNY payment.
	_, err := svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.ErrorIs(t, err, service.ErrInvoiceFunds)
	invoiceFlowPayment(t, userID, "299.99", "1000", "0", "CNY", "COMPLETED", "balance")
	_, err = svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.ErrorIs(t, err, service.ErrInvoiceFunds)
	invoiceFlowPayment(t, userID, "0.01", "1", "0", "CNY", "COMPLETED", "balance")
	_, err = svc.Create(ctx, userID, invoiceFlowInput(29999))
	require.ErrorIs(t, err, service.ErrInvoiceAmount)
	inv, err := svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.NoError(t, err)
	require.Equal(t, "pending", inv.Status)
	require.NotEmpty(t, inv.ApplicationNo)

	_, err = svc.Get(ctx, inv.ID, otherID)
	require.ErrorIs(t, err, service.ErrInvoiceNotFound)
	_, err = svc.Cancel(ctx, inv.ID, otherID)
	require.ErrorIs(t, err, service.ErrInvoiceNotFound)
	items, total, err := svc.List(ctx, service.InvoiceFilter{UserID: otherID, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)

	require.NoError(t, svc.SetEnabled(ctx, false))
	_, err = svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.ErrorIs(t, err, service.ErrInvoiceDisabled)
	historical, err := svc.Get(ctx, inv.ID, userID)
	require.NoError(t, err)
	require.Equal(t, inv.ID, historical.ID)
	cancelled, err := svc.Cancel(ctx, inv.ID, userID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelled.Status)
	quota, err := svc.Eligibility(ctx, userID)
	require.NoError(t, err)
	require.False(t, quota.Enabled)
	require.Equal(t, int64(30000), quota.AvailableAmountCents)
}

func TestInvoiceFlowConcurrentReservationsCannotExceedPaidAmount(t *testing.T) {
	svc := invoiceFlowService(t)
	userID := invoiceFlowUser(t)
	invoiceFlowPayment(t, userID, "600", "6000", "0", "CNY", "COMPLETED", "balance")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.Create(ctx, userID, invoiceFlowInput(30000))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrInvoiceFunds)
		}
	}
	require.Equal(t, 2, success)
	quota, err := svc.Eligibility(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(60000), quota.ReservedAmountCents)
	require.Zero(t, quota.AvailableAmountCents)
}

func TestInvoiceFlowCountsOnlyNetVerifiedCNYRecharge(t *testing.T) {
	svc := invoiceFlowService(t)
	ctx := context.Background()
	userID := invoiceFlowUser(t)
	// Refunds are stored in wallet units; 10/60 of CNY 600 is CNY 100.
	invoiceFlowPayment(t, userID, "600", "60", "10", "CNY", "PARTIALLY_REFUNDED", "balance")
	invoiceFlowPayment(t, userID, "10000", "10000", "0", "USD", "COMPLETED", "balance")
	invoiceFlowPayment(t, userID, "1000", "1000", "0", "CNY", "COMPLETED", "subscription")
	invoiceFlowPayment(t, userID, "1000", "1000", "0", "CNY", "PENDING", "balance")
	invoiceFlowPayment(t, userID, "1000", "1000", "1000", "CNY", "REFUNDED", "balance")
	unknownID := invoiceFlowPayment(t, userID, "1000", "1000", "0", "", "COMPLETED", "balance")
	_, err := integrationDB.Exec(`UPDATE payment_orders SET provider_snapshot=NULL WHERE id=$1`, unknownID)
	require.NoError(t, err)
	quota, err := svc.Eligibility(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(50000), quota.TotalRechargedCents)
	require.Equal(t, int64(50000), quota.AvailableAmountCents)
	_, err = svc.Create(ctx, userID, invoiceFlowInput(50001))
	require.ErrorIs(t, err, service.ErrInvoiceFunds)
	_, err = svc.Create(ctx, userID, invoiceFlowInput(50000))
	require.NoError(t, err)
}

func TestInvoiceFlowReviewReleasesRejectedAndPreservesIssuedQuota(t *testing.T) {
	svc := invoiceFlowService(t)
	ctx := context.Background()
	userID := invoiceFlowUser(t)
	invoiceFlowPayment(t, userID, "300", "30", "0", "CNY", "COMPLETED", "balance")
	inv, err := svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.NoError(t, err)
	_, err = svc.Update(ctx, inv.ID, service.InvoiceUpdate{Status: "processing"})
	require.NoError(t, err)
	_, err = svc.Cancel(ctx, inv.ID, userID)
	require.ErrorIs(t, err, service.ErrInvoiceStatus)
	_, err = svc.Update(ctx, inv.ID, service.InvoiceUpdate{Status: "rejected", AdminRemark: "请核实税号"})
	require.NoError(t, err)
	inv, err = svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.NoError(t, err)
	issued, err := svc.Update(ctx, inv.ID, service.InvoiceUpdate{Status: "issued", InvoiceNumber: "00012345678901234567"})
	require.NoError(t, err)
	require.NotNil(t, issued.ProcessedAt)
	_, err = svc.Update(ctx, inv.ID, service.InvoiceUpdate{Status: "rejected", AdminRemark: "late rejection"})
	require.ErrorIs(t, err, service.ErrInvoiceStatus)
	_, err = svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.ErrorIs(t, err, service.ErrInvoiceFunds)
	_, err = integrationDB.Exec(`UPDATE invoice_applications SET amount_cents=29999 WHERE id=$1`, inv.ID)
	require.Error(t, err, "database must independently reject an amount below 300 CNY")
}

func TestInvoiceFlowDomesticProviderWithoutCurrencySnapshot(t *testing.T) {
	svc := invoiceFlowService(t)
	userID := invoiceFlowUser(t)
	for _, provider := range []string{"alipay", "wxpay", "easypay"} {
		id := invoiceFlowPayment(t, userID, "100", "10", "0", "", "COMPLETED", "balance")
		_, err := integrationDB.Exec(`UPDATE payment_orders SET provider_snapshot=jsonb_build_object('provider_key',$2::text) WHERE id=$1`, id, provider)
		require.NoError(t, err)
	}
	quota, err := svc.Eligibility(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, int64(30000), quota.AvailableAmountCents)
}

func TestInvoiceFlowFailedRefundRestoresEligibilityAndUnknownPendingStaysHeld(t *testing.T) {
	svc := invoiceFlowService(t)
	userID := invoiceFlowUser(t)
	invoiceFlowPayment(t, userID, "300", "30", "30", "CNY", "REFUND_FAILED", "balance")
	// Legacy pending refunds without a stored amount must remain fully reserved.
	invoiceFlowPayment(t, userID, "600", "60", "0", "CNY", "REFUND_PENDING", "balance")
	quota, err := svc.Eligibility(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, int64(30000), quota.AvailableAmountCents)
	_, err = svc.Create(context.Background(), userID, invoiceFlowInput(30000))
	require.NoError(t, err)
}

func TestInvoiceFlowPartialRefundOnlyReservesItsCashAmount(t *testing.T) {
	svc := invoiceFlowService(t)
	ctx := context.Background()
	userID := invoiceFlowUser(t)
	orderID := invoiceFlowPayment(t, userID, "600", "60", "0", "CNY", "COMPLETED", "balance")
	_, err := svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.NoError(t, err)
	repo := NewInvoiceRepository(integrationDB)
	tx, err := testEntClient(t).Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	order, err := tx.PaymentOrder.Get(ctx, orderID)
	require.NoError(t, err)
	// A CNY 400 refund would leave only CNY 200 behind an existing CNY 300 invoice.
	require.ErrorIs(t, repo.BeforeRefund(ctx, tx.Client(), order, 40), service.ErrInvoiceRefund)
	// CNY 100 can still be refunded; this must not block every refund on the account.
	require.NoError(t, repo.BeforeRefund(ctx, tx.Client(), order, 10))
	_, err = tx.PaymentOrder.UpdateOneID(orderID).SetRefundAmount(10).SetStatus("REFUNDING").Save(ctx)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	quota, err := svc.Eligibility(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(50000), quota.TotalRechargedCents)
	require.Equal(t, int64(20000), quota.AvailableAmountCents)
	_, err = svc.Create(ctx, userID, invoiceFlowInput(30000))
	require.ErrorIs(t, err, service.ErrInvoiceFunds)
}

func TestInvoiceFlowConcurrentRefundAndApplicationCannotSpendSameCash(t *testing.T) {
	svc := invoiceFlowService(t)
	userID := invoiceFlowUser(t)
	orderID := invoiceFlowPayment(t, userID, "300", "30", "0", "CNY", "COMPLETED", "balance")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	applicationResult := make(chan error, 1)
	refundResult := make(chan error, 1)
	go func() {
		<-start
		_, err := svc.Create(ctx, userID, invoiceFlowInput(30000))
		applicationResult <- err
	}()
	go func() {
		<-start
		tx, err := testEntClient(t).Tx(ctx)
		if err != nil {
			refundResult <- err
			return
		}
		defer tx.Rollback()
		order, err := tx.PaymentOrder.Get(ctx, orderID)
		if err == nil {
			err = NewInvoiceRepository(integrationDB).BeforeRefund(ctx, tx.Client(), order, 30)
		}
		if err == nil {
			_, err = tx.PaymentOrder.UpdateOneID(orderID).SetStatus("REFUNDING").SetRefundAmount(30).Save(ctx)
		}
		if err == nil {
			err = tx.Commit()
		}
		refundResult <- err
	}()
	close(start)
	applicationErr, refundErr := <-applicationResult, <-refundResult
	if applicationErr == nil {
		require.ErrorIs(t, refundErr, service.ErrInvoiceRefund)
	} else {
		require.ErrorIs(t, applicationErr, service.ErrInvoiceFunds)
		require.NoError(t, refundErr)
	}
}
