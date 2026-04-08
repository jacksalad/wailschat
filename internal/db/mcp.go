package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"wailschat/internal/model"
)

// MCPServerService MCP服务器数据库服务
type MCPServerService struct {
	db *sql.DB
}

// NewMCPServerService 创建MCP服务器服务
func NewMCPServerService(db *sql.DB) *MCPServerService {
	return &MCPServerService{db: db}
}

// ListMCPServers 获取所有MCP服务器
func (s *MCPServerService) ListMCPServers() ([]model.MCPServer, error) {
	rows, err := s.db.Query(`
		SELECT id, name, command, url, transport, env, enabled, auth_token, created_at
		FROM mcp_servers
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("mcp: list: %w", err)
	}
	defer rows.Close()

	var servers []model.MCPServer
	for rows.Next() {
		var server model.MCPServer
		var command, url, authToken sql.NullString
		var envStr string

		err := rows.Scan(
			&server.ID, &server.Name, &command, &url, &server.Transport,
			&envStr, &server.Enabled, &authToken, &server.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("mcp: scan: %w", err)
		}

		server.Command = command.String
		server.URL = url.String
		server.AuthToken = authToken.String

		if envStr != "" {
			json.Unmarshal([]byte(envStr), &server.Env)
		} else {
			server.Env = make(map[string]string)
		}

		servers = append(servers, server)
	}

	if servers == nil {
		servers = []model.MCPServer{}
	}
	return servers, nil
}

// GetMCPServer 获取单个MCP服务器
func (s *MCPServerService) GetMCPServer(id string) (*model.MCPServer, error) {
	var server model.MCPServer
	var command, url, authToken sql.NullString
	var envStr string

	err := s.db.QueryRow(`
		SELECT id, name, command, url, transport, env, enabled, auth_token, created_at
		FROM mcp_servers
		WHERE id = ?
	`, id).Scan(
		&server.ID, &server.Name, &command, &url, &server.Transport,
		&envStr, &server.Enabled, &authToken, &server.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, model.ErrMCPServerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mcp: get: %w", err)
	}

	server.Command = command.String
	server.URL = url.String
	server.AuthToken = authToken.String

	if envStr != "" {
		json.Unmarshal([]byte(envStr), &server.Env)
	} else {
		server.Env = make(map[string]string)
	}

	return &server, nil
}

// CreateMCPServer 创建MCP服务器
func (s *MCPServerService) CreateMCPServer(server *model.MCPServer) error {
	if err := server.Validate(); err != nil {
		return fmt.Errorf("mcp: validate: %w", err)
	}

	envJSON, err := json.Marshal(server.Env)
	if err != nil {
		envJSON = []byte("{}")
	}

	_, err = s.db.Exec(`
		INSERT INTO mcp_servers (id, name, command, url, transport, env, enabled, auth_token, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		server.ID, server.Name, server.Command, server.URL, server.Transport,
		string(envJSON), server.Enabled, server.AuthToken, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("mcp: create: %w", err)
	}

	server.CreatedAt = time.Now()
	return nil
}

// UpdateMCPServer 更新MCP服务器
func (s *MCPServerService) UpdateMCPServer(server *model.MCPServer) error {
	if err := server.Validate(); err != nil {
		return fmt.Errorf("mcp: validate: %w", err)
	}

	envJSON, err := json.Marshal(server.Env)
	if err != nil {
		envJSON = []byte("{}")
	}

	result, err := s.db.Exec(`
		UPDATE mcp_servers
		SET name = ?, command = ?, url = ?, transport = ?, env = ?, enabled = ?, auth_token = ?
		WHERE id = ?
	`,
		server.Name, server.Command, server.URL, server.Transport,
		string(envJSON), server.Enabled, server.AuthToken, server.ID,
	)
	if err != nil {
		return fmt.Errorf("mcp: update: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return model.ErrMCPServerNotFound
	}

	return nil
}

// DeleteMCPServer 删除MCP服务器
func (s *MCPServerService) DeleteMCPServer(id string) error {
	result, err := s.db.Exec("DELETE FROM mcp_servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("mcp: delete: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return model.ErrMCPServerNotFound
	}

	return nil
}
