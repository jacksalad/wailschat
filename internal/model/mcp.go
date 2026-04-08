package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// MCP相关错误定义
var (
	ErrMCPServerNotFound = errors.New("mcp server not found")
)

// TransportType MCP传输类型
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportHTTP  TransportType = "http"
)

// MCPServer MCP服务器配置
type MCPServer struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Command   string            `json:"command,omitempty"` // stdio模式：启动命令
	URL       string            `json:"url,omitempty"`     // HTTP模式：服务器地址
	Transport TransportType     `json:"transport"`         // "stdio" | "http"
	Env       map[string]string `json:"env,omitempty"`     // 环境变量
	Enabled   bool              `json:"enabled"`
	AuthToken string            `json:"auth_token,omitempty"` // 可选认证token
	CreatedAt time.Time         `json:"created_at"`
}

// MCPTool MCP工具定义
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// MCPServerInfo MCP服务器信息（包含工具列表）
type MCPServerInfo struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tools       []MCPTool `json:"tools"`
}

// ToolCallResult 工具调用结果
type ToolCallResult struct {
	ToolName   string `json:"tool_name"`
	ServerName string `json:"server_name,omitempty"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// MCPServerTestResult 测试连接结果
type MCPServerTestResult struct {
	Success bool      `json:"success"`
	Tools   []MCPTool `json:"tools,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// Validate 验证MCPServer配置
func (s *MCPServer) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if s.Transport != TransportStdio && s.Transport != TransportHTTP {
		return fmt.Errorf("transport must be 'stdio' or 'http'")
	}
	if s.Transport == TransportStdio && s.Command == "" {
		return fmt.Errorf("command is required for stdio transport")
	}
	if s.Transport == TransportHTTP && s.URL == "" {
		return fmt.Errorf("url is required for http transport")
	}
	return nil
}
