package repository

import (
	"context"
	"errors"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *MessageRepository { return &MessageRepository{db: db} }

// Create inserts a message. When idempotency_key is set, a conflict returns the existing row
// and inserted=false so callers avoid duplicate realtime fan-out.
func (r *MessageRepository) Create(ctx context.Context, chatID, senderID, content, idemKey string) (*model.Message, bool, error) {
	id := uuid.NewString()
	var idem any
	if idemKey == "" {
		idem = nil
	} else {
		idem = idemKey
	}
	var m model.Message
	if idemKey != "" {
		err := r.db.QueryRow(ctx, `
			INSERT INTO messages (id, chat_id, sender_id, content, idempotency_key)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id, chat_id, sender_id, content, created_at`,
			id, chatID, senderID, content, idem).
			Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.CreatedAt)
		if err == nil {
			return &m, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, err
		}
		err = r.db.QueryRow(ctx, `
			SELECT id, chat_id, sender_id, content, created_at FROM messages WHERE idempotency_key=$1`,
			idemKey).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.CreatedAt)
		if err != nil {
			return nil, false, err
		}
		return &m, false, nil
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO messages (id, chat_id, sender_id, content, idempotency_key)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, chat_id, sender_id, content, created_at`,
		id, chatID, senderID, content, idem).
		Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.CreatedAt)
	if err != nil {
		return nil, false, err
	}
	return &m, true, nil
}

func (r *MessageRepository) History(ctx context.Context, chatID string, limit, offset int) ([]model.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, chat_id, sender_id, content, created_at
		FROM messages WHERE chat_id=$1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Message, 0, limit)
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

