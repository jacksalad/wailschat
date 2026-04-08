package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const schemaV1 = `
CREATE TABLE IF NOT EXISTS providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_key TEXT NOT NULL,
    base_url TEXT NOT NULL,
    models TEXT NOT NULL DEFAULT '[]',
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('user','assistant','system')),
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);
INSERT INTO schema_version VALUES (1);
`

const schemaV2 = `
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('system_prompt', 'You are a helpful assistant.'),
    ('font_family', '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'),
    ('font_size', '14'),
    ('chat_width', '768');
UPDATE schema_version SET version = 2;
`

const schemaV3 = `
INSERT OR IGNORE INTO settings (key, value) VALUES ('theme', 'dark');
UPDATE schema_version SET version = 3;
`

const schemaV4 = `
ALTER TABLE messages ADD COLUMN images TEXT DEFAULT '[]';
UPDATE schema_version SET version = 4;
`

const schemaV5 = `
ALTER TABLE messages ADD COLUMN stats TEXT DEFAULT '';
UPDATE schema_version SET version = 5;
`

// defaultStyles is the built-in CSS used as the default for the custom_styles setting.
const defaultStyles = `/* ── WailsChat Default Styles ── */
/* Customizable via Settings → Styles */

/* ── Settings Modal Protection ── */
/* These rules protect the Settings modal from being affected by custom CSS */
.settings-modal-overlay {
  position: fixed !important;
  top: 0 !important;
  left: 0 !important;
  right: 0 !important;
  bottom: 0 !important;
  width: 100vw !important;
  height: 100vh !important;
  z-index: 9999 !important;
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  margin: 0 !important;
  padding: 0 !important;
  transform: none !important;
  isolation: isolate;
}
.settings-modal-content {
  position: relative !important;
  z-index: 10000 !important;
  transform: none !important;
}

/* ── App Layout ── */
.app-container { }

/* ── Light mode (default) ── */
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
  overflow: hidden;
  background-color: #f8fafc;
  color: #1e293b;
  font-family: var(--chat-font-family, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif);
  font-size: var(--chat-font-size, 14px);
}

#app {
  height: 100vh;
}

/* ── Dark mode ── */
html.dark, html.dark body {
  background-color: #0f172a;
  color: #e2e8f0;
}

/* ── App Layout ── */
.app-container { }
.sidebar { }
.chat-window { }
.resize-handle { }

/* ── Sidebar Styles ── */
.sidebar-container { }
.sidebar-header { }
.new-chat-btn {
  background-color: var(--new-chat-btn-bg, #2563eb);
  transition: background-color var(--transition-speed) ease;
}
.new-chat-btn:hover {
  background-color: var(--new-chat-btn-hover-bg, #1d4ed8);
}
.session-list { }
.session-item { }
.session-item:hover {
  background-color: var(--session-item-hover-bg-light, rgba(37, 99, 235, 0.1));
}
html.dark .session-item:hover {
  background-color: var(--session-item-hover-bg-dark, rgba(100, 116, 139, 0.2));
}
.session-item.active {
  background-color: rgba(37, 99, 235, 0.15);
}
.settings-btn { }

/* ── Chat Header ── */
.chat-header {
  background-color: var(--header-bg-light, rgba(255, 255, 255, 0.5));
}
html.dark .chat-header {
  background-color: var(--header-bg-dark, rgba(30, 41, 59, 0.5));
}
.chat-title { }
.model-picker { }
.model-picker-btn { }
.model-dropdown { }
.provider-group-label { }
.model-option { }

/* ── Empty State ── */
.empty-state { }
.app-logo { }

/* ── Message Bubbles ── */
.message-wrapper { }
.message-bubble { }
.user-bubble {
  background-color: var(--user-bubble-bg, rgba(37, 99, 235, 0.5));
  color: var(--user-bubble-text, #ffffff);
}
html.dark .user-bubble {
  background-color: var(--user-bubble-bg, rgba(37, 99, 235, 0.5));
}
.ai-bubble {
  background-color: var(--ai-bubble-bg-light, rgba(241, 245, 249, 0.5));
  color: var(--ai-bubble-text, inherit);
}
html.dark .ai-bubble {
  background-color: var(--ai-bubble-bg-dark, rgba(51, 65, 85, 0.5));
}
.message-content { }
.message-images { }
.message-actions { }
.copy-btn { }
.copy-btn:hover { }
.retry-btn { }
.retry-btn:hover { }
.stats-btn { }
.stats-btn:hover { }

/* Stats Popup */
.stats-popup {
  background-color: var(--header-bg-light, rgba(255, 255, 255, 0.5));
  border-color: var(--border-color-light, #e2e8f0);
  box-shadow: var(--shadow-lg);
}
html.dark .stats-popup {
  background-color: var(--header-bg-dark, rgba(30, 41, 59, 0.5));
  border-color: var(--border-color-dark, #334155);
}

/* ── MCP Tool Calls ── */
.tool-calls-panel { }
.tool-calls-toggle { }
.tool-calls-list { }
.tool-call-item { }
.tool-call-name { }
.tool-call-status { }
.tool-call-args { }
.tool-call-result { }

/* ── Chat Input ── */
.chat-input-area {
  border-color: var(--border-color-light, #e2e8f0);
  background-color: var(--header-bg-light, rgba(255, 255, 255, 0.5));
}
html.dark .chat-input-area {
  border-color: var(--border-color-dark, #334155);
  background-color: var(--header-bg-dark, rgba(30, 41, 59, 0.5));
}
.input-container { }
.input-textarea {
  background-color: var(--input-bg-light, #ffffff);
  border-color: var(--input-border-light, #cbd5e1);
  color: var(--text-primary-light, #1e293b);
}
.input-textarea:focus {
  border-color: var(--input-focus-border, #3b82f6);
}
.input-textarea::placeholder {
  color: var(--text-secondary-light, #64748b);
}
html.dark .input-textarea {
  background-color: var(--input-bg-dark, #334155);
  border-color: var(--input-border-dark, #475569);
  color: var(--text-primary-dark, #e2e8f0);
}
html.dark .input-textarea::placeholder {
  color: var(--text-secondary-dark, #94a3b8);
}
.image-preview-container { }
.image-preview-item { }
.image-remove-btn { }
.image-upload-btn { }
.send-btn {
  background-color: var(--send-btn-bg, #2563eb);
  transition: background-color var(--transition-speed) ease;
}
.send-btn:hover {
  background-color: var(--send-btn-hover-bg, #1d4ed8);
}
.stop-btn {
  background-color: var(--stop-btn-bg, #dc2626);
  transition: background-color var(--transition-speed) ease;
}
.stop-btn:hover {
  background-color: var(--stop-btn-hover-bg, #b91c1c);
}

/* ── Loading & Streaming ── */
.loading-indicator { }
.thinking-bubble {
  background-color: var(--ai-bubble-bg-light, rgba(241, 245, 249, 0.5));
}
html.dark .thinking-bubble {
  background-color: var(--ai-bubble-bg-dark, rgba(51, 65, 85, 0.5));
}
.mcp-tools-loading { }
.mcp-loading-badge { }

/* ── Markdown styles ── */
.markdown-body {
  line-height: 1.6;
  word-wrap: break-word;
}
.markdown-body p {
  margin: 0.5em 0;
}

/* Code block wrapper */
.code-block {
  margin: 0.5em 0;
  position: relative;
  border-radius: var(--radius-md, 0.5rem);
  border: 1px solid var(--border-color-light, #e2e8f0);
}
html.dark .code-block {
  border-color: var(--border-color-dark, #1e293b);
}
.code-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.4rem 0.75rem;
  background-color: var(--border-color-light, #e2e8f0);
  font-size: 0.75rem;
  line-height: 1;
  position: sticky;
  top: 0;
  z-index: 10;
  border-radius: var(--radius-md, 0.5rem) var(--radius-md, 0.5rem) 0 0;
  transition: box-shadow var(--transition-speed) ease;
}
html.dark .code-header {
  background-color: #0f172a;
}
.code-header.is-stuck {
  box-shadow: var(--shadow-md);
}
.code-lang {
  text-transform: uppercase;
  font-weight: 600;
  color: var(--text-secondary-light, #64748b);
  letter-spacing: 0.05em;
}
html.dark .code-lang {
  color: var(--text-secondary-dark, #94a3b8);
}
.code-actions {
  display: flex;
  gap: 0.375rem;
}
.code-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.15rem 0.5rem;
  border-radius: var(--radius-sm, 0.25rem);
  letter-spacing: 0.05em;
  transition: background-color var(--transition-speed) ease;
}
.copy-btn {
  color: var(--text-secondary-light, #64748b);
}
.copy-btn:hover {
  background-color: var(--border-color-light, #cbd5e1);
  color: var(--text-primary-light, #1e293b);
}
html.dark .copy-btn {
  color: var(--text-secondary-dark, #94a3b8);
}
html.dark .copy-btn:hover {
  background-color: var(--border-color-dark, #1e293b);
  color: var(--text-primary-dark, #e2e8f0);
}
.run-btn {
  color: #16a34a;
}
.run-btn:hover {
  background-color: #dcfce7;
  color: #15803d;
}
html.dark .run-btn {
  color: #4ade80;
}
html.dark .run-btn:hover {
  background-color: #052e16;
  color: #4ade80;
}

/* Code content */
.markdown-body .code-block pre,
.markdown-body .code-block pre.hljs {
  background-color: #f1f5f9;
  border-radius: 0 0 var(--radius-md, 0.5rem) var(--radius-md, 0.5rem);
  padding: 1rem;
  overflow-x: auto;
  margin: 0 !important;
  border: none !important;
}
html.dark .markdown-body .code-block pre,
html.dark .markdown-body .code-block pre.hljs {
  background-color: #1e293b;
  margin: 0 !important;
  border: none !important;
}
.markdown-body .code-block pre code {
  line-height: 1.5;
}
.markdown-body code {
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 0.875rem;
}
.markdown-body :not(pre) > code {
  background-color: var(--border-color-light, #e2e8f0);
  padding: 0.125rem 0.375rem;
  border-radius: var(--radius-sm, 0.25rem);
}
html.dark .markdown-body :not(pre) > code {
  background-color: var(--border-color-dark, #334155);
}
.markdown-body ul, .markdown-body ol {
  padding-left: 1.5rem;
  margin: 0.5em 0;
}
.markdown-body blockquote {
  border-left: 3px solid var(--border-color-light, #cbd5e1);
  padding-left: 1rem;
  margin: 0.5em 0;
  color: var(--text-secondary-light, #64748b);
}
html.dark .markdown-body blockquote {
  border-left-color: var(--border-color-dark, #475569);
  color: var(--text-secondary-dark, #94a3b8);
}
.markdown-body h1, .markdown-body h2, .markdown-body h3 {
  margin-top: 1rem;
  margin-bottom: 0.5rem;
  font-weight: 600;
}
.markdown-body table {
  border-collapse: collapse;
  margin: 0.5em 0;
}
.markdown-body th, .markdown-body td {
  border: 1px solid var(--border-color-light, #cbd5e1);
  padding: 0.5rem 0.75rem;
}
html.dark .markdown-body th,
html.dark .markdown-body td {
  border-color: var(--border-color-dark, #475569);
}
.markdown-body th {
  background-color: #f1f5f9;
}
html.dark .markdown-body th {
  background-color: #1e293b;
}

/* ── Typing cursor ── */
@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
.typing-cursor::after {
  content: '';
  display: inline-block;
  width: 2px;
  height: 1em;
  margin-left: 2px;
  vertical-align: text-bottom;
  background: #60a5fa;
  animation: blink 1s infinite;
}

/* ── Scrollbar ── */
::-webkit-scrollbar {
  width: 6px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--border-color-light, #cbd5e1);
  border-radius: 3px;
}
::-webkit-scrollbar-thumb:hover {
  background: var(--text-secondary-light, #94a3b8);
}
html.dark ::-webkit-scrollbar-thumb {
  background: var(--border-color-dark, #475569);
}
html.dark ::-webkit-scrollbar-thumb:hover {
  background: var(--text-secondary-dark, #64748b);
}

/* ── Code highlight: dark overrides ── */
html.dark .hljs {
  background: #1e293b;
  color: #e2e8f0;
  margin: 0 !important;
  border: none !important;
}`

