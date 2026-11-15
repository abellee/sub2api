package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const accountInputRoutingSetting = "account_matching_input_routing"

type AccountInputRoutingSettings struct {
	Enabled    bool    `json:"enabled"`
	Threshold  int     `json:"threshold"`
	GroupIDs   []int64 `json:"group_ids"`
	AccountIDs []int64 `json:"account_ids"`
	Revision   string  `json:"revision"`
}

type AccountInputRoutingStore interface {
	Restricted(context.Context, string, int64, int64) (bool, error)
	Observe(context.Context, string, int64, int64, int64, string, int, int) error
}

type cachedAccountInputRouting struct {
	settings AccountInputRoutingSettings
	expires  time.Time
}

type AccountInputRoutingService struct {
	settings SettingRepository
	store    AccountInputRoutingStore
	groups   GroupRepository
	accounts AccountRepository
	cache    atomic.Pointer[cachedAccountInputRouting]
	mu       sync.Mutex // Serialize infrequent settings refreshes and admin saves.
}

func NewAccountInputRoutingService(settings SettingRepository, store AccountInputRoutingStore, groups GroupRepository, accounts AccountRepository) *AccountInputRoutingService {
	return &AccountInputRoutingService{settings: settings, store: store, groups: groups, accounts: accounts}
}

func (s *AccountInputRoutingService) Settings(ctx context.Context) (AccountInputRoutingSettings, error) {
	if cached := s.cache.Load(); cached != nil && time.Now().Before(cached.expires) {
		return cloneAccountInputRoutingSettings(cached.settings), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached := s.cache.Load(); cached != nil && time.Now().Before(cached.expires) {
		return cloneAccountInputRoutingSettings(cached.settings), nil
	}
	cfg := AccountInputRoutingSettings{Threshold: 100000, GroupIDs: []int64{}, AccountIDs: []int64{}}
	raw, err := s.settings.GetValue(ctx, accountInputRoutingSetting)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return cfg, err
	}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return cfg, err
		}
		if cfg.Threshold <= 0 || (cfg.Enabled && (len(cfg.GroupIDs) == 0 || len(cfg.AccountIDs) != 2 || cfg.Revision == "")) {
			return cfg, fmt.Errorf("invalid input routing settings")
		}
	}
	s.cache.Store(&cachedAccountInputRouting{settings: cfg, expires: time.Now().Add(5 * time.Second)})
	return cloneAccountInputRoutingSettings(cfg), nil
}

func cloneAccountInputRoutingSettings(cfg AccountInputRoutingSettings) AccountInputRoutingSettings {
	cfg.GroupIDs = slices.Clone(cfg.GroupIDs)
	cfg.AccountIDs = slices.Clone(cfg.AccountIDs)
	return cfg
}

var ErrAccountInputRoutingInvalid = infraerrors.BadRequest("ACCOUNT_INPUT_ROUTING_INVALID", "请填写有效的普通输入阈值、分组和两个不同的账号")

