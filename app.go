package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wailschat/internal/db"
	"wailschat/internal/fonts"
	"wailschat/internal/llm"
	"wailschat/internal/mcp"
	"wailschat/internal/message"
	"wailschat/internal/model"
	"wailschat/internal/notify"
	"wailschat/internal/prompt"
	"wailschat/internal/provider"
	"wailschat/internal/session"
	"wailschat/internal/settings"
	"wailschat/internal/tools"
)

type mcpToolMapping struct {
	serverID string
	toolName string
}

type App struct {
	ctx                    context.Context
	db                     *sql.DB
	providerSvc            *provider.Service
	promptSvc              *prompt.Service
	sessionSvc             *session.Service
	messageSvc             *message.Service
	settingsSvc            *settings.Service
	mcpServerSvc           *db.MCPServerService
	mcpClient              *mcp.Client
	llmClient              *llm.Client
	cancelFuncs            sync.Map
	toolManager            *tools.Manager
	wasMaxBeforeFullscreen bool
	mcpToolMu              sync.RWMutex
	mcpToolMap             map[string]mcpToolMapping // sanitizedFQName -> {serverID, toolName}
	serverNameMu           sync.RWMutex
	serverNameCache        map[string]string // serverID -> name (cached)
	selectionChannels      sync.Map          // requestID -> chan model.SelectionResponse
}

func NewApp() *App {
	// Create tool manager with default security constraints
	// Allow all directories by default (can be configured via settings later)
	allowedDirs := []string{} // Empty means no restriction
	cmdBlacklist := tools.DefaultCommandBlacklist()

	return &App{
		llmClient:       llm.NewClient(),
		mcpClient:       mcp.NewClient(),
		toolManager:     tools.NewManager(allowedDirs, cmdBlacklist),
		mcpToolMap:      make(map[string]mcpToolMapping),
		serverNameCache: make(map[string]string),
	}
}

// getServerNameMap returns the cached server ID -> name map.
// If the cache is empty, it refreshes from DB.
func (a *App) getServerNameMap() map[string]string {
	a.serverNameMu.RLock()
	if len(a.serverNameCache) > 0 {
		// Return a snapshot
		result := make(map[string]string, len(a.serverNameCache))
		for k, v := range a.serverNameCache {
			result[k] = v
		}
		a.serverNameMu.RUnlock()
		return result
	}
	a.serverNameMu.RUnlock()

	// Cache miss — refresh
	return a.refreshServerNameCache()
}

func (a *App) refreshServerNameCache() map[string]string {
	m := make(map[string]string)
	if servers, err := a.mcpServerSvc.ListMCPServers(); err == nil {
		for _, s := range servers {
			m[s.ID] = s.Name
		}
	}
	a.serverNameMu.Lock()
	a.serverNameCache = m
	a.serverNameMu.Unlock()

	// Return a snapshot
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	database, err := db.Init()
	if err != nil {
		log.Printf("Failed to init database: %v", err)
		return
	}
	a.db = database
	a.providerSvc = provider.NewService(database)
	a.promptSvc = prompt.NewService(database)
	a.sessionSvc = session.NewService(database)
	a.messageSvc = message.NewService(database)
	a.settingsSvc = settings.NewService(database)
	a.mcpServerSvc = db.NewMCPServerService(database)

	wailsRuntime.WindowSetMaxSize(ctx, 7680, 4320)
	// Register built-in tools
	tools.RegisterBuiltInTools(a.toolManager, a)

	// Restore window position for non-maximised windows
	// (Size and maximised state are already set via options in main.go)
	if saved := loadWindowStateFromFile(); saved != nil && !saved.Maximised && !saved.Fullscreen {
		if isWindowOffScreen(saved, 300) {
			// Window edges exceed screen by >300px — reset to centred default
			x, y, w, h := centerWindow()
			wailsRuntime.WindowSetSize(ctx, w, h)
			wailsRuntime.WindowSetPosition(ctx, x, y)
		} else {
			wailsRuntime.WindowSetPosition(ctx, saved.X, saved.Y)
		}
	}

	// Connect to enabled MCP servers
	go a.connectEnabledMCPServers()
}

func (a *App) ToggleFullscreen() {
	if wailsRuntime.WindowIsFullscreen(a.ctx) {
		wasMax := a.wasMaxBeforeFullscreen
		wailsRuntime.WindowUnfullscreen(a.ctx)
		if wasMax {
			// UnFullscreen uses SetWindowPlacement which may not properly
			// restore maximized state respecting the taskbar. Fix by
			// un-maximizing then re-maximizing through proper Windows path.
			go func() {
				time.Sleep(200 * time.Millisecond)
				wailsRuntime.WindowUnmaximise(a.ctx)
				time.Sleep(100 * time.Millisecond)
				wailsRuntime.WindowMaximise(a.ctx)
			}()
		}
	} else {
		a.wasMaxBeforeFullscreen = wailsRuntime.WindowIsMaximised(a.ctx)
		wailsRuntime.WindowFullscreen(a.ctx)
	}
}

func (a *App) connectEnabledMCPServers() {
	servers, err := a.mcpServerSvc.ListMCPServers()
	if err != nil {
		log.Printf("Failed to list MCP servers: %v", err)
		return
	}

	for _, server := range servers {
		if server.Enabled {
			a.connectMCPServerWithRetry(&server)
		}
	}
}

// connectMCPServerWithRetry connects to an MCP server with exponential backoff retry
func (a *App) connectMCPServerWithRetry(server *model.MCPServer) {
	const maxRetries = 5
	const initialDelay = 1 * time.Second
	const maxDelay = 30 * time.Second

	var lastErr error
	delay := initialDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := a.mcpClient.Connect(context.Background(), server)
		if err == nil {
			if attempt > 1 {
				log.Printf("[MCP] Successfully connected to %s after %d attempts", server.Name, attempt)
			}
			// Emit connected status event
			wailsRuntime.EventsEmit(a.ctx, "mcp_server_status", server.ID, map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
				"status":      "connected",
				"attempt":     attempt,
			})
			return
		}

		lastErr = err
		log.Printf("[MCP] Failed to connect to %s (attempt %d/%d): %v", server.Name, attempt, maxRetries, err)

		// Emit disconnected status event on failure
		wailsRuntime.EventsEmit(a.ctx, "mcp_server_status", server.ID, map[string]interface{}{
			"server_id":   server.ID,
			"server_name": server.Name,
			"status":      "error",
			"attempt":     attempt,
			"error":       err.Error(),
		})

		if attempt < maxRetries {
			// Wait with exponential backoff before retrying
			log.Printf("[MCP] Retrying %s in %v...", server.Name, delay)
			time.Sleep(delay)
			// Exponential backoff: double the delay, cap at maxDelay
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	// All retries exhausted
	log.Printf("[MCP] Failed to connect to %s after %d attempts: %v", server.Name, maxRetries, lastErr)
	wailsRuntime.EventsEmit(a.ctx, "mcp_server_status", server.ID, map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"status":      "failed",
		"error":       lastErr.Error(),
	})
}

func (a *App) shutdown(ctx context.Context) {
	if a.mcpClient != nil {
		a.mcpClient.Close()
	}
	// 关闭 message service 的 prepared statements
	if a.messageSvc != nil {
		a.messageSvc.Close()
	}
	if a.db != nil {
		a.db.Close()
	}
}

