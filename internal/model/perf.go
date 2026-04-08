package model

// PerformanceStats holds metrics for a single LLM streaming response.
type PerformanceStats struct {
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	FirstTokenTime float64 `json:"first_token_time"` // seconds to first content token
	TotalTime      float64 `json:"total_time"`       // seconds for full response
	Speed          float64 `json:"speed"`            // output tokens per second
}
