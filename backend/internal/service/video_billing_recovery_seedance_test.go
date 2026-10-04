package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoRecoveryResultBillablePreservesSeedanceBillingModes(t *testing.T) {
	tests := []struct {
		name    string
		pending GrokVideoPendingBilling
		result  OpenAIForwardResult
		want    bool
	}{
		{
			name: "first class seedance bills a playable video without completion tokens",
			pending: GrokVideoPendingBilling{
				RequestID: "seedance:task-1", QuotaPlatform: PlatformSeedance, NativeProtocol: true,
			},
			result: OpenAIForwardResult{VideoCount: 1},
			want:   true,
		},
		{
			name: "first class seedance does not bill tokens without a playable video",
			pending: GrokVideoPendingBilling{
				RequestID: "seedance:task-1", QuotaPlatform: PlatformSeedance, NativeProtocol: true,
			},
			result: OpenAIForwardResult{Usage: OpenAIUsage{OutputTokens: 100}},
			want:   false,
		},
		{
			name: "legacy OpenAI Seedance keeps token billing",
			pending: GrokVideoPendingBilling{
				RequestID: "seedance:task-1", QuotaPlatform: PlatformOpenAI, NativeProtocol: true,
			},
			result: OpenAIForwardResult{Usage: OpenAIUsage{OutputTokens: 100}, VideoCount: 1},
			want:   true,
		},
		{
			name: "legacy Seedance playable video can use explicit video pricing",
			pending: GrokVideoPendingBilling{
				RequestID: "seedance:task-1", QuotaPlatform: PlatformOpenAI, NativeProtocol: true,
			},
			result: OpenAIForwardResult{VideoCount: 1},
			want:   true,
		},
		{
			name: "failed Seedance result is not billable",
			pending: GrokVideoPendingBilling{
				RequestID: "seedance:task-1", QuotaPlatform: PlatformSeedance,
			},
			result: OpenAIForwardResult{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, videoRecoveryResultBillable(&tt.pending, &tt.result))
		})
	}
}

func TestVideoRecoveryTreatsDeletedTaskAsTerminal(t *testing.T) {
	require.True(t, videoRecoveryTerminalStatus([]byte(`{"status":"deleted"}`)))
}
