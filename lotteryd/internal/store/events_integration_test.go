package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"lotteryd/api/events"
	"lotteryd/internal/store"
	"lotteryd/internal/store/storetest"
)

const gameplayTestUserID int64 = 2

func TestGameplayDerivedCountersAndClockUseReceivedFacts(t *testing.T) {
	st := storetest.Open(t)
	rule := gameplayRule("累计消费", 3)
	rule.EventType = "usage.cost_updated"
	rule.Conditions = []store.EventCondition{{Field: "total_amount", Operator: "gte", Value: "0.30"}}
	saveGameplayRule(t, st, rule)
	for i := 0; i < 2; i++ {
		e := gameplayEvent(fmt.Sprintf("billing-%d", i))
		e.Type = "billing.completed"
		e.Data = map[string]string{"actual_cost": "0.15", "input_tokens": "10", "output_tokens": "5", "model": "sample", "group_id": "27"}
		receiveGameplayEvent(t, st, e)
		drainGameplayEvents(t, st)
	}
	wantGameplayPoints(t, st, 3)
	page, err := st.EventLogs(context.Background(), "usage.tokens_updated", "", 1, 20, false)
	if err != nil || page.Total != 0 {
		t.Fatalf("unconfigured token events were retained: %#v %v", page, err)
	}
	rule = gameplayRule("注册天数", 7)
	rule.EventType = "user.age_updated"
	rule.UserLimit = 1
	saveGameplayRule(t, st, rule)
	if err = st.UpsertRegisteredAt(gameplayTestUserID, "", time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	d := events.NewDispatcher("lotteryd", 1, 32, st.ReceiveEvent)
	events.SetSink(d.Submit)
	defer func() { events.SetSink(nil); d.Stop() }()
	if err = st.EmitEventClock(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for d.Sent.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if d.Sent.Load() == 0 {
		t.Fatal("clock event was not received")
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 10)
}

func TestGameplayMixedBillingUsesOneSpendingTotalAndDailyFirst(t *testing.T) {
	st := storetest.Open(t)
	r := gameplayRule("每日首次", 2)
	r.EventType = "user.daily_spend"
	saveGameplayRule(t, st, r)
	r = gameplayRule("累计消费", 3)
	r.EventType = "usage.cost_updated"
	r.Conditions = []store.EventCondition{{Field: "total_amount", Operator: "gte", Value: "0.30"}}
	saveGameplayRule(t, st, r)
	for i, kind := range []string{"billing.completed", "billing.captured"} {
		e := gameplayEvent(fmt.Sprintf("mixed-%d", i))
		e.Type = kind
		e.Data = map[string]string{"actual_cost": "0.15"}
		receiveGameplayEvent(t, st, e)
		drainGameplayEvents(t, st)
	}
	wantGameplayPoints(t, st, 5)
	logs, err := st.EventLogs(context.Background(), "user.daily_spend", "", 1, 20, false)
	if err != nil || logs.Total != 1 {
		t.Fatalf("daily spend duplicated: %#v %v", logs, err)
	}
}

func gameplayRule(name string, points int64) store.EventRule {
	return store.EventRule{Name: name, EventType: "announcement.read", Enabled: true, Probability: 1000000, Recipient: "user", EveryN: 1, Action: store.EventAction{Kind: "grant_points", Points: points}}
}

func gameplayEvent(id string) events.Event {
	return events.Event{ID: id, Type: "announcement.read", Source: "test", UserID: gameplayTestUserID, Reference: id, OccurredAt: time.Now().UTC(), Data: map[string]string{"resource": "test-announcement"}}
}

func saveGameplayRule(t *testing.T, st *store.Store, rule store.EventRule) store.EventRule {
	t.Helper()
	saved, err := st.SaveEventRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("save rule: %v", err)
	}
	return saved
}

func receiveGameplayEvent(t *testing.T, st *store.Store, event events.Event) {
	t.Helper()
	if err := st.ReceiveEvent(context.Background(), event); err != nil {
		t.Fatalf("receive event %s: %v", event.ID, err)
	}
}

func drainGameplayEvents(t *testing.T, st *store.Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for i := 0; i < 500; i++ {
		worked, err := st.ProcessNextEvent(ctx)
		if err != nil {
			t.Fatalf("process event: %v", err)
		}
		if !worked {
			return
		}
	}
	t.Fatal("event processing did not become idle")
}

func wantGameplayPoints(t *testing.T, st *store.Store, want int64) {
	t.Helper()
	balance, err := st.GetPointsBalance(gameplayTestUserID)
	if err != nil || balance.Balance != want {
		t.Fatalf("points = %d, want %d; error = %v", balance.Balance, want, err)
	}
}

func wantGameplayLogCount(t *testing.T, st *store.Store, status string, count int64) {
	t.Helper()
	page, err := st.EventLogs(context.Background(), "announcement.read", status, 1, 20, true)
	if err != nil || page.Total != count {
		t.Fatalf("%s executions = %d, want %d; error = %v", status, page.Total, count, err)
	}
}

func gameplayInventory(t *testing.T, st *store.Store, itemID int64) int64 {
	t.Helper()
	tx, err := st.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var quantity int64
	if err := tx.QueryRow(`SELECT quantity FROM user_items WHERE user_id=$1 AND item_id=$2`, gameplayTestUserID, itemID).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	return quantity
}

func TestGameplayDuplicateEventAppliesMultipleIndependentRulesOnce(t *testing.T) {
	st := storetest.Open(t)
	saveGameplayRule(t, st, gameplayRule("first", 10))
	saveGameplayRule(t, st, gameplayRule("second", 25))
	event := gameplayEvent("multiple-once")
	receiveGameplayEvent(t, st, event)
	receiveGameplayEvent(t, st, event)
	drainGameplayEvents(t, st)
	receiveGameplayEvent(t, st, event)
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 35)
	wantGameplayLogCount(t, st, "applied", 2)
	transactions, total, err := st.ListPointsTransactions(gameplayTestUserID, "grant", 20, 0)
	if err != nil || total != 2 || len(transactions) != 2 {
		t.Fatalf("reward transactions: total=%d rows=%d error=%v", total, len(transactions), err)
	}
}