const schemaV6 = `
INSERT OR IGNORE INTO settings (key, value) VALUES ('custom_styles', '');
UPDATE schema_version SET version = 6;
`

const schemaV7 = `
INSERT OR IGNORE INTO settings (key, value) VALUES ('shortcuts', '{"new_chat":"ctrl+n","clear_context":"ctrl+l","focus_input":"/"}');
UPDATE schema_version SET version = 7;
`

const schemaV8 = `
INSERT OR IGNORE INTO settings (key, value) VALUES ('bg_image', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('bg_opacity', '0.15');
INSERT OR IGNORE INTO settings (key, value) VALUES ('sidebar_width', '288');
UPDATE schema_version SET version = 8;
`

const schemaV9 = `
CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id, id);
UPDATE schema_version SET version = 9;
`

const schemaV10 = `
CREATE TABLE IF NOT EXISTS mcp_servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    command TEXT,
    url TEXT,
    transport TEXT DEFAULT 'stdio',
    env TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    auth_token TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
UPDATE schema_version SET version = 10;
`

const schemaV11 = `
-- Migration: Add transport column to existing mcp_servers table
ALTER TABLE mcp_servers ADD COLUMN transport TEXT DEFAULT 'stdio';
UPDATE schema_version SET version = 11;
`

const schemaV12 = `
-- Migration: Add missing columns to mcp_servers table
ALTER TABLE mcp_servers ADD COLUMN auth_token TEXT DEFAULT '';
ALTER TABLE mcp_servers ADD COLUMN env TEXT DEFAULT '{}';
ALTER TABLE mcp_servers ADD COLUMN enabled INTEGER DEFAULT 1;
UPDATE schema_version SET version = 12;
`

