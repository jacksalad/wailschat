package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"wailschat/internal/model"
)

// pendingRequest 跟踪待处理的请求
type pendingRequest struct {
	resultCh chan<- *JSONRPCResponse
	errCh    chan<- error
	method   string
}

// Client MCP客户端管理器
type Client struct {
	mu      sync.RWMutex
	servers map[string]*ServerConnection // serverID -> connection
}

// ServerConnection MCP服务器连接
type ServerConnection struct {
	Config      *model.MCPServer
	Process     *exec.Cmd
	connected   bool
	lastUsed    time.Time
	stdinPipe   io.WriteCloser // 原始的 stdin pipe，用于关闭
	stdin       *bufio.Writer
	stdout      *bufio.Reader
	mu          sync.Mutex
	requestID   atomic.Int64
	initialized bool
	toolsCache  []model.MCPTool

	// 单一 reader 相关
	readerCtx    context.Context
	readerCancel context.CancelFunc
	readerDone   chan struct{}
	pending      map[int64]*pendingRequest // 待处理请求 by ID
	pendingMu    sync.Mutex
	lineCh       chan string // 用于传递读取的行
	closeOnce    sync.Once   // 确保 lineCh 只关闭一次

	// HTTP/SSE 传输相关
	httpClient      *http.Client
	messageEndpoint string // POST JSON-RPC 消息的 URL (从 SSE endpoint 事件获取)
	sseCancel       context.CancelFunc
	sseDone         chan struct{} // SSE 读取 goroutine 完成信号
	endpointReady   chan struct{} // 信号：SSE endpoint 事件已接收

	// Streamable HTTP 传输相关
	httpMode  string // "streamable" 或 "sse"（自动检测后设定）
	sessionID string // MCP-Session-Id（Streamable HTTP 模式）
}

// NewClient 创建新的MCP客户端管理器
func NewClient() *Client {
	return &Client{
		servers: make(map[string]*ServerConnection),
	}
}

// errStreamableNotSupported 表示服务器不支持 Streamable HTTP
var errStreamableNotSupported = fmt.Errorf("server does not support streamable HTTP transport")

// connectStreamableHTTP 尝试使用 Streamable HTTP 传输连接到 MCP 服务器
// 返回 nil 表示成功，errStreamableNotSupported 表示应回退到旧版 SSE
func (conn *ServerConnection) connectStreamableHTTP(ctx context.Context) error {
	url := conn.Config.URL

	initParams := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
		ClientInfo: ClientInfo{
			Name:    "wailschat",
			Version: "1.0.0",
		},
	}

	paramsJSON, err := json.Marshal(initParams)
	if err != nil {
		return fmt.Errorf("failed to marshal init params: %w", err)
	}

	// 构建 initialize 请求
	reqBody := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "initialize",
		Params:  paramsJSON,
	}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if conn.Config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.Config.AuthToken)
	}

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("streamable HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// 405/404/400 表示不支持 Streamable HTTP，应回退
	if resp.StatusCode == http.StatusMethodNotAllowed ||
		resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusBadRequest {
		return errStreamableNotSupported
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("streamable HTTP returned status %d: %s", resp.StatusCode, string(body))
	}

	// 保存 session ID
	if sid := resp.Header.Get("MCP-Session-Id"); sid != "" {
		conn.sessionID = sid
		log.Printf("[MCP StreamableHTTP] Session ID: %s", sid)
	}

	// 根据 Content-Type 解析响应
	contentType := resp.Header.Get("Content-Type")
	var jsonrpcResp JSONRPCResponse

	if strings.Contains(contentType, "text/event-stream") {
		// SSE 响应流：从响应体解析 SSE 事件
		result, err := conn.parseSSEResponse(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to parse SSE response: %w", err)
		}
		jsonrpcResp = *result
	} else {
		// JSON 响应
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		if err := json.Unmarshal(body, &jsonrpcResp); err != nil {
			return fmt.Errorf("failed to parse JSON response: %w", err)
		}
	}

	if jsonrpcResp.Error != nil {
		// 检查是否是协议版本不匹配等可回退的错误
		if jsonrpcResp.Error.Code == -32600 || jsonrpcResp.Error.Code == -32601 {
			return errStreamableNotSupported
		}
		return fmt.Errorf("initialize error: %s (code %d)", jsonrpcResp.Error.Message, jsonrpcResp.Error.Code)
	}

	log.Printf("[MCP StreamableHTTP] Initialize response received successfully")

	conn.initialized = true

	// 发送 initialized 通知 (MCP 协议要求)
	conn.sendNotificationLocked("initialized", struct{}{})

	// 获取工具列表
	tools, err := conn.listToolsLocked(ctx)
	if err != nil {
		log.Printf("[MCP StreamableHTTP] Warning: failed to list tools: %v", err)
	} else {
		conn.toolsCache = tools
	}

	return nil
}

