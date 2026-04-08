package settings

import (
	"database/sql"
	"fmt"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetAll() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("settings: get all: %w", err)
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("settings: scan: %w", err)
		}
		m[k] = v
	}
	return m, rows.Err()
}

func (s *Service) Set(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("settings: set: %w", err)
	}
	return nil
}

// Get retrieves a single setting value by key. Returns empty string if not found.
func (s *Service) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("settings: get: %w", err)
	}
	return value, nil
}

// GetSystemPrompt retrieves the system_prompt setting directly.
func (s *Service) GetSystemPrompt() (string, error) {
	return s.Get("system_prompt")
}
