package service

import (
	"context"
	"encoding/json"

	"realtime-chat-system/services/message-service/internal/model"
	"realtime-chat-system/services/message-service/internal/repository"

	"github.com/nats-io/nats.go"
)

type MessageService struct {
	repo *repository.MessageRepository
	nc   *nats.Conn
}

func New(repo *repository.MessageRepository, nc *nats.Conn) *MessageService {
	return &MessageService{repo: repo, nc: nc}
}

func (s *MessageService) Create(ctx context.Context, chatID, senderID, content, idem string) (*model.Message, error) {
	m, err := s.repo.Create(ctx, chatID, senderID, content, idem)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(m)
	_ = s.nc.Publish("chat.message.created", b)
	return m, nil
}

func (s *MessageService) History(ctx context.Context, chatID string, limit, offset int) ([]model.Message, error) {
	return s.repo.History(ctx, chatID, limit, offset)
}

func (s *MessageService) PersistFromEvent(ctx context.Context, payload []byte) error {
	var in struct {
		ChatID   string `json:"chat_id"`
		SenderID string `json:"sender_id"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return err
	}
	_, err := s.repo.Create(ctx, in.ChatID, in.SenderID, in.Content, "")
	return err
}