// parseSSEResponse 从 HTTP 响应体解析 SSE 事件流，返回第一个 JSON-RPC 响应
func (conn *ServerConnection) parseSSEResponse(body io.Reader) (*JSONRPCResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 256*1024), 10*1024*1024)

	var currentEvent string
	var currentData strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if currentData.Len() > 0 {
				data := strings.TrimSpace(currentData.String())
				resp, err := conn.tryParseSSEMessage(currentEvent, data)
				if err != nil {
					return nil, err
				}
				if resp != nil {
					return resp, nil
				}
			}
			currentEvent = ""
			currentData.Reset()
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if currentData.Len() > 0 {
				currentData.WriteString("\n")
			}
			currentData.WriteString(data)
		}
	}

	// 处理末尾无空行的情况
	if currentData.Len() > 0 {
		data := strings.TrimSpace(currentData.String())
		resp, err := conn.tryParseSSEMessage(currentEvent, data)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("no JSON-RPC response found in SSE stream")
}

// tryParseSSEMessage 尝试将 SSE 事件数据解析为 JSON-RPC 响应
func (conn *ServerConnection) tryParseSSEMessage(event, data string) (*JSONRPCResponse, error) {
	if data == "" || !strings.HasPrefix(data, "{") {
		return nil, nil
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, nil // 不是有效的 JSON，跳过
	}

	// 只返回有 ID 的响应（不是通知）
	if resp.ID != nil {
		return &resp, nil
	}
	return nil, nil
}

// sendStreamableHTTPRequest 通过 Streamable HTTP 发送请求并返回响应
func (conn *ServerConnection) sendStreamableHTTPRequest(reqJSON []byte) (*JSONRPCResponse, error) {
	url := conn.Config.URL

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if conn.sessionID != "" {
		req.Header.Set("MCP-Session-Id", conn.sessionID)
	}
	if conn.Config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.Config.AuthToken)
	}

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP POST failed: %w", err)
	}
	defer resp.Body.Close()

	// 更新 session ID（服务器可能在响应中更新）
	if sid := resp.Header.Get("MCP-Session-Id"); sid != "" {
		conn.sessionID = sid
	}

	if resp.StatusCode == http.StatusAccepted {
		// 202 Accepted：通知已接受，无响应体
		return nil, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP POST returned status %d: %s", resp.StatusCode, string(body))
	}

	// 根据 Content-Type 解析响应
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "text/event-stream") {
		return conn.parseSSEResponse(resp.Body)
	}

	// JSON 响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var jsonrpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonrpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &jsonrpcResp, nil
}

// sendStreamableNotification 通过 Streamable HTTP 发送通知（无响应）
func (conn *ServerConnection) sendStreamableNotification(notifJSON []byte) error {
	url := conn.Config.URL

	req, err := http.NewRequest("POST", url, bytes.NewReader(notifJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if conn.sessionID != "" {
		req.Header.Set("MCP-Session-Id", conn.sessionID)
	}
	if conn.Config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.Config.AuthToken)
	}

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed: %w", err)
	}
	resp.Body.Close()
	return nil
}

