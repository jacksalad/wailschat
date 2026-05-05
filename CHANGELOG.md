# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- GitHub Actions CI/CD for automatic multi-platform builds

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
