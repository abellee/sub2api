//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupUsageTiersTokensLinksExclusionsAndRetention(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	source := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("tiers-source-%d", suffix), RateMultiplier: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive})
	linked := mustCreateGroup(t, integrationEntClient, &service.Group{Name: fmt.Sprintf("tiers-linked-%d", suffix), RateMultiplier: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive})
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("tiers-%d@example.com", suffix)})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("tiers-%d", suffix)})
	key := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: user.ID, Key: fmt.Sprintf("sk-tiers-%d", suffix), Name: "one"})
	key2 := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: user.ID, Key: fmt.Sprintf("sk-tiers-second-%d", suffix), Name: "two"})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE user_id=$1`, user.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE user_id=$1`, user.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, account.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2)`, source.ID, linked.ID)
	})
	repo := NewUserGroupRateRepository(integrationDB).(service.GroupUsageTierRepository)
	initialRate, err := repo.GetUsageTierRate(ctx, user.ID, linked.ID)
	require.NoError(t, err)
	require.Nil(t, initialRate)
	config := service.GroupUsageTierConfig{Enabled: true, Tiers: []service.UsageTier{{DailyTokens: 10, ConsecutiveDays: 1, MaxMissedDays: 2, RateMultiplier: 0.8}, {DailyTokens: 20, ConsecutiveDays: 1, MaxMissedDays: 2, RateMultiplier: 0.5}}, LinkedGroupIDs: []int64{linked.ID}}
	require.NoError(t, repo.SaveUsageTierConfig(ctx, source.ID, config))
	now := time.Now().In(service.UsageTierLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, service.UsageTierLocation)
	yesterday := today.AddDate(0, 0, -1)
	_, err = integrationDB.ExecContext(ctx, `UPDATE group_usage_tier_rules SET started_at=$2 WHERE group_id=$1`, source.ID, yesterday)
	require.NoError(t, err)
	insert := func(groupID, keyID int64, tokens int64, at time.Time) {
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,group_id,request_id,model,total_cost,actual_cost,input_tokens,created_at)
  VALUES($1,$2,$3,$4,$5,'test-model',1000,999,$6,$7)`, user.ID, keyID, account.ID, groupID, fmt.Sprintf("tiers-%d-%d", suffix, time.Now().UnixNano()), tokens, at)
		require.NoError(t, err)
	}
	insert(linked.ID, key.ID, 100, now)
	progress, err := repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID, linked.ID})
	require.NoError(t, err)
	require.Zero(t, progress[source.ID][0].Stage)
	require.Zero(t, progress[source.ID][0].TodayTokens)
	// Two keys share one token target; charges and linked-group usage do not count.
	insert(source.ID, key.ID, 6, yesterday.Add(time.Hour))
	insert(source.ID, key2.ID, 4, yesterday.Add(2*time.Hour))
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID, linked.ID})
	require.NoError(t, err)
	require.Equal(t, 1, progress[source.ID][0].Stage)
	require.Equal(t, progress[source.ID], progress[linked.ID])
	firstRate, err := repo.GetUsageTierRate(ctx, user.ID, linked.ID)
	require.NoError(t, err)
	require.NotNil(t, firstRate)
	require.Equal(t, 0.8, *firstRate)
	insert(source.ID, key.ID, 20, now)
	// All four token counters count once; later token corrections apply a delta.
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET input_tokens=1,output_tokens=1,cache_creation_tokens=1,cache_read_tokens=2 WHERE user_id=$1 AND group_id=$2 AND created_at=$3`, user.ID, source.ID, now)
	require.NoError(t, err)
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{linked.ID})
	require.NoError(t, err)
	require.Equal(t, 1, progress[linked.ID][0].Stage)
	require.EqualValues(t, 5, progress[linked.ID][0].TodayTokens)
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET input_tokens=16 WHERE user_id=$1 AND group_id=$2 AND created_at=$3`, user.ID, source.ID, now)
	require.NoError(t, err)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := repo.GetUsageTierProgress(ctx, user.ID, []int64{linked.ID})
			if err != nil {
				t.Error(err)
				return
			}
			if p[linked.ID][0].Stage != 2 {
				t.Errorf("stage=%d", p[linked.ID][0].Stage)
			}
		}()
	}
	wg.Wait()
	rate, err := repo.GetUsageTierRate(ctx, user.ID, linked.ID)
	require.NoError(t, err)
	require.NotNil(t, rate)
	require.Equal(t, 0.5, *rate)
	// Raw-log retention must not revoke earned tiers or erase today's progress.
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE user_id=$1`, user.ID)
	require.NoError(t, err)
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{linked.ID})
	require.NoError(t, err)
	require.Equal(t, 2, progress[linked.ID][0].Stage)
	require.EqualValues(t, 20, progress[linked.ID][0].TodayTokens)
	// An unchanged save preserves revision and start time.
	require.NoError(t, repo.SaveUsageTierConfig(ctx, source.ID, config))
	var revision int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT revision FROM group_usage_tier_rules WHERE group_id=$1`, source.ID).Scan(&revision))
	require.Equal(t, 1, revision)
	config.ExcludedUserIDs = []int64{user.ID}
	require.NoError(t, repo.SaveUsageTierConfig(ctx, source.ID, config))
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{linked.ID})
	require.NoError(t, err)
	require.True(t, progress[linked.ID][0].Excluded)
	require.Nil(t, progress[linked.ID][0].RateMultiplier)
	config.ExcludedUserIDs = nil
	require.NoError(t, repo.SaveUsageTierConfig(ctx, source.ID, config))
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID})
	require.NoError(t, err)
	require.Equal(t, 2, progress[source.ID][0].Stage)
	// Exclusions and links preserve progress; changing tier targets restarts it.
	config.Tiers[0].DailyTokens = 11
	require.NoError(t, repo.SaveUsageTierConfig(ctx, source.ID, config))
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID})
	require.NoError(t, err)
	require.Zero(t, progress[source.ID][0].Stage)
	require.Zero(t, progress[source.ID][0].TodayTokens)
	// Invalid references must not partially replace the saved rule.
	invalid := config
	invalid.LinkedGroupIDs = []int64{9223372036854775800}
	require.Error(t, repo.SaveUsageTierConfig(ctx, source.ID, invalid))
	saved, err := repo.GetUsageTierConfig(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, config.LinkedGroupIDs, saved.LinkedGroupIDs)
	// Changing the selected models restarts the rule; unmatched requests stay out.
	config.Models = []string{"client-model", "second-model"}
	require.NoError(t, repo.SaveUsageTierConfig(ctx, source.ID, config))
	_, err = integrationDB.ExecContext(ctx, `UPDATE group_usage_tier_rules SET started_at=$2 WHERE group_id=$1`, source.ID, yesterday)
	require.NoError(t, err)
	insert(source.ID, key.ID, 50, now)
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID})
	require.NoError(t, err)
	require.Zero(t, progress[source.ID][0].TodayTokens)
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET requested_model='client-model',model='upstream-model' WHERE user_id=$1 AND group_id=$2`, user.ID, source.ID)
	require.NoError(t, err)
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID})
	require.NoError(t, err)
	require.EqualValues(t, 50, progress[source.ID][0].TodayTokens)
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET requested_model='other-model' WHERE user_id=$1 AND group_id=$2`, user.ID, source.ID)
	require.NoError(t, err)
	progress, err = repo.GetUsageTierProgress(ctx, user.ID, []int64{source.ID})
	require.NoError(t, err)
	require.Zero(t, progress[source.ID][0].TodayTokens)
}
