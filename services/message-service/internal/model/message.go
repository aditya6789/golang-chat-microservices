package model

import "time"

type ReadReceipt struct {
	UserID string    `json:"user_id"`
	ReadAt time.Time `json:"read_at"`
}

// ReactionAgg is aggregated emoji counts for a message (history + client merge).
type ReactionAgg struct {
	Emoji   string   `json:"emoji"`
	UserIDs []string `json:"user_ids"`
}

// ReplyPreview is a denormalized snippet of the parent message for clients (history + realtime).
type ReplyPreview struct {
	ID        string    `json:"id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	ID                 string        `json:"id"`
	ChatID             string        `json:"chat_id"`
	SenderID           string        `json:"sender_id"`
	Content            string        `json:"content"`
	CreatedAt          time.Time     `json:"created_at"`
	ReplyToMessageID   *string       `json:"reply_to_message_id,omitempty"`
	ReplyTo            *ReplyPreview `json:"reply_to,omitempty"`
	ReadBy             []ReadReceipt   `json:"read_by,omitempty"`
	Reactions          []ReactionAgg   `json:"reactions,omitempty"`
}

