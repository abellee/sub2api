package repository

import (
	"context"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *groupRepository) UpdateWithRateSync(ctx context.Context, source *service.Group, targetIDs []int64) error {
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	if tx != nil {
		defer func() { _ = tx.Rollback() }()
		client = tx.Client()
	}
	if err := updateGroupRecord(ctx, client, source); err != nil {
		return err
	}
	// Updating only the rate avoids copying target settings or cascading their links.
	count, err := client.Group.Update().Where(group.IDIn(targetIDs...), group.DeletedAtIsNil()).
		SetRateMultiplier(source.RateMultiplier).Save(ctx)
	if err != nil {
		return err
	}
	if count != len(targetIDs) {
		return service.ErrGroupNotFound
	}
	for _, id := range append([]int64{source.ID}, targetIDs...) {
		if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &id, nil); err != nil {
			return err
		}
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}
