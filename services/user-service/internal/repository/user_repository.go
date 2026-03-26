package repository

import (
	"context"

	"realtime-chat-system/services/user-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	var out model.UserProfile
	err := r.db.QueryRow(ctx, `SELECT id,email,username FROM users WHERE id=$1`, userID).Scan(&out.ID, &out.Email, &out.Username)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

