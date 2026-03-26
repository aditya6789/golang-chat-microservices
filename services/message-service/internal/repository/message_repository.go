package repository

import (
	"context"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *MessageRepository { return &MessageRepository{db: db} }

func (r *MessageRepository) Create(ctx context.Context, chatID, senderID, content, idemKey string) (*model.Message, error) {
	q := `
	INSERT INTO messages (id, chat_id, sender_id, content, idempotency_key)
	VALUES ($1,$2,$3,$4,$5)
	ON CONFLICT (idempotency_key) DO UPDATE SET content = EXCLUDED.content
	RETURNING id, chat_id, sender_id, content, created_at`
	id := uuid.NewString()
	var idem any
	if idemKey == "" {
		idem = nil
	} else {
		idem = idemKey
	}
	var m model.Message
	if err := r.db.QueryRow(ctx, q, id, chatID, senderID, content, idem).
		Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
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

