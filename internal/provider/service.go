package provider

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

func (s *Service) Add(p *model.Provider) error {
	result, err := s.db.Exec(
		"INSERT INTO providers (name, api_key, base_url, models, is_default) VALUES (?, ?, ?, ?, ?)",
		p.Name, p.APIKey, p.BaseURL, p.Models, p.IsDefault,
	)
	if err != nil {
		return fmt.Errorf("provider: add: %w", err)
	}
	p.ID, _ = result.LastInsertId()
	return nil
}

func (s *Service) GetAll() ([]model.Provider, error) {
	rows, err := s.db.Query("SELECT id, name, api_key, base_url, models, is_default, created_at FROM providers ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("provider: get all: %w", err)
	}
	defer rows.Close()

	var providers []model.Provider
	for rows.Next() {
		var p model.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &p.Models, &p.IsDefault, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("provider: scan: %w", err)
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func (s *Service) GetByID(id int64) (*model.Provider, error) {
	var p model.Provider
	err := s.db.QueryRow(
		"SELECT id, name, api_key, base_url, models, is_default, created_at FROM providers WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &p.Models, &p.IsDefault, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("provider: get by id: %w", err)
	}
	return &p, nil
}

func (s *Service) Update(p *model.Provider) error {
	_, err := s.db.Exec(
		"UPDATE providers SET name=?, api_key=?, base_url=?, models=?, is_default=? WHERE id=?",
		p.Name, p.APIKey, p.BaseURL, p.Models, p.IsDefault, p.ID,
	)
	if err != nil {
		return fmt.Errorf("provider: update: %w", err)
	}
	return nil
}

func (s *Service) GetDefault() (*model.Provider, error) {
	var p model.Provider
	err := s.db.QueryRow(
		"SELECT id, name, api_key, base_url, models, is_default, created_at FROM providers WHERE is_default = 1 LIMIT 1",
	).Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &p.Models, &p.IsDefault, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("provider: get default: %w", err)
	}
	return &p, nil
}

func (s *Service) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM providers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("provider: delete: %w", err)
	}
	return nil
}
