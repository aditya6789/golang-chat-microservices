package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"realtime-chat-system/services/chat-service/internal/model"
	"realtime-chat-system/services/chat-service/internal/repository"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

type Hub struct {
	repo  *repository.RedisRepository
	nc    *nats.Conn
	conns map[string]*websocket.Conn
	mu    sync.RWMutex
}

func NewHub(repo *repository.RedisRepository, nc *nats.Conn) *Hub {
	return &Hub{repo: repo, nc: nc, conns: make(map[string]*websocket.Conn)}
}

func (h *Hub) Register(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	h.conns[userID] = conn
	h.mu.Unlock()
}

func (h *Hub) Unregister(userID string) {
	h.mu.Lock()
	delete(h.conns, userID)
	h.mu.Unlock()
}

func (h *Hub) HandleInbound(ctx context.Context, e model.Event) error {
	e.At = time.Now().UTC()
	if err := h.repo.Publish(ctx, e.ChatID, e); err != nil {
		return err
	}
	if e.Type == "message" {
		b, _ := json.Marshal(e)
		_ = h.nc.Publish("chat.message.persist", b)
	}
	return nil
}

func (h *Hub) StreamChat(ctx context.Context, userID, chatID string) {
	pubsub := h.repo.Subscribe(ctx, chatID)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			h.mu.RLock()
			conn, ok := h.conns[userID]
			h.mu.RUnlock()
			if ok && conn != nil {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			}
		}
	}
}

