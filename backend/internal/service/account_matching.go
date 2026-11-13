package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AccountMatchingRule struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	GroupIDs   []int64   `json:"group_ids"`
	AccountIDs []int64   `json:"account_ids"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AccountMatchingUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type AccountMatchingRepository interface {
	List(context.Context) ([]AccountMatchingRule, error)
	Save(context.Context, *AccountMatchingRule) error
	Delete(context.Context, int64) error
	AllowedAccounts(context.Context, int64, int64) ([]int64, error)
	SearchUsers(context.Context, string) ([]AccountMatchingUser, error)
}

var ErrAccountMatchingInvalid = infraerrors.BadRequest("ACCOUNT_MATCHING_INVALID", "请选择有效用户、分组和账号")
var ErrAccountMatchingConflict = infraerrors.Conflict("ACCOUNT_MATCHING_CONFLICT", "该用户的指定分组已有匹配规则，请编辑原规则")
var ErrAccountMatchingNotFound = infraerrors.NotFound("ACCOUNT_MATCHING_NOT_FOUND", "账号匹配规则不存在")

// A request keeps its original group's restriction through fallback and retries.
// nil means no rule; an empty non-nil set deliberately denies every account.
type accountMatchingContextKey struct{}

func withAccountMatching(ctx context.Context, ids []int64) context.Context {
	if ids == nil {
		return ctx
	}
	allowed := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	return context.WithValue(ctx, accountMatchingContextKey{}, allowed)
}

func AccountMatchingAllows(ctx context.Context, accountID int64) bool {
	allowed, restricted := ctx.Value(accountMatchingContextKey{}).(map[int64]struct{})
	if !restricted {
		return true
	}
	_, ok := allowed[accountID]
	return ok
}

func filterAccountMatching(ctx context.Context, accounts []Account) []Account {
	if _, restricted := ctx.Value(accountMatchingContextKey{}).(map[int64]struct{}); !restricted {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if AccountMatchingAllows(ctx, account.ID) {
			filtered = append(filtered, account)
		}
	}
	return filtered
}

func (s *APIKeyService) WithAccountMatching(ctx context.Context, userID int64, groupID *int64) (context.Context, error) {
	if s.accountMatching == nil || groupID == nil {
		return ctx, nil
	}
	ids, err := s.accountMatching.AllowedAccounts(ctx, userID, *groupID)
	if err != nil {
		return ctx, err
	}
	return withAccountMatching(ctx, ids), nil
}
