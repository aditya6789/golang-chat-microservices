package repository

import (
	"context"
	"errors"
	"fmt"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepository struct{ db *pgxpool.Pool }

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateChat(ctx context.Context, typ string, name *string, creatorID string, others []string) (string, error) {
	members := map[string]struct{}{creatorID: {}}
	for _, o := range others {
		if o == "" {
			continue
		}
		members[o] = struct{}{}
	}
	if typ == "direct" && len(members) != 2 {
		return "", errors.New("direct chat requires creator and exactly one other user")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	chatID := uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO chats (id, type, name) VALUES ($1,$2,$3)`, chatID, typ, name)
	if err != nil {
		return "", err
	}
	for uid := range members {
		role := "member"
		if uid == creatorID {
			role = "owner"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1,$2,$3)`, chatID, uid, role); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return chatID, nil
}

func (r *ChatRepository) IsMember(ctx context.Context, chatID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id=$1 AND user_id=$2)`,
		chatID, userID).Scan(&exists)
	return exists, err
}

func (r *ChatRepository) ListByUser(ctx context.Context, userID string) ([]model.Chat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.type, c.name, c.created_at
		FROM chats c
		INNER JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1
		ORDER BY c.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Chat
	for rows.Next() {
		var c model.Chat
		if err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *ChatRepository) AddMember(ctx context.Context, chatID, actorID, newUserID string) error {
	ok, err := r.IsMember(ctx, chatID, actorID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("forbidden")
	}
	var typ string
	if err := r.db.QueryRow(ctx, `SELECT type FROM chats WHERE id=$1`, chatID).Scan(&typ); err != nil {
		return err
	}
	if typ == "direct" {
		return errors.New("cannot add members to a direct chat")
	}
	_, err = r.db.Exec(ctx, `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1,$2,'member') ON CONFLICT DO NOTHING`, chatID, newUserID)
	return err
}

func (r *ChatRepository) ListMembers(ctx context.Context, chatID string) ([]model.ChatMember, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id, role, joined_at::text FROM chat_members WHERE chat_id=$1 ORDER BY joined_at`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChatMember
	for rows.Next() {
		var m model.ChatMember
		if err := rows.Scan(&m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
