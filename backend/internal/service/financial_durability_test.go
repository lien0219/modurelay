//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingTenantUsageBilling struct{ UsageBillingRepository }

func (*failingTenantUsageBilling) ApplyTenantUsage(context.Context, *UsageBillingCommand, *UsageLog) (*UsageBillingApplyResult, error) {
	return nil, errors.New("temporary tenant billing failure")
}

func TestTenantBillingFailureDoesNotCreateFalseUsageReceipt(t *testing.T) {
	for _, protocol := range []string{"gateway", "openai"} {
		t.Run(protocol, func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{}
			billing := &failingTenantUsageBilling{}
			key := &APIKey{ID: 5, UserID: 6, BillingPrincipal: &User{ID: 8}, Tenant: &TenantContext{
				WorkspaceID: 9, ProjectID: 10, BillingPrincipalUserID: 8, BudgetReservationID: "b684554b-c6c8-451b-91d8-5a2c771c1a64",
			}}
			var err error
			if protocol == "gateway" {
				s := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				err = s.RecordUsage(context.Background(), &RecordUsageInput{Result: &ForwardResult{RequestID: "tenant-failure", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 10, OutputTokens: 6}}, APIKey: key, User: &User{ID: 6}, Account: &Account{ID: 7}})
			} else {
				s := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				err = s.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "tenant-failure", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 10, OutputTokens: 6}}, APIKey: key, User: &User{ID: 6}, Account: &Account{ID: 7}})
			}
			require.ErrorContains(t, err, "temporary tenant billing failure")
			require.Zero(t, logs.calls, "a failed tenant transaction must not poison immutable allocation evidence with a false zero-cost receipt")
		})
	}
}

func TestFrozenVideoRecoveryDoesNotRequireDeletedProviderOrKey(t *testing.T) {
	gateway, cache, _, billing, input, _ := videoRecoveryEdgeFixture(t)
	billing.err = errors.New("temporary billing failure")
	require.Error(t, gateway.RecordUsage(context.Background(), input))
	first := *billing.lastCmd
	billing.err = nil
	gateway.accountRepo = &openAIRecordUsageAccountRepoStub{}
	gateway.billingCacheService.apiKeyRateLimitLoader = &deletedVideoAPIKeyLoader{}
	terminal, err := gateway.recoverVideoBillingKey(context.Background(), cache.key)
	require.NoError(t, err, "the frozen accounting command must recover without live provider credentials or key data")
	require.True(t, terminal)
	require.Equal(t, first.RequestFingerprint, billing.lastCmd.RequestFingerprint)
	require.Equal(t, first.BalanceCost, billing.lastCmd.BalanceCost)
}

func TestImageProviderStartPersistenceFailurePreventsUpstreamSubmission(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"draw"}`)
	c, _ := newOpenAIImagesTestContext(t, body)
	upstream := &httpUpstreamRecorder{resp: openAIImagesJSONResponse()}
	s := newOpenAIImagesTestService(upstream)
	parsed, err := s.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	storeErr := errors.New("durable media store unavailable")
	ctx := context.WithValue(context.Background(), mediaProviderStartContextKey{}, func(context.Context, int64) error { return storeErr })
	_, err = s.ForwardImages(ctx, c, directImagesTestAccount(), body, parsed, "")
	require.ErrorIs(t, err, storeErr)
	require.Nil(t, upstream.lastReq, "never send an image request after its durable start marker fails")
}