// beforeClose is called when the window is about to close.
// The window is still alive here, so we can read its state.
func (a *App) beforeClose(ctx context.Context) bool {
	saveWindowState(ctx)
	return false // allow close
}

// --- Provider Methods ---

func (a *App) AddProvider(name, apiKey, baseURL string, models []string, isDefault bool) (*model.Provider, error) {
	p := &model.Provider{
		Name:      name,
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Models:    model.StringArray(models),
		IsDefault: isDefault,
	}
	if err := a.providerSvc.Add(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (a *App) GetProviders() ([]model.Provider, error) {
	return a.providerSvc.GetAll()
}

func (a *App) UpdateProvider(id int64, name, apiKey, baseURL string, models []string, isDefault bool) error {
	p := &model.Provider{
		ID:        id,
		Name:      name,
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Models:    model.StringArray(models),
		IsDefault: isDefault,
	}
	return a.providerSvc.Update(p)
}

func (a *App) DeleteProvider(id int64) error {
	return a.providerSvc.Delete(id)
}

func (a *App) TestConnection(baseURL, apiKey, model string) error {
	return a.llmClient.TestConnection(baseURL, apiKey, model)
}

func (a *App) GetModels(baseURL, apiKey string) ([]string, error) {
	return a.llmClient.GetModels(baseURL, apiKey)
}

// --- Prompt Methods ---

func (a *App) PromptList() ([]model.Prompt, error) {
	return a.promptSvc.GetAll()
}

func (a *App) PromptCreate(p *model.Prompt) (*model.Prompt, error) {
	if err := a.promptSvc.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (a *App) PromptUpdate(p *model.Prompt) (*model.Prompt, error) {
	if err := a.promptSvc.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (a *App) PromptDelete(id int64) error {
	return a.promptSvc.Delete(id)
}

func (a *App) PromptSetDefault(id int64) error {
	return a.promptSvc.SetDefault(id)
}

// resolveSystemPrompt returns the system prompt content for a given session.
// Priority: session-specific prompt > default prompt > legacy setting.
func (a *App) resolveSystemPrompt(sessionID int64) string {
	// Check if session has a specific prompt assigned
	sess, err := a.sessionSvc.GetByID(sessionID)
	if err == nil && sess.PromptID != nil && *sess.PromptID > 0 {
		p, err := a.promptSvc.GetByID(*sess.PromptID)
		if err == nil && p != nil {
			return p.Content
		}
	}

	// Fall back to the default prompt
	p, err := a.promptSvc.GetDefault()
	if err == nil && p != nil {
		return p.Content
	}

	// Final fallback: legacy system_prompt setting
	content, _ := a.settingsSvc.GetSystemPrompt()
	return content
}

// --- Session Methods ---

func (a *App) CreateSession(providerID int64, name, modelName string, promptID *int64) (*model.Session, error) {
	sess := &model.Session{
		ProviderID: providerID,
		Name:       name,
		Model:      modelName,
		PromptID:   promptID,
	}
	if err := a.sessionSvc.Create(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (a *App) GetSessions() ([]model.Session, error) {
	return a.sessionSvc.GetAll()
}

func (a *App) UpdateSession(id int64, providerID int64, name, modelName string, promptID *int64) error {
	sess := &model.Session{
		ID:         id,
		ProviderID: providerID,
		Name:       name,
		Model:      modelName,
		PromptID:   promptID,
	}
	return a.sessionSvc.Update(sess)
}

func (a *App) DeleteSession(id int64) error {
	return a.sessionSvc.Delete(id)
}

// --- Settings Methods ---

func (a *App) GetSettings() (map[string]string, error) {
	return a.settingsSvc.GetAll()
}

func (a *App) SaveSettings(s map[string]string) error {
	for k, v := range s {
		if err := a.settingsSvc.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) GetSystemFonts() ([]string, error) {
	return fonts.GetSystemFonts()
}

func (a *App) GetDefaultStyles() string {
	return db.DefaultStyles()
}

// --- Theme Methods ---

// ThemeInfo describes a theme available for selection.
type ThemeInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	CSS       string `json:"css"`
}

// GetThemes returns all available themes (built-in Default + custom from themes folder).
func (a *App) GetThemes() ([]ThemeInfo, error) {
	var result []ThemeInfo

	// Always include the built-in Default theme
	result = append(result, ThemeInfo{
		Name:      "Default",
		IsDefault: true,
		CSS:       db.DefaultStyles(),
	})

	// Read custom themes from the themes directory
	dataDir, err := db.DataDir()
	if err != nil {
		return result, nil // Return at least Default
	}
	themesDir := filepath.Join(dataDir, "themes")
	os.MkdirAll(themesDir, 0755)

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return result, nil
	}

	var customNames []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".css") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if name == "Default" {
			continue
		}
		customNames = append(customNames, name)
	}
	sort.Strings(customNames)

	for _, name := range customNames {
		data, err := os.ReadFile(filepath.Join(themesDir, name+".css"))
		if err != nil {
			continue
		}
		result = append(result, ThemeInfo{
			Name:      name,
			IsDefault: false,
			CSS:       string(data),
		})
	}

	return result, nil
}

// GetThemeCSS returns the CSS content for a given theme name.
// For custom themes, relative url() paths are resolved to absolute file:// URLs.
func (a *App) GetThemeCSS(themeName string) (string, error) {
	if themeName == "" || themeName == "Default" {
		return db.DefaultStyles(), nil
	}

	// Validate theme name — no path traversal
	if strings.ContainsAny(themeName, `/\`) || strings.Contains(themeName, "..") {
		return "", fmt.Errorf("invalid theme name: %s", themeName)
	}

	dataDir, err := db.DataDir()
	if err != nil {
		return "", err
	}
	themesDir := filepath.Join(dataDir, "themes")
	cssPath := filepath.Join(themesDir, themeName+".css")

	data, err := os.ReadFile(cssPath)
	if err != nil {
		return "", fmt.Errorf("read theme %s: %w", themeName, err)
	}

	css := string(data)
	css = resolveThemeCSSPaths(css, themesDir, themeName)
	return css, nil
}

// SaveThemeCSS writes CSS content to a theme file on disk.
func (a *App) SaveThemeCSS(themeName string, css string) error {
	if themeName == "" {
		return fmt.Errorf("theme name cannot be empty")
	}
	if themeName == "Default" {
		return fmt.Errorf("cannot overwrite the built-in Default theme")
	}
	// Validate: alphanumeric, hyphens, underscores only
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(themeName) {
		return fmt.Errorf("theme name must contain only letters, numbers, hyphens, and underscores")
	}
	if len(themeName) > 64 {
		return fmt.Errorf("theme name too long (max 64 characters)")
	}

	dataDir, err := db.DataDir()
	if err != nil {
		return err
	}
	themesDir := filepath.Join(dataDir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("create themes dir: %w", err)
	}

	cssPath := filepath.Join(themesDir, themeName+".css")
	if err := os.WriteFile(cssPath, []byte(css), 0644); err != nil {
		return fmt.Errorf("write theme %s: %w", themeName, err)
	}
	return nil
}

// OpenThemeFolder opens the themes directory in the system file explorer.
func (a *App) OpenThemeFolder() error {
	dataDir, err := db.DataDir()
	if err != nil {
		return err
	}
	themesDir := filepath.Join(dataDir, "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		return fmt.Errorf("create themes dir: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", themesDir)
	case "darwin":
		cmd = exec.Command("open", themesDir)
	default:
		cmd = exec.Command("xdg-open", themesDir)
	}
	return cmd.Start()
}

// resolveThemeCSSPaths rewrites relative url() and @import paths in CSS to absolute file:// URLs.
// Resources are resolved relative to <themesDir>/<themeName>/ subdirectory.
func resolveThemeCSSPaths(css string, themesDir string, themeName string) string {
	resourceDir := filepath.Join(themesDir, themeName)
	absResourceDir := filepath.ToSlash(resourceDir)

	// Resolve url() references
	urlRe := regexp.MustCompile(`url\(\s*["']?([^"')]+?)["']?\s*\)`)
	css = urlRe.ReplaceAllStringFunc(css, func(match string) string {
		sub := urlRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		path := sub[1]
		if isAbsoluteURL(path) {
			return match
		}
		absPath := filepath.Join(absResourceDir, filepath.FromSlash(path))
		absPath = filepath.ToSlash(absPath)
		return `url("` + "file:///" + absPath + `")`
	})

	// Resolve @import with relative paths
	importRe := regexp.MustCompile(`@import\s+["']([^"']+)["']`)
	css = importRe.ReplaceAllStringFunc(css, func(match string) string {
		sub := importRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		path := sub[1]
		if isAbsoluteURL(path) {
			return match
		}
		absPath := filepath.Join(absResourceDir, filepath.FromSlash(path))
		absPath = filepath.ToSlash(absPath)
		return `@import "` + "file:///" + absPath + `"`
	})

	return css
}

func isAbsoluteURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "data:") ||
		strings.HasPrefix(s, "file://") ||
		strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "#")
}

