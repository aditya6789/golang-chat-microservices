package repository

import (
	"context"

	"realtime-chat-system/services/auth-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, email, username, passwordHash string) (*model.User, error) {
	id := uuid.NewString()
	q := `INSERT INTO users (id, email, username, password_hash) VALUES ($1,$2,$3,$4) RETURNING created_at`
	var createdAt model.User
	if err := r.db.QueryRow(ctx, q, id, email, username, passwordHash).Scan(&createdAt.CreatedAt); err != nil {
		return nil, err
	}
	return &model.User{ID: id, Email: email, Username: username, PasswordHash: passwordHash, CreatedAt: createdAt.CreatedAt}, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	q := `SELECT id,email,username,password_hash,created_at FROM users WHERE email=$1`
	var u model.User
	if err := r.db.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

