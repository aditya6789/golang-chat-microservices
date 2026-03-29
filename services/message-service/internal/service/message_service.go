package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"realtime-chat-system/services/message-service/internal/attachment"
	"realtime-chat-system/services/message-service/internal/model"
	"realtime-chat-system/services/message-service/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
)

// FileAttachment is the client-provided metadata after a successful presigned PUT.
type FileAttachment struct {
	ObjectKey  string `json:"object_key"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mime_type"`
	SizeBytes  int64  `json:"size_bytes"`
}

type fileContentJSON struct {
	ObjectKey   string `json:"object_key"`
	Filename    string `json:"filename"`
	MimeType    string `json:"mime_type"`
	SizeBytes   int64  `json:"size_bytes"`
	DownloadURL string `json:"download_url"`
}

type MessageService struct {
	repo      *repository.MessageRepository
	chats     *repository.ChatRepository
	receipts  *repository.ReceiptRepository
	reactions *repository.ReactionRepository
	attach    *attachment.Store
	nc        *nats.Conn
}

func New(repo *repository.MessageRepository, chats *repository.ChatRepository, receipts *repository.ReceiptRepository, reactions *repository.ReactionRepository, attach *attachment.Store, nc *nats.Conn) *MessageService {
	return &MessageService{repo: repo, chats: chats, receipts: receipts, reactions: reactions, attach: attach, nc: nc}
}

func normMIME(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(s, ";")[0]))
}

func mimeCompatible(declared, actual string) bool {
	d, a := normMIME(declared), normMIME(actual)
	if d == "" || a == "" {
		return false
	}
	if d == a {
		return true
	}
	if strings.HasPrefix(d, "image/") && strings.HasPrefix(a, "image/") {
		return true
	}
	if strings.HasPrefix(d, "text/") && strings.HasPrefix(a, "text/") {
		return true
	}
	return false
}

func (s *MessageService) finalizeFileAttachment(ctx context.Context, chatID string, f *FileAttachment) (string, error) {
	if f == nil || strings.TrimSpace(f.ObjectKey) == "" {
		return "", errors.New("file.object_key required")
	}
	if s.attach == nil {
		return "", errors.New("attachments not configured")
	}
	prefix := attachment.KeyPrefixForChat(chatID)
	if !strings.HasPrefix(f.ObjectKey, prefix) {
		return "", errors.New("invalid object_key for this chat")
	}
	if !attachment.AllowedContentType(f.MimeType) {
		return "", errors.New("mime type not allowed")
	}
	headSize, headCT, err := s.attach.HeadObject(ctx, f.ObjectKey)
	if err != nil {
		return "", errors.New("uploaded object not found")
	}
	if headSize != f.SizeBytes {
		return "", errors.New("size mismatch with stored object")
	}
	if !mimeCompatible(f.MimeType, headCT) {
		return "", errors.New("content type mismatch")
	}
	out := fileContentJSON{
		ObjectKey:   f.ObjectKey,
		Filename:    f.Filename,
		MimeType:    f.MimeType,
		SizeBytes:   f.SizeBytes,
		DownloadURL: s.attach.DownloadURL(f.ObjectKey),
	}
	b, err := json.Marshal(out)
	return string(b), err
}

func (s *MessageService) persistMessageBody(ctx context.Context, chatID string, msgType string, text string, file *FileAttachment) (body string, mt string, err error) {
	mt = msgType
	if mt == "" {
		mt = "text"
	}
	if mt == "file" {
		body, err = s.finalizeFileAttachment(ctx, chatID, file)
		if err != nil {
			return "", "", err
		}
		return body, "file", nil
	}
	if strings.TrimSpace(text) == "" {
		return "", "", errors.New("content required")
	}
	return text, "text", nil
}

// PresignAttachment returns a PUT URL for an object under this chat prefix.
func (s *MessageService) PresignAttachment(ctx context.Context, chatID, userID, filename, contentType string, size int64) (uploadURL, objectKey string, headers map[string]string, err error) {
	if s.attach == nil {
		return "", "", nil, errors.New("attachments not configured")
	}
	ok, err := s.chats.IsMember(ctx, chatID, userID)
	if err != nil {
		return "", "", nil, err
	}
	if !ok {
		return "", "", nil, errors.New("forbidden: not a chat member")
	}
	return s.attach.PresignPut(ctx, chatID, filename, contentType, size)
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

func (s *MessageService) Create(ctx context.Context, chatID, senderID, idem string, replyTo *string, msgType string, text string, file *FileAttachment) (*model.Message, error) {
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
	body, mt, err := s.persistMessageBody(ctx, chatID, msgType, text, file)
	if err != nil {
		return nil, err
	}
	m, inserted, err := s.repo.Create(ctx, chatID, senderID, body, idem, replyTo, mt)
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
		ChatID           string           `json:"chat_id"`
		SenderID         string           `json:"sender_id"`
		Content          string           `json:"content"`
		MessageType      string           `json:"message_type"`
		File             *FileAttachment  `json:"file"`
		IdempotencyKey   string           `json:"idempotency_key"`
		ReplyToMessageID string           `json:"reply_to_message_id"`
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
	body, mt, err := s.persistMessageBody(ctx, in.ChatID, in.MessageType, in.Content, in.File)
	if err != nil {
		return err
	}
	m, inserted, err := s.repo.Create(ctx, in.ChatID, in.SenderID, body, in.IdempotencyKey, replyPtr, mt)
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
