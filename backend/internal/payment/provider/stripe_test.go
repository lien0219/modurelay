//go:build unit

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v85"
)

type stripeRefundBackend struct {
	params  []*stripe.RefundCreateParams
	paths   []string
	refunds []*stripe.Refund
}

func (b *stripeRefundBackend) Call(_ string, _ string, path string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	b.paths = append(b.paths, path)
	if createParams, ok := params.(*stripe.RefundCreateParams); ok {
		b.params = append(b.params, createParams)
	}
	refund := v.(*stripe.Refund)
	refund.ID = "re_123"
	refund.Status = stripe.RefundStatusSucceeded
	return nil
}

func (*stripeRefundBackend) CallStreaming(string, string, string, stripe.ParamsContainer, stripe.StreamingLastResponseSetter) error {
	return nil
}

func (b *stripeRefundBackend) CallRaw(_ string, path string, _ string, _ []byte, _ *stripe.Params, v stripe.LastResponseSetter) error {
	b.paths = append(b.paths, path)
	raw, err := json.Marshal(map[string]any{
		"object":   "list",
		"has_more": false,
		"data":     b.refunds,
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func (*stripeRefundBackend) CallMultipart(string, string, string, string, *bytes.Buffer, *stripe.Params, stripe.LastResponseSetter) error {
	return nil
}

func (*stripeRefundBackend) SetMaxNetworkRetries(int64) {}

func TestStripeRefundUsesStableAmountSpecificIdempotencyKey(t *testing.T) {
	backend := &stripeRefundBackend{}
	client := stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))
	provider := &Stripe{
		config:      map[string]string{"currency": "CNY"},
		initialized: true,
		sc:          client,
	}

	refund := func(amount string) {
		_, err := provider.Refund(context.Background(), payment.RefundRequest{
			TradeNo: "pi_123",
			OrderID: "sub2_order_456",
			Amount:  amount,
		})
		require.NoError(t, err)
	}

	refund("12.34")
	refund("12.34")
	refund("12.35")

	require.Len(t, backend.params, 3)
	require.Equal(t, int64(1234), *backend.params[0].Amount)
	require.Equal(t, "re-sub2_order_456-1234", *backend.params[0].IdempotencyKey)
	require.Equal(t, backend.params[0].IdempotencyKey, backend.params[1].IdempotencyKey)
	require.Equal(t, int64(1235), *backend.params[2].Amount)
	require.Equal(t, "re-sub2_order_456-1235", *backend.params[2].IdempotencyKey)
	require.NotEqual(t, *backend.params[0].IdempotencyKey, *backend.params[2].IdempotencyKey)
}

func TestStripeQueryRefundRejectsSyntheticRequestIDAsResourceID(t *testing.T) {
	backend := &stripeRefundBackend{}
	client := stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))
	provider := &Stripe{
		config:      map[string]string{"currency": "CNY"},
		initialized: true,
		sc:          client,
	}

	_, err := provider.QueryRefund(context.Background(), payment.RefundQueryRequest{
		TradeNo:  "pi_123",
		RefundID: "sub2_order_456-refund-12.34",
	})

	require.Error(t, err)
	require.Empty(t, backend.paths, "synthetic request IDs are not Stripe refund resource IDs")
}

func TestStripeRefundUsesAttemptRequestIDForIdempotency(t *testing.T) {
	backend := &stripeRefundBackend{}
	client := stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))
	provider := &Stripe{
		config:      map[string]string{"currency": "CNY"},
		initialized: true,
		sc:          client,
	}

	for _, requestID := range []string{"mr-refund-attempt-1", "mr-refund-attempt-2"} {
		_, err := provider.Refund(context.Background(), payment.RefundRequest{
			TradeNo:   "pi_123",
			OrderID:   "sub2_order_456",
			RequestID: requestID,
			Amount:    "12.34",
		})
		require.NoError(t, err)
	}

	require.Len(t, backend.params, 2)
	require.Equal(t, "modurelay-refund-mr-refund-attempt-1", *backend.params[0].IdempotencyKey)
	require.Equal(t, "modurelay-refund-mr-refund-attempt-2", *backend.params[1].IdempotencyKey)
	require.Equal(t, "mr-refund-attempt-1", backend.params[0].Metadata["modurelay_request_id"])
	require.Equal(t, "mr-refund-attempt-2", backend.params[1].Metadata["modurelay_request_id"])
}

func TestStripeQueryRefundFindsAttemptByRequestID(t *testing.T) {
	backend := &stripeRefundBackend{refunds: []*stripe.Refund{{
		ID:       "re_attempt_2",
		Status:   stripe.RefundStatusSucceeded,
		Metadata: map[string]string{"modurelay_request_id": "mr-refund-attempt-2"},
	}}}
	client := stripe.NewClient("sk_test", stripe.WithBackends(&stripe.Backends{API: backend}))
	provider := &Stripe{
		config:      map[string]string{"currency": "CNY"},
		initialized: true,
		sc:          client,
	}

	resp, err := provider.QueryRefund(context.Background(), payment.RefundQueryRequest{
		TradeNo:   "pi_123",
		RequestID: "mr-refund-attempt-2",
	})

	require.NoError(t, err)
	require.Equal(t, "re_attempt_2", resp.RefundID)
	require.Equal(t, payment.ProviderStatusSuccess, resp.Status)
}
