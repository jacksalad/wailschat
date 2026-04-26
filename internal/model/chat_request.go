package model

import (
	"encoding/json"
	"log"
)

// ContentText represents text content in a message
type ContentText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ContentImage represents image content in a message
type ContentImage struct {
	Type     string `json:"type"`
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

// ChatContent represents a single content item (text or image)
type ChatContent interface{}

// ChatMessage represents a single message in the chat completion request.
// Content can be a string (for text-only) or an array of ChatContent (for multimodal)
type ChatMessage struct {
	Role             string      `json:"role"`
	Content          interface{} `json:"content"`
	ReasoningContent string      `json:"reasoning_content,omitempty"` // For reasoning models like DeepSeek R1
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	Name             string      `json:"name,omitempty"`         // For tool role messages
	ToolCallID       string      `json:"tool_call_id,omitempty"` // For tool role messages
}

// Tool represents an OpenAI function tool definition.
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines a callable function.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a tool call from the LLM.
type ToolCall struct {
	Index    int          `json:"index"` // index field for delta chunks
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ParseArgumentsResult holds the result of parsing arguments.
type ParseArgumentsResult struct {
	Args    map[string]any
	RawArgs string // Original string if parsing failed
}

// ParseArguments parses the JSON arguments string into a map.
// Returns a result with empty map and original string if parsing fails.
func (f *FunctionCall) ParseArguments() ParseArgumentsResult {
	if f.Arguments == "" || f.Arguments == "{}" {
		return ParseArgumentsResult{Args: make(map[string]any)}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(f.Arguments), &args); err != nil {
		// Try to handle JavaScript-style object notation (unquoted keys)
		// by attempting to parse it as-is
		log.Printf("[FunctionCall] Failed to parse arguments as JSON: %v, raw: %s", err, f.Arguments)
		// Return empty map with raw args - MCP server will validate
		return ParseArgumentsResult{Args: make(map[string]any), RawArgs: f.Arguments}
	}
	return ParseArgumentsResult{Args: args}
}

// ChatCompletionRequest is the request body for OpenAI-compatible chat API.
type ChatCompletionRequest struct {
	Model         string         `json:"model"`
	Messages      []ChatMessage  `json:"messages"`
	Stream        bool           `json:"stream"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
	Tools         []Tool         `json:"tools,omitempty"`
	ToolChoice    interface{}    `json:"tool_choice,omitempty"`
}

// StreamOptions requests additional data in streaming responses.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatCompletionChunk represents a single SSE chunk from the streaming API.
type ChatCompletionChunk struct {
	ID      string     `json:"id"`
	Choices []Choice   `json:"choices"`
	Usage   *UsageInfo `json:"usage,omitempty"`
}

// UsageInfo holds token usage statistics returned by the API.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice represents a choice in the SSE chunk.
type Choice struct {
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason,omitempty"`
}

// Delta represents the delta content in a streaming chunk.
type Delta struct {
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}
