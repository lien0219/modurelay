package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func validInvoiceInput() InvoiceInput {
	return InvoiceInput{AmountCents: 30000, InvoiceType: "vat_normal", TitleType: "personal", Title: "张三", Email: "finance@example.test"}
}

func TestInvoiceInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*InvoiceInput)
		valid  bool
	}{
		{"exact minimum", func(*InvoiceInput) {}, true},
		{"below minimum", func(i *InvoiceInput) { i.AmountCents = 29999 }, false},
		{"negative", func(i *InvoiceInput) { i.AmountCents = -30000 }, false},
		{"unsafe JSON integer", func(i *InvoiceInput) { i.AmountCents = 9007199254740992 }, false},
		{"enterprise tax missing", func(i *InvoiceInput) { i.TitleType = "enterprise" }, false},
		{"special personal", func(i *InvoiceInput) { i.InvoiceType = "vat_special" }, false},
		{"special details missing", func(i *InvoiceInput) {
			i.InvoiceType = "vat_special"
			i.TitleType = "enterprise"
			i.TaxNumber = "913100000000000000"
		}, false},
		{"complete special", func(i *InvoiceInput) {
			i.InvoiceType = "vat_special"
			i.TitleType = "enterprise"
			i.TaxNumber = "913100000000000000"
			i.Address = "地址"
			i.Phone = "01012345678"
			i.BankName = "银行"
			i.BankAccount = "12345678901234567890"
		}, true},
		{"invalid email", func(i *InvoiceInput) { i.Email = "finance" }, false},
		{"email display name", func(i *InvoiceInput) { i.Email = "Finance <finance@example.test>" }, false},
		{"long title", func(i *InvoiceInput) { i.Title = strings.Repeat("长", 201) }, false},
		{"nul", func(i *InvoiceInput) { i.Remark = "one\x00two" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := validInvoiceInput()
			tc.mutate(&input)
			err := ValidateInvoiceInput(&input)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInvoiceTerminalStatesCannotReopenAndUserOnlyCancelsPending(t *testing.T) {
	for _, terminal := range []string{"issued", "rejected", "cancelled"} {
		for _, next := range []string{"pending", "processing", "issued", "rejected", "cancelled"} {
			require.False(t, CanTransitionInvoice(terminal, next, false))
			require.False(t, CanTransitionInvoice(terminal, next, true))
		}
	}
	require.True(t, CanTransitionInvoice("pending", "cancelled", true))
	require.False(t, CanTransitionInvoice("processing", "cancelled", true))
	require.False(t, CanTransitionInvoice("pending", "issued", true))
	require.True(t, CanTransitionInvoice("processing", "issued", false))
}

func TestInvoiceCSVNeutralizesFormulasAndPreservesFinancialIdentifiers(t *testing.T) {
	for _, input := range []string{"=SUM(1,1)", " +CMD()", "\x01@SUM(A1)", "\ufeff-1", "\t=1", "safe\n=1"} {
		require.True(t, strings.HasPrefix(InvoiceCSVText(input), "'"), input)
	}
	var buf bytes.Buffer
	require.NoError(t, WriteInvoiceCSV(&buf, []*Invoice{{InvoiceInput: InvoiceInput{AmountCents: 30001, Title: " =1+1", TaxNumber: "00123456789012345678", BankAccount: "12345678901234567890", Phone: "01012345678"}, InvoiceNumber: "00001234567890123456"}}))
	require.True(t, strings.HasPrefix(buf.String(), "\ufeff"))
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(buf.String(), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "300.01", rows[1][4])
	require.Equal(t, "' =1+1", rows[1][7])
	require.Equal(t, "'00123456789012345678", rows[1][8])
	require.Equal(t, "'12345678901234567890", rows[1][13])
	require.Equal(t, "'00001234567890123456", rows[1][17])
}

func TestInvoiceRefundRemainderUsesCashCurrencyAndBonusRatio(t *testing.T) {
	now := time.Now()
	base := dbent.PaymentOrder{Amount: 60, PayAmount: 600, PaidAt: &now, OrderType: "balance", ProviderSnapshot: map[string]any{"currency": "CNY"}}
	require.Equal(t, int64(50000), InvoicePaymentRemainderCents(&base, 10))
	require.Zero(t, InvoicePaymentRemainderCents(&base, 60))
	base.ProviderSnapshot = map[string]any{"currency": "USD"}
	require.Zero(t, InvoicePaymentRemainderCents(&base, 10))
	base.ProviderSnapshot = map[string]any{"provider_key": "alipay"}
	require.Equal(t, int64(50000), InvoicePaymentRemainderCents(&base, 10))
	base.ProviderSnapshot = map[string]any{"provider_key": "stripe"}
	require.Zero(t, InvoicePaymentRemainderCents(&base, 10))
	base.ProviderSnapshot = map[string]any{"currency": "CNY"}
	base.OrderType = "subscription"
	require.Zero(t, InvoicePaymentRemainderCents(&base, 10))
}

type invoiceSettingsStub struct {
	SettingRepository
	values map[string]string
	err    error
}

func (s *invoiceSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	v, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (s *invoiceSettingsStub) Set(_ context.Context, key, value string) error {
	if s.err != nil {
		return s.err
	}
	s.values[key] = value
	return nil
}
func (s *invoiceSettingsStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	return s.values, s.err
}

func TestInvoiceFeatureFlagDefaultsOffAndInvalidatesPublicCache(t *testing.T) {
	repo := &invoiceSettingsStub{values: map[string]string{}}
	svc := NewInvoiceService(nil, repo)
	enabled, err := svc.Enabled(context.Background())
	require.NoError(t, err)
	require.False(t, enabled)
	_, err = svc.Create(context.Background(), 1, validInvoiceInput())
	require.ErrorIs(t, err, ErrInvoiceDisabled)
	called := false
	svc.SetOnUpdateCallback(func() { called = true })
	require.NoError(t, svc.SetEnabled(context.Background(), true))
	require.True(t, called)
	settings := NewSettingService(repo, &config.Config{})
	public, err := settings.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.InvoiceEnabled)
	injected, err := settings.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	require.True(t, injected.(*PublicSettingsInjectionPayload).InvoiceEnabled)
	repo.err = errors.New("settings unavailable")
	_, err = svc.Create(context.Background(), 1, validInvoiceInput())
	require.Error(t, err)
}

func TestInvoiceAdminUpdateRequiresFinancialCompletionFields(t *testing.T) {
	svc := NewInvoiceService(nil, nil)
	for _, update := range []InvoiceUpdate{{Status: "issued"}, {Status: "rejected"}, {Status: "cancelled"}, {Status: "processing", AdminRemark: strings.Repeat("x", 1001)}} {
		_, err := svc.Update(context.Background(), 1, update)
		require.Error(t, err)
	}
}

func TestRefundAmountCannotLosePrecisionBeforeInvoiceReconciliation(t *testing.T) {
	require.True(t, refundAmountFitsStorage(30.01))
	require.True(t, refundAmountFitsStorage(0))
	require.False(t, refundAmountFitsStorage(30.004))
	require.False(t, refundAmountFitsStorage(0.001))
}
