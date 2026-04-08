package llm

import (
	"encoding/json"
	"strings"

	"wailschat/internal/model"
)

// SSEResult holds the parsed result of a single SSE line.
type SSEResult struct {
	Content          string
	ReasoningContent string
	ToolCalls        []model.ToolCall
	FinishReason     string
	Done             bool
	Usage            *model.UsageInfo
}

// ParseSSELine parses a single line from the SSE stream.
func ParseSSELine(line string) SSEResult {
	line = strings.TrimSpace(line)
	if line == "" {
		return SSEResult{}
	}
	if !strings.HasPrefix(line, "data: ") {
		return SSEResult{}
	}
	data := strings.TrimPrefix(line, "data: ")
	if strings.TrimSpace(data) == "[DONE]" {
		return SSEResult{Done: true}
	}
	var chunk model.ChatCompletionChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return SSEResult{} // skip malformed lines
	}
	result := SSEResult{}
	if len(chunk.Choices) > 0 {
		result.Content = chunk.Choices[0].Delta.Content
		result.ReasoningContent = chunk.Choices[0].Delta.ReasoningContent
		result.ToolCalls = chunk.Choices[0].Delta.ToolCalls
		// Extract finish_reason
		if chunk.Choices[0].FinishReason != nil {
			result.FinishReason = *chunk.Choices[0].FinishReason
		}
	}
	if chunk.Usage != nil {
		result.Usage = chunk.Usage
	}
	return result
}
