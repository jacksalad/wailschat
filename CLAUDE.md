# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

WailsChat is a desktop AI chat application built with **Wails v2** (Go backend + Vue 3 frontend). It connects to OpenAI-compatible LLM APIs with SSE streaming, stores data locally in SQLite, supports multiple providers/models, and includes **multimodal image input support** for vision-capable models like GPT-4 Vision. Features **performance statistics tracking**, **custom CSS styling**, **message retry/cancel functionality**, **configurable keyboard shortcuts**, **background image customization**, **resizable sidebar**, and **session drag-and-drop reordering**.

**Key Features**:
- **Built-in Tools (Read/Write/Execute/Selection)**: AI models can read local files, write files, execute shell commands, and present interactive selections directly
- **MCP (Model Context Protocol) Tool Calling**: AI models can use external tools via standardized MCP protocol
- **LaTeX Mathematical Rendering**: High-quality math formula rendering using KaTeX
- **Mermaid Diagram Support**: Render Mermaid diagrams in code blocks
- **Code Block Actions**: Copy, download (HTML/SVG), preview (HTML/SVG), and Mermaid toggle
- **External Link Routing**: http(s) links open in the system default browser via `BrowserOpenURL`
- **Enhanced Markdown Processing**: Improved bold rendering for Unicode punctuation

## Commands

```bash
# Development (starts Go backend + Vite dev server with hot reload)
wails dev

# Production build (outputs to build/bin/wailschat)
wails build

# Frontend only (no Go backend)
cd frontend && npm run dev

# Frontend type check + build
cd frontend && npm run build

# Go dependency management
go mod tidy
```

Dev server runs at http://localhost:34115 with Wails Inspector for debugging Go bindings.

## Architecture

### Backend → Frontend Communication

- **RPC calls**: Frontend calls Go methods via auto-generated Wails bindings in `frontend/wailsjs/go/main/`. These are regenerated on `wails dev`/`wails build` — **never edit them manually**.
- **Events (streaming)**: Backend emits events via `runtime.EventsEmit()`:
  - `message_chunk` — SSE content delta for real-time rendering
  - `message_done` — Streaming completed successfully
  - `message_error` — Error during streaming
  - `message_stats` — Performance statistics (tokens, timing, speed)
  - `session_renamed` — Auto-generated title update
  - `mcp_tool_call_start` — MCP tool call started
  - `mcp_tool_result` — MCP tool call result
  - `mcp_server_status` — MCP server connection status change
- Frontend listens via `EventsOn()` in Pinia stores.

### Backend Structure

- `main.go` — Entry point. Initializes Wails window (1200x900, min 800x600), embeds frontend assets and logo icon, binds `App` struct.
- `app.go` — `App` struct with all exported methods that become Wails RPC endpoints. Includes:
  - Provider/Session/Settings CRUD operations
  - `SendMessage()` — Saves user message, starts streaming goroutine with MCP tool calling support
  - `RetryMessage()` — Deletes message and all after it, re-streams
  - `RetryFromUserMessage()` — Deletes from a user message onwards, re-saves it, and re-streams
  - `CancelMessage()` — Cancels in-flight streaming via context
  - `ClearSessionMessages()` — Deletes all messages in a session (clears context)
  - `ReorderSessions()` — Persists new display order after drag-and-drop
  - `GetSystemFonts()` — Returns system font list (cross-platform)
  - `GetDefaultStyles()` — Returns built-in CSS stylesheet
  - `GetModels()` — Fetches available models from API endpoint
  - `OpenFileDialog()` — Native file picker for background image selection
  - `ReadImageAsBase64()` — Reads local image file as data URI
  - **MCP Server Management**: `MCPServerList()`, `MCPServerCreate()`, `MCPServerUpdate()`, `MCPServerDelete()`, `MCPServerTest()`, `MCPServerConnect()`, `MCPServerDisconnect()`, `MCPServerGetStatus()`, `MCPServerGetAllStatuses()`, `MCPServerGetTools()`
  - **MCP Tool Calling**: `GetEnabledMCPTools()`, `CallMCP_tool()`
