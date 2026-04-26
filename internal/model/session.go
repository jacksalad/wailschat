package model

import "time"

type Session struct {
	ID         int64     `json:"id"`
	ProviderID int64     `json:"provider_id"`
	Name       string    `json:"name"`
	Model      string    `json:"model"`
	PromptID   *int64    `json:"prompt_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
