package prompt

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

func (s *Service) Create(p *model.Prompt) error {
	// If this is set as default, clear default on all others first
	if p.IsDefault {
		s.db.Exec("UPDATE prompts SET is_default = 0")
	}

	result, err := s.db.Exec(
		"INSERT INTO prompts (name, content, category, is_default, sort_order) VALUES (?, ?, ?, ?, ?)",
		p.Name, p.Content, p.Category, p.IsDefault, p.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("prompt: create: %w", err)
	}
	p.ID, _ = result.LastInsertId()
	return nil
}

func (s *Service) GetAll() ([]model.Prompt, error) {
	rows, err := s.db.Query("SELECT id, name, content, category, is_default, sort_order, created_at, updated_at FROM prompts ORDER BY sort_order ASC, created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("prompt: get all: %w", err)
	}
	defer rows.Close()

	var prompts []model.Prompt
	for rows.Next() {
		var p model.Prompt
		if err := rows.Scan(&p.ID, &p.Name, &p.Content, &p.Category, &p.IsDefault, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("prompt: scan: %w", err)
		}
		prompts = append(prompts, p)
	}
	return prompts, rows.Err()
}

func (s *Service) GetByID(id int64) (*model.Prompt, error) {
	var p model.Prompt
	err := s.db.QueryRow(
		"SELECT id, name, content, category, is_default, sort_order, created_at, updated_at FROM prompts WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Content, &p.Category, &p.IsDefault, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("prompt: get by id: %w", err)
	}
	return &p, nil
}

func (s *Service) GetDefault() (*model.Prompt, error) {
	var p model.Prompt
	err := s.db.QueryRow(
		"SELECT id, name, content, category, is_default, sort_order, created_at, updated_at FROM prompts WHERE is_default = 1 LIMIT 1",
	).Scan(&p.ID, &p.Name, &p.Content, &p.Category, &p.IsDefault, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("prompt: get default: %w", err)
	}
	return &p, nil
}

func (s *Service) Update(p *model.Prompt) error {
	// If this is set as default, clear default on all others first
	if p.IsDefault {
		s.db.Exec("UPDATE prompts SET is_default = 0")
	}
	_, err := s.db.Exec(
		"UPDATE prompts SET name=?, content=?, category=?, is_default=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		p.Name, p.Content, p.Category, p.IsDefault, p.ID,
	)
	if err != nil {
		return fmt.Errorf("prompt: update: %w", err)
	}
	return nil
}

func (s *Service) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM prompts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("prompt: delete: %w", err)
	}
	return nil
}

func (s *Service) SetDefault(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("prompt: set default: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE prompts SET is_default = 0"); err != nil {
		return fmt.Errorf("prompt: set default: clear: %w", err)
	}
	if _, err := tx.Exec("UPDATE prompts SET is_default = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		return fmt.Errorf("prompt: set default: set: %w", err)
	}
	return tx.Commit()
}
