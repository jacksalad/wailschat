package model

import "time"

type Prompt struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	IsDefault bool      `json:"is_default"`
	SortOrder int64     `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
