package model

import "time"

type ReadReceipt struct {
	UserID string    `json:"user_id"`
	ReadAt time.Time `json:"read_at"`
}

type Message struct {
	ID        string        `json:"id"`
	ChatID    string        `json:"chat_id"`
	SenderID  string        `json:"sender_id"`
	Content   string        `json:"content"`
	CreatedAt time.Time     `json:"created_at"`
	ReadBy    []ReadReceipt `json:"read_by,omitempty"`
}