// OpenFileDialog opens a native file dialog for selecting an image file.
// Returns the selected file path, or empty string if cancelled.
func (a *App) OpenFileDialog() (string, error) {
	result, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Background Image",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp;*.svg"},
		},
	})
	return result, err
}

// ReadImageAsBase64 reads a local image file and returns it as a data URI (base64 encoded).
func (a *App) ReadImageAsBase64(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	mime := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	case ".bmp":
		mime = "image/bmp"
	case ".svg":
		mime = "image/svg+xml"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// FetchImageAsBase64 downloads an image from a URL and returns it as a base64 data URI.
// This bypasses CORS restrictions that prevent canvas-based copying in the frontend.
func (a *App) FetchImageAsBase64(imageURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return "", fmt.Errorf("read image data: %w", err)
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = http.DetectContentType(data)
	}

	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// --- Message Methods ---

func (a *App) GetHistory(sessionID int64) ([]model.Message, error) {
	return a.messageSvc.GetBySession(sessionID)
}

// ClearSessionMessages deletes all messages in a session (clears context).
func (a *App) ClearSessionMessages(sessionID int64) error {
	return a.messageSvc.DeleteBySession(sessionID)
}

// SendMessage sends a user message and streams the LLM response via Wails events.
// It returns the saved user message ID as a string; the response is streamed asynchronously.
func (a *App) SendMessage(sessionID int64, content string, images []string) (string, error) {
	// Convert images array to JSON string
	imagesJSON := "[]"
	if len(images) > 0 {
		imagesBytes, err := json.Marshal(images)
		if err == nil {
			imagesJSON = string(imagesBytes)
		}
	}

	// Save user message
	userMsg := &model.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   content,
		Images:    imagesJSON,
	}
	if err := a.messageSvc.Create(userMsg); err != nil {
		return "", fmt.Errorf("send message: save user message: %w", err)
	}

	// Move session to top of list
	a.sessionSvc.TouchSession(sessionID)

	// Load session to get provider and model
	sess, err := a.sessionSvc.GetByID(sessionID)
	if err != nil {
		return "", fmt.Errorf("send message: get session: %w", err)
	}

	// Auto-generate title if session still has default name (right after user sends message)
	if sess.Name == "New Chat" {
		go a.generateTitleAsync(sessionID, content, sess)
	}

	// Parallelize independent DB queries: provider, history, system prompt
	var (
		provider     *model.Provider
		history      []model.Message
		systemPrompt string
		providerErr  error
		historyErr   error
	)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		provider, providerErr = a.providerSvc.GetByID(sess.ProviderID)
	}()
	go func() {
		defer wg.Done()
		history, historyErr = a.messageSvc.GetBySession(sessionID)
	}()
	// resolveSystemPrompt does DB queries too but is fast; run inline
	systemPrompt = a.resolveSystemPrompt(sessionID)
	wg.Wait()

	if providerErr != nil {
		return "", fmt.Errorf("send message: get provider: %w", providerErr)
	}
	if historyErr != nil {
		return "", fmt.Errorf("send message: get history: %w", historyErr)
	}

	chatMsgs := buildChatMessages(history, systemPrompt)

	// Set up cancellation - generous timeout for reasoning models that can think for many minutes.
	// The LLM client manages per-activity timeouts internally; this is a safety net.
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	a.cancelFuncs.Store(sessionID, cancel)

	// Stream in goroutine
	go func() {
		defer func() {
			a.cancelFuncs.Delete(sessionID)
			cancel()
		}()

		a.streamAndSave(ctx, sessionID, sess, provider, chatMsgs)
	}()

	return fmt.Sprintf("%d", userMsg.ID), nil
}

// RetryMessage deletes the given assistant message (and all after it) and re-streams.
func (a *App) RetryMessage(sessionID int64, messageID string) error {
	// Delete the assistant message and all messages after it
	if err := a.messageSvc.DeleteFromID(sessionID, messageID); err != nil {
		return fmt.Errorf("retry: delete message: %w", err)
	}

	// Move session to top of list
	a.sessionSvc.TouchSession(sessionID)

	// Load session
	sess, err := a.sessionSvc.GetByID(sessionID)
	if err != nil {
		return fmt.Errorf("retry: get session: %w", err)
	}

	// Load provider
	p, err := a.providerSvc.GetByID(sess.ProviderID)
	if err != nil {
		return fmt.Errorf("retry: get provider: %w", err)
	}

	// Load remaining messages
	history, err := a.messageSvc.GetBySession(sessionID)
	if err != nil {
		return fmt.Errorf("retry: get history: %w", err)
	}

	// Resolve system prompt for this session
	systemPrompt := a.resolveSystemPrompt(sessionID)
	chatMsgs := buildChatMessages(history, systemPrompt)

	// Set up cancellation - generous timeout for reasoning models
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	a.cancelFuncs.Store(sessionID, cancel)

	go func() {
		defer func() {
			a.cancelFuncs.Delete(sessionID)
			cancel()
		}()

		a.streamAndSave(ctx, sessionID, sess, p, chatMsgs)
	}()

	return nil
}

// CancelMessage cancels an in-flight streaming response.
func (a *App) CancelMessage(sessionID int64) {
	if val, ok := a.cancelFuncs.Load(sessionID); ok {
		val.(context.CancelFunc)()
	}
}