const schemaV13 = `
-- Migration: Add tool_calls and tool_results columns for MCP persistence
ALTER TABLE messages ADD COLUMN tool_calls TEXT DEFAULT '[]';
ALTER TABLE messages ADD COLUMN tool_results TEXT DEFAULT '[]';
UPDATE schema_version SET version = 13;
`

const schemaV14 = `
-- Migration: Bump schema version for typing cursor fix (applied in Go code)
UPDATE schema_version SET version = 14;
`

const schemaV15 = `
-- Migration: Add sort_order column to sessions for custom ordering
ALTER TABLE sessions ADD COLUMN sort_order INTEGER DEFAULT 0;
UPDATE schema_version SET version = 15;
`

const schemaV16 = `
-- Migration: Add tool_enabled setting for built-in tools
INSERT OR IGNORE INTO settings (key, value) VALUES ('tool_enabled', '0');
UPDATE schema_version SET version = 16;
`

const schemaV17 = `
-- Migration: Add individual tool settings for file_read, file_write, shell_exec
INSERT OR IGNORE INTO settings (key, value) VALUES ('tool_file_read', '0');
INSERT OR IGNORE INTO settings (key, value) VALUES ('tool_file_write', '0');
INSERT OR IGNORE INTO settings (key, value) VALUES ('tool_shell_exec', '0');
UPDATE schema_version SET version = 17;
`