func TestGameplayEventIdentityConflictIsRejected(t *testing.T) {
	st := storetest.Open(t)
	saveGameplayRule(t, st, gameplayRule("read", 1))
	rule := gameplayRule("logout", 1)
	rule.EventType = "user.logged_out"
	saveGameplayRule(t, st, rule)
	event := gameplayEvent("conflicting-id")
	receiveGameplayEvent(t, st, event)
	event.Type = "user.logged_out"
	if err := st.ReceiveEvent(context.Background(), event); !errors.Is(err, store.ErrInvalidEventRule) {
		t.Fatalf("identity conflict error = %v", err)
	}
}

func TestGameplayZeroProbabilityDoesNotRewardOrConsumeLimit(t *testing.T) {
	st := storetest.Open(t)
	rule := gameplayRule("never", 10)
	rule.Probability = 0
	rule.TotalLimit = 1
	saved := saveGameplayRule(t, st, rule)
	receiveGameplayEvent(t, st, gameplayEvent("probability-zero"))
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 0)
	wantGameplayLogCount(t, st, "missed", 1)
	rules, err := st.ListEventRules(context.Background())
	if err != nil || len(rules) != 1 || rules[0].ID != saved.ID || rules[0].TotalTriggered != 0 {
		t.Fatalf("zero probability consumed limit: %+v error=%v", rules, err)
	}
}

func TestGameplayDailyUserAndTotalLimits(t *testing.T) {
	for _, kind := range []string{"daily", "user", "total"} {
		t.Run(kind, func(t *testing.T) {
			st := storetest.Open(t)
			rule := gameplayRule("limited", 10)
			switch kind {
			case "daily":
				rule.DailyLimit = 2
			case "user":
				rule.UserLimit = 2
			case "total":
				rule.TotalLimit = 2
			}
			saveGameplayRule(t, st, rule)
			for i := 0; i < 5; i++ {
				receiveGameplayEvent(t, st, gameplayEvent(fmt.Sprintf("limit-%s-%d", kind, i)))
			}
			drainGameplayEvents(t, st)
			wantGameplayPoints(t, st, 20)
			wantGameplayLogCount(t, st, "applied", 2)
			wantGameplayLogCount(t, st, "limited", 3)
		})
	}
}

func TestGameplayCooldownAndEveryN(t *testing.T) {
	st := storetest.Open(t)
	cooldown := gameplayRule("cooldown", 10)
	cooldown.CooldownSeconds = 3600
	every := gameplayRule("every-second", 3)
	every.EveryN = 2
	saveGameplayRule(t, st, cooldown)
	saveGameplayRule(t, st, every)
	for i := 0; i < 4; i++ {
		receiveGameplayEvent(t, st, gameplayEvent(fmt.Sprintf("interval-%d", i)))
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 16)
	wantGameplayLogCount(t, st, "applied", 3)
	wantGameplayLogCount(t, st, "limited", 3)
	wantGameplayLogCount(t, st, "not_matched", 2)
}

func TestGameplayConcurrentDeliveryAndProcessingDoNotDuplicateRewards(t *testing.T) {
	st := storetest.Open(t)
	saveGameplayRule(t, st, gameplayRule("concurrent", 7))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	event := gameplayEvent("concurrent-same-event")
	var wg sync.WaitGroup
	errorsCh := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errorsCh <- st.ReceiveEvent(ctx, event) }()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent delivery: %v", err)
		}
	}
	errorsCh = make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := st.ProcessNextEvent(ctx); errorsCh <- err }()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent processing: %v", err)
		}
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 7)
	wantGameplayLogCount(t, st, "applied", 1)
}