// RetryFromUserMessage deletes a user message (and all after it), re-saves it, and re-streams.
// Returns the new user message ID as a string.
func (a *App) RetryFromUserMessage(sessionID int64, messageID string) (string, error) {
	// Read the user message before deleting
	history, err := a.messageSvc.GetBySession(sessionID)
	if err != nil {
		return "", fmt.Errorf("retry from user: get history: %w", err)
	}

	var userContent string
	var userImages string
	var found bool
	for _, m := range history {
		if fmt.Sprintf("%d", m.ID) == messageID {
			userContent = m.Content
			userImages = m.Images
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("retry from user: message not found")
	}

	// Delete from user message onwards
	if err := a.messageSvc.DeleteFromID(sessionID, messageID); err != nil {
		return "", fmt.Errorf("retry from user: delete: %w", err)
	}

	// Re-save user message with a new record
	newUserMsg := &model.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   userContent,
		Images:    userImages,
	}
	if err := a.messageSvc.Create(newUserMsg); err != nil {
		return "", fmt.Errorf("retry from user: re-save user message: %w", err)
	}

	// Move session to top of list
	a.sessionSvc.TouchSession(sessionID)

	// Load session and provider
	sess, err := a.sessionSvc.GetByID(sessionID)
	if err != nil {
		return "", fmt.Errorf("retry from user: get session: %w", err)
	}
	p, err := a.providerSvc.GetByID(sess.ProviderID)
	if err != nil {
		return "", fmt.Errorf("retry from user: get provider: %w", err)
	}

	// Load remaining history (including the re-saved user message)
	remainingHistory, err := a.messageSvc.GetBySession(sessionID)
	if err != nil {
		return "", fmt.Errorf("retry from user: get history: %w", err)
	}

	systemPrompt := a.resolveSystemPrompt(sessionID)
	chatMsgs := buildChatMessages(remainingHistory, systemPrompt)

	// Set up cancellation - generous timeout for reasoning models
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	a.cancelFuncs.Store(sessionID, cancel)

	go func() {
		defer func() {
			a.cancelFuncs.Delete(sessionID)
			cancel()
		}()
		a.streamAndSave(ctx, sessionID, sess, p, chatMsgs)
	}()

	return fmt.Sprintf("%d", newUserMsg.ID), nil
}

// EditAndResendMessage replaces a user message with edited content, deletes all
// subsequent messages, and re-streams the AI response.
func (a *App) EditAndResendMessage(sessionID int64, messageID string, newContent string, newImages []string) (string, error) {
	imagesJSON := "[]"
	if len(newImages) > 0 {
		if b, err := json.Marshal(newImages); err == nil {
			imagesJSON = string(b)
		}
	}

	if err := a.messageSvc.DeleteFromID(sessionID, messageID); err != nil {
		return "", fmt.Errorf("edit and resend: delete: %w", err)
	}

	newUserMsg := &model.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   newContent,
		Images:    imagesJSON,
	}
	if err := a.messageSvc.Create(newUserMsg); err != nil {
		return "", fmt.Errorf("edit and resend: save user message: %w", err)
	}

	a.sessionSvc.TouchSession(sessionID)

	sess, err := a.sessionSvc.GetByID(sessionID)
	if err != nil {
		return "", fmt.Errorf("edit and resend: get session: %w", err)
	}
	p, err := a.providerSvc.GetByID(sess.ProviderID)
	if err != nil {
		return "", fmt.Errorf("edit and resend: get provider: %w", err)
	}

	history, err := a.messageSvc.GetBySession(sessionID)
	if err != nil {
		return "", fmt.Errorf("edit and resend: get history: %w", err)
	}

	systemPrompt := a.resolveSystemPrompt(sessionID)
	chatMsgs := buildChatMessages(history, systemPrompt)

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Minute)
	a.cancelFuncs.Store(sessionID, cancel)

	go func() {
		defer func() {
			a.cancelFuncs.Delete(sessionID)
			cancel()
		}()
		a.streamAndSave(ctx, sessionID, sess, p, chatMsgs)
	}()

	return fmt.Sprintf("%d", newUserMsg.ID), nil
}

// ReorderSessions persists a new display order for sessions.
// orderedIDs should be in the desired display order (first = top).
func (a *App) ReorderSessions(orderedIDs []int64) error {
	return a.sessionSvc.ReorderSessions(orderedIDs)
}

// chunkBatcher buffers streaming chunks and flushes them in batches to reduce IPC overhead.
// Instead of one EventsEmit per SSE chunk (~30-60/sec), it batches and emits every ~50ms.
type chunkBatcher struct {
	ctx       context.Context
	sessionID int64
	ch        chan chunkPair
	done      chan struct{}
}

type chunkPair struct {
	content   string
	reasoning string
}

func newChunkBatcher(ctx context.Context, sessionID int64) *chunkBatcher {
	b := &chunkBatcher{
		ctx:       ctx,
		sessionID: sessionID,
		ch:        make(chan chunkPair, 64),
		done:      make(chan struct{}),
	}
	go b.flushLoop()
	return b
}

func (b *chunkBatcher) flushLoop() {
	defer close(b.done)
	var contentBuf strings.Builder
	var reasoningBuf strings.Builder
	contentBuf.Grow(4096)
	reasoningBuf.Grow(1024)
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()

	flush := func() {
		c := contentBuf.String()
		r := reasoningBuf.String()
		if c != "" {
			wailsRuntime.EventsEmit(b.ctx, "message_chunk", b.sessionID, c)
		}
		if r != "" {
			wailsRuntime.EventsEmit(b.ctx, "message_reasoning", b.sessionID, r)
		}
		contentBuf.Reset()
		reasoningBuf.Reset()
	}

	for {
		select {
		case p, ok := <-b.ch:
			if !ok {
				flush()
				return
			}
			if p.content != "" {
				contentBuf.WriteString(p.content)
			}
			if p.reasoning != "" {
				reasoningBuf.WriteString(p.reasoning)
			}
			// Flush immediately if buffer is large enough
			if contentBuf.Len() > 512 || reasoningBuf.Len() > 512 {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				flush()
				timer.Reset(50 * time.Millisecond)
			}
		case <-timer.C:
			timer.Reset(50 * time.Millisecond)
			flush()
		}
	}
}

func (b *chunkBatcher) emit(chunk, reasoningChunk string) {
	b.ch <- chunkPair{content: chunk, reasoning: reasoningChunk}
}

func (b *chunkBatcher) close() {
	close(b.ch)
	<-b.done
}

