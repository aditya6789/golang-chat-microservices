package repository

import (
	"context"
	"errors"
	"time"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrReceiptNotApplied = errors.New("receipt not applied: message missing or wrong chat")

type ReceiptRepository struct{ db *pgxpool.Pool }

func NewReceiptRepository(db *pgxpool.Pool) *ReceiptRepository {
	return &ReceiptRepository{db: db}
}

func (r *ReceiptRepository) MarkRead(ctx context.Context, messageID, userID, chatID string) error {
	var msgChat string
	err := r.db.QueryRow(ctx, `SELECT chat_id::text FROM messages WHERE id=$1`, messageID).Scan(&msgChat)
	if err != nil || msgChat != chatID {
		return ErrReceiptNotApplied
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO message_receipts (message_id, user_id, read_at) VALUES ($1::uuid, $2::uuid, NOW())
		ON CONFLICT (message_id, user_id) DO UPDATE SET read_at = EXCLUDED.read_at`,
		messageID, userID)
	return err
}

// ListByMessageIDs returns read receipts grouped by message id.
func (r *ReceiptRepository) ListByMessageIDs(ctx context.Context, messageIDs []string) (map[string][]model.ReadReceipt, error) {
	out := make(map[string][]model.ReadReceipt)
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT message_id::text, user_id::text, read_at
		FROM message_receipts
		WHERE message_id::text = ANY($1::text[])`, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid, uid string
		var readAt *time.Time
		if err := rows.Scan(&mid, &uid, &readAt); err != nil {
			return nil, err
		}
		rr := model.ReadReceipt{UserID: uid}
		if readAt != nil {
			rr.ReadAt = *readAt
		}
		out[mid] = append(out[mid], rr)
	}
	return out, rows.Err()
}