func TestGameplayGlobalPauseAndRuleDisableAffectPendingEvents(t *testing.T) {
	for _, pause := range []string{"global", "rule"} {
		t.Run(pause, func(t *testing.T) {
			st := storetest.Open(t)
			rule := saveGameplayRule(t, st, gameplayRule("paused", 10))
			receiveGameplayEvent(t, st, gameplayEvent("pending-before-pause"))
			var err error
			if pause == "global" {
				err = st.SetState("gameplay_rewards_enabled", "false")
			} else {
				err = st.DisableEventRule(context.Background(), rule.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			drainGameplayEvents(t, st)
			wantGameplayPoints(t, st, 0)
			wantGameplayLogCount(t, st, "skipped", 1)
		})
	}
}

func TestGameplayConcurrentDifferentEventsRespectTotalLimit(t *testing.T) {
	st := storetest.Open(t)
	rule := gameplayRule("concurrent-cap", 5)
	rule.TotalLimit = 3
	saveGameplayRule(t, st, rule)
	for i := 0; i < 20; i++ {
		receiveGameplayEvent(t, st, gameplayEvent(fmt.Sprintf("concurrent-cap-%d", i)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errorsCh := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				worked, err := st.ProcessNextEvent(ctx)
				if err != nil {
					errorsCh <- err
					return
				}
				if !worked {
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("concurrent limit processing: %v", err)
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 15)
	wantGameplayLogCount(t, st, "applied", 3)
	wantGameplayLogCount(t, st, "limited", 17)
}

func TestGameplayRuleSnapshotSurvivesLaterEdits(t *testing.T) {
	st := storetest.Open(t)
	rule := saveGameplayRule(t, st, gameplayRule("before", 10))
	receiveGameplayEvent(t, st, gameplayEvent("old-rule-snapshot"))
	rule.Name = "after"
	rule.Action.Points = 99
	saveGameplayRule(t, st, rule)
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 10)
	page, err := st.EventLogs(context.Background(), "announcement.read", "applied", 1, 20, true)
	if err != nil || len(page.Items) != 1 || page.Items[0]["rule_name"] != "before" {
		t.Fatalf("rule snapshot receipt: %+v error=%v", page, err)
	}
}

func TestGameplayGrantsItemsExactlyOnce(t *testing.T) {
	st := storetest.Open(t)
	itemID, err := st.UpsertItem("event-gift", "Gift", "", "", "{}")
	if err != nil {
		t.Fatal(err)
	}
	rule := gameplayRule("item", 1)
	rule.Action = store.EventAction{Kind: "grant_item", ItemID: itemID, Quantity: 3}
	saveGameplayRule(t, st, rule)
	event := gameplayEvent("item-once")
	receiveGameplayEvent(t, st, event)
	receiveGameplayEvent(t, st, event)
	drainGameplayEvents(t, st)
	if got := gameplayInventory(t, st, itemID); got != 3 {
		t.Fatalf("item quantity=%d, want 3", got)
	}
	wantGameplayLogCount(t, st, "applied", 1)
}

func TestGameplayItemEffectUsesAcceptedSnapshotAndConsumesAtomically(t *testing.T) {
	st := storetest.Open(t)
	itemID, err := st.UpsertItem("event-effect", "Effect", "", "grant_points", `{"points":11}`)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.AddUserItems(gameplayTestUserID, itemID, 1); err != nil {
		t.Fatal(err)
	}
	rule := gameplayRule("effect", 1)
	rule.Action = store.EventAction{Kind: "item_effect", ItemID: itemID, Quantity: 1, ConsumeItem: true}
	saveGameplayRule(t, st, rule)
	receiveGameplayEvent(t, st, gameplayEvent("item-effect-snapshot"))
	if _, err = st.UpsertItem("event-effect", "Effect", "", "grant_points", `{"points":999}`); err != nil {
		t.Fatal(err)
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 11)
	if got := gameplayInventory(t, st, itemID); got != 0 {
		t.Fatalf("consumed item quantity=%d, want 0", got)
	}
	wantGameplayLogCount(t, st, "applied", 1)
}

func TestGameplayFailedActionDoesNotRollBackOtherTrigger(t *testing.T) {
	st := storetest.Open(t)
	itemID, err := st.UpsertItem("missing-owned-item", "Effect", "", "grant_points", `{"points":100}`)
	if err != nil {
		t.Fatal(err)
	}
	failing := gameplayRule("no-inventory", 1)
	failing.Action = store.EventAction{Kind: "item_effect", ItemID: itemID, Quantity: 1, ConsumeItem: true}
	saveGameplayRule(t, st, failing)
	saveGameplayRule(t, st, gameplayRule("independent-points", 13))
	receiveGameplayEvent(t, st, gameplayEvent("independent-actions"))
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 13)
	wantGameplayLogCount(t, st, "failed", 1)
	wantGameplayLogCount(t, st, "applied", 1)
	rules, err := st.ListEventRules(context.Background())
	if err != nil || len(rules) != 2 || rules[0].TotalTriggered != 0 || rules[1].TotalTriggered != 1 {
		t.Fatalf("failure consumed limits or affected other trigger: %+v error=%v", rules, err)
	}
}