// streamAndSave is the shared streaming logic for SendMessage and RetryMessage.
// It handles MCP tool calling loop integration.
func (a *App) streamAndSave(
	ctx context.Context,
	sessionID int64,
	sess *model.Session,
	p *model.Provider,
	chatMsgs []model.ChatMessage,
) {
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	// 预分配容量：典型响应约 1-4KB，避免频繁扩容
	fullContent.Grow(4096)
	maxToolCalls := 10 // Limit tool call iterations

	// Get enabled MCP tools
	tools := a.GetEnabledMCPTools()

	// Build server ID -> name mapping for UI display (cached)
	serverNameMap := a.getServerNameMap()

	// Tool call handling state - track for persistence
	var toolCalls []model.ToolCall
	toolCallMap := make(map[string]model.ToolCall) // id -> toolCall
	var allToolCalls []model.ToolCall              // all tool calls made during this session
	var allToolResults []*model.ToolCallResult     // all tool results received

	// Initialize chunk batcher to reduce IPC overhead
	batcher := newChunkBatcher(a.ctx, sessionID)

	stats, err := a.llmClient.StreamChat(ctx, p.BaseURL, p.APIKey, sess.Model, chatMsgs,
		func(chunk string, reasoningChunk string, newToolCalls []model.ToolCall, finishReason string) {
			fullContent.WriteString(chunk)
			if reasoningChunk != "" {
				fullReasoning.WriteString(reasoningChunk)
			}

			// Collect tool calls (now they come pre-accumulated)
			for _, tc := range newToolCalls {
				if tc.ID != "" {
					toolCallMap[tc.ID] = tc
				}
			}

			if chunk != "" || reasoningChunk != "" {
				batcher.emit(chunk, reasoningChunk)
			}
		}, tools)

	if err != nil {
		batcher.close()
		wailsRuntime.EventsEmit(a.ctx, "message_error", sessionID, err.Error())
		a.notifyIfEnabled(sess.Name, "生成出错")
		return
	}

	// Convert tool call map to slice (only keep the final set)
	for _, tc := range toolCallMap {
		toolCalls = append(toolCalls, tc)
	}

	// Tool call loop: if LLM made tool calls, execute them and continue
	iteration := 0
	for len(toolCalls) > 0 && iteration < maxToolCalls {
		iteration++
		log.Printf("[MCP] Processing %d tool calls (iteration %d)", len(toolCalls), iteration)

		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			log.Printf("[MCP] Context cancelled, exiting tool call loop")
			batcher.close()
			wailsRuntime.EventsEmit(a.ctx, "message_error", sessionID, "context cancelled")
			return
		default:
		}

		// 如果 iteration > 1，说明之前的 tool results 可能没有被 LLM 正确处理
		// 添加短暂延迟让 LLM 处理响应
		if iteration > 1 {
			time.Sleep(200 * time.Millisecond)
		}

		// Emit tool call start event
		wailsRuntime.EventsEmit(a.ctx, "mcp_tool_call_start", sessionID, map[string]interface{}{
			"tool_calls":   toolCalls,
			"server_names": serverNameMap,
		})

		// Add assistant message with tool calls to chat history (OpenAI format)
		assistantMsg := model.ChatMessage{
			Role:      "assistant",
			Content:   fullContent.String(),
			ToolCalls: toolCalls,
		}
		chatMsgs = append(chatMsgs, assistantMsg)

		// Execute each tool call
		var toolResults []model.ChatMessage
		for _, tc := range toolCalls {
			var toolResult *model.ToolCallResult

			if a.toolManager.GetTool(tc.Function.Name) != nil {
				// Built-in tool: name is not sanitized, use directly
				toolResult = a.executeBuiltInTool(tc.Function.Name, tc.Function.Arguments)
			} else if realServerID, realToolName, ok := a.resolveMCPToolName(tc.Function.Name); ok {
				// MCP tool: resolve original serverID and toolName from mapping
				var err error
				toolResult, err = a.CallMCP_tool(realServerID, tc.Function.Name, tc.Function.Arguments)
				if err != nil {
					toolResult = &model.ToolCallResult{
						ToolName:   realToolName,
						ServerName: serverNameMap[realServerID],
						Error:      err.Error(),
					}
				} else {
					toolResult.ServerName = serverNameMap[realServerID]
				}
			} else {
				// Fallback: try extracting from FQ name (shouldn't normally reach here)
				toolResult = &model.ToolCallResult{
					ToolName: extractToolName(tc.Function.Name),
					Error:    "tool not found in mapping",
				}
			}
			// Truncate result to prevent context overflow (10000 chars max)
			const maxResultLen = 10000
			if len(toolResult.Result) > maxResultLen {
				truncated := toolResult.Result[:maxResultLen]
				lastNewline := strings.LastIndex(truncated, "\n")
				if lastNewline > maxResultLen-200 {
					truncated = truncated[:lastNewline]
				}
				toolResult.Result = truncated + "\n\n[... result truncated, original length: " + fmt.Sprintf("%d", len(toolResult.Result)) + " chars ...]"
			}

			allToolResults = append(allToolResults, toolResult) // track for persistence

			// 构建工具结果内容
			var resultContent string
			if toolResult.Error != "" {
				// 直接传递错误信息，不做额外包装
				resultContent = fmt.Sprintf("Error calling tool %s: %s", toolResult.ToolName, toolResult.Error)
			} else {
				// 直接传递结果，不做额外包装和转义
				resultContent = toolResult.Result
				if resultContent == "" {
					resultContent = "{}"
				}
			}

			// 构建 tool 角色消息（OpenAI 格式）
			toolMsg := model.ChatMessage{
				Role:       "tool",
				Content:    resultContent,
				Name:       extractToolName(tc.Function.Name),
				ToolCallID: tc.ID,
			}
			toolResults = append(toolResults, toolMsg)

			// Emit tool result event
			wailsRuntime.EventsEmit(a.ctx, "mcp_tool_result", sessionID, toolResult)
		}

		// Add tool results to chat history
		chatMsgs = append(chatMsgs, toolResults...)

		// Reset for next iteration
		fullContent.Reset()
		fullReasoning.Reset()
		toolCallMap = make(map[string]model.ToolCall)

		// Continue streaming with tool results
		stats, err = a.llmClient.StreamChat(ctx, p.BaseURL, p.APIKey, sess.Model, chatMsgs,
			func(chunk string, reasoningChunk string, newToolCalls []model.ToolCall, finishReason string) {
				fullContent.WriteString(chunk)
				if reasoningChunk != "" {
					fullReasoning.WriteString(reasoningChunk)
				}
				for _, tc := range newToolCalls {
					if tc.ID != "" {
						toolCallMap[tc.ID] = tc
					}
				}
				if chunk != "" || reasoningChunk != "" {
					batcher.emit(chunk, reasoningChunk)
				}
			}, tools)

		if err != nil {
			// 如果有错误，先检查是否还有未处理的 toolCalls
			if len(toolCallMap) > 0 {
				log.Printf("[MCP] StreamChat error but still have %d toolCalls, continuing", len(toolCallMap))
				toolCalls = nil
				for _, tc := range toolCallMap {
					toolCalls = append(toolCalls, tc)
				}
				// 继续下一次迭代，而不是直接返回
				continue
			}
			batcher.close()
			wailsRuntime.EventsEmit(a.ctx, "message_error", sessionID, err.Error())
			return
		}

		// Collect new tool calls
		toolCalls = nil
		for _, tc := range toolCallMap {
			toolCalls = append(toolCalls, tc)
		}
	}

	if iteration >= maxToolCalls {
		log.Printf("[MCP] Max tool call iterations (%d) reached", maxToolCalls)
	}

	// Emit performance stats and prepare JSON for persistence
	var statsJSON string
	if stats != nil {
		wailsRuntime.EventsEmit(a.ctx, "message_stats", sessionID, stats)
		if b, err := json.Marshal(stats); err == nil {
			statsJSON = string(b)
		}
	}

	// Serialize tool calls and results for persistence
	var toolCallsJSON, toolResultsJSON string
	if len(allToolCalls) > 0 {
		if b, err := json.Marshal(allToolCalls); err == nil {
			toolCallsJSON = string(b)
		}
	}
	if len(allToolResults) > 0 {
		if b, err := json.Marshal(allToolResults); err == nil {
			toolResultsJSON = string(b)
		}
	}

	// Save assistant message with tool calls and results
	assistantMsg := &model.Message{
		SessionID:        sessionID,
		Role:             "assistant",
		Content:          fullContent.String(),
		ReasoningContent: fullReasoning.String(),
		Images:           "[]",
		StatsJSON:        statsJSON,
		ToolCallsJSON:    toolCallsJSON,
		ToolResultsJSON:  toolResultsJSON,
	}
	if saveErr := a.messageSvc.Create(assistantMsg); saveErr != nil {
		log.Printf("Failed to save assistant message: %v", saveErr)
	}

	// Flush and close the batcher BEFORE emitting message_done.
	// This ensures all streaming chunks have been delivered to the frontend
	// before the frontend finalizes the message. Without this, a race condition
	// causes the frontend to finalize with incomplete content (while the DB save
	// is correct since fullContent is accumulated synchronously).
	batcher.close()

	// Emit message_done with the saved message ID (as string for frontend compatibility)
	wailsRuntime.EventsEmit(a.ctx, "message_done", sessionID, fmt.Sprintf("%d", assistantMsg.ID))

	// Show Windows toast notification if enabled
	a.notifyIfEnabled(sess.Name, "AI 回复完成")
}