// Connect 连接MCP服务器
func (c *Client) Connect(ctx context.Context, server *model.MCPServer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.servers[server.ID]; ok && existing.connected {
		return nil // 已连接
	}

	conn := &ServerConnection{
		Config:        server,
		connected:     true,
		lastUsed:      time.Now(),
		pending:       make(map[int64]*pendingRequest),
		endpointReady: make(chan struct{}),
	}

	// 对于 stdio 模式，启动子进程
	if server.Transport == model.TransportStdio && server.Command != "" {
		cmd := c.buildCommand(server.Command, server.Env)
		conn.Process = cmd

		// 设置 stdout 和 stdin
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdin pipe: %w", err)
		}
		conn.stdinPipe = stdin

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			stdin.Close()
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		// 同时捕获 stderr 用于调试
		stderr, err := cmd.StderrPipe()
		if err != nil {
			stdin.Close()
			stdout.Close()
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// 日志 stderr 输出 (stderr 仍然使用 Scanner，因为不需要非阻塞)
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				log.Printf("[MCP STDERR] %s", scanner.Text())
			}
		}()

		// 使用带缓冲的 writer 和 scanner
		conn.stdin = bufio.NewWriter(stdin)
		scanner := bufio.NewScanner(stdout)
		// 增加缓冲区大小以处理大型工具定义响应 (10MB max)
		scanner.Buffer(make([]byte, 0, 256*1024), 10*1024*1024)
		conn.stdout = bufio.NewReader(stdout) // 保留原始 reader

		// 创建 line channel 用于传递行数据
		conn.lineCh = make(chan string, 100) // 带缓冲的 channel

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start MCP server: %w", err)
		}

		// 启动 scanner goroutine 持续读取行并发送到 lineCh
		conn.readerCtx, conn.readerCancel = context.WithCancel(context.Background())
		conn.sseDone = make(chan struct{})
		conn.readerDone = make(chan struct{})
		go conn.scannerLoop(scanner)
		go conn.readerLoop()

		// 后台等待进程退出
		go func() {
			cmd.Wait()
			conn.mu.Lock()
			conn.connected = false
			conn.mu.Unlock()
			conn.readerCancel() // 通知 reader 停止
			conn.closeOnce.Do(func() {
				close(conn.lineCh) // 关闭 channel 通知 scanner
			})
			<-conn.readerDone
			log.Printf("[MCP] Process exited")
		}()
	} else if server.Transport == model.TransportHTTP && server.URL != "" {
		// HTTP 模式：自动检测 Streamable HTTP 或旧版 SSE
		conn.httpClient = &http.Client{
			Timeout: 60 * time.Second,
		}

		// 1. 先尝试 Streamable HTTP
		streamableCtx, streamableCancel := context.WithTimeout(ctx, 15*time.Second)
		err := conn.connectStreamableHTTP(streamableCtx)
		streamableCancel()

		if err == nil {
			// Streamable HTTP 成功
			conn.httpMode = "streamable"
			log.Printf("[MCP] Using Streamable HTTP transport for %s", server.URL)
		} else if err == errStreamableNotSupported {
			// 2. 回退到旧版 SSE 传输
			log.Printf("[MCP] Streamable HTTP not supported, falling back to SSE transport")
			conn.httpMode = "sse"

			// 创建 line channel 用于传递 SSE 数据
			conn.lineCh = make(chan string, 100)

			// 创建 SSE 读取上下文
			sseCtx, sseCancel := context.WithCancel(context.Background())
			conn.sseCancel = sseCancel
			conn.readerCtx = sseCtx
			conn.readerCancel = sseCancel

			// 启动 SSE 连接 goroutine
			conn.sseDone = make(chan struct{})
			conn.readerDone = make(chan struct{})
			go conn.sseReaderLoop(sseCtx, server.URL, server.AuthToken)

			// 启动 readerLoop 处理响应
			go conn.readerLoop()

			// 等待 endpoint 事件到达或超时
			select {
			case <-conn.endpointReady:
				log.Printf("[MCP] SSE endpoint received: %s", conn.messageEndpoint)
			case <-time.After(15 * time.Second):
				sseCancel()
				conn.mu.Lock()
				conn.connected = false
				conn.mu.Unlock()
				delete(c.servers, server.ID)
				return fmt.Errorf("timeout waiting for SSE endpoint event from %s", server.URL)
			}
		} else {
			// Streamable HTTP 尝试出错
			conn.connected = false
			delete(c.servers, server.ID)
			return fmt.Errorf("failed to connect to MCP server %s: %w", server.URL, err)
		}
	} else {
		// 不支持的传输类型或缺少必要参数
		return fmt.Errorf("unsupported transport or missing parameters: transport=%s", server.Transport)
	}

	c.servers[server.ID] = conn

	// 对于 Streamable HTTP，初始化已在 connectStreamableHTTP 中完成
	if conn.httpMode == "streamable" {
		return nil
	}

	// 初始化 MCP 协议（stdio 和旧版 SSE 模式）
	if err := conn.initialize(ctx); err != nil {
		conn.mu.Lock()
		conn.connected = false
		conn.mu.Unlock()
		if conn.readerCancel != nil {
			conn.readerCancel()
		}
		if conn.readerDone != nil {
			<-conn.readerDone
		}
		delete(c.servers, server.ID)
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	return nil
}

// sseReaderLoop 连接到 SSE 端点并持续读取事件
func (conn *ServerConnection) sseReaderLoop(ctx context.Context, sseURL, authToken string) {
	defer close(conn.sseDone)
	log.Printf("[MCP SSE] Starting SSE reader loop for %s", sseURL)

	const (
		maxRetries    = 10               // 最大重试次数
		baseDelay     = 1 * time.Second  // 基础延迟
		maxDelay      = 30 * time.Second // 最大延迟
		resetDuration = 60 * time.Second // 重置重试计数的时间间隔
	)

	var retryCount int
	var retryDelay = baseDelay
	lastRetryTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[MCP SSE] Reader loop exiting: context cancelled")
			return
		default:
		}

		// 重置重试计数：如果超过 resetDuration 时间没有重试
		if time.Since(lastRetryTime) > resetDuration {
			retryCount = 0
			retryDelay = baseDelay
		}

		if err := conn.connectSSE(ctx, sseURL, authToken); err != nil {
			log.Printf("[MCP SSE] Connection error: %v", err)

			retryCount++
			lastRetryTime = time.Now()

			if retryCount >= maxRetries {
				log.Printf("[MCP SSE] Max retries (%d) exceeded, giving up", maxRetries)
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
				// 指数退避：延迟时间翻倍，但不超过最大延迟
				retryDelay = retryDelay * 2
				if retryDelay > maxDelay {
					retryDelay = maxDelay
				}
				continue
			}
		}

		// SSE 连接正常断开，重置重试计数
		log.Printf("[MCP SSE] Connection closed, attempting reconnect")
		retryCount = 0
		retryDelay = baseDelay
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			continue
		}
	}
}

