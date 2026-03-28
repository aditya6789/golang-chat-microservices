package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"realtime-chat-system/services/chat-service/internal/model"
	"realtime-chat-system/services/chat-service/internal/repository"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

type MemberChecker interface {
	IsMember(ctx context.Context, chatID, userID string) bool
}

type Hub struct {
	repo    *repository.RedisRepository
	nc      *nats.Conn
	members MemberChecker
	conns   map[string]*websocket.Conn
	mu      sync.RWMutex
}

func NewHub(repo *repository.RedisRepository, nc *nats.Conn, members MemberChecker) *Hub {
	return &Hub{repo: repo, nc: nc, members: members, conns: make(map[string]*websocket.Conn)}
}

func (h *Hub) Register(connKey string, conn *websocket.Conn) {
	h.mu.Lock()
	h.conns[connKey] = conn
	h.mu.Unlock()
}

func (h *Hub) Unregister(connKey string) {
	h.mu.Lock()
	delete(h.conns, connKey)
	h.mu.Unlock()
}

func (h *Hub) VerifyMember(ctx context.Context, chatID, userID string) bool {
	if h.members == nil {
		return false
	}
	return h.members.IsMember(ctx, chatID, userID)
}

func (h *Hub) HandleInbound(ctx context.Context, e model.Event) error {
	if !h.VerifyMember(ctx, e.ChatID, e.SenderID) {
		return errors.New("forbidden")
	}
	e.At = time.Now().UTC()
	if err := h.repo.Publish(ctx, e.ChatID, e); err != nil {
		return err
	}
	if e.Type == "message" {
		idem := "ws:" + uuid.NewString()
		b, _ := json.Marshal(struct {
			ChatID         string `json:"chat_id"`
			SenderID       string `json:"sender_id"`
			Content        string `json:"content"`
			IdempotencyKey string `json:"idempotency_key"`
		}{ChatID: e.ChatID, SenderID: e.SenderID, Content: e.Content, IdempotencyKey: idem})
		_ = h.nc.Publish("chat.message.persist", b)
	}
	if e.Type == "read_receipt" && e.MessageID != "" {
		b, _ := json.Marshal(struct {
			ChatID    string `json:"chat_id"`
			SenderID  string `json:"sender_id"`
			MessageID string `json:"message_id"`
		}{ChatID: e.ChatID, SenderID: e.SenderID, MessageID: e.MessageID})
		_ = h.nc.Publish("chat.receipt.persist", b)
	}
	return nil
}

func (h *Hub) StreamChat(ctx context.Context, connKey, chatID string) {
	pubsub := h.repo.Subscribe(ctx, chatID)
	defer pubsub.Close()
	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			return
		}
		h.mu.RLock()
		conn, ok := h.conns[connKey]
		h.mu.RUnlock()
		if ok && conn != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		}
	}
}