// notifyIfEnabled shows a Windows toast notification if the notify_on_complete setting is enabled.
func (a *App) notifyIfEnabled(sessionName, body string) {
	val, err := a.settingsSvc.Get("notify_on_complete")
	if err != nil || val != "1" {
		return
	}
	title := "WailsChat"
	if sessionName != "" && sessionName != "New Chat" {
		title = sessionName
	}
	notify.Show(title, body)
}

// generateTitleAsync generates a session title based on user's question content.
// If LLM title generation fails, it falls back to using the first 15 characters of the content.
func (a *App) generateTitleAsync(sessionID int64, userContent string, sess *model.Session) {
	titlePrompt := []model.ChatMessage{
		{Role: "system", Content: "为以下用户提问生成一个简短的会话标题，推荐10个字以内，最长不超过15个字。只输出标题本身，不要加引号，末尾不要加标点符号。"},
		{Role: "user", Content: userContent},
	}

	title := a.generateTitleWithRetry(titlePrompt, sess)

	if title == "" {
		// Fallback: use first 15 characters of user content
		runes := []rune(userContent)
		if len(runes) > 15 {
			truncated := string(runes[:15])
			if idx := strings.LastIndexAny(truncated, " \t\n，。！？、；："); idx > 0 {
				truncated = truncated[:idx]
			}
			title = truncated + "…"
		} else {
			title = string(runes)
		}
	}

	// Reload session to check if it was renamed by the user in the meantime
	currentSess, err := a.sessionSvc.GetByID(sessionID)
	if err != nil {
		log.Printf("Title generation: failed to reload session: %v", err)
		return
	}
	if currentSess.Name != "New Chat" {
		return // User already renamed it, skip
	}

	currentSess.Name = title
	if updateErr := a.sessionSvc.Update(currentSess); updateErr != nil {
		log.Printf("Title generation: failed to update session name: %v", updateErr)
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "session_renamed", sessionID, title)
}

// generateTitleWithRetry attempts to generate a title using the session's provider,
// and falls back to the default provider if it fails.
func (a *App) generateTitleWithRetry(prompt []model.ChatMessage, sess *model.Session) string {
	// Try with session's provider first
	p, err := a.providerSvc.GetByID(sess.ProviderID)
	if err == nil && len(p.Models) > 0 {
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		defer cancel()

		title, chatErr := a.llmClient.Chat(ctx, p.BaseURL, p.APIKey, p.Models[0], prompt)
		if chatErr == nil && title != "" {
			return cleanTitle(title)
		}
		if chatErr != nil {
			log.Printf("Title generation failed with session provider: %v", chatErr)
		}
	}

	// Retry with default provider
	defProv, defErr := a.providerSvc.GetDefault()
	if defErr != nil {
		log.Printf("Failed to get default provider for title retry: %v", defErr)
		return ""
	}

	if len(defProv.Models) == 0 {
		log.Printf("Default provider has no models, skipping title retry")
		return ""
	}

	retryCtx, retryCancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer retryCancel()

	title, err := a.llmClient.Chat(retryCtx, defProv.BaseURL, defProv.APIKey, defProv.Models[0], prompt)
	if err != nil {
		log.Printf("Title generation retry with default provider also failed: %v", err)
		return ""
	}
	if title == "" {
		return ""
	}
	return cleanTitle(title)
}

// cleanTitle trims whitespace, quotes, punctuation, and truncates to maxRunes characters.
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'"+"\u201c\u201d")
	// Truncate to maxRunes characters (runes, not bytes) at a word boundary if possible
	const maxRunes = 20
	if utf8.RuneCountInString(title) <= maxRunes {
		return title
	}
	runes := []rune(title)
	title = string(runes[:maxRunes])
	// Trim trailing incomplete word (cut at last space/punctuation if reasonable)
	if idx := strings.LastIndexAny(title, " \t，。！？、；："); idx > maxRunes/2 {
		title = title[:idx]
	}
	return strings.TrimRight(title, " \t，。！？、；：")
}

// extractServerID extracts the server ID from a fully qualified tool name (format: "{serverID}___{toolName}")
func extractServerID(fqToolName string) string {
	idx := strings.Index(fqToolName, "___")
	if idx > 0 {
		return fqToolName[:idx]
	}
	return ""
}

// sanitizeToolName converts an arbitrary string to a valid function identifier.
// Must match [a-zA-Z_][a-zA-Z0-9_]* for compatibility with all LLM APIs (e.g. Kimi).
func sanitizeToolName(s string) string {
	var b strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
			b.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			if i == 0 {
				// Leading digit — prefix with underscore
				b.WriteString("_")
			}
			b.WriteRune(r)
		} else if r == '-' {
			b.WriteRune('_')
		}
		// drop other chars (braces, dots, etc.)
	}
	result := b.String()
	if result == "" {
		return "_tool"
	}
	return result
}

// makeMCPToolName creates a fully qualified tool name from serverID and toolName.
// Both parts are sanitized so the result is always a valid identifier.
func makeMCPToolName(serverID, toolName string) string {
	return sanitizeToolName(serverID) + "___" + sanitizeToolName(toolName)
}

// extractToolName extracts the original tool name from a fully qualified tool name
func extractToolName(fqToolName string) string {
	idx := strings.Index(fqToolName, "___")
	if idx > 0 && idx+3 < len(fqToolName) {
		return fqToolName[idx+3:]
	}
	return fqToolName
}

// resolveMCPToolName looks up the original (serverID, toolName) for a sanitized FQ name.
func (a *App) resolveMCPToolName(sanitizedFQName string) (serverID, toolName string, ok bool) {
	a.mcpToolMu.RLock()
	defer a.mcpToolMu.RUnlock()
	m, found := a.mcpToolMap[sanitizedFQName]
	if !found {
		return "", "", false
	}
	return m.serverID, m.toolName, true
}

