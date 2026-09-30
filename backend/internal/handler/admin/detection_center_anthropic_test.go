package admin

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const anthropicProbeAPIKey = "test-secret-api-key"

func TestProbeAnthropicTools(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		status     string
		reasonCode string
		stopReason bool
	}{
		{
			name:       "valid tool call",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"tool_use","name":"sum_numbers","input":{"a":17,"b":25}}],"stop_reason":"tool_use"}`,
			status:     "success",
		},
		{
			name:       "valid tool call without stop reason",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"tool_use","name":"sum_numbers","input":{"a":17,"b":25}}]}`,
			status:     "success",
		},
		{
			name:       "argument mismatch",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"tool_use","name":"sum_numbers","input":{"a":18,"b":25}}],"stop_reason":"tool_use"}`,
			status:     "partial",
			reasonCode: "TOOL_ARGUMENT_MISMATCH",
		},
		{
			name:       "non-integer argument",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"tool_use","name":"sum_numbers","input":{"a":17.5,"b":25}}],"stop_reason":"tool_use"}`,
			status:     "partial",
			reasonCode: "TOOL_ARGUMENT_MISMATCH",
		},
		{
			name:       "wrong tool name",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"tool_use","name":"other_tool","input":{"a":17,"b":25}}],"stop_reason":"tool_use"}`,
			status:     "partial",
			reasonCode: "TOOL_ARGUMENT_MISMATCH",
		},
		{
			name:       "wrong stop reason",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"tool_use","name":"sum_numbers","input":{"a":17,"b":25}}],"stop_reason":"end_turn"}`,
			status:     "partial",
			reasonCode: "TOOL_TERMINATION_MISMATCH",
		},
		{
			name:       "stop reason refusal is inconclusive and redacted",
			statusCode: http.StatusOK,
			response:   `{"content":[],"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber","explanation":"` + anthropicProbeAPIKey + ` private-upstream-detail"}}`,
			status:     "inconclusive",
			reasonCode: "PROBE_REFUSED",
			stopReason: true,
		},
		{
			name:       "stop details refusal is inconclusive",
			statusCode: http.StatusOK,
			response:   `{"content":[],"stop_reason":"end_turn","stop_details":{"type":"refusal","category":"cyber","explanation":"private-upstream-detail"}}`,
			status:     "inconclusive",
			reasonCode: "PROBE_REFUSED",
		},
		{
			name:       "plain text without tool use or refusal fails",
			statusCode: http.StatusOK,
			response:   `{"content":[{"type":"text","text":"I cannot call tools"}],"stop_reason":"end_turn"}`,
			status:     "failed",
			reasonCode: "TOOL_BLOCK_MISSING",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, requestBody := runAnthropicToolsProbeWithMock(t, test.statusCode, test.response)

			require.Equal(t, test.status, result.Status)
			require.Equal(t, test.reasonCode, result.ReasonCode)
			assertAnthropicSafeToolRequest(t, requestBody)

			if test.reasonCode == "PROBE_REFUSED" {
				require.NotEqual(t, "failed", result.Status)
				require.NotEqual(t, "TOOL_BLOCK_MISSING", result.ReasonCode)
				require.Contains(t, result.Evidence[0].Actual, "stop_details.type=refusal")
				require.Contains(t, result.Evidence[0].Actual, "category=cyber")
				if test.stopReason {
					require.Contains(t, result.Evidence[0].Actual, "stop_reason=refusal")
				} else {
					require.NotContains(t, result.Evidence[0].Actual, "stop_reason=refusal")
				}
				require.Empty(t, result.Evidence[0].ResponseExcerpt)
				serialized, err := json.Marshal(result)
				require.NoError(t, err)
				require.NotContains(t, string(serialized), anthropicProbeAPIKey)
				require.NotContains(t, string(serialized), "private-upstream-detail")
			}
		})
	}
}

func TestProbeAnthropicToolsNon2xxIsUnavailable(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusPaymentRequired,
		http.StatusTooManyRequests,
		http.StatusBadRequest,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			result, requestBody := runAnthropicToolsProbeWithMock(t, statusCode, `{"error":{"message":"request rejected"}}`)

			require.Equal(t, "unavailable", result.Status)
			require.NotEqual(t, "TOOL_BLOCK_MISSING", result.ReasonCode)
			assertAnthropicSafeToolRequest(t, requestBody)
		})
	}
}

