package repository

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type accountInputRoutingStore struct{ client *redis.Client }

func NewAccountInputRoutingStore(client *redis.Client) service.AccountInputRoutingStore {
	return &accountInputRoutingStore{client: client}
}

func inputRoutingStateKey(revision string, userID, groupID int64) string {
	// Isolate revisions so delayed requests on another replica cannot reset new counters.
	return fmt.Sprintf("account_input_routing:{%d:%d}:%s:state", userID, groupID, revision)
}

func (s *accountInputRoutingStore) Restricted(ctx context.Context, revision string, userID, groupID int64) (bool, error) {
	values, err := s.client.HMGet(ctx, inputRoutingStateKey(revision, userID, groupID), "revision", "restricted").Result()
	if err != nil {
		return false, err
	}
	return values[0] == revision && values[1] == "1", nil
}

var inputRoutingObserveScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
local high, low, restricted = 0, 0, 0
if redis.call('HGET', KEYS[1], 'revision') == ARGV[1] then
  high = tonumber(redis.call('HGET', KEYS[1], 'high') or '0')
  low = tonumber(redis.call('HGET', KEYS[1], 'low') or '0')
  restricted = tonumber(redis.call('HGET', KEYS[1], 'restricted') or '0')
end
local input, threshold = tonumber(ARGV[2]), tonumber(ARGV[3])
if restricted == 0 then
  if input > threshold then
    high = math.min(3, high + 1)
    if high == 3 then restricted = 1; low = 0 end
  end
else
  if input < threshold then
    low = low + 1
    if low == 3 then high = 0; low = 0; restricted = 0 end
  else
    low = 0
  end
end
redis.call('HSET', KEYS[1], 'revision', ARGV[1], 'high', high, 'low', low, 'restricted', restricted)
redis.call('SET', KEYS[2], '1', 'EX', 604800)
return restricted
`)

func (s *accountInputRoutingStore) Observe(ctx context.Context, revision string, userID, groupID, keyID int64, requestID string, input, threshold int) error {
	// Scope duplicate suppression by key, request and configuration revision.
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", revision, keyID, requestID)))
	state := inputRoutingStateKey(revision, userID, groupID)
	_, err := inputRoutingObserveScript.Run(ctx, s.client, []string{state, fmt.Sprintf("%s:seen:%x", state, hash)}, revision, input, threshold).Result()
	return err
}
