package model

type Notification struct {
	Type    string `json:"type"`
	ChatID  string `json:"chat_id"`
	UserID  string `json:"sender_id"`
	Content string `json:"content"`
}

