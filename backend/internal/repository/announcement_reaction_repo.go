package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type announcementReactionRepository struct{ db *sql.DB }

func NewAnnouncementReactionRepository(db *sql.DB) service.AnnouncementReactionRepository {
	return &announcementReactionRepository{db: db}
}

func (r *announcementReactionRepository) List(ctx context.Context, announcementID, userID int64) ([]service.AnnouncementReaction, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.emoji_id, c.reaction_count, EXISTS (
 SELECT 1 FROM announcement_reactions r WHERE r.announcement_id=c.announcement_id AND r.user_id=$2 AND r.emoji_id=c.emoji_id)
 FROM announcement_reaction_counts c WHERE c.announcement_id=$1
 ORDER BY c.reaction_count DESC,c.emoji_id`, announcementID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.AnnouncementReaction, 0)
	for rows.Next() {
		var item service.AnnouncementReaction
		if err := rows.Scan(&item.EmojiID, &item.Count, &item.Reacted); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *announcementReactionRepository) Set(ctx context.Context, announcementID, userID int64, emojiID int16) error {
	if emojiID == 0 {
		_, err := r.db.ExecContext(ctx, `DELETE FROM announcement_reactions WHERE announcement_id=$1 AND user_id=$2`, announcementID, userID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO announcement_reactions(announcement_id,user_id,emoji_id) VALUES($1,$2,$3)
 ON CONFLICT(announcement_id,user_id) DO UPDATE SET emoji_id=EXCLUDED.emoji_id`, announcementID, userID, emojiID)
	return err
}
