package message

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"

	"wailschat/internal/model"
)

type Service struct {
	db    *sql.DB
	stmts struct {
		sync.Mutex
		create       *sql.Stmt
		getBySession *sql.Stmt
		deleteBySess *sql.Stmt
		deleteFromID *sql.Stmt
	}
}

func NewService(db *sql.DB) *Service {
	s := &Service{db: db}
	s.initStatements()
	return s
}

// initStatements 预编译常用 SQL 语句
func (s *Service) initStatements() {
	var err error

	s.stmts.create, err = s.db.Prepare(
		"INSERT INTO messages (session_id, role, content, images, stats, tool_calls, tool_results) VALUES (?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		panic("failed to prepare message create statement: " + err.Error())
	}

	s.stmts.getBySession, err = s.db.Prepare(
		"SELECT id, session_id, role, content, images, stats, tool_calls, tool_results, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC",
	)
	if err != nil {
		panic("failed to prepare message get statement: " + err.Error())
	}

	s.stmts.deleteBySess, err = s.db.Prepare("DELETE FROM messages WHERE session_id = ?")
	if err != nil {
		panic("failed to prepare message delete statement: " + err.Error())
	}

	s.stmts.deleteFromID, err = s.db.Prepare("DELETE FROM messages WHERE session_id = ? AND id >= ?")
	if err != nil {
		panic("failed to prepare message delete from id statement: " + err.Error())
	}
}

// Close 关闭所有 prepared statements
func (s *Service) Close() {
	s.stmts.Lock()
	defer s.stmts.Unlock()
	if s.stmts.create != nil {
		s.stmts.create.Close()
	}
	if s.stmts.getBySession != nil {
		s.stmts.getBySession.Close()
	}
	if s.stmts.deleteBySess != nil {
		s.stmts.deleteBySess.Close()
	}
	if s.stmts.deleteFromID != nil {
		s.stmts.deleteFromID.Close()
	}
}

func (s *Service) Create(m *model.Message) error {
	s.stmts.Lock()
	defer s.stmts.Unlock()
	result, err := s.stmts.create.Exec(
		m.SessionID, m.Role, m.Content, m.Images, m.StatsJSON, m.ToolCallsJSON, m.ToolResultsJSON,
	)
	if err != nil {
		return fmt.Errorf("message: create: %w", err)
	}
	m.ID, _ = result.LastInsertId()
	return nil
}

func (s *Service) GetBySession(sessionID int64) ([]model.Message, error) {
	s.stmts.Lock()
	defer s.stmts.Unlock()
	rows, err := s.stmts.getBySession.Query(sessionID)
	if err != nil {
		return nil, fmt.Errorf("message: get by session: %w", err)
	}
	defer rows.Close()

	var messages []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Images, &m.StatsJSON, &m.ToolCallsJSON, &m.ToolResultsJSON, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("message: scan: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *Service) DeleteBySession(sessionID int64) error {
	s.stmts.Lock()
	defer s.stmts.Unlock()
	_, err := s.stmts.deleteBySess.Exec(sessionID)
	if err != nil {
		return fmt.Errorf("message: delete by session: %w", err)
	}
	return nil
}

func (s *Service) DeleteFromID(sessionID int64, messageIDStr string) error {
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("message: invalid message ID: %w", err)
	}
	s.stmts.Lock()
	defer s.stmts.Unlock()
	_, err = s.stmts.deleteFromID.Exec(sessionID, messageID)
	if err != nil {
		return fmt.Errorf("message: delete from id: %w", err)
	}
	return nil
}
