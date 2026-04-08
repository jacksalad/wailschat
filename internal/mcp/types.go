package mcp

import "encoding/json"

// MCP Protocol Constants
const (
	ProtocolVersion = "2025-03-26"
	JSONRPCVersion  = "2.0"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no id)
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// InitializeParams for the initialize request
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      ClientInfo   `json:"clientInfo"`
}

// Capabilities represents MCP server capabilities
type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability indicates tool support
type ToolsCapability struct{}

// ClientInfo describes the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult from server
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

// ServerInfo describes the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListParams for tools/list request
type ToolsListParams struct{}

// ToolsListResult from tools/list response
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ToolsCallParams for tools/call request
type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolsCallResult from tools/call response
type ToolsCallResult struct {
	Content           []ContentBlock `json:"content"`
	IsError           bool           `json:"isError,omitempty"`
	StructuredContent interface{}    `json:"structuredContent,omitempty"`
}

// ContentBlock represents content in a tool result
type ContentBlock struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
}

// Resource represents a resource in content
type Resource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
}
