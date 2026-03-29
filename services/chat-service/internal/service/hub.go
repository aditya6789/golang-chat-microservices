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

// clientSession holds one browser/tab WebSocket: only one StreamChat may write per key;
// reconnecting cancels the previous Redis subscriber so two goroutines never share one Conn.
type clientSession struct {
	conn       *websocket.Conn
	writeMu    sync.Mutex
	cancelStream context.CancelFunc
}

type Hub struct {
	repo    *repository.RedisRepository
	nc      *nats.Conn
	members MemberChecker
	sessions map[string]*clientSession
	mu       sync.RWMutex
}

func NewHub(repo *repository.RedisRepository, nc *nats.Conn, members MemberChecker) *Hub {
	return &Hub{repo: repo, nc: nc, members: members, sessions: make(map[string]*clientSession)}
}

// BeginSession registers this WebSocket for connKey, cancels any prior session (same user+chat),
// and returns a context that StreamChat should use. Caller must defer cancel and UnregisterIf(connKey, conn).
func (h *Hub) BeginSession(connKey string, conn *websocket.Conn) (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(context.Background())

	h.mu.Lock()
	if prev, ok := h.sessions[connKey]; ok && prev.cancelStream != nil {
		prev.cancelStream()
	}
	s := &clientSession{conn: conn, cancelStream: cancel}
	h.sessions[connKey] = s
	h.mu.Unlock()

	return ctx, cancel
}

// UnregisterIf removes the session only if this WebSocket is still the one registered
// (avoids a stale tab closing after a reconnect wiping the new session).
func (h *Hub) UnregisterIf(connKey string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[connKey]
	if !ok || s.conn != conn {
		return
	}
	if s.cancelStream != nil {
		s.cancelStream()
	}
	delete(h.sessions, connKey)
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
	// Chat messages and reactions are fan-out only after Postgres (NATS → message-service → Redis).
	if e.Type != "message" && e.Type != "reaction" {
		if err := h.repo.Publish(ctx, e.ChatID, e); err != nil {
			return err
		}
	}
	if e.Type == "message" {
		idem := "ws:" + uuid.NewString()
		type persistPayload struct {
			ChatID               string `json:"chat_id"`
			SenderID             string `json:"sender_id"`
			Content              string `json:"content"`
			IdempotencyKey       string `json:"idempotency_key"`
			ReplyToMessageID     string `json:"reply_to_message_id,omitempty"`
		}
		p := persistPayload{
			ChatID:           e.ChatID,
			SenderID:         e.SenderID,
			Content:          e.Content,
			IdempotencyKey:   idem,
			ReplyToMessageID: e.ReplyToMessageID,
		}
		b, _ := json.Marshal(p)
		_ = h.nc.Publish("chat.message.persist", b)
		return nil
	}
	if e.Type == "reaction" {
		if e.MessageID == "" || e.Emoji == "" {
			return errors.New("invalid reaction")
		}
		add := e.ReactionAction != "remove"
		b, _ := json.Marshal(struct {
			ChatID    string `json:"chat_id"`
			SenderID  string `json:"sender_id"`
			MessageID string `json:"message_id"`
			Emoji     string `json:"emoji"`
			Add       bool   `json:"add"`
		}{ChatID: e.ChatID, SenderID: e.SenderID, MessageID: e.MessageID, Emoji: e.Emoji, Add: add})
		_ = h.nc.Publish("chat.reaction.persist", b)
		return nil
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

// StartMessageFanout subscribes to durable message creates and publishes to Redis for WebSocket clients.
func (h *Hub) StartMessageFanout() {
	_, _ = h.nc.Subscribe("chat.message.created", func(msg *nats.Msg) {
		var m struct {
			ID               string            `json:"id"`
			ChatID           string            `json:"chat_id"`
			SenderID         string            `json:"sender_id"`
			Content          string            `json:"content"`
			CreatedAt        time.Time         `json:"created_at"`
			ReplyToMessageID *string           `json:"reply_to_message_id"`
			ReplyTo          *model.ReplyQuote `json:"reply_to"`
		}
		if err := json.Unmarshal(msg.Data, &m); err != nil || m.ID == "" || m.ChatID == "" {
			return
		}
		at := m.CreatedAt
		if at.IsZero() {
			at = time.Now().UTC()
		}
		e := model.Event{
			Type:      "message",
			ChatID:    m.ChatID,
			SenderID:  m.SenderID,
			Content:   m.Content,
			MessageID: m.ID,
			At:        at.UTC(),
			ReplyTo:   m.ReplyTo,
		}
		if m.ReplyToMessageID != nil && *m.ReplyToMessageID != "" {
			e.ReplyToMessageID = *m.ReplyToMessageID
		}
		_ = h.repo.Publish(context.Background(), m.ChatID, e)
	})
}

// StartReactionFanout pushes persisted reaction changes to Redis for WebSocket clients.
func (h *Hub) StartReactionFanout() {
	_, _ = h.nc.Subscribe("chat.reaction.updated", func(msg *nats.Msg) {
		var p struct {
			ChatID         string    `json:"chat_id"`
			MessageID      string    `json:"message_id"`
			UserID         string    `json:"user_id"`
			Emoji          string    `json:"emoji"`
			ReactionAction string    `json:"reaction_action"`
			At             time.Time `json:"at"`
		}
		if err := json.Unmarshal(msg.Data, &p); err != nil || p.ChatID == "" || p.MessageID == "" {
			return
		}
		at := p.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		e := model.Event{
			Type:             "reaction",
			ChatID:           p.ChatID,
			SenderID:         p.UserID,
			MessageID:        p.MessageID,
			Emoji:            p.Emoji,
			ReactionAction:   p.ReactionAction,
			At:               at.UTC(),
		}
		_ = h.repo.Publish(context.Background(), p.ChatID, e)
	})
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
		s := h.sessions[connKey]
		h.mu.RUnlock()
		if s == nil || s.conn == nil {
			continue
		}
		s.writeMu.Lock()
		err = s.conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		s.writeMu.Unlock()
		if err != nil {
			return
		}
	}
}
