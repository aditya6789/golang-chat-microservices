package repository

import (
	"context"
	"strings"

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

func sanitizeSearch(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	if len(q) > 64 {
		q = q[:64]
	}
	return q
}

func (r *UserRepository) SearchUsers(ctx context.Context, selfID, query string, limit int) ([]model.UserProfile, error) {
	q := sanitizeSearch(query)
	if len(q) < 2 {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	pattern := "%" + q + "%"
	rows, err := r.db.Query(ctx, `
		SELECT id, email, username FROM users
		WHERE id <> $1::uuid
		AND (email ILIKE $2 OR username ILIKE $2)
		ORDER BY username ASC
		LIMIT $3`, selfID, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.UserProfile
	for rows.Next() {
		var p model.UserProfile
		if err := rows.Scan(&p.ID, &p.Email, &p.Username); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *UserRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid)`, userID).Scan(&ok)
	return ok, err
}

func (r *UserRepository) AddFriendship(ctx context.Context, a, b string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO friendships (user_1, user_2)
		SELECT LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING`, a, b)
	return err
}

// AreFriends reports whether two users have a friendship row.
func (r *UserRepository) AreFriends(ctx context.Context, a, b string) (bool, error) {
	if a == b {
		return false, nil
	}
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM friendships
			WHERE user_1 = LEAST($1::uuid, $2::uuid) AND user_2 = GREATEST($1::uuid, $2::uuid)
		)`, a, b).Scan(&ok)
	return ok, err
}

func (r *UserRepository) ListFriends(ctx context.Context, userID string) ([]model.UserProfile, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.email, u.username
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.user_1 = $1::uuid THEN f.user_2 ELSE f.user_1 END
		WHERE f.user_1 = $1::uuid OR f.user_2 = $1::uuid
		ORDER BY u.username ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.UserProfile
	for rows.Next() {
		var p model.UserProfile
		if err := rows.Scan(&p.ID, &p.Email, &p.Username); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