- `window_state.go` — Window position/size/maximized state persistence across sessions.
- `internal/db/` — SQLite initialization, schema creation, migrations (V1→V23). DB stored at platform config directory (`%LOCALAPPDATA%/wailschat/wailschat_data.db` on Windows). Contains `DefaultStyles()` with built-in CSS.
- `internal/llm/` — HTTP client for OpenAI-compatible APIs:
  - `client.go` — `StreamChat()` (SSE with performance stats and tool calling support), `Chat()` (non-streaming for title generation), `TestConnection()`, `GetModels()`
  - `sse_parser.go` — Parses `data: {...}` lines, extracts content, usage stats, and tool calls
- `internal/model/` — Data models:
  - `Provider`, `Session`, `Message` (with `images`, `stats`, `tool_calls`, `tool_results` fields)
  - `ChatMessage` (multimodal content support), `ChatCompletionRequest`
  - `PerformanceStats` — Input/output tokens, first token time, total time, speed (tokens/sec)
  - `UsageInfo`, `StreamOptions`, `ContentText`, `ContentImage`
  - **MCP Models**: `MCPServer`, `MCPTool`, `MCPServerTestResult`, `ToolCall`, `ToolCallResult`, `FunctionCall`, `FunctionDef`, `Tool`
- `internal/mcp/` — MCP protocol implementation:
  - `client.go` — MCP client manager with stdio transport support, connection pooling, tool caching, and tool execution
  - `types.go` — MCP protocol type definitions (JSON-RPC 2.0), including `TransportStdio` and `TransportHTTP` constants
- `internal/provider/`, `internal/session/`, `internal/message/`, `internal/prompt/` — CRUD services for each entity. `message/service.go` includes `DeleteFromID()` for retry and `DeleteBySession()` for clearing context. `session/service.go` includes `TouchSession()` and `ReorderSessions()` for ordering. `prompt/service.go` manages multiple system prompts.
- `internal/tools/` — Built-in tool implementations (AI can read/write/execute/present-selections):
  - `tools.go` — Tool interface, Manager with security constraints (allowed directories, command blacklist), `RegisterBuiltInTools()`, `GetBuiltInToolNames()`
  - `file_read.go` — Read local files or list directory contents (max 1MB for files, non-recursive directory listing with type/size/modified time, path validation, directory traversal prevention)
  - `file_write.go` — Write files (dangerous extension blocking, parent dir creation, secure permissions 0644)
  - `shell_exec.go` — Execute shell commands (Windows hide window, Unix bash/sh, configurable timeout 5-30min, blacklist for dangerous commands like rm, shutdown, dd, etc.)
  - `provide_selection.go` — Presents interactive single/multi-choice selections to the user via the `SelectionPanel` UI
- `internal/settings/` — Key-value settings service. `GetAll()` returns all settings as `map[string]string`, `Set()` does upsert.
- `internal/fonts/` — Cross-platform system font enumeration (Windows/Darwin/Linux).
- `internal/notify/` — Desktop notifications. `notify_windows.go` shows WinRT toast notifications via a hidden PowerShell; `notify_other.go` is a no-op stub for non-Windows.

### Frontend Structure

- `frontend/src/App.vue` — Two-column layout: Sidebar + ChatWindow. Initializes stores on mount, applies settings to DOM. Registers global listeners: `keydown` for configurable keyboard shortcuts, `mousemove`/`mouseup` for sidebar resize drag.
- `frontend/src/stores/` — Pinia stores:
  - `provider.ts` — Provider CRUD + default provider tracking + `testConnection()` + `getModels()`
  - `session.ts` — Session CRUD + `moveToTop()`, `reorderSessions()` + listens for `session_renamed` event to update auto-generated titles
  - `message.ts` — SSE event listeners (`message_chunk`/`message_done`/`message_error`/`message_stats`), streaming state, optimistic user message rendering, image array handling, `retryMessage()`, `retryFromUserMessage()`, `editAndResendMessage()`, `cancelStream()`, `clearHistory()`, `getStats()`/`parseStats()` for performance stats. MCP tool call event listeners (`mcp_tool_call_start`, `mcp_tool_result`) and tool state management, plus `selection_request` handling for the `provide_selection` tool.
  - `prompt.ts` — Multi-prompt CRUD (`PromptList`/`Create`/`Update`/`Delete`/`SetDefault`).
  - `search.ts` — In-chat search state (`isOpen`, `query`, matches, navigation) for the Ctrl+F search bar.
  - `settings.ts` — Reactive settings map, `applyToDOM()` writes CSS vars and `.dark` class, `applyCustomStyles()` injects user CSS with highest priority, loads selected theme CSS via `GetThemeCSS()`. Includes `shortcuts` computed property (parses JSON from DB) with `ShortcutBindings` interface. Handles `bg_image` with `local://` prefix for local file paths, uses `ReadImageAsBase64()` to load local images.
