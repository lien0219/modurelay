package service

import "github.com/tidwall/gjson"

// Generation output excludes usage, keepalive, role-only and terminal metadata.
// Tool calls and reasoning are output even when no ordinary text is produced.
func anthropicHasFirstOutput(data string) bool {
	if !gjson.Valid(data) {
		return false
	}
	event := gjson.Parse(data)
	switch event.Get("type").String() {
	case "content_block_start":
		block := event.Get("content_block")
		return block.Get("type").String() == "tool_use" || block.Get("text").String() != "" || block.Get("thinking").String() != "" || block.Get("source.data").String() != ""
	case "content_block_delta":
		delta := event.Get("delta")
		return delta.Get("text").String() != "" || delta.Get("thinking").String() != "" || delta.Get("partial_json").String() != ""
	}
	return false
}

func geminiHasFirstOutput(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}
	root := gjson.ParseBytes(data)
	if inner := root.Get("response"); inner.IsObject() {
		root = inner
	}
	found := false
	root.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			found = part.Get("text").String() != "" || part.Get("functionCall.name").String() != "" || part.Get("inlineData.data").String() != "" || part.Get("fileData.fileUri").String() != "" || part.Get("executableCode.code").String() != "" || part.Get("codeExecutionResult.output").String() != ""
			return !found
		})
		return !found
	})
	return found
}

// Chat Completions uses choices/delta rather than Responses event types. A role,
// tool index/id alone, usage report or finish marker is structural metadata;
// text, reasoning, audio and named tool/function calls are generation output.
// This predicate does not change Responses' configurable semantic/visible mode.
func chatCompletionsHasFirstOutput(data string) bool {
	if !gjson.Valid(data) {
		return false
	}
	found := false
	gjson.Get(data, "choices").ForEach(func(_, choice gjson.Result) bool {
		delta := choice.Get("delta")
		for _, path := range []string{"content", "reasoning_content", "reasoning", "thinking", "refusal", "audio.data", "audio.transcript", "function_call.name", "function_call.arguments"} {
			value := delta.Get(path)
			if value.Type == gjson.String && value.String() != "" {
				found = true
				return false
			}
		}
		delta.Get("content").ForEach(func(_, part gjson.Result) bool {
			if part.Get("text").Type == gjson.String && part.Get("text").String() != "" {
				found = true
			}
			return !found
		})
		if found {
			return false
		}
		delta.Get("tool_calls").ForEach(func(_, call gjson.Result) bool {
			found = call.Get("function.name").String() != "" || call.Get("function.arguments").String() != ""
			return !found
		})
		return !found
	})
	return found
}
