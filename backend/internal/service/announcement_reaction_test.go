package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type reactionUserRepo struct{ UserRepository }

func (reactionUserRepo) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 2, Balance: 10}, nil
}

type reactionSubRepo struct{ UserSubscriptionRepository }

func (reactionSubRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

type reactionRepoStub struct {
	emoji  int16
	writes int
}

func (r *reactionRepoStub) Set(_ context.Context, announcementID, userID int64, emoji int16) error {
	r.emoji = emoji
	r.writes++
	return nil
}
func (r *reactionRepoStub) List(context.Context, int64, int64) ([]AnnouncementReaction, error) {
	if r.emoji == 0 {
		return []AnnouncementReaction{}, nil
	}
	return []AnnouncementReaction{{EmojiID: r.emoji, Count: 1, Reacted: true}}, nil
}

func TestAnnouncementReactionsVisibilityAndReplacement(t *testing.T) {
	ctx := context.Background()
	ann := &Announcement{ID: 1, Status: AnnouncementStatusActive}
	reactions := &reactionRepoStub{}
	svc := NewAnnouncementService(&announcementRepoStub{item: ann}, nil, reactionUserRepo{}, reactionSubRepo{}, reactions)
	for _, emoji := range []int16{1, 2, 0} {
		items, err := svc.SetReaction(ctx, 2, 1, emoji)
		require.NoError(t, err)
		if emoji == 0 {
			require.Empty(t, items)
		} else {
			require.Equal(t, emoji, items[0].EmojiID)
			require.EqualValues(t, 1, items[0].Count)
		}
	}
	for _, id := range []int16{-1, 31} {
		_, err := svc.SetReaction(ctx, 2, 1, id)
		require.Error(t, err)
	}
	require.Equal(t, 3, reactions.writes)
	for _, change := range []func(){
		func() { ann.Status = AnnouncementStatusDraft },
		func() { ann.Status = AnnouncementStatusActive; past := time.Now().Add(-time.Hour); ann.EndsAt = &past },
		func() {
			ann.EndsAt = nil
			ann.Targeting = AnnouncementTargeting{AnyOf: []AnnouncementConditionGroup{{AllOf: []AnnouncementCondition{{Type: AnnouncementConditionTypeBalance, Operator: AnnouncementOperatorGT, Value: 100}}}}}
		},
	} {
		change()
		_, err := svc.SetReaction(ctx, 2, 1, 2)
		require.ErrorIs(t, err, ErrAnnouncementNotFound)
		_, err = svc.ListReactions(ctx, 2, 1)
		require.ErrorIs(t, err, ErrAnnouncementNotFound)
		require.Equal(t, 3, reactions.writes)
	}
}

func TestAnnouncementReactionIDs(t *testing.T) {
	reactions := &reactionRepoStub{}
	svc := NewAnnouncementService(&announcementRepoStub{item: &Announcement{ID: 1, Status: AnnouncementStatusActive}}, nil, reactionUserRepo{}, reactionSubRepo{}, reactions)
	for id := int16(1); id <= 30; id++ {
		items, err := svc.SetReaction(context.Background(), 2, 1, id)
		require.NoError(t, err)
		require.Equal(t, id, items[0].EmojiID)
	}
}