- `frontend/src/components/` — UI components:
  - `Sidebar.vue` — New chat button, settings button, resizable via drag handle
  - `SessionList.vue` — Chat session list with delete and drag-and-drop reordering
  - `ChatWindow.vue` — Message list with model picker dropdown, streaming indicator, auto-scroll, retry on last assistant message. MCP tool call loading state display.
  - `MessageBubble.vue` — Message display with image preview, copy button (both user and assistant messages), retry button, performance stats popup. MCP tool call display with collapsible panel showing tool arguments and results.
  - `MarkdownMessage.vue` — markdown-it + highlight.js rendering with code block header: copy button (all blocks), download + run buttons (HTML/SVG), and Mermaid toggle. LaTeX math rendering using KaTeX. External links are routed through the system default browser via Wails `BrowserOpenURL` (falls back to `window.open`).
  - `ChatInput.vue` — Textarea with auto-resize, image selection/paste support, send/stop buttons. Uses `chat-input-textarea` CSS class for global shortcut targeting.
  - `SettingsModal.vue` — **Six tabs**:
    - **General**: Theme (light/dark), custom font family dropdown (with search filter, each font rendered in its own typeface, max-height scrollable), font size, chat width, background image (URL or local file via `OpenFileDialog`), background opacity
    - **API Providers**: CRUD, test connection, dynamic model list with "Get Models" fetcher (includes filter input to search hundreds of models)
    - **Prompt**: System prompt editor
    - **Styles**: Custom CSS editor with reset to default
    - **Shortcuts**: Click-to-rebind keyboard shortcut UI
    - **MCP**: MCP server management with stdio/HTTP transport support, environment variables, auth token, connection testing, and real-time status display
- `frontend/src/utils/markdown.ts` — Enhanced: LaTeX protection and rendering using KaTeX, improved bold rendering for Unicode punctuation, Mermaid diagram support (via `mermaid` global), support for `$$...$$`, `$...$`, `\[...\]`, `\(...\)` LaTeX delimiters
- `frontend/src/assets/logo.png` — Application logo, also copied to `build/appicon.png` for the exe icon.
- `frontend/wailsjs/` — **Auto-generated** Wails bindings. Do not edit.

### Data Flow (Chat with Image and MCP Tool Support)

1. User clicks "New Chat" → immediately creates session with default provider/model, name "New Chat"
2. User types message and/or selects/pastes images → `ChatInput` → Pinia `messageStore.sendMessage(content, images)`
3. Calls Wails RPC → `App.SendMessage(sessionID, content, images)` in `app.go`
4. Saves user message to SQLite with images (as JSON array of base64 strings) → prepends system prompt from settings
5. Converts messages to multimodal format: text-only messages use string content, messages with images use array format with `ContentText` and `ContentImage` objects
6. Gathers enabled MCP tools from connected servers and converts to OpenAI function calling format
7. Calls `llm.Client.StreamChat()` with multimodal messages, `stream_options.include_usage: true`, and MCP tools list
8. SSE chunks parsed → `runtime.EventsEmit("message_chunk", sessionID, chunk)` per chunk
9. Frontend `EventsOn("message_chunk")` → appends to `streamingContent` → Markdown renders in real-time (with LaTeX and Mermaid support)
10. If LLM returns tool calls:
    - Backend emits `mcp_tool_call_start` event → frontend shows loading state
    - Backend executes each tool call via MCP protocol
    - Backend emits `mcp_tool_result` event with results
    - Tool results added to conversation context
    - LLM continues streaming with tool results (up to 10 iterations)
    - Frontend displays tool calls and results in collapsible panel
11. On stream completion → `EventsOn("message_stats")` with `PerformanceStats` → `EventsOn("message_done")`
12. On first exchange, background goroutine calls `llm.Client.Chat()` to generate a title → updates session → emits `session_renamed` event

### Retry Flow

