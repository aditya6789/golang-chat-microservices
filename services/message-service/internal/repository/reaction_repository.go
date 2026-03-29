package repository

import (
	"context"
	"strings"

	"realtime-chat-system/services/message-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReactionRepository struct{ db *pgxpool.Pool }

func NewReactionRepository(db *pgxpool.Pool) *ReactionRepository {
	return &ReactionRepository{db: db}
}

// Add inserts a reaction row. inserted is false if the user already had that emoji on the message.
func (r *ReactionRepository) Add(ctx context.Context, messageID, userID, emoji string) (inserted bool, err error) {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO message_reactions (message_id, user_id, emoji)
		VALUES ($1::uuid, $2::uuid, $3)
		ON CONFLICT (message_id, user_id, emoji) DO NOTHING`,
		messageID, userID, emoji)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Remove deletes a reaction row. removed is false if no row matched.
func (r *ReactionRepository) Remove(ctx context.Context, messageID, userID, emoji string) (removed bool, err error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM message_reactions
		WHERE message_id=$1::uuid AND user_id=$2::uuid AND emoji=$3`,
		messageID, userID, emoji)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListAggregated returns reaction groups keyed by message id.
func (r *ReactionRepository) ListAggregated(ctx context.Context, messageIDs []string) (map[string][]model.ReactionAgg, error) {
	out := make(map[string][]model.ReactionAgg)
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT message_id::text, emoji,
			string_agg(user_id::text, ',' ORDER BY created_at) AS user_csv
		FROM message_reactions
		WHERE message_id::text = ANY($1::text[])
		GROUP BY message_id, emoji`,
		messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid, emoji, csv string
		if err := rows.Scan(&mid, &emoji, &csv); err != nil {
			return nil, err
		}
		var uids []string
		if csv != "" {
			for _, p := range strings.Split(csv, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					uids = append(uids, p)
				}
			}
		}
		out[mid] = append(out[mid], model.ReactionAgg{Emoji: emoji, UserIDs: uids})
	}
	return out, rows.Err()
}
