//go:build unit

package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAccountMatchingGatewayRoutingStickyAndFailover(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		f := newLoadAwareRestrictionFixture(t, false, map[string]int64{"sticky": 1}, map[string][]int64{"claude-fable-5-1": {1, 2}})
		f.svc.cfg.Gateway.Scheduling.LoadBatchEnabled = loadAware
		ctx := withAccountMatching(f.ctx, []int64{2})
		selection, err := f.svc.SelectAccountWithLoadAwareness(ctx, &f.groupID, "sticky", "claude-fable-5-1", nil, "", 2)
		require.NoError(t, err)
		require.Equal(t, int64(2), selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		selection, err = f.svc.SelectAccountWithLoadAwareness(ctx, &f.groupID, "sticky", "claude-fable-5-1", map[int64]struct{}{2: {}}, "", 2)
		require.Error(t, err)
		require.Nil(t, selection)
	}
}
