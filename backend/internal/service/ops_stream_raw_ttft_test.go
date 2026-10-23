//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsChatCompletionsFirstOutputPredicate(t *testing.T) {
	for _, payload := range []string{
		`[DONE]`, `not-json`, `{"choices":[]}`, `{"choices":[],"usage":{"completion_tokens":2}}`,
		`{"choices":[{"delta":{"role":"assistant"}}]}`, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"","arguments":""}}]}}]}`,
	} {
		require.False(t, chatCompletionsHasFirstOutput(payload), payload)
	}
	for _, payload := range []string{
		`{"choices":[{"delta":{"content":" "}}]}`,
		`{"choices":[{"delta":{"content":[{"type":"text","text":"answer"}]}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"think"}}]}`,
		`{"choices":[{"delta":{"reasoning":"think"}}]}`,
		`{"choices":[{"delta":{"thinking":"think"}}]}`,
		`{"choices":[{"delta":{"refusal":"cannot"}}]}`,
		`{"choices":[{"delta":{"audio":{"data":"YWJj"}}}]}`,
		`{"choices":[{"delta":{"function_call":{"name":"lookup"}}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"{"}}]}}]}`,
	} {
		require.True(t, chatCompletionsHasFirstOutput(payload), payload)
	}
}

func TestOpsRawStreamsFirstOutputExcludesMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, protocol, output string }{
		{"chat-metadata-only", "chat", ""},
		{"chat-text", "chat", `{"choices":[{"index":0,"delta":{"content":"hello"}}]}`},
		{"chat-tool", "chat", `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]}}]}`},
		{"chat-reasoning", "chat", `{"choices":[{"index":0,"delta":{"reasoning_content":"think"}}]}`},
		{"anthropic-metadata-only", "anthropic", ""},
		{"anthropic-text", "anthropic", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`},
		{"anthropic-tool", "anthropic", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool_1","name":"lookup","input":{}}}`},
		{"anthropic-reasoning", "anthropic", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"think"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"}}]}\n\n"
			suffix := "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":4}}\n\ndata: [DONE]\n\n"
			if tc.protocol == "anthropic" {
				prefix = "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"role\":\"assistant\",\"usage\":{\"input_tokens\":7,\"output_tokens\":0}}}\n\ndata: {\"type\":\"ping\"}\n\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"
				suffix = "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":4}}\n\ndata: {\"type\":\"message_stop\"}\n\n"
			}
			reader, writer := io.Pipe()
			done := make(chan struct{})
			t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
			started := time.Now()
			go func() {
				defer close(done)
				defer writer.Close()
				_, _ = io.WriteString(writer, prefix)
				if tc.output != "" {
					time.Sleep(120 * time.Millisecond)
					_, _ = io.WriteString(writer, "data: "+tc.output+"\n\n")
				}
				_, _ = io.WriteString(writer, suffix)
			}()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader}
			account := &Account{ID: 1, Platform: PlatformOpenAI}
			var result *OpenAIForwardResult
			var err error
			if tc.protocol == "chat" {
				result, err = svc.streamRawChatCompletions(c, resp, account, "model", "model", "model", nil, nil, started, 0)
			} else {
				result, err = svc.handleNativeAnthropicStreamingResponse(context.Background(), resp, c, account, "model", "model", "model", nil, started)
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			if tc.output == "" {
				require.Nil(t, result.FirstTokenMs)
			} else {
				require.NotNil(t, result.FirstTokenMs)
				require.GreaterOrEqual(t, *result.FirstTokenMs, 100)
				require.Contains(t, recorder.Body.String(), tc.output)
			}
			require.Equal(t, 7, result.Usage.InputTokens)
			require.Equal(t, 4, result.Usage.OutputTokens)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("synthetic upstream did not finish")
			}
		})
	}
}
