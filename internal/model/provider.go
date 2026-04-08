package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// StringArray is a custom type for storing []string as JSON in SQLite.
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *StringArray) Scan(src any) error {
	if src == nil {
		*s = StringArray{}
		return nil
	}
	var bytes []byte
	switch v := src.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return nil
	}
	return json.Unmarshal(bytes, s)
}

type Provider struct {
	ID        int64       `json:"id"`
	Name      string      `json:"name"`
	APIKey    string      `json:"api_key"`
	BaseURL   string      `json:"base_url"`
	Models    StringArray `json:"models"`
	IsDefault bool        `json:"is_default"`
	CreatedAt time.Time   `json:"created_at"`
}
