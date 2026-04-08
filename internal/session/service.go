package session

import (
	"database/sql"
	"fmt"

	"wailschat/internal/model"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(sess *model.Session) error {
	result, err := s.db.Exec(
		"INSERT INTO sessions (provider_id, name, model) VALUES (?, ?, ?)",
		sess.ProviderID, sess.Name, sess.Model,
	)
	if err != nil {
		return fmt.Errorf("session: create: %w", err)
	}
	sess.ID, _ = result.LastInsertId()
	return nil
}

func (s *Service) GetAll() ([]model.Session, error) {
	rows, err := s.db.Query("SELECT id, provider_id, name, model, created_at, updated_at FROM sessions ORDER BY sort_order DESC, updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("session: get all: %w", err)
	}
	defer rows.Close()

	var sessions []model.Session
	for rows.Next() {
		var sess model.Session
		if err := rows.Scan(&sess.ID, &sess.ProviderID, &sess.Name, &sess.Model, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return nil, fmt.Errorf("session: scan: %w", err)
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *Service) GetByID(id int64) (*model.Session, error) {
	var sess model.Session
	err := s.db.QueryRow(
		"SELECT id, provider_id, name, model, created_at, updated_at FROM sessions WHERE id = ?", id,
	).Scan(&sess.ID, &sess.ProviderID, &sess.Name, &sess.Model, &sess.CreatedAt, &sess.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("session: get by id: %w", err)
	}
	return &sess, nil
}

func (s *Service) Update(sess *model.Session) error {
	_, err := s.db.Exec(
		"UPDATE sessions SET provider_id=?, name=?, model=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		sess.ProviderID, sess.Name, sess.Model, sess.ID,
	)
	if err != nil {
		return fmt.Errorf("session: update: %w", err)
	}
	return nil
}

func (s *Service) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("session: delete: %w", err)
	}
	// Reclaim disk space freed by CASCADE-deleted messages
	s.db.Exec("VACUUM")
	return nil
}

// TouchSession moves a session to the top of the list by setting its sort_order to MAX+1.
func (s *Service) TouchSession(id int64) error {
	_, err := s.db.Exec(
		"UPDATE sessions SET sort_order = (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM sessions), updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		id,
	)
	if err != nil {
		return fmt.Errorf("session: touch: %w", err)
	}
	return nil
}

// ReorderSessions updates the sort_order of all sessions to match the given order.
// orderedIDs should be in the desired display order (first = top).
func (s *Service) ReorderSessions(orderedIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("session: reorder: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE sessions SET sort_order = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("session: reorder: prepare: %w", err)
	}
	defer stmt.Close()

	for i, id := range orderedIDs {
		sortOrder := int64(len(orderedIDs) - i)
		if _, err := stmt.Exec(sortOrder, id); err != nil {
			return fmt.Errorf("session: reorder: exec: %w", err)
		}
	}

	return tx.Commit()
}