// connectSSE 建立单个 SSE 连接并读取事件
func (conn *ServerConnection) connectSSE(ctx context.Context, sseURL, authToken string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE connection returned status %d", resp.StatusCode)
	}

	log.Printf("[MCP SSE] Connected to %s (status %d)", sseURL, resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 10*1024*1024)

	var currentEvent string
	var currentData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Text()

		if line == "" {
			// 空行表示事件结束，处理当前事件
			if currentData.Len() > 0 {
				data := currentData.String()
				conn.handleSSEEvent(currentEvent, data)
			}
			currentEvent = ""
			currentData.Reset()
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			if currentData.Len() > 0 {
				currentData.WriteString("\n")
			}
			currentData.WriteString(data)
		}
		// 忽略 id:, retry: 等其他 SSE 字段
	}

	// 处理最后一个事件（如果文件不以空行结尾）
	if currentData.Len() > 0 {
		conn.handleSSEEvent(currentEvent, currentData.String())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("SSE read error: %w", err)
	}

	return nil
}

// handleSSEEvent 处理单个 SSE 事件
func (conn *ServerConnection) handleSSEEvent(event, data string) {
	data = strings.TrimSpace(data)

	switch event {
	case "endpoint":
		// 接收到 message endpoint URL
		endpoint := data
		// 如果是相对路径，拼接为完整 URL
		if strings.HasPrefix(endpoint, "/") && conn.Config != nil {
			// 从配置的 URL 提取 scheme + host，避免路径重复
			parsedURL, err := url.Parse(conn.Config.URL)
			if err == nil {
				base := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
				endpoint = base + endpoint
			} else {
				// 回退：直接拼接
				endpoint = conn.Config.URL + endpoint
			}
		}
		conn.messageEndpoint = endpoint
		log.Printf("[MCP SSE] Received endpoint: %s", endpoint)

		// 通知 Connect() endpoint 已就绪（只通知一次）
		select {
		case conn.endpointReady <- struct{}{}:
		default:
		}

	case "message":
		// JSON-RPC 响应消息，转发到 lineCh
		if data != "" {
			select {
			case conn.lineCh <- data:
			case <-conn.readerCtx.Done():
			}
		}

	default:
		// 如果没有 event 字段但有 data，可能是 JSON-RPC 消息
		if data != "" && strings.HasPrefix(data, "{") {
			select {
			case conn.lineCh <- data:
			case <-conn.readerCtx.Done():
			}
		}
	}
}

// scannerLoop 持续用 scanner 读取行并发送到 lineCh
func (conn *ServerConnection) scannerLoop(scanner *bufio.Scanner) {
	log.Printf("[MCP] Scanner loop started")
	for scanner.Scan() {
		line := scanner.Text()
		select {
		case conn.lineCh <- line:
			// 行已发送到 channel
		case <-conn.readerCtx.Done():
			log.Printf("[MCP] Scanner loop exiting: context cancelled")
			return
		}
	}

	// scanner 结束
	err := scanner.Err()
	if err != nil {
		log.Printf("[MCP] Scanner loop error: %v", err)
	} else {
		log.Printf("[MCP] Scanner loop EOF")
	}

	// 通知 readerLoop scanner 已结束 (使用 closeOnce 确保只关闭一次)
	conn.closeOnce.Do(func() {
		close(conn.lineCh)
	})
}

