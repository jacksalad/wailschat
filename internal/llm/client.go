package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wailschat/internal/model"
)

// 注意：不再使用 sync.Pool 复用 map，因为 Go 的 map 不是线程安全的。
// 并发访问同一个 map 会导致数据竞争。改为每次分配新 map。

type Client struct {
	httpClient   *http.Client // For non-streaming requests (has overall timeout)
	streamClient *http.Client // For streaming SSE (no overall timeout, per-activity deadline)
}

func NewClient() *Client {
	// Clone default transport with generous ResponseHeaderTimeout for streaming.
	// This is essential for reasoning models that can think for many minutes before
	// sending the first response byte — http.Client.Timeout is a total timeout that
	// would kill SSE streams prematurely.
	streamTransport := http.DefaultTransport.(*http.Transport).Clone()
	streamTransport.ResponseHeaderTimeout = 180 * time.Second

	return &Client{
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
		streamClient: &http.Client{
			Transport: streamTransport,
		},
	}
}

// TestConnection sends a minimal request to verify the API key and endpoint.
func (c *Client) TestConnection(baseURL, apiKey, modelName string) error {
	reqBody := model.ChatCompletionRequest{
		Model:    modelName,
		Messages: []model.ChatMessage{{Role: "user", Content: "hi"}},
		Stream:   false,
	}
	body, _ := json.Marshal(reqBody)

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm: test connection: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("llm: connection failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("llm: invalid API key (HTTP %d)", resp.StatusCode)
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm: test failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
}

// Chat sends a non-streaming chat request and returns the assistant's reply.
func (c *Client) Chat(ctx context.Context, baseURL, apiKey, modelName string, messages []model.ChatMessage) (string, error) {
	reqBody := model.ChatCompletionRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
	}
	body, _ := json.Marshal(reqBody)

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: chat: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm: chat failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm: chat decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm: chat: no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

// StreamChat calls the OpenAI-compatible API with streaming and calls onChunk for each content delta.
// It returns performance stats including timing and token usage.
// If tools is provided, the request will include tools for MCP integration.
// Note: onChunk will be called with accumulated tool_calls when finish_reason is "tool_calls".
//
// Dynamic timeout handling for reasoning models:
//   - Initial timeout (waiting for first data): 120 seconds
//   - Per-chunk timeout during content streaming: 120 seconds
//   - Per-chunk timeout during reasoning (thinking): 180 seconds
//   - Timer resets on every SSE line received, so only a stuck connection will time out
func (c *Client) StreamChat(
	ctx context.Context,
	baseURL, apiKey, modelName string,
	messages []model.ChatMessage,
	onChunk func(content string, toolCalls []model.ToolCall, finishReason string),
	tools []model.Tool,
) (*model.PerformanceStats, error) {
	reqBody := model.ChatCompletionRequest{
		Model:         modelName,
		Messages:      messages,
		Stream:        true,
		StreamOptions: &model.StreamOptions{IncludeUsage: true},
		Tools:         tools,
	}
	body, _ := json.Marshal(reqBody)

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"

	// Create a cancellable context for this stream session.
	// We manage timeouts via a timer goroutine (not context.WithTimeout) to avoid
	// race conditions and to support dynamic deadline extension per activity.
	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()

	req, err := http.NewRequestWithContext(streamCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: stream chat: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	startTime := time.Now()
	var firstTokenTime time.Time
	firstTokenReceived := false
	var usageInfo *model.UsageInfo

	// Use streamClient (no overall timeout) for SSE requests.
	// This is essential for reasoning models that can think for many minutes.
	resp, err := c.streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm: stream failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// Tool call accumulator: index -> accumulated ToolCall
	// 直接分配新 map，避免 sync.Pool 导致的线程安全问题
	toolAccum := make(map[int]*model.ToolCall)

	// Dynamic timeout configuration for reasoning models:
	// Reasoning models can think for many minutes, sending reasoning_content chunks slowly.
	// We use generous timeouts to allow the model to complete its reasoning process.
	// The timer resets on every SSE line received, so the connection is only terminated
	// if no data arrives for the configured duration (i.e., the connection is truly stuck).
	const (
		initialTimeout   = 120 * time.Second // Wait for first SSE data after connection
		activeTimeout    = 120 * time.Second // Per-chunk timeout during normal content streaming
		reasoningTimeout = 180 * time.Second // Per-chunk timeout during reasoning content (thinking)
	)

	timeoutTimer := time.NewTimer(initialTimeout)
	defer timeoutTimer.Stop()

	// done signals goroutines to exit when StreamChat returns
	done := make(chan struct{})
	defer close(done)

	// Timer goroutine: cancel the stream when no activity is detected for too long.
	// This is the only way the stream is timed out — there is no http.Client.Timeout
	// or context.WithTimeout that could prematurely kill a long-running reasoning session.
	go func() {
		select {
		case <-done:
			return
		case <-timeoutTimer.C:
			streamCancel()
		}
	}()

	// resetTimer safely resets the timeout timer with a new duration.
	resetTimer := func(d time.Duration) {
		if !timeoutTimer.Stop() {
			select {
			case <-timeoutTimer.C:
			default:
			}
		}
		timeoutTimer.Reset(d)
	}

	// Channel for scanner goroutine to send SSE lines
	type chunkData struct {
		line  string
		isEOF bool
		err   error
	}
	chunkChan := make(chan chunkData, 4)

	// Scanner goroutine: reads lines from the SSE response body.
	// Uses select on done channel to prevent goroutine leak when StreamChat returns early.
	go func() {
		defer close(chunkChan)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case chunkChan <- chunkData{line: scanner.Text()}:
			case <-done:
				return
			}
		}
		select {
		case chunkChan <- chunkData{isEOF: true, err: scanner.Err()}:
		case <-done:
			return
		}
	}()

	for cd := range chunkChan {
		// Reset timeout on any SSE activity (any line received from server)
		resetTimer(activeTimeout)

		// Check if stream was cancelled (timeout or parent cancellation)
		select {
		case <-streamCtx.Done():
			clearToolAccum(toolAccum)
			if ctx.Err() != nil {
				return nil, fmt.Errorf("llm: stream cancelled: %w", ctx.Err())
			}
			return nil, fmt.Errorf("llm: stream timeout: no data received for %v", activeTimeout)
		default:
		}

		if cd.isEOF {
			if cd.err != nil {
				clearToolAccum(toolAccum)
				if ctx.Err() != nil {
					return nil, fmt.Errorf("llm: stream cancelled: %w", ctx.Err())
				}
				if streamCtx.Err() != nil {
					return nil, fmt.Errorf("llm: stream timeout: no data received for %v", activeTimeout)
				}
				return nil, fmt.Errorf("llm: stream read: %w", cd.err)
			}
			// Normal stream end
			if len(toolAccum) > 0 {
				flushToolCalls(toolAccum, onChunk)
			}
			clearToolAccum(toolAccum)
			return buildStats(startTime, firstTokenTime, firstTokenReceived, usageInfo), nil
		}

		result := ParseSSELine(cd.line)
		if result.Done {
			if len(toolAccum) > 0 {
				flushToolCalls(toolAccum, onChunk)
			}
			clearToolAccum(toolAccum)
			return buildStats(startTime, firstTokenTime, firstTokenReceived, usageInfo), nil
		}
		if result.Usage != nil {
			usageInfo = result.Usage
		}

		// Accumulate tool_calls deltas
		for _, tc := range result.ToolCalls {
			idx := tc.Index
			if existing, ok := toolAccum[idx]; ok {
				existing.Function.Arguments += tc.Function.Arguments
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Type != "" {
					existing.Type = tc.Type
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
			} else {
				tcCopy := tc
				toolAccum[idx] = &tcCopy
			}
		}

		// If finish_reason is "tool_calls", flush accumulated tool calls
		if result.FinishReason == "tool_calls" {
			flushToolCalls(toolAccum, onChunk)
		}

		// Handle reasoning content and regular content
		if result.ReasoningContent != "" || result.Content != "" || len(toolAccum) > 0 {
			if !firstTokenReceived {
				firstTokenTime = time.Now()
				firstTokenReceived = true
			}
			// Use longer timeout during reasoning (model is thinking)
			if result.ReasoningContent != "" {
				resetTimer(reasoningTimeout)
			}
			// Forward reasoning content as regular content for display
			if result.ReasoningContent != "" {
				onChunk(result.ReasoningContent, nil, result.FinishReason)
			}
			if result.Content != "" {
				onChunk(result.Content, nil, result.FinishReason)
			}
		}
	}

	// Should not reach here, but handle gracefully
	if len(toolAccum) > 0 {
		flushToolCalls(toolAccum, onChunk)
	}
	clearToolAccum(toolAccum)
	return buildStats(startTime, firstTokenTime, firstTokenReceived, usageInfo), nil
}

// flushToolCalls converts accumulated tool calls to slice and calls onChunk
func flushToolCalls(toolAccum map[int]*model.ToolCall, onChunk func(content string, toolCalls []model.ToolCall, finishReason string)) {
	if len(toolAccum) == 0 {
		return
	}
	// Convert map to sorted slice
	var toolCalls []model.ToolCall
	for i := 0; i < len(toolAccum); i++ {
		if tc, ok := toolAccum[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}
	// Clear accumulator (map will be returned to pool by caller)
	clearToolAccum(toolAccum)
	// Call onChunk with accumulated tool calls
	onChunk("", toolCalls, "tool_calls")
}

// clearToolAccum clears all entries from the toolAccum map
func clearToolAccum(toolAccum map[int]*model.ToolCall) {
	for k := range toolAccum {
		delete(toolAccum, k)
	}
}

// GetModels fetches available model IDs from an OpenAI-compatible /v1/models endpoint.
func (c *Client) GetModels(baseURL, apiKey string) ([]string, error) {
	url := strings.TrimRight(baseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("llm: get models: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: get models failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm: get models failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("llm: get models decode: %w", err)
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func buildStats(startTime, firstTokenTime time.Time, firstTokenReceived bool, usage *model.UsageInfo) *model.PerformanceStats {
	endTime := time.Now()
	stats := &model.PerformanceStats{
		TotalTime: endTime.Sub(startTime).Seconds(),
	}
	if usage != nil {
		stats.InputTokens = usage.PromptTokens
		stats.OutputTokens = usage.CompletionTokens
	}
	if firstTokenReceived {
		stats.FirstTokenTime = firstTokenTime.Sub(startTime).Seconds()
	}
	if stats.OutputTokens > 0 && stats.TotalTime > stats.FirstTokenTime {
		genTime := stats.TotalTime - stats.FirstTokenTime
		if genTime > 0 {
			stats.Speed = float64(stats.OutputTokens) / genTime
		}
	}
	return stats
}
