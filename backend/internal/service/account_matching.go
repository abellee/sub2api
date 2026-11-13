package service

import (
	"context"
	"sort"
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

// Matching rules define a request-local account pool, independent of normal group membership.
// Never publish this pool into shared group snapshots.
func listAccountMatchingAccounts(ctx context.Context, repo AccountRepository, platform string, mixed bool) ([]Account, bool, error) {
	allowed, restricted := ctx.Value(accountMatchingContextKey{}).(map[int64]struct{})
	if !restricted {
		return nil, false, nil
	}
	accounts := make([]Account, 0, len(allowed))
	if len(allowed) == 0 {
		return accounts, true, nil
	}
	if repo == nil {
		return nil, true, ErrNoAvailableAccounts
	}
	ids := make([]int64, 0, len(allowed))
	for id := range allowed {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	loaded, err := repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, true, err
	}
	for _, account := range loaded {
		if account == nil || !AccountMatchingAllows(ctx, account.ID) || !account.IsSchedulable() {
			continue
		}
		if account.Platform != platform && !(mixed && account.Platform == PlatformAntigravity && account.IsMixedSchedulingEnabled()) {
			continue
		}
		accounts = append(accounts, *account)
	}
	return accounts, true, nil
}

func accountMatchingOverridesMembership(ctx context.Context, account *Account) (bool, bool) {
	allowed, restricted := ctx.Value(accountMatchingContextKey{}).(map[int64]struct{})
	if !restricted || account == nil {
		return false, restricted
	}
	_, ok := allowed[account.ID]
	return ok, true
}