// readerLoop 从 lineCh 接收行并分发响应
func (conn *ServerConnection) readerLoop() {
	defer close(conn.readerDone)
	log.Printf("[MCP] Reader loop started")

	for {
		// 等待行或 context 取消
		select {
		case <-conn.readerCtx.Done():
			log.Printf("[MCP] Reader loop exiting: context cancelled")
			// 通知所有待处理的请求 context 被取消
			conn.pendingMu.Lock()
			for id, req := range conn.pending {
				log.Printf("[MCP] Notifying pending request %d (%s) of context cancellation", id, req.method)
				req.errCh <- fmt.Errorf("context cancelled")
			}
			conn.pending = make(map[int64]*pendingRequest)
			conn.pendingMu.Unlock()
			return
		case line, ok := <-conn.lineCh:
			if !ok {
				// channel 已关闭 (scanner 结束)
				log.Printf("[MCP] Reader loop: lineCh closed")
				// 通知所有待处理的请求失败
				conn.pendingMu.Lock()
				for id, req := range conn.pending {
					log.Printf("[MCP] Notifying pending request %d (%s) of scanner EOF", id, req.method)
					req.errCh <- fmt.Errorf("scanner EOF")
				}
				conn.pending = make(map[int64]*pendingRequest)
				conn.pendingMu.Unlock()
				return
			}

			// 处理行
			if line == "" {
				continue
			}

			log.Printf("[MCP] Raw received: %s", truncate(line, 300))

			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				log.Printf("[MCP] Parse error: %v", err)
				continue
			}

			// 跳过通知 (ID 为 nil)
			if resp.ID == nil {
				if strings.Contains(line, "notifications/") {
					log.Printf("[MCP] Received notification: %s", truncate(line, 200))
					// 处理 tools/list_changed 通知
					if strings.Contains(line, "notifications/tools/list_changed") {
						log.Printf("[MCP] Tools list changed, refreshing cache")
						conn.mu.Lock()
						conn.toolsCache = nil // Clear cache, will be re-fetched on next request
						conn.mu.Unlock()
					}
				}
				continue
			}

			// 查找对应的待处理请求
			conn.pendingMu.Lock()
			respID := int64(0)
			if fid, ok := resp.ID.(float64); ok {
				respID = int64(fid)
			}
			req, ok := conn.pending[respID]
			if ok {
				delete(conn.pending, respID)
			}
			conn.pendingMu.Unlock()

			if ok {
				log.Printf("[MCP] Matched response for id=%d (%s)", respID, req.method)
				// 使用非阻塞发送，避免 context 取消时阻塞
				select {
				case req.resultCh <- &resp:
				case <-conn.readerCtx.Done():
					log.Printf("[MCP] Response arrived but context cancelled, discarding for id=%d", respID)
				}
			} else {
				log.Printf("[MCP] No pending request for id=%d, ignoring", respID)
			}
		}
	}
}

// initialize 发送 MCP 初始化请求
func (conn *ServerConnection) initialize(ctx context.Context) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// 发送 initialize 请求
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
		ClientInfo: ClientInfo{
			Name:    "wailschat",
			Version: "1.0.0",
		},
	}

	resp, err := conn.sendRequestLocked(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	// 解析 server info
	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	log.Printf("[MCP] Connected to server: %s v%s (protocol: %s)",
		result.ServerInfo.Name, result.ServerInfo.Version, result.ProtocolVersion)

	conn.initialized = true

	// 发送 initialized 通知 (MCP 协议要求)
	conn.sendNotificationLocked("initialized", struct{}{})

	// 获取工具列表
	conn.toolsCache, err = conn.listToolsLocked(ctx)
	if err != nil {
		log.Printf("[MCP] Warning: failed to list tools: %v", err)
	}

	return nil
}

