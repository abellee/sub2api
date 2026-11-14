package service

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *adminServiceImpl) normalizeRateSyncGroupIDs(ctx context.Context, sourceID int64, ids []int64) ([]int64, error) {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || id == sourceID {
			return nil, infraerrors.BadRequest("INVALID_RATE_SYNC_GROUP", "倍率同步分组不能是自身，且 ID 必须大于 0")
		}
		if seen[id] {
			continue
		}
		target, err := s.groupRepo.GetByIDLite(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("倍率同步分组 %d: %w", id, err)
		}
		if target == nil {
			return nil, ErrGroupNotFound
		}
		if err := s.validateSimpleModeGroupAccess(target); err != nil {
			return nil, err
		}
		seen[id] = true
		result = append(result, id)
	}
	return result, nil
}
