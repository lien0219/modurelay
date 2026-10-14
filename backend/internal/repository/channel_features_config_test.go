package repository

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelFeaturesConfigBedrockCCCompatRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{"enabled", `true`, true},
		{"disabled", `false`, false},
		{"legacy enabled", `{"anthropic":true}`, true},
		{"legacy disabled", `{"anthropic":false}`, false},
		{"string", `"true"`, false},
		{"null", `null`, false},
		{"invalid nested value", `{"anthropic":"true"}`, false},
		{"unknown platform", `{"bedrock":true}`, false},
		{"ambiguous map", `{"anthropic":true,"openai":true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `{"bedrock_cc_compat":` + tc.value + `,"other":{"nested":[true,"keep"]},"web_search_emulation":{"anthropic":true}}`
			var raw map[string]any
			require.NoError(t, json.Unmarshal([]byte(input), &raw))
			read := unmarshalFeaturesConfig([]byte(input))
			require.Equal(t, tc.want, read["bedrock_cc_compat"])
			require.Equal(t, raw["other"], read["other"])
			require.Equal(t, raw["web_search_emulation"], read["web_search_emulation"])
			written, err := marshalFeaturesConfig(raw)
			require.NoError(t, err)
			var stored map[string]any
			require.NoError(t, json.Unmarshal(written, &stored))
			require.Equal(t, read, stored)
			// Serialization must not mutate caller-owned configuration or cached maps.
			unchanged, err := json.Marshal(raw)
			require.NoError(t, err)
			require.JSONEq(t, input, string(unchanged))
		})
	}
}

func TestChannelFeaturesConfigWithoutBedrockPreserved(t *testing.T) {
	for _, input := range []string{`{}`, `{"other":true}`} {
		out, err := marshalFeaturesConfig(unmarshalFeaturesConfig([]byte(input)))
		require.NoError(t, err)
		require.JSONEq(t, input, string(out))
	}
}