const schemaV18 = `
-- Migration: Purge orphaned messages whose parent session no longer exists
DELETE FROM messages WHERE session_id NOT IN (SELECT id FROM sessions);
UPDATE schema_version SET version = 18;
`
const schemaV18Vacuum = `VACUUM`

// Init opens the SQLite database and runs migrations.
func Init() (*sql.DB, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("db: get config dir: %w", err)
	}
	dbDir := filepath.Join(configDir, "wailschat")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("db: create dir: %w", err)
	}
	dbPath := filepath.Join(dbDir, "wailschat_data.db")

	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1&_synchronous=NORMAL&_cache_size=-65536", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	// Single connection is optimal for SQLite — serializes writes, avoids lock contention
	db.SetMaxOpenConns(1)

	// Explicitly enable foreign keys — DSN _foreign_keys=1 may not work with modernc.org/sqlite
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: enable foreign keys: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: migrate: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	var version int
	row := db.QueryRow("SELECT version FROM schema_version LIMIT 1")
	if row.Scan(&version) != nil {
		// schema_version table doesn't exist yet, run initial migration
		if _, err := db.Exec(schemaV1); err != nil {
			return fmt.Errorf("db: schema v1: %w", err)
		}
	}
	// Future: add version-specific migrations here
	if version < 2 {
		if _, err := db.Exec(schemaV2); err != nil {
			return fmt.Errorf("db: schema v2: %w", err)
		}
	}
	if version < 3 {
		if _, err := db.Exec(schemaV3); err != nil {
			return fmt.Errorf("db: schema v3: %w", err)
		}
	}
	if version < 4 {
		if _, err := db.Exec(schemaV4); err != nil {
			return fmt.Errorf("db: schema v4: %w", err)
		}
	}
	if version < 5 {
		if _, err := db.Exec(schemaV5); err != nil {
			return fmt.Errorf("db: schema v5: %w", err)
		}
	}
	if version < 6 {
		if _, err := db.Exec(schemaV6); err != nil {
			return fmt.Errorf("db: schema v6: %w", err)
		}
	}
	if version < 7 {
		if _, err := db.Exec(schemaV7); err != nil {
			return fmt.Errorf("db: schema v7: %w", err)
		}
	}
	if version < 8 {
		if _, err := db.Exec(schemaV8); err != nil {
			return fmt.Errorf("db: schema v8: %w", err)
		}
	}
	if version < 9 {
		if _, err := db.Exec(schemaV9); err != nil {
			return fmt.Errorf("db: schema v9: %w", err)
		}
	}
	if version < 10 {
		if _, err := db.Exec(schemaV10); err != nil {
			return fmt.Errorf("db: schema v10: %w", err)
		}
	}
	if version < 11 {
		// Migration v11: Add transport column to existing mcp_servers table
		db.Exec(schemaV11)
	}
	if version < 12 {
		// Migration v12: Add auth_token column to existing mcp_servers table
		db.Exec(schemaV12)
	}
	if version < 13 {
		// Migration v13: Add tool_calls and tool_results columns for MCP persistence
		if _, err := db.Exec(schemaV13); err != nil {
			return fmt.Errorf("db: schema v13: %w", err)
		}
	}
	if version < 14 {
		// Migration v14: Fix typing cursor in custom_styles
		if _, err := db.Exec(schemaV14); err != nil {
			return fmt.Errorf("db: schema v14: %w", err)
		}
		fixTypingCursorCSS(db)
	}
	if version < 15 {
		// Migration v15: Add sort_order column to sessions for custom ordering
		db.Exec(schemaV15)
	}
	if version < 16 {
		// Migration v16: Add tool_enabled setting for built-in tools
		db.Exec(schemaV16)
	}
	if version < 17 {
		// Migration v17: Add individual tool settings
		db.Exec(schemaV17)
	}
	if version < 18 {
		// Migration v18: Purge orphaned messages left by prior foreign key enforcement gap
		db.Exec(schemaV18)
		db.Exec(schemaV18Vacuum)
	}
	return nil
}

// fixTypingCursorCSS replaces the old character-based typing cursor in saved custom_styles
// with a pure CSS bar, so existing databases get the fix automatically.
func fixTypingCursorCSS(db *sql.DB) {
	var val string
	err := db.QueryRow("SELECT value FROM settings WHERE key = 'custom_styles'").Scan(&val)
	if err != nil {
		return // no custom_styles saved, nothing to fix
	}
	if !strings.Contains(val, "25CB") && !strings.Contains(val, "25BC") {
		return // already fixed or no cursor rule
	}

	old := ".typing-cursor::after {\n  content: '\\25CB';\n  animation: blink 1s infinite;\n  color: #60a5fa;\n}"
	newCSS := ".typing-cursor::after {\n  content: '';\n  display: inline-block;\n  width: 2px;\n  height: 1em;\n  margin-left: 2px;\n  vertical-align: text-bottom;\n  background: #60a5fa;\n  animation: blink 1s infinite;\n}"
	fixed := strings.Replace(val, old, newCSS, 1)
	if fixed != val {
		db.Exec("UPDATE settings SET value = ? WHERE key = 'custom_styles'", fixed)
	}
}

// DefaultStyles returns the built-in default CSS stylesheet content.
func DefaultStyles() string {
	return defaultStyles
}