// listToolsLocked 获取工具列表 (需要在持有锁时调用)
func (conn *ServerConnection) listToolsLocked(ctx context.Context) ([]model.MCPTool, error) {
	resp, err := conn.sendRequestLocked(ctx, "tools/list", ToolsListParams{})
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	tools := make([]model.MCPTool, len(result.Tools))
	for i, t := range result.Tools {
		tools[i] = model.MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	log.Printf("[MCP] Found %d tools", len(tools))
	return tools, nil
}

// sendRequestLocked 发送 JSON-RPC 请求并等待响应 (需要在持有锁时调用)
func (conn *ServerConnection) sendRequestLocked(ctx context.Context, method string, params any) (*JSONRPCResponse, error) {
	id := conn.requestID.Add(1)

	// 序列化 params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// 构建请求
	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建通道用于接收响应
	resultCh := make(chan *JSONRPCResponse, 1)
	errCh := make(chan error, 1)

	// 注册待处理请求
	conn.pendingMu.Lock()
	conn.pending[id] = &pendingRequest{
		resultCh: resultCh,
		errCh:    errCh,
		method:   method,
	}
	conn.pendingMu.Unlock()

	// 根据传输方式发送请求
	log.Printf("[MCP] Sending request: %s (id=%d)", method, id)
	if conn.Config.Transport == model.TransportHTTP {
		if conn.httpMode == "streamable" {
			// Streamable HTTP：直接在 POST 响应中获取结果
			resp, err := conn.sendStreamableHTTPRequest(reqJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to send streamable HTTP request: %w", err)
			}
			if resp == nil {
				return nil, fmt.Errorf("streamable HTTP returned nil response")
			}
			return resp, nil
		}
		// 旧版 SSE 模式
		if err := conn.sendHTTPRequest(reqJSON); err != nil {
			conn.pendingMu.Lock()
			delete(conn.pending, id)
			conn.pendingMu.Unlock()
			return nil, fmt.Errorf("failed to send HTTP request: %w", err)
		}
	} else {
		if conn.stdin == nil {
			conn.pendingMu.Lock()
			delete(conn.pending, id)
			conn.pendingMu.Unlock()
			return nil, fmt.Errorf("stdin not initialized (transport: %s)", conn.Config.Transport)
		}
		if _, err := conn.stdin.WriteString(string(reqJSON) + "\n"); err != nil {
			conn.pendingMu.Lock()
			delete(conn.pending, id)
			conn.pendingMu.Unlock()
			return nil, fmt.Errorf("failed to write request: %w", err)
		}
		if err := conn.stdin.Flush(); err != nil {
			conn.pendingMu.Lock()
			delete(conn.pending, id)
			conn.pendingMu.Unlock()
			return nil, fmt.Errorf("failed to flush stdin: %w", err)
		}
	}

	// 创建请求级别的超时
	reqCtx, reqCancel := context.WithTimeout(ctx, 30*time.Second)
	defer reqCancel()

	// 等待响应或上下文取消
	select {
	case <-reqCtx.Done():
		// 超时或取消，移除待处理请求
		conn.pendingMu.Lock()
		delete(conn.pending, id)
		conn.pendingMu.Unlock()
		log.Printf("[MCP] Request %s (id=%d) cancelled/timeout: %v", method, id, reqCtx.Err())
		return nil, fmt.Errorf("request %s (id=%d) failed: %w", method, id, reqCtx.Err())
	case resp := <-resultCh:
		log.Printf("[MCP] Response received for %s (id=%d)", method, id)
		return resp, nil
	case err := <-errCh:
		log.Printf("[MCP] Error for %s (id=%d): %v", method, id, err)
		return nil, err
	}
}

// sendHTTPRequest 通过 HTTP POST 发送 JSON-RPC 请求
func (conn *ServerConnection) sendHTTPRequest(reqJSON []byte) error {
	if conn.messageEndpoint == "" {
		return fmt.Errorf("message endpoint not configured")
	}

	req, err := http.NewRequest("POST", conn.messageEndpoint, bytes.NewReader(reqJSON))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if conn.Config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+conn.Config.AuthToken)
	}

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP POST returned status %d", resp.StatusCode)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// sendNotificationLocked 发送 JSON-RPC 通知 (不需要响应, 需要在持有锁时调用)
func (conn *ServerConnection) sendNotificationLocked(method string, params any) {
	paramsJSON, _ := json.Marshal(params)
	notif := JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  paramsJSON,
	}
	notifJSON, _ := json.Marshal(notif)

	if conn.Config.Transport == model.TransportHTTP {
		if conn.httpMode == "streamable" {
			if err := conn.sendStreamableNotification(notifJSON); err != nil {
				log.Printf("[MCP] Failed to send streamable HTTP notification %s: %v", method, err)
			}
			return
		}
		// 旧版 SSE 模式
		if err := conn.sendHTTPRequest(notifJSON); err != nil {
			log.Printf("[MCP] Failed to send HTTP notification %s: %v", method, err)
		}
		return
	}

	// stdio 模式
	if conn.stdin != nil {
		conn.stdin.WriteString(string(notifJSON) + "\n")
		conn.stdin.Flush()
	}
}

// parseCommandLine 解析命令行字符串，正确处理引号
func parseCommandLine(command string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)
	escape := false

	for _, r := range command {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}

		if r == '\\' && !inQuotes {
			// Windows 路径中的反斜杠在引号外需要保留
			current.WriteRune(r)
			continue
		}

		if r == '\\' && inQuotes {
			escape = true
			continue
		}

		if (r == '"' || r == '\'') && !inQuotes {
			inQuotes = true
			quoteChar = r
			continue
		}

		if r == quoteChar && inQuotes {
			inQuotes = false
			quoteChar = 0
			continue
		}

		if unicode.IsSpace(r) && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// Disconnect 断开MCP服务器连接
func (c *Client) Disconnect(serverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, ok := c.servers[serverID]
	if !ok {
		return nil
	}

	if conn.Config.Transport == model.TransportHTTP {
		// HTTP/SSE 模式：取消 SSE 连接
		if conn.sseCancel != nil {
			conn.sseCancel()
		}
		if conn.sseDone != nil {
			select {
			case <-conn.sseDone:
			case <-time.After(3 * time.Second):
				log.Printf("[MCP] Timeout waiting for SSE reader to exit")
			}
		}
		if conn.readerDone != nil {
			select {
			case <-conn.readerDone:
			case <-time.After(3 * time.Second):
				log.Printf("[MCP] Timeout waiting for response reader to exit")
			}
		}
	} else {
		// stdio 模式：关闭进程
		if conn.stdinPipe != nil {
			conn.stdinPipe.Close()
		}
		if conn.readerCancel != nil {
			conn.readerCancel()
		}
		if conn.readerDone != nil {
			select {
			case <-conn.readerDone:
			case <-time.After(3 * time.Second):
				log.Printf("[MCP] Timeout waiting for reader to exit")
			}
		}
		// 等待一小段时间让进程自然退出
		time.Sleep(500 * time.Millisecond)
		if conn.Process != nil && conn.Process.Process != nil {
			if conn.Process.ProcessState == nil || !conn.Process.ProcessState.Exited() {
				conn.Process.Process.Kill()
			}
		}
	}

	delete(c.servers, serverID)
	return nil
}