func TestAnthropicProbesTreatExplicitRefusalAsInconclusive(t *testing.T) {
	refusal := `{"content":[],"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber","explanation":"` + anthropicProbeAPIKey + ` private-upstream-detail"}}`
	probes := []struct {
		name string
		run  func(*detectionTarget) detectionProbeResult
	}{
		{name: "basic", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicBasic(context.Background(), target, "test-model")
		}},
		{name: "structured outputs", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicStructured(context.Background(), target, "test-model")
		}},
		{name: "adaptive thinking", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicThinking(context.Background(), target, "test-model")
		}},
		{name: "citations", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicCitations(context.Background(), target, "test-model")
		}},
		{name: "prompt cache", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicPromptCache(context.Background(), target, "test-model")
		}},
		{name: "undeclared cache", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicUndeclaredCache(context.Background(), target, "test-model")
		}},
		{name: "context management", run: func(target *detectionTarget) detectionProbeResult {
			return probeAnthropicContextManagement(context.Background(), target, "test-model")
		}},
	}

	for _, test := range probes {
		t.Run(test.name, func(t *testing.T) {
			result, _ := runAnthropicProbeWithMock(t, http.StatusOK, refusal, test.run)

			require.Equal(t, "inconclusive", result.Status)
			require.Equal(t, "PROBE_REFUSED", result.ReasonCode)
			require.NotEmpty(t, result.PossibleCauses)
			require.NotEmpty(t, result.Recommendations)
			for _, evidence := range result.Evidence {
				require.Empty(t, evidence.ResponseExcerpt)
				require.Contains(t, evidence.Actual, "stop_reason=refusal")
			}
			serialized, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(serialized), anthropicProbeAPIKey)
			require.NotContains(t, string(serialized), "private-upstream-detail")
		})
	}
}

func runAnthropicToolsProbeWithMock(t *testing.T, statusCode int, response string) (detectionProbeResult, map[string]any) {
	result, requestBodies := runAnthropicProbeWithMock(t, statusCode, response, func(target *detectionTarget) detectionProbeResult {
		return probeAnthropicTools(context.Background(), target, "test-model")
	})
	require.Len(t, requestBodies, 1)
	return result, requestBodies[0]
}

func runAnthropicProbeWithMock(t *testing.T, statusCode int, response string, probe func(*detectionTarget) detectionProbeResult) (detectionProbeResult, []map[string]any) {
	t.Helper()
	requestBodyChannel := make(chan []byte, 16)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "could not read body", http.StatusBadRequest)
			return
		}
		requestBodyChannel <- body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(upstream.Close)

	transport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, upstream.Listener.Addr().String())
	}}
	t.Cleanup(transport.CloseIdleConnections)
	target := &detectionTarget{
		baseURL: "http://1.1.1.1",
		apiKey:  anthropicProbeAPIKey,
		client:  &http.Client{Transport: transport},
	}
	result := probe(target)
	requestBodies := make([]map[string]any, 0, len(requestBodyChannel))
	for len(requestBodyChannel) > 0 {
		var requestBody map[string]any
		require.NoError(t, json.Unmarshal(<-requestBodyChannel, &requestBody))
		requestBodies = append(requestBodies, requestBody)
	}
	return result, requestBodies
}

func assertAnthropicSafeToolRequest(t *testing.T, requestBody map[string]any) {
	t.Helper()
	require.Equal(t, "test-model", requestBody["model"])
	messages := sliceValue(requestBody["messages"])
	require.Len(t, messages, 1)
	message := mapValue(messages[0])
	require.Equal(t, "user", message["role"])
	require.Equal(t, "Use the sum_numbers tool with a=17 and b=25. Do not answer in text.", message["content"])

	tools := sliceValue(requestBody["tools"])
	require.Len(t, tools, 1)
	tool := mapValue(tools[0])
	require.Equal(t, "sum_numbers", tool["name"])
	require.Equal(t, "Add two integers and return their sum.", tool["description"])
	schema := mapValue(tool["input_schema"])
	require.Equal(t, "object", schema["type"])
	require.Equal(t, []any{"a", "b"}, schema["required"])
	require.Equal(t, false, schema["additionalProperties"])
	properties := mapValue(schema["properties"])
	require.Equal(t, "integer", mapValue(properties["a"])["type"])
	require.Equal(t, "integer", mapValue(properties["b"])["type"])
	require.Equal(t, map[string]any{"type": "tool", "name": "sum_numbers"}, requestBody["tool_choice"])
}
