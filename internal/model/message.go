package model

import "time"

type Message struct {
	ID              int64     `json:"id"`
	SessionID       int64     `json:"session_id"`
	Role            string    `json:"role"`
	Content         string    `json:"content"`
	Images          string    `json:"images"`       // JSON array of base64 images
	StatsJSON       string    `json:"stats"`        // JSON-serialized PerformanceStats (empty if none)
	ToolCallsJSON   string    `json:"tool_calls"`   // JSON array of MCP tool calls (empty if none)
	ToolResultsJSON string    `json:"tool_results"` // JSON array of MCP tool results (empty if none)
	CreatedAt       time.Time `json:"created_at"`
}
