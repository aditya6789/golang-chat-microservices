package model

import "time"

type ReplyQuote struct {
	ID        string    `json:"id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	Type               string      `json:"type"`
	ChatID             string      `json:"chat_id"`
	SenderID           string      `json:"sender_id"`
	Content            string      `json:"content,omitempty"`
	MessageID          string      `json:"message_id,omitempty"`
	ReplyToMessageID   string      `json:"reply_to_message_id,omitempty"`
	ReplyTo            *ReplyQuote `json:"reply_to,omitempty"`
	Emoji              string      `json:"emoji,omitempty"`
	ReactionAction     string      `json:"reaction_action,omitempty"` // add | remove
	At                 time.Time   `json:"at"`
}
