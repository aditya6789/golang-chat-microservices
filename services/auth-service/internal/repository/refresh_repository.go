package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshRepository struct{ db *pgxpool.Pool }

func NewRefreshRepository(db *pgxpool.Pool) *RefreshRepository {
	return &RefreshRepository{db: db}
}

func (r *RefreshRepository) Save(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	id := uuid.NewString()
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4)`, id, userID, tokenHash, expiresAt)
	return err
}

func (r *RefreshRepository) UserIDByHash(ctx context.Context, tokenHash string) (string, time.Time, error) {
	var uid string
	var exp time.Time
	err := r.db.QueryRow(ctx, `
		SELECT user_id, expires_at FROM refresh_tokens
		WHERE token_hash=$1 AND expires_at > NOW()`, tokenHash).Scan(&uid, &exp)
	return uid, exp, err
}

func (r *RefreshRepository) DeleteByHash(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, tokenHash)
	return err
}
