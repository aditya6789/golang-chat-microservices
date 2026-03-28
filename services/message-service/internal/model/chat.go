package model

import "time"

type Chat struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      *string   `json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatMember struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at,omitempty"`
}
