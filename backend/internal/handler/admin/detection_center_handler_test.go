package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinDetectionEndpointAvoidsDuplicateVersion(t *testing.T) {
	got, err := joinDetectionEndpoint("https://api.example.com/v1", "/v1/models")
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1/models", got)

	got, err = joinDetectionEndpoint("https://api.example.com/proxy", "/v1/messages")
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/proxy/v1/messages", got)

	got, err = joinDetectionEndpoint("https://generativelanguage.googleapis.com/v1beta", "/v1beta/models/gemini:streamGenerateContent?alt=sse")
	require.NoError(t, err)
	require.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini:streamGenerateContent?alt=sse", got)
}

func TestValidateDetectionSchemaPayloadIsStrict(t *testing.T) {
	ok, detail := validateDetectionSchemaPayload(`{"probe":"SCHEMA_OK_91","count":7}`, "SCHEMA_OK_91", 7)
	require.True(t, ok, detail)
	ok, _ = validateDetectionSchemaPayload(`{"probe":"SCHEMA_OK_91","count":7,"extra":true}`, "SCHEMA_OK_91", 7)
	require.False(t, ok)
	ok, _ = validateDetectionSchemaPayload("plain text", "SCHEMA_OK_91", 7)
	require.False(t, ok)
}

func TestMergeDetectedModelsPreservesProtocols(t *testing.T) {
	models := mergeDetectedModels([]detectedModel{
		{ID:"same-model", Name:"Same", Protocols:[]string{"openai"}},
		{ID:"same-model", Name:"Same", Protocols:[]string{"anthropic"}},
	})
	require.Len(t, models, 1)
	require.Equal(t, []string{"anthropic","openai"}, models[0].Protocols)
}

func TestSanitizeDetectionTextRedactsKey(t *testing.T) {
	got := sanitizeDetectionText("Bearer sk-secret sk-secret abcdef", "sk-secret", 40)
	require.NotContains(t, got, "sk-secret")
}

func TestSummarizeDetectionProbes(t *testing.T) {
	s := summarizeDetectionProbes([]detectionProbeResult{{Status:"success"},{Status:"failed"},{Status:"partial"},{Status:"inconclusive"},{Status:"not_applicable"},{Status:"unavailable"}})
	require.Equal(t, 6, s.Total)
	require.Equal(t, 1, s.Success)
	require.Equal(t, 1, s.Failed)
	require.Equal(t, 1, s.Partial)
	require.Equal(t, 1, s.Inconclusive)
	require.Equal(t, 1, s.NotApplicable)
	require.Equal(t, 1, s.Unavailable)
}
