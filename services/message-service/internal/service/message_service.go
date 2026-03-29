package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"realtime-chat-system/services/message-service/internal/model"
	"realtime-chat-system/services/message-service/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

type MessageService struct {
	repo      *repository.MessageRepository
	chats     *repository.ChatRepository
	receipts  *repository.ReceiptRepository
	reactions *repository.ReactionRepository
	nc        *nats.Conn
}

func New(repo *repository.MessageRepository, chats *repository.ChatRepository, receipts *repository.ReceiptRepository, reactions *repository.ReactionRepository, nc *nats.Conn) *MessageService {
	return &MessageService{repo: repo, chats: chats, receipts: receipts, reactions: reactions, nc: nc}
}

func normalizeEmoji(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("emoji required")
	}
	if utf8.RuneCountInString(s) > 16 {
		return "", errors.New("emoji too long")
	}
	return s, nil
}

func (s *MessageService) publishReactionUpdated(chatID, messageID, userID, emoji string, add bool) {
	act := "remove"
	if add {
		act = "add"
	}
	b, _ := json.Marshal(struct {
		ChatID         string    `json:"chat_id"`
		MessageID      string    `json:"message_id"`
		UserID         string    `json:"user_id"`
		Emoji          string    `json:"emoji"`
		ReactionAction string    `json:"reaction_action"`
		At             time.Time `json:"at"`
	}{
		ChatID:         chatID,
		MessageID:      messageID,
		UserID:         userID,
		Emoji:          emoji,
		ReactionAction: act,
		At:             time.Now().UTC(),
	})
	_ = s.nc.Publish("chat.reaction.updated", b)
}

// ApplyReaction validates membership and message chat, mutates DB, then broadcasts if state changed.
func (s *MessageService) ApplyReaction(ctx context.Context, chatID, userID, messageID, emoji string, add bool) error {
	emoji, err := normalizeEmoji(emoji)
	if err != nil {
		return err
	}
	ok, err := s.chats.IsMember(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("forbidden: not a chat member")
	}
	ok, err = s.repo.MessageInChat(ctx, messageID, chatID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("message not in chat")
	}
	if add {
		inserted, err := s.reactions.Add(ctx, messageID, userID, emoji)
		if err != nil {
			return err
		}
		if inserted {
			s.publishReactionUpdated(chatID, messageID, userID, emoji, true)
		}
		return nil
	}
	removed, err := s.reactions.Remove(ctx, messageID, userID, emoji)
	if err != nil {
		return err
	}
	if removed {
		s.publishReactionUpdated(chatID, messageID, userID, emoji, false)
	}
	return nil
}

func (s *MessageService) ToggleReaction(ctx context.Context, userID, messageID, emoji string, add bool) error {
	emoji, err := normalizeEmoji(emoji)
	if err != nil {
		return err
	}
	chatID, err := s.repo.ChatIDByMessageID(ctx, messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("message not found")
		}
		return err
	}
	return s.ApplyReaction(ctx, chatID, userID, messageID, emoji, add)
}

func (s *MessageService) PersistReactionFromEvent(ctx context.Context, payload []byte) error {
	var in struct {
		ChatID    string `json:"chat_id"`
		SenderID  string `json:"sender_id"`
		MessageID string `json:"message_id"`
		Emoji     string `json:"emoji"`
		Add       bool   `json:"add"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return err
	}
	if in.ChatID == "" || in.SenderID == "" || in.MessageID == "" {
		return errors.New("invalid reaction event")
	}
	return s.ApplyReaction(ctx, in.ChatID, in.SenderID, in.MessageID, in.Emoji, in.Add)
}

func (s *MessageService) attachReplyPreview(ctx context.Context, m *model.Message) {
	if m.ReplyToMessageID == nil || *m.ReplyToMessageID == "" {
		m.ReplyTo = nil
		return
	}
	p, err := s.repo.GetReplyPreview(ctx, *m.ReplyToMessageID, m.ChatID)
	if err != nil || p == nil {
		m.ReplyTo = nil
		return
	}
	m.ReplyTo = p
}

func (s *MessageService) Create(ctx context.Context, chatID, senderID, content, idem string, replyTo *string) (*model.Message, error) {
	if replyTo != nil && *replyTo == "" {
		replyTo = nil
	}
	if replyTo != nil {
		ok, err := s.repo.MessageInChat(ctx, *replyTo, chatID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("invalid reply_to_message_id: not in this chat")
		}
	}
	ok, err := s.chats.IsMember(ctx, chatID, senderID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("forbidden: not a chat member")
	}
	m, inserted, err := s.repo.Create(ctx, chatID, senderID, content, idem, replyTo)
	if err != nil {
		return nil, err
	}
	s.attachReplyPreview(ctx, m)
	if inserted {
		b, _ := json.Marshal(m)
		_ = s.nc.Publish("chat.message.created", b)
	}
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
	items, err := s.repo.History(ctx, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	byMsg, err := s.receipts.ListByMessageIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ReadBy = byMsg[items[i].ID]
	}
	byReact, err := s.reactions.ListAggregated(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Reactions = byReact[items[i].ID]
	}
	return items, nil
}

func (s *MessageService) PersistFromEvent(ctx context.Context, payload []byte) error {
	var in struct {
		ChatID             string `json:"chat_id"`
		SenderID           string `json:"sender_id"`
		Content            string `json:"content"`
		IdempotencyKey     string `json:"idempotency_key"`
		ReplyToMessageID   string `json:"reply_to_message_id"`
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
	var replyPtr *string
	if in.ReplyToMessageID != "" {
		okm, err := s.repo.MessageInChat(ctx, in.ReplyToMessageID, in.ChatID)
		if err != nil {
			return err
		}
		if !okm {
			return errors.New("invalid reply_to_message_id")
		}
		replyPtr = &in.ReplyToMessageID
	}
	m, inserted, err := s.repo.Create(ctx, in.ChatID, in.SenderID, in.Content, in.IdempotencyKey, replyPtr)
	if err != nil {
		return err
	}
	s.attachReplyPreview(ctx, m)
	if inserted {
		b, _ := json.Marshal(m)
		_ = s.nc.Publish("chat.message.created", b)
	}
	return nil
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