// executeBuiltInTool executes a built-in tool and returns the result
func (a *App) executeBuiltInTool(toolName string, arguments string) *model.ToolCallResult {
	startTime := time.Now()

	// Parse arguments
	parseResult := (&model.FunctionCall{Name: toolName, Arguments: arguments}).ParseArguments()
	args := parseResult.Args

	// If parsing failed, try raw args as fallback
	if parseResult.RawArgs != "" && len(args) == 0 {
		if err := json.Unmarshal([]byte(parseResult.RawArgs), &args); err != nil {
			// If still fails, return error
			return &model.ToolCallResult{
				ToolName:   toolName,
				ServerName: "Built-in",
				Error:      fmt.Sprintf("failed to parse arguments: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
		}
	}

	// Execute the tool
	result, err := a.toolManager.ExecuteTool(toolName, args)
	if err != nil {
		return &model.ToolCallResult{
			ToolName:   toolName,
			ServerName: "Built-in",
			Error:      err.Error(),
			DurationMs: time.Since(startTime).Milliseconds(),
		}
	}

	return &model.ToolCallResult{
		ToolName:   toolName,
		ServerName: "Built-in",
		Result:     result,
		DurationMs: time.Since(startTime).Milliseconds(),
	}
}

// truncateLog truncates a string for safe logging (max 200 chars by default).
func truncateLog(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 200
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

// buildChatMessages converts message history to ChatMessage slice, prepending system prompt.
// systemPrompt is passed in to avoid repeated DB queries.
func buildChatMessages(history []model.Message, systemPrompt string) []model.ChatMessage {
	var chatMsgs []model.ChatMessage

	for _, m := range history {
		// Parse images from JSON
		var imgList []string
		if m.Images != "" {
			json.Unmarshal([]byte(m.Images), &imgList)
		}

		// Build content based on whether there are images
		if len(imgList) > 0 {
			contentArray := []interface{}{
				map[string]interface{}{"type": "text", "text": m.Content},
			}
			for _, img := range imgList {
				contentArray = append(contentArray, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": img,
					},
				})
			}
			chatMsgs = append(chatMsgs, model.ChatMessage{Role: m.Role, Content: contentArray, ReasoningContent: m.ReasoningContent})
		} else {
			chatMsgs = append(chatMsgs, model.ChatMessage{Role: m.Role, Content: m.Content, ReasoningContent: m.ReasoningContent})
		}
	}

	// Prepend system prompt if provided
	if systemPrompt != "" {
		chatMsgs = append([]model.ChatMessage{{Role: "system", Content: systemPrompt}}, chatMsgs...)
	}

	return chatMsgs
}

// --- MCP Server Methods ---

// MCPServerList 获取所有MCP服务器
func (a *App) MCPServerList() ([]model.MCPServer, error) {
	return a.mcpServerSvc.ListMCPServers()
}

// MCPServerCreate 创建MCP服务器
func (a *App) MCPServerCreate(server *model.MCPServer) (*model.MCPServer, error) {
	if err := a.mcpServerSvc.CreateMCPServer(server); err != nil {
		return nil, err
	}

	// If enabled, connect to the server
	if server.Enabled {
		if err := a.mcpClient.Connect(context.Background(), server); err != nil {
			log.Printf("Failed to connect to new MCP server %s: %v", server.Name, err)
		}
	}

	return server, nil
}

// MCPServerUpdate 更新MCP服务器
func (a *App) MCPServerUpdate(server *model.MCPServer) (*model.MCPServer, error) {
	// Get old server to check if enabled status changed
	oldServer, err := a.mcpServerSvc.GetMCPServer(server.ID)
	if err != nil {
		return nil, err
	}

	if err := a.mcpServerSvc.UpdateMCPServer(server); err != nil {
		return nil, err
	}

	// Handle enabled status change
	if oldServer.Enabled && !server.Enabled {
		// Disconnect if disabled
		a.mcpClient.Disconnect(server.ID)
	} else if !oldServer.Enabled && server.Enabled {
		// Connect if enabled
		a.mcpClient.Connect(context.Background(), server)
	} else if server.Enabled {
		// Reconnect if enabled and config changed
		a.mcpClient.Disconnect(server.ID)
		a.mcpClient.Connect(context.Background(), server)
	}

	return server, nil
}

// MCPServerDelete 删除MCP服务器
func (a *App) MCPServerDelete(id string) error {
	// Disconnect first
	a.mcpClient.Disconnect(id)

	return a.mcpServerSvc.DeleteMCPServer(id)
}

// MCPServerTest 测试MCP服务器连接
func (a *App) MCPServerTest(server *model.MCPServer) (*model.MCPServerTestResult, error) {
	// 添加 30 秒超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.mcpClient.TestConnection(ctx, server)
}

// MCPServerGetTools 获取MCP服务器工具列表
func (a *App) MCPServerGetTools(id string) ([]model.MCPTool, error) {
	if !a.mcpClient.IsConnected(id) {
		return nil, fmt.Errorf("server not connected")
	}
	return a.mcpClient.ListTools(context.Background(), id)
}

// MCPServerConnect 连接到MCP服务器
func (a *App) MCPServerConnect(id string) error {
	server, err := a.mcpServerSvc.GetMCPServer(id)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}
	if err := a.mcpClient.Connect(context.Background(), server); err != nil {
		return err
	}
	server.Enabled = true
	a.refreshServerNameCache() // Invalidate cache
	return a.mcpServerSvc.UpdateMCPServer(server)
}

// ConnectMCPServerFromConfig 从配置连接MCP服务器（内部使用）
func (a *App) ConnectMCPServerFromConfig(server *model.MCPServer) error {
	return a.mcpClient.Connect(context.Background(), server)
}

// MCPServerDisconnect 断开MCP服务器连接
func (a *App) MCPServerDisconnect(id string) error {
	if err := a.mcpClient.Disconnect(id); err != nil {
		return err
	}
	server, err := a.mcpServerSvc.GetMCPServer(id)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}
	server.Enabled = false
	a.refreshServerNameCache() // Invalidate cache
	return a.mcpServerSvc.UpdateMCPServer(server)
}

// MCPServerGetStatus 获取MCP服务器连接状态
func (a *App) MCPServerGetStatus(id string) string {
	if a.mcpClient.IsConnected(id) {
		return "connected"
	}
	return "disconnected"
}

// MCPServerGetAllStatuses 获取所有MCP服务器连接状态
func (a *App) MCPServerGetAllStatuses() map[string]string {
	statuses := make(map[string]string)
	connected := a.mcpClient.GetConnectedServers()
	connectedSet := make(map[string]bool)
	for _, id := range connected {
		connectedSet[id] = true
	}

	servers, err := a.mcpServerSvc.ListMCPServers()
	if err != nil {
		return statuses
	}

	for _, s := range servers {
		if connectedSet[s.ID] {
			statuses[s.ID] = "connected"
		} else {
			statuses[s.ID] = "disconnected"
		}
	}
	return statuses
}

// GetEnabledMCPTools 获取所有已启用且已连接的 MCP 服务器的工具列表
// 并转换为 OpenAI function calling 格式
func (a *App) GetEnabledMCPTools() []model.Tool {
	var allTools []model.Tool

	// Add MCP tools
	connectedServers := a.mcpClient.GetConnectedServers()
	if len(connectedServers) > 0 {
		connectedSet := make(map[string]bool)
		for _, id := range connectedServers {
			connectedSet[id] = true
		}

		servers, err := a.mcpServerSvc.ListMCPServers()
		if err != nil {
			log.Printf("Failed to list MCP servers for tools: %v", err)
		} else {
			for _, server := range servers {
				if !server.Enabled || !connectedSet[server.ID] {
					continue
				}

				mcpTools, err := a.mcpClient.GetTools(server.ID)
				if err != nil {
					log.Printf("Failed to get tools from MCP server %s: %v", server.Name, err)
					continue
				}

				for _, tool := range mcpTools {
					// Prefix tool name with server ID for routing
					fqName := makeMCPToolName(server.ID, tool.Name)

					// Store mapping from sanitized FQ name to original serverID + toolName
					a.mcpToolMu.Lock()
					a.mcpToolMap[fqName] = mcpToolMapping{serverID: server.ID, toolName: tool.Name}
					a.mcpToolMu.Unlock()

					allTools = append(allTools, model.Tool{
						Type: "function",
						Function: model.FunctionDef{
							Name:        fqName,
							Description: "[" + server.Name + "] " + tool.Description,
							Parameters:  tool.InputSchema,
						},
					})
				}
			}
		}
	}

	// Add built-in tools (each tool's individual enable switch is checked inside getBuiltInTools)
	builtInTools := a.getBuiltInTools()
	allTools = append(allTools, builtInTools...)

	return allTools
}