1. User clicks retry button on last assistant message → `messageStore.retryMessage(sessionId, messageId)`
2. Calls Wails RPC → `App.RetryMessage(sessionID, messageID)`
3. Backend calls `messageSvc.DeleteFromID(sessionID, messageID)` — deletes that message and all after it
4. Reloads remaining history, rebuilds chat messages, starts fresh stream
5. Frontend removes messages from local state, invalidates cache, sets up new stream listeners

### Session Reordering

1. User drags session in sidebar → `sessionStore.reorderSessions(orderedIDs)` is called
2. Frontend optimistically updates local session list order
3. Backend `App.ReorderSessions(orderedIDs)` persists the new `sort_order` values
4. Sessions are ordered by `sort_order DESC, updated_at DESC` in queries

### Database Schema

- `providers` — API configs (name, api_key, base_url, models as JSON, is_default)
- `sessions` — Chats linked to providers via FK, with ON DELETE CASCADE. Includes `sort_order` column for custom ordering and `prompt_id` linking a session to a managed system prompt (V21).
- `messages` — Linked to sessions via FK with ON DELETE CASCADE. Role CHECK: user|assistant|system. Columns:
  - `images` — JSON array of base64 strings for multimodal support
  - `stats` — JSON-serialized PerformanceStats
  - `reasoning_content` — Text for reasoning/thinking tokens (DeepSeek R1-style models, V20)
  - `tool_calls` — JSON array of MCP tool calls made during this response (V13)
  - `tool_results` — JSON array of MCP tool results received (V13)
- `prompts` — Managed system prompts (V21): `id, name, content, category, is_default, sort_order`. Sessions reference one via `prompt_id`. The legacy `system_prompt` setting was migrated into a default prompt row.
- `settings` — Key-value table (`key TEXT PRIMARY KEY, value TEXT`). Keys:
  - `system_prompt`, `font_family`, `font_size`, `chat_width`, `theme`
  - `custom_styles` — User-defined CSS
  - `shortcuts` — JSON-serialized keyboard shortcut bindings. Format: `{"new_chat":"ctrl+n","clear_context":"ctrl+l","focus_input":"/","toggle_sidebar":"ctrl+b"}`
  - `bg_image` — Background image URL or `local://<path>` for local files
  - `bg_opacity` — Background opacity 0-1
  - `sidebar_width` — Sidebar width in pixels
  - `selected_theme` — Name of the loaded CSS theme file (V19)
  - `tool_enabled`, `tool_file_read`, `tool_file_write`, `tool_shell_exec` — Built-in tool toggles (V16/V17)
  - `notify_on_complete` — Whether to show a desktop notification when generation completes (V22)
  - `show_message_time` — Whether to show timestamps on messages (V23)
- `mcp_servers` — MCP server configurations:
  - `id TEXT PRIMARY KEY` — UUID
  - `name TEXT NOT NULL` — Display name
  - `command TEXT` — stdio mode command
  - `url TEXT` — HTTP mode URL
  - `transport TEXT DEFAULT 'stdio'` — 'stdio' or 'http'
  - `env TEXT DEFAULT '{}'` — JSON environment variables
  - `enabled INTEGER DEFAULT 1` — 0/1 boolean
  - `auth_token TEXT` — HTTP auth token
  - `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`
- `schema_version` — Tracks migration version (currently V23)

### Settings & Theming

- Settings stored in SQLite `settings` table, loaded on app startup
- `settingsStore.applyToDOM()` writes CSS custom properties (`--chat-font-family`, `--chat-font-size`, `--chat-width`) and toggles `.dark` class on `<html>`
- `settingsStore.applyCustomStyles()` injects user CSS via `<style id="custom-user-styles">` with highest priority
- Tailwind configured with `darkMode: 'class'` — all components use light-default + `dark:` prefix classes
- Built-in CSS in `internal/db/db.go` (`defaultStyles` constant) provides base light/dark styles
- Chat width is dynamic via `:style="{maxWidth: chatWidthPx}"` on message container and input area
- highlight.js uses `github.css` (light base), dark mode overrides in default styles
- Background image supports URLs and local files (stored as `local://<path>`, resolved via `ReadImageAsBase64()`)
- Sidebar is resizable via drag handle (200–500px), width persisted in settings

### Keyboard Shortcuts

