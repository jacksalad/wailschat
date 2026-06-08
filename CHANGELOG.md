# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v1.6.0] - 2026-06-07

### Changed
- Lazy-load Mermaid (~1MB), KaTeX (~300KB), and 35 highlight.js language definitions — initial JS payload reduced from ~2MB+ to ~300KB
- Lazy-load SettingsModal, SearchBar, and SelectionPanel components via `defineAsyncComponent`
- Defer prompt store initialization from app startup to first ChatWindow mount
- KaTeX CSS injected on demand instead of loaded globally
- Add Vite `manualChunks` configuration for optimal code splitting

## [Unreleased]

### Added
- GitHub Actions CI/CD for automatic multi-platform builds

## [v1.5.0] - 2026-05-18

### Added
- Image thumbnail lightbox preview — click any image in chat to view full-size in an overlay
- Built-in `provide_selection` tool allowing AI models to present interactive single-choice (radio) and multi-choice (checkbox) selections to the user
- `SelectionPanel` component with confirm/cancel actions and default value support
- Window maximum size constraint (7680x4320) for Linux desktop environment maximize compatibility

### Changed
- Streaming chunk delivery now uses batched emission (~50ms intervals) to reduce Go↔JS IPC overhead
- Frontend chunk rendering uses `requestAnimationFrame` aggregation instead of per-chunk DOM updates
- Sidebar resize drag uses `requestAnimationFrame` for smoother interaction
- Message store uses `triggerRef` instead of array spread for reactivity, reducing unnecessary copies
- Stats parsing is done once at load time (`parsedStats` field) instead of on every render
- MCP server name mapping is cached and refreshed on connect/disconnect instead of queried per message
- Provider, history, and system prompt are loaded in parallel during `SendMessage`

### Fixed
- Streaming content race condition where `message_done` could fire before all chunks were flushed to the frontend
- Sidebar resize performance jitter on rapid mouse movement

## [v1.4.0] - 2026-05-11

### Added
- Linux distribution compilation support with platform-specific notification module
- MCP tool manual toggle persistence, auto-saving enabled state after connect/disconnect

### Changed
- UI styling refinement and unification, improved chat input spacing and code block copy button styles

### Fixed
- Session list time not refreshing when conversation is updated
- MCP tool connection and routing mapping issue

## [v1.3.0] - 2026-05-05

### Added
- Date and time display in session list
- Windows desktop notification when chat response generation completes
- Right-click context menu with "Quote", "Copy" and other actions
- Ctrl+F search functionality in chat
- Edit and regenerate for sent user messages
- Extended highlight.js language support for Markdown code blocks

### Fixed
- Prompt save anomaly issue
- Built-in tool function naming convention normalization

## [v1.2.0] - 2026-04-26

### Added
- Multiple prompt management support
- Explicit reasoning message handling for models such as DeepSeek R1

### Fixed
- Automatic correction and restoration for abnormal window positions

## [v1.1.0] - 2026-04-18

### Added
- F11 fullscreen toggle support
- Scroll-to-top button for chat sessions

### Changed
- Software interaction and UX improvements
- Mermaid diagram rendering performance optimization
- Theme switching management settings with local persistence

## [v1.0.0] - Initial Release

### Added
- Multiple LLM Providers (OpenAI, Claude, DeepSeek, etc.)
- Streaming Responses with SSE
- Multimodal Support with Image Input
- Multiple Sessions with drag-and-drop reordering
- Local SQLite Storage
- Dark/Light Theme
- Built-in Tools (file_read, file_write, shell_exec)
- MCP Tool Calling Support
- LaTeX Rendering with KaTeX
- Mermaid Diagram Support
- Performance Statistics
- Keyboard Shortcuts
- Custom CSS Support
- Resizable Sidebar
- Window State Persistence
