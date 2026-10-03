//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedanceSchedulingEligibilityIsVideoCapabilityOnly(t *testing.T) {
	seedance := &Account{
		Platform: PlatformSeedance,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "key",
			"base_url": "https://provider.example/api/v3",
		},
	}
	require.True(t, isOpenAIEndpointPlatformCompatible(seedance, PlatformSeedance, OpenAIEndpointCapabilitySeedance))
	require.False(t, isOpenAIEndpointPlatformCompatible(seedance, PlatformSeedance, OpenAIEndpointCapabilityChatCompletions))
	require.False(t, isOpenAIEndpointPlatformCompatible(seedance, PlatformOpenAI, OpenAIEndpointCapabilitySeedance))

	legacy := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "key", "base_url": "https://legacy.example/v1",
			"openai_capabilities": []string{"seedance"},
		},
	}
	require.True(t, isOpenAIEndpointPlatformCompatible(legacy, PlatformOpenAI, OpenAIEndpointCapabilitySeedance))
}