// isToolEnabled checks if built-in tools are enabled via settings
func (a *App) isToolEnabled() bool {
	settings, err := a.settingsSvc.GetAll()
	if err != nil {
		return false
	}
	enabled := settings["tool_enabled"]
	return enabled == "1" || enabled == "true"
}

// isToolEnabledByName checks if a specific built-in tool is enabled via settings
func (a *App) isToolEnabledByName(toolName string) bool {
	settings, err := a.settingsSvc.GetAll()
	if err != nil {
		return false
	}
	// Map tool names to setting keys
	settingKey := map[string]string{
		"file_read":          "tool_file_read",
		"file_write":         "tool_file_write",
		"shell_exec":         "tool_shell_exec",
		"provide_selection":  "tool_provide_selection",
	}[toolName]
	if settingKey == "" {
		return false
	}
	enabled := settings[settingKey]
	// provide_selection defaults to enabled when setting is absent
	if enabled == "" && toolName == "provide_selection" {
		return true
	}
	return enabled == "1" || enabled == "true"
}

// getBuiltInTools returns built-in tools that are individually enabled
func (a *App) getBuiltInTools() []model.Tool {
	toolList := a.toolManager.GetAllTools()
	var tools []model.Tool

	for _, t := range toolList {
		// Check if this specific tool is enabled
		if !a.isToolEnabledByName(t.Name()) {
			continue
		}
		tools = append(tools, model.Tool{
			Type: "function",
			Function: model.FunctionDef{
				Name:        t.Name(),
				Description: "[Built-in] " + t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}

	return tools
}

// CallMCP_tool 调用指定的 MCP 工具并返回结果
func (a *App) CallMCP_tool(serverID, fqToolName, arguments string) (*model.ToolCallResult, error) {
	startTime := time.Now()

	// Log with truncated arguments for security (avoid leaking sensitive data)
	log.Printf("[MCP CallMCP_tool] serverID=%s, fqToolName=%s, arguments=%s", serverID, fqToolName, truncateLog(arguments, 200))

	// Resolve original serverID and toolName from mapping
	originalToolName := fqToolName
	if realServerID, realToolName, ok := a.resolveMCPToolName(fqToolName); ok {
		originalToolName = realToolName
		if serverID == "" {
			serverID = realServerID
		}
		log.Printf("[MCP CallMCP_tool] Resolved serverID=%s, toolName=%s", serverID, originalToolName)
	} else {
		log.Printf("[MCP CallMCP_tool] Warning: tool name not found in mapping, falling back to extract")
		originalToolName = extractToolName(fqToolName)
		extractedServerID := extractServerID(fqToolName)
		if serverID == "" && extractedServerID != "" {
			serverID = extractedServerID
		}
	}

	// Parse arguments using the helper method
	parseResult := (&model.FunctionCall{Name: fqToolName, Arguments: arguments}).ParseArguments()
	args := parseResult.Args

	// Log with truncated rawArgs for security
	log.Printf("[MCP CallMCP_tool] Parsed arguments: %v, rawArgs: %s", args, truncateLog(parseResult.RawArgs, 200))

	// Check if arguments parsing failed and we have raw args
	// Try to use raw args as fallback - pass to MCP server for validation
	if parseResult.RawArgs != "" && len(args) == 0 {
		log.Printf("[MCP CallMCP_tool] Arguments parsing failed, trying raw args: %s", truncateLog(parseResult.RawArgs, 200))
		// Try to parse as JSON map directly from raw string
		if err := json.Unmarshal([]byte(parseResult.RawArgs), &args); err != nil {
			// If still fails, pass raw string as-is to MCP server
			log.Printf("[MCP CallMCP_tool] Raw args also failed to parse, passing as-is to server")
			args = map[string]any{"_raw": parseResult.RawArgs}
		}
	}

	// 添加 MCP 工具调用超时 (60秒)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := a.mcpClient.CallTool(ctx, serverID, originalToolName, args)
	if err != nil {
		log.Printf("[MCP CallMCP_tool] CallTool error: %v", err)
		return &model.ToolCallResult{
			ToolName:   originalToolName,
			Error:      err.Error(),
			DurationMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 如果 result 本身包含错误信息，也传递出去
	if result.Error != "" {
		log.Printf("[MCP CallMCP_tool] Tool returned error: %s", result.Error)
		return &model.ToolCallResult{
			ToolName:   originalToolName,
			Error:      result.Error,
			Result:     result.Result,
			DurationMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	log.Printf("[MCP CallMCP_tool] Success, result length: %d", len(result.Result))
	return &model.ToolCallResult{
		ToolName:   originalToolName,
		Result:     result.Result,
		DurationMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// --- SelectionResponder interface implementation ---

// RegisterSelectionChannel stores the response channel for a pending selection.
func (a *App) RegisterSelectionChannel(requestID string, ch chan model.SelectionResponse) {
	a.selectionChannels.Store(requestID, ch)
}

// DeleteSelectionChannel removes and returns the channel for the given request.
func (a *App) DeleteSelectionChannel(requestID string) (chan model.SelectionResponse, bool) {
	val, ok := a.selectionChannels.LoadAndDelete(requestID)
	if !ok {
		return nil, false
	}
	return val.(chan model.SelectionResponse), true
}

// EmitSelectionRequest emits a Wails event to the frontend to show the selection UI.
func (a *App) EmitSelectionRequest(requestID, prompt, selectionType string, options []map[string]string, defaultValue any, sessionID int64) {
	wailsRuntime.EventsEmit(a.ctx, "selection_request", map[string]any{
		"request_id":    requestID,
		"prompt":        prompt,
		"type":          selectionType,
		"options":       options,
		"default_value": defaultValue,
		"session_id":    sessionID,
	})
}

// --- Selection RPC Methods ---

// RespondToSelection is called by the frontend when the user confirms a selection.
func (a *App) RespondToSelection(requestID string, selectedValues []string) error {
	ch, ok := a.selectionChannels.LoadAndDelete(requestID)
	if !ok {
		return fmt.Errorf("selection request not found or already responded: %s", requestID)
	}
	ch.(chan model.SelectionResponse) <- model.SelectionResponse{
		Selected:  selectedValues,
		Cancelled: false,
	}
	return nil
}

// CancelSelection is called by the frontend when the user cancels a selection.
func (a *App) CancelSelection(requestID string) error {
	ch, ok := a.selectionChannels.LoadAndDelete(requestID)
	if !ok {
		return fmt.Errorf("selection request not found or already responded: %s", requestID)
	}
	ch.(chan model.SelectionResponse) <- model.SelectionResponse{
		Selected:  []string{},
		Cancelled: true,
	}
	return nil
}