func (s *AccountInputRoutingService) SaveSettings(ctx context.Context, cfg AccountInputRoutingSettings) (AccountInputRoutingSettings, error) {
	if cfg.Threshold <= 0 || cfg.Threshold > 100000000 || len(cfg.GroupIDs) > 10000 || len(cfg.AccountIDs) > 2 {
		return cfg, ErrAccountInputRoutingInvalid
	}
	if cfg.Enabled && (len(cfg.GroupIDs) == 0 || len(cfg.AccountIDs) != 2) {
		return cfg, ErrAccountInputRoutingInvalid
	}
	for _, ids := range [][]int64{cfg.GroupIDs, cfg.AccountIDs} {
		seen := map[int64]bool{}
		for _, id := range ids {
			if id <= 0 || seen[id] {
				return cfg, ErrAccountInputRoutingInvalid
			}
			seen[id] = true
		}
	}
	// Turning the feature off must remain possible after a selected resource is deleted.
	if cfg.Enabled {
		for _, id := range cfg.GroupIDs {
			group, err := s.groups.GetByID(ctx, id)
			if err != nil {
				return cfg, err
			}
			if group == nil {
				return cfg, ErrAccountInputRoutingInvalid
			}
		}
		accounts, err := s.accounts.GetByIDs(ctx, cfg.AccountIDs)
		if err != nil {
			return cfg, err
		}
		if len(accounts) != 2 || accounts[0] == nil || accounts[1] == nil {
			return cfg, ErrAccountInputRoutingInvalid
		}
	}
	cfg.Revision = uuid.NewString()
	raw, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.settings.Set(ctx, accountInputRoutingSetting, string(raw)); err != nil {
		return cfg, err
	}
	s.cache.Store(&cachedAccountInputRouting{settings: cloneAccountInputRoutingSettings(cfg), expires: time.Now().Add(5 * time.Second)})
	return cfg, nil
}

type accountInputRoutingKey struct{}
type accountInputRoutingInputKey struct{}
type accountInputRoutingObservation struct {
	service         *AccountInputRoutingService
	settings        AccountInputRoutingSettings
	userID, groupID int64
}

func CopyAccountInputRoutingContext(parent, target context.Context) context.Context {
	if observation, ok := parent.Value(accountInputRoutingKey{}).(*accountInputRoutingObservation); ok && observation != nil {
		return context.WithValue(target, accountInputRoutingKey{}, observation)
	}
	return target
}

func (s *AccountInputRoutingService) apply(ctx context.Context, userID, groupID int64, manual []int64) (context.Context, error) {
	cfg, err := s.Settings(ctx)
	if err != nil {
		if manual != nil {
			return withAccountMatching(ctx, manual), nil
		}
		return ctx, err
	}
	if !cfg.Enabled || !slices.Contains(cfg.GroupIDs, groupID) {
		return withAccountMatching(ctx, manual), nil
	}
	ctx = context.WithValue(ctx, accountInputRoutingKey{}, &accountInputRoutingObservation{s, cfg, userID, groupID})
	if manual != nil {
		return withAccountMatching(ctx, manual), nil
	}
	if s.store == nil {
		return ctx, fmt.Errorf("input routing store unavailable")
	}
	restricted, err := s.store.Restricted(ctx, cfg.Revision, userID, groupID)
	if err != nil {
		return ctx, err
	}
	if restricted {
		return withAccountMatching(ctx, cfg.AccountIDs), nil
	}
	return ctx, nil
}

func observeAccountInputRouting(ctx context.Context, usage *UsageLog) {
	observation, _ := ctx.Value(accountInputRoutingKey{}).(*accountInputRoutingObservation)
	if observation == nil || usage == nil || usage.UserID != observation.userID || usage.RequestID == "" || usage.APIKeyID <= 0 ||
		usage.InputTokens < 0 || usage.ImageCount > 0 || usage.VideoCount > 0 ||
		(usage.BillingMode != nil && *usage.BillingMode != string(BillingModeToken)) ||
		usage.InputTokens+usage.OutputTokens+usage.CacheReadTokens+usage.CacheCreationTokens == 0 {
		return
	}
	// Completed upstream usage is already normalized: InputTokens excludes caches.
	s := observation.service
	current, err := s.Settings(ctx)
	if err == nil && (!current.Enabled || current.Revision != observation.settings.Revision) {
		return
	}
	if err == nil && s.store != nil {
		input := usage.InputTokens
		if original, ok := ctx.Value(accountInputRoutingInputKey{}).(int); ok {
			input = original
		}
		err = s.store.Observe(ctx, current.Revision, observation.userID, observation.groupID, usage.APIKeyID, usage.RequestID, input, current.Threshold)
	}
	if err != nil {
		logger.LegacyPrintf("service.account_input_routing", "Update input routing count failed: %v", err)
	}
}
