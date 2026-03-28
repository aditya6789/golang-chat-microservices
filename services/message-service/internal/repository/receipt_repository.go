package repository

import (
	"context"
	"errors"

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
