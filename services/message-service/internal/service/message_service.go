package service

import (
	"context"
	"encoding/json"
	"errors"

	"realtime-chat-system/services/message-service/internal/model"
	"realtime-chat-system/services/message-service/internal/repository"

	"github.com/nats-io/nats.go"
)

type MessageService struct {
	repo     *repository.MessageRepository
	chats    *repository.ChatRepository
	receipts *repository.ReceiptRepository
	nc       *nats.Conn
}

func New(repo *repository.MessageRepository, chats *repository.ChatRepository, receipts *repository.ReceiptRepository, nc *nats.Conn) *MessageService {
	return &MessageService{repo: repo, chats: chats, receipts: receipts, nc: nc}
}

func (s *MessageService) Create(ctx context.Context, chatID, senderID, content, idem string) (*model.Message, error) {
	ok, err := s.chats.IsMember(ctx, chatID, senderID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("forbidden: not a chat member")
	}
	m, err := s.repo.Create(ctx, chatID, senderID, content, idem)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(m)
	_ = s.nc.Publish("chat.message.created", b)
	return m, nil
}

func (s *MessageService) History(ctx context.Context, chatID, userID string, limit, offset int) ([]model.Message, error) {
	ok, err := s.chats.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("forbidden: not a chat member")
	}
	return s.repo.History(ctx, chatID, limit, offset)
}

func (s *MessageService) PersistFromEvent(ctx context.Context, payload []byte) error {
	var in struct {
		ChatID         string `json:"chat_id"`
		SenderID       string `json:"sender_id"`
		Content        string `json:"content"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return err
	}
	if in.IdempotencyKey == "" {
		return errors.New("missing idempotency_key")
	}
	ok, err := s.chats.IsMember(ctx, in.ChatID, in.SenderID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("forbidden: not a chat member")
	}
	_, err = s.repo.Create(ctx, in.ChatID, in.SenderID, in.Content, in.IdempotencyKey)
	return err
}

func (s *MessageService) PersistReceiptFromEvent(ctx context.Context, payload []byte) error {
	var in struct {
		ChatID    string `json:"chat_id"`
		SenderID  string `json:"sender_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return err
	}
	if in.ChatID == "" || in.SenderID == "" || in.MessageID == "" {
		return errors.New("invalid receipt event")
	}
	ok, err := s.chats.IsMember(ctx, in.ChatID, in.SenderID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("forbidden: not a chat member")
	}
	return s.receipts.MarkRead(ctx, in.MessageID, in.SenderID, in.ChatID)
}