- Shortcuts are stored in `settings` table as JSON (`key: "shortcuts"`), loaded via `settingsStore.shortcuts` computed property
- Global `keydown` listener in `App.vue` matches key combos against configured bindings using `parseBinding()` helper
- `ChatInput.vue` has `chat-input-textarea` CSS class so `App.vue` can find and focus it via `document.querySelector`
- Default bindings: `Ctrl+N` (new chat), `Ctrl+L` (clear context), `/` (focus input), `Ctrl+B` (toggle sidebar)
- Single-key shortcuts (like `/`) are ignored when user is typing in input/textarea/contenteditable elements
- Settings → Shortcuts tab provides click-to-rebind UI: click a binding → enter "recording" mode → next keypress captures new combo → Escape cancels

### MCP (Model Context Protocol)

- **Transport Modes**: Supports both `stdio` (local subprocess) and `http` (remote server)
- **Server Management**: Create, edit, delete, connect, disconnect MCP servers via Settings → MCP tab
- **Connection Status**: Real-time connection status display (disconnected/connecting/connected/error/failed) with exponential backoff retry (up to 5 attempts)
- **Tool Discovery**: Automatic tool discovery via `tools/list` on server connect
- **Tool Caching**: Tools are cached after discovery for performance
- **Tool Naming**: Tools are prefixed with server ID (`{serverID}___toolName`) for routing
- **Tool Call Loop**: Up to 10 iterations of tool calls per message for complex workflows
- **Environment Variables**: Support for custom environment variables in stdio mode
- **Auth Token**: Optional Bearer token/API key for HTTP transport servers
- **Tool Call Display**: Collapsible panels in MessageBubble showing tool name, arguments, results, and duration

### Built-in Tools (Read/Write/Execute)

AI models can directly use these tools to interact with the local filesystem and execute commands:

- **`file_read`**: Read local file contents or list directory entries. For files, returns text content with configurable max size (default 100KB, max 1MB). For directories, returns a non-recursive listing of all immediate children showing type (`[DIR]`/`[FILE]`), name, size, and modified time. Parameters: `path` (required, accepts file or directory), `max_size` (optional, file only). Path validation prevents directory traversal.

- **`file_write`**: Create or overwrite files with text content. Parameters: `path` (required), `content` (required), `create_dirs` (optional). Dangerous extensions (.exe, .bat, .sh, .ps1, etc.) are blocked for security. Parent directories can be auto-created with `create_dirs: true`.

- **`shell_exec`**: Execute shell commands with timeout (default 5min, max 30min). Parameters: `command` (required), `timeout` (optional, seconds). Windows commands run via `cmd /c` with hidden window. Dangerous commands (rm, rmdir, del, shutdown, reboot, dd, kill, chmod, chown, sudo, etc.) are blacklisted. Results include stdout/stderr, exit code, and execution duration.

**Security Features**:
- Path validation prevents access outside allowed directories
- Dangerous file extensions blocked for write operations
- Command blacklist prevents destructive operations
- Configurable timeout prevents runaway processes
- Result truncation (10KB max) prevents context overflow

**Settings**: Built-in tools are individually enabled/disabled via settings keys: `tool_enabled` (master switch), `tool_file_read`, `tool_file_write`, `tool_shell_exec`. Default: all disabled.

### Window State Persistence

- Window position, size, and maximized state are persisted to `%LOCALAPPDATA%/wailschat/window_state.json`
- State is saved on window close and restored on startup
- Works for non-maximized windows; maximized state is applied on startup

## Key Conventions

- Vue 3 `<script setup>` Composition API throughout
- Tailwind CSS with dual theme support — light colors as default, `dark:` prefix for dark mode variants
- All Go methods on `App` struct are exported (uppercase) to expose them to Wails runtime
- SQLite uses pure-Go driver `modernc.org/sqlite` (no CGO required)
- LLM API format: OpenAI-compatible `/v1/chat/completions` with `stream: true` and `stream_options.include_usage: true`
- Database migrations are sequential (V1–V23). Use `INSERT OR IGNORE` for new settings to be safe against re-runs
- Auto-generated title uses non-streaming `Chat()` call with a summarization prompt, runs in background goroutine after first AI response
- Performance stats include: input tokens, output tokens, first token time (TTFT), total time, generation speed (tokens/sec)
- Streaming cancellation uses Go context with `context.CancelFunc` stored in `sync.Map`
- Font names in settings are truncated (max 30 chars, cut at first space after 20 chars) for display cleanliness
