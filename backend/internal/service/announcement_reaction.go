package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AnnouncementReaction struct {
	EmojiID int16 `json:"emoji_id"`
	Count   int64 `json:"count"`
	Reacted bool  `json:"reacted"`
}

type AnnouncementReactionRepository interface {
	List(context.Context, int64, int64) ([]AnnouncementReaction, error)
	Set(context.Context, int64, int64, int16) error
}

func (s *AnnouncementService) ListReactions(ctx context.Context, userID, announcementID int64) ([]AnnouncementReaction, error) {
	if err := s.checkVisibleToUser(ctx, userID, announcementID); err != nil {
		return nil, err
	}
	return s.reactionRepo.List(ctx, announcementID, userID)
}

func (s *AnnouncementService) SetReaction(ctx context.Context, userID, announcementID int64, emojiID int16) ([]AnnouncementReaction, error) {
	if emojiID < 0 || emojiID > 30 {
		return nil, infraerrors.BadRequest("ANNOUNCEMENT_REACTION_INVALID", "Unsupported announcement reaction")
	}
	if err := s.checkVisibleToUser(ctx, userID, announcementID); err != nil {
		return nil, err
	}
	if err := s.reactionRepo.Set(ctx, announcementID, userID, emojiID); err != nil {
		return nil, err
	}
	kind := "announcement.reacted"
	if emojiID == 0 {
		kind = "announcement.reaction_removed"
	}
	publishGameplay(ctx, kind, userID, "", "source_id", announcementID, "quantity", emojiID)
	return s.reactionRepo.List(ctx, announcementID, userID)
}
