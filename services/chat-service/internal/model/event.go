package model

import "time"

type Event struct {
	Type      string    `json:"type"`
	ChatID    string    `json:"chat_id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content,omitempty"`
	MessageID string    `json:"message_id,omitempty"`
	At        time.Time `json:"at"`
}

