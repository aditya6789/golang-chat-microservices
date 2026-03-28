package repository

import (
	"context"
	"errors"
	"fmt"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (r *ChatRepository) AreFriends(ctx context.Context, a, b string) (bool, error) {
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

func (r *ChatRepository) findDirectChatID(ctx context.Context, u1, u2 string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT c.id::text FROM chats c
		WHERE c.type = 'direct'
		AND (SELECT COUNT(*)::int FROM chat_members WHERE chat_id = c.id) = 2
		AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $1::uuid)
		AND EXISTS (SELECT 1 FROM chat_members WHERE chat_id = c.id AND user_id = $2::uuid)
		LIMIT 1`, u1, u2).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *ChatRepository) GetOrCreateDirectChat(ctx context.Context, me, other string) (chatID string, created bool, err error) {
	if me == other {
		return "", false, errors.New("cannot open direct chat with yourself")
	}
	existing, err := r.findDirectChatID(ctx, me, other)
	if err != nil {
		return "", false, err
	}
	if existing != "" {
		return existing, false, nil
	}
	id, err := r.CreateChat(ctx, "direct", nil, me, []string{other})
	if err != nil {
		return "", false, err
	}
	return id, true, nil
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
	rows, err := r.db.Query(ctx, `
		SELECT cm.user_id::text, cm.role, cm.joined_at::text, COALESCE(u.username, ''), COALESCE(u.email, '')
		FROM chat_members cm
		LEFT JOIN users u ON u.id = cm.user_id
		WHERE cm.chat_id = $1::uuid
		ORDER BY cm.joined_at`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChatMember
	for rows.Next() {
		var m model.ChatMember
		if err := rows.Scan(&m.UserID, &m.Role, &m.JoinedAt, &m.Username, &m.Email); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
