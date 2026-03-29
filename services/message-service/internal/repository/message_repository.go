package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *MessageRepository { return &MessageRepository{db: db} }

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	n := 0
	for i := range s {
		if n >= max {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

func scanReplyKey(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	s := ns.String
	return &s
}

// MessageInChat reports whether a message id belongs to the given chat.
func (r *MessageRepository) MessageInChat(ctx context.Context, messageID, chatID string) (bool, error) {
	var one int
	err := r.db.QueryRow(ctx, `SELECT 1 FROM messages WHERE id=$1::uuid AND chat_id=$2::uuid`, messageID, chatID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ChatIDByMessageID returns the chat that owns the message.
func (r *MessageRepository) ChatIDByMessageID(ctx context.Context, messageID string) (string, error) {
	var cid string
	err := r.db.QueryRow(ctx, `SELECT chat_id::text FROM messages WHERE id=$1::uuid`, messageID).Scan(&cid)
	return cid, err
}

// GetReplyPreview loads a short quote for the parent message (same chat only).
func (r *MessageRepository) GetReplyPreview(ctx context.Context, parentID, chatID string) (*model.ReplyPreview, error) {
	var p model.ReplyPreview
	var content string
	var msgType string
	err := r.db.QueryRow(ctx, `
		SELECT id::text, sender_id::text, content, created_at, message_type
		FROM messages WHERE id=$1::uuid AND chat_id=$2::uuid`,
		parentID, chatID).Scan(&p.ID, &p.SenderID, &content, &p.CreatedAt, &msgType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if msgType == "file" {
		var meta struct {
			Filename string `json:"filename"`
		}
		if json.Unmarshal([]byte(content), &meta) == nil && strings.TrimSpace(meta.Filename) != "" {
			p.Content = truncateRunes("📎 "+meta.Filename, 280)
		} else {
			p.Content = "📎 File"
		}
	} else {
		p.Content = truncateRunes(content, 280)
	}
	return &p, nil
}

// Create inserts a message. When idempotency_key is set, a conflict returns the existing row
// and inserted=false so callers avoid duplicate realtime fan-out.
func (r *MessageRepository) Create(ctx context.Context, chatID, senderID, content, idemKey string, replyTo *string, msgType string) (*model.Message, bool, error) {
	if msgType == "" {
		msgType = "text"
	}
	id := uuid.NewString()
	var idem any
	if idemKey == "" {
		idem = nil
	} else {
		idem = idemKey
	}
	var replyAny any
	if replyTo != nil && *replyTo != "" {
		replyAny = *replyTo
	}
	var m model.Message
	var replyNS sql.NullString

	if idemKey != "" {
		err := r.db.QueryRow(ctx, `
			INSERT INTO messages (id, chat_id, sender_id, content, idempotency_key, reply_to_message_id, message_type)
			VALUES ($1,$2,$3,$4,$5,$6::uuid,$7)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id, chat_id, sender_id, message_type, content, created_at, reply_to_message_id::text`,
			id, chatID, senderID, content, idem, replyAny, msgType).
			Scan(&m.ID, &m.ChatID, &m.SenderID, &m.MessageType, &m.Content, &m.CreatedAt, &replyNS)
		if err == nil {
			m.ReplyToMessageID = scanReplyKey(replyNS)
			return &m, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, err
		}
		err = r.db.QueryRow(ctx, `
			SELECT id, chat_id, sender_id, message_type, content, created_at, reply_to_message_id::text
			FROM messages WHERE idempotency_key=$1`,
			idemKey).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.MessageType, &m.Content, &m.CreatedAt, &replyNS)
		if err != nil {
			return nil, false, err
		}
		m.ReplyToMessageID = scanReplyKey(replyNS)
		return &m, false, nil
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO messages (id, chat_id, sender_id, content, idempotency_key, reply_to_message_id, message_type)
		VALUES ($1,$2,$3,$4,$5,$6::uuid,$7)
		RETURNING id, chat_id, sender_id, message_type, content, created_at, reply_to_message_id::text`,
		id, chatID, senderID, content, idem, replyAny, msgType).
		Scan(&m.ID, &m.ChatID, &m.SenderID, &m.MessageType, &m.Content, &m.CreatedAt, &replyNS)
	if err != nil {
		return nil, false, err
	}
	m.ReplyToMessageID = scanReplyKey(replyNS)
	return &m, true, nil
}

func (r *MessageRepository) History(ctx context.Context, chatID string, limit, offset int) ([]model.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			m.id, m.chat_id, m.sender_id, m.message_type, m.content, m.created_at, m.reply_to_message_id::text,
			p.id::text, p.sender_id::text, p.content, p.created_at, p.message_type
		FROM messages m
		LEFT JOIN messages p ON p.id = m.reply_to_message_id
		WHERE m.chat_id=$1
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Message, 0, limit)
	for rows.Next() {
		var m model.Message
		var replyNS sql.NullString
		var pid, psender, pcontent sql.NullString
		var pcreated sql.NullTime
		var pmsgType sql.NullString
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.SenderID, &m.MessageType, &m.Content, &m.CreatedAt, &replyNS,
			&pid, &psender, &pcontent, &pcreated, &pmsgType,
		); err != nil {
			return nil, err
		}
		m.ReplyToMessageID = scanReplyKey(replyNS)
		if pid.Valid && psender.Valid && pcontent.Valid && pcreated.Valid {
			quote := truncateRunes(pcontent.String, 280)
			if pmsgType.Valid && pmsgType.String == "file" {
				var meta struct {
					Filename string `json:"filename"`
				}
				if json.Unmarshal([]byte(pcontent.String), &meta) == nil && strings.TrimSpace(meta.Filename) != "" {
					quote = truncateRunes("📎 "+meta.Filename, 280)
				} else {
					quote = "📎 File"
				}
			}
			m.ReplyTo = &model.ReplyPreview{
				ID:        pid.String,
				SenderID:  psender.String,
				Content:   quote,
				CreatedAt: pcreated.Time,
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