// ListTools 获取服务器工具列表
func (c *Client) ListTools(ctx context.Context, serverID string) ([]model.MCPTool, error) {
	c.mu.RLock()
	conn, ok := c.servers[serverID]
	c.mu.RUnlock()

	if !ok || !conn.connected {
		return nil, fmt.Errorf("server not connected: %s", serverID)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	// 如果有缓存，直接返回
	if conn.toolsCache != nil {
		return conn.toolsCache, nil
	}

	// 否则重新获取
	return conn.listToolsLocked(ctx)
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, serverID string, toolName string, args map[string]any) (*model.ToolCallResult, error) {
	start := time.Now()

	c.mu.RLock()
	conn, ok := c.servers[serverID]
	c.mu.RUnlock()

	if !ok || !conn.connected {
		return &model.ToolCallResult{
			ToolName:   toolName,
			Error:      fmt.Sprintf("server not connected: %s", serverID),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	// 序列化参数 (确保 arguments 是对象而不是 null)
	var argsJSON json.RawMessage
	var err error
	if len(args) == 0 {
		argsJSON = json.RawMessage("{}")
	} else {
		argsJSON, err = json.Marshal(args)
		if err != nil {
			return &model.ToolCallResult{
				ToolName:   toolName,
				Error:      fmt.Sprintf("failed to marshal arguments: %v", err),
				DurationMs: time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// 发送工具调用请求
	params := ToolsCallParams{
		Name:      toolName,
		Arguments: argsJSON,
	}

	// 打印调试信息
	paramsJSON, _ := json.Marshal(params)
	log.Printf("[MCP] Calling tool with params: %s", string(paramsJSON))

	conn.mu.Lock()
	resp, err := conn.sendRequestLocked(ctx, "tools/call", params)
	conn.mu.Unlock()

	if err != nil {
		return &model.ToolCallResult{
			ToolName:   toolName,
			Error:      fmt.Sprintf("tools/call failed: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	if resp.Error != nil {
		return &model.ToolCallResult{
			ToolName:   toolName,
			Error:      fmt.Sprintf("tools/call error: %s (code %d)", resp.Error.Message, resp.Error.Code),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	log.Printf("[MCP] Tool response raw: %s", string(resp.Result))

	var result ToolsCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return &model.ToolCallResult{
			ToolName:   toolName,
			Error:      fmt.Sprintf("failed to parse result: %v", err),
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	// 提取文本内容
	var content strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	log.Printf("[MCP] Called tool %s on server %s, result length: %d", toolName, serverID, content.Len())

	return &model.ToolCallResult{
		ToolName:   toolName,
		Result:     content.String(),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// TestConnection 测试服务器连接
func (c *Client) TestConnection(ctx context.Context, server *model.MCPServer) (*model.MCPServerTestResult, error) {
	result := &model.MCPServerTestResult{}

	// 临时创建一个完整的 ServerConnection
	tempConn := &ServerConnection{
		Config:        server,
		connected:     true,
		pending:       make(map[int64]*pendingRequest),
		lineCh:        make(chan string, 100),
		endpointReady: make(chan struct{}),
	}
	tempConn.requestID.Store(0)

	cleanup := func() {
		if tempConn.sseCancel != nil {
			tempConn.sseCancel()
		}
		if tempConn.readerCancel != nil {
			tempConn.readerCancel()
		}
		if tempConn.readerDone != nil {
			select {
			case <-tempConn.readerDone:
			case <-time.After(2 * time.Second):
			}
		}
		if tempConn.stdin != nil {
			tempConn.stdin.Flush()
		}
		if tempConn.stdinPipe != nil {
			tempConn.stdinPipe.Close()
		}
		if tempConn.Process != nil && tempConn.Process.Process != nil {
			tempConn.Process.Process.Kill()
		}
	}
	defer cleanup()

	if server.Transport == model.TransportStdio && server.Command != "" {
		// stdio 模式：启动子进程
		cmd := c.buildCommand(server.Command, server.Env)
		tempConn.Process = cmd

		stdin, err := cmd.StdinPipe()
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to create stdin pipe: %v", err)
			return result, nil
		}
		tempConn.stdinPipe = stdin
		tempConn.stdin = bufio.NewWriter(stdin)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to create stdout pipe: %v", err)
			return result, nil
		}

		// 启动 readerLoop 来处理响应
		tempConn.readerCtx, tempConn.readerCancel = context.WithCancel(context.Background())
		tempConn.readerDone = make(chan struct{})
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		go tempConn.scannerLoop(scanner)
		go tempConn.readerLoop()

		if err := cmd.Start(); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to start process: %v", err)
			return result, nil
		}
	} else if server.Transport == model.TransportHTTP && server.URL != "" {
		// HTTP 模式：自动检测 Streamable HTTP 或旧版 SSE
		tempConn.httpClient = &http.Client{
			Timeout: 60 * time.Second,
		}

		// 1. 先尝试 Streamable HTTP
		streamableCtx, streamableCancel := context.WithTimeout(ctx, 10*time.Second)
		serr := tempConn.connectStreamableHTTP(streamableCtx)
		streamableCancel()

		if serr == nil {
			// Streamable HTTP 成功，直接测试工具列表
			tempConn.httpMode = "streamable"
			tempConn.initialized = true
			log.Printf("[MCP Test] Streamable HTTP connected to %s", server.URL)

			// 发送 initialized 通知
			tempConn.sendNotificationLocked("initialized", struct{}{})

			// 获取工具列表
			toolsResp, err := tempConn.sendRequestLocked(ctx, "tools/list", ToolsListParams{})
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("tools/list failed: %v", err)
				return result, nil
			}

			if toolsResp.Error != nil {
				result.Success = false
				result.Error = fmt.Sprintf("tools/list error: %s", toolsResp.Error.Message)
				return result, nil
			}

			var toolsResult ToolsListResult
			if err := json.Unmarshal(toolsResp.Result, &toolsResult); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("failed to parse tools list: %v", err)
				return result, nil
			}

			result.Success = true
			result.Tools = make([]model.MCPTool, len(toolsResult.Tools))
			for i, t := range toolsResult.Tools {
				result.Tools[i] = model.MCPTool{
					Name:        t.Name,
					Description: t.Description,
					InputSchema: t.InputSchema,
				}
			}
			return result, nil
		}

		// 2. 回退到旧版 SSE
		log.Printf("[MCP Test] Streamable HTTP not supported, falling back to SSE")
		tempConn.httpMode = "sse"

		sseCtx, sseCancel := context.WithCancel(context.Background())
		tempConn.sseCancel = sseCancel
		tempConn.readerCtx = sseCtx
		tempConn.readerCancel = sseCancel
		tempConn.sseDone = make(chan struct{})
		tempConn.readerDone = make(chan struct{})

		go tempConn.sseReaderLoop(sseCtx, server.URL, server.AuthToken)
		go tempConn.readerLoop()

		// 等待 endpoint 事件
		select {
		case <-tempConn.endpointReady:
			log.Printf("[MCP Test] SSE endpoint received: %s", tempConn.messageEndpoint)
		case <-time.After(10 * time.Second):
			result.Success = false
			result.Error = fmt.Sprintf("timeout waiting for SSE endpoint event from %s", server.URL)
			return result, nil
		}
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("unsupported transport: %s", server.Transport)
		return result, nil
	}

	// 尝试初始化
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
		ClientInfo: ClientInfo{
			Name:    "wailschat-test",
			Version: "1.0.0",
		},
	}

	resp, err := tempConn.sendRequestLocked(ctx, "initialize", params)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("initialize failed: %v", err)
		return result, nil
	}

	if resp.Error != nil {
		result.Success = false
		result.Error = fmt.Sprintf("initialize error: %s", resp.Error.Message)
		return result, nil
	}

	tempConn.initialized = true

	// 发送 initialized 通知
	tempConn.sendNotificationLocked("initialized", struct{}{})

	// 获取工具列表
	toolsResp, err := tempConn.sendRequestLocked(ctx, "tools/list", ToolsListParams{})
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("tools/list failed: %v", err)
		return result, nil
	}

	if toolsResp.Error != nil {
		result.Success = false
		result.Error = fmt.Sprintf("tools/list error: %s", toolsResp.Error.Message)
		return result, nil
	}

	var toolsResult ToolsListResult
	if err := json.Unmarshal(toolsResp.Result, &toolsResult); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to parse tools list: %v", err)
		return result, nil
	}

	result.Success = true
	result.Tools = make([]model.MCPTool, len(toolsResult.Tools))
	for i, t := range toolsResult.Tools {
		result.Tools[i] = model.MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	return result, nil
}

// IsConnected 检查服务器是否已连接
func (c *Client) IsConnected(serverID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	conn, ok := c.servers[serverID]
	return ok && conn.connected
}

// GetConnectedServers 获取所有已连接的服务器ID
func (c *Client) GetConnectedServers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.servers))
	for id, conn := range c.servers {
		if conn.connected {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetTools 获取指定服务器的可用工具列表（内部使用缓存）
func (c *Client) GetTools(serverID string) ([]model.MCPTool, error) {
	return c.ListTools(context.Background(), serverID)
}

// Close 关闭所有连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id := range c.servers {
		conn := c.servers[id]
		if conn.sseCancel != nil {
			conn.sseCancel()
		}
		if conn.readerCancel != nil {
			conn.readerCancel()
		}
		if conn.readerDone != nil {
			select {
			case <-conn.readerDone:
			case <-time.After(2 * time.Second):
			}
		}
		if conn.stdin != nil {
			conn.stdin.Flush()
		}
		if conn.stdinPipe != nil {
			conn.stdinPipe.Close()
		}
		if conn.Process != nil && conn.Process.Process != nil {
			conn.Process.Process.Kill()
		}
		delete(c.servers, id)
	}
	return nil
}
