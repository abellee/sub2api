package store_test

import (
	"context"
	"testing"

	"lotteryd/internal/store/storetest"
)

func TestGameplayOnlyEnabledTriggersCollectEvents(t *testing.T) {
	st := storetest.Open(t)
	ctx := context.Background()
	receiveGameplayEvent(t, st, gameplayEvent("no-rules"))
	rule := gameplayRule("disabled", 3)
	rule.Enabled = false
	rule = saveGameplayRule(t, st, rule)
	receiveGameplayEvent(t, st, gameplayEvent("disabled-rule"))
	logs, err := st.EventLogs(ctx, "", "", 1, 20, false)
	if err != nil || logs.Total != 0 {
		t.Fatalf("unconfigured events stored: %+v %v", logs, err)
	}
	rule.Enabled = true
	rule = saveGameplayRule(t, st, rule)
	receiveGameplayEvent(t, st, gameplayEvent("enabled-rule"))
	unrelated := gameplayEvent("unrelated")
	unrelated.Type = "request.completed"
	receiveGameplayEvent(t, st, unrelated)
	if err := st.DisableEventRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
	receiveGameplayEvent(t, st, gameplayEvent("disabled-again"))
	drainGameplayEvents(t, st)
	logs, err = st.EventLogs(ctx, "", "", 1, 20, false)
	if err != nil || logs.Total != 1 {
		t.Fatalf("enable/disable did not take effect: %+v %v", logs, err)
	}
	wantGameplayPoints(t, st, 0)
	tx, err := st.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var metrics int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM gameplay_metrics`).Scan(&metrics); err != nil || metrics != 0 {
		t.Fatalf("disabled event updated metrics: %d %v", metrics, err)
	}
}

func TestGameplayCleanupSkipsEmptySnapshotsAndPreservesAcceptedRewards(t *testing.T) {
	st := storetest.Open(t)
	ctx := context.Background()
	tx, err := st.Begin()
	if err != nil {
		t.Fatal(err)
	}
	e := gameplayEvent("legacy-no-rule")
	_, err = tx.Exec(`INSERT INTO gameplay_events(id,event_type,user_id,payload,rules) VALUES($1,$2,$3,$4,'[]')`, e.ID, e.Type, e.UserID, `{"id":"legacy-no-rule","type":"announcement.read","user_id":2}`)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = st.SkipUnconfiguredEvents(ctx); err != nil {
		t.Fatal(err)
	}
	logs, err := st.EventLogs(ctx, "", "skipped", 1, 20, false)
	if err != nil || logs.Total != 1 {
		t.Fatalf("legacy work not skipped: %+v %v", logs, err)
	}
	saveGameplayRule(t, st, gameplayRule("reward", 7))
	receiveGameplayEvent(t, st, gameplayEvent("accepted-reward"))
	if err = st.SkipUnconfiguredEvents(ctx); err != nil {
		t.Fatal(err)
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 7)
}

func TestGameplayDerivedTriggerCollectsOnlyItsRequiredFacts(t *testing.T) {
	st := storetest.Open(t)
	rule := gameplayRule("tokens", 4)
	rule.EventType = "usage.tokens_updated"
	saveGameplayRule(t, st, rule)
	e := gameplayEvent("billing-fact")
	e.Type = "billing.completed"
	e.Data = map[string]string{"input_tokens": "10", "output_tokens": "5", "actual_cost": "0.15"}
	receiveGameplayEvent(t, st, e)
	if err := st.SkipUnconfiguredEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	drainGameplayEvents(t, st)
	wantGameplayPoints(t, st, 4)
	logs, err := st.EventLogs(context.Background(), "", "", 1, 20, false)
	if err != nil || logs.Total != 2 {
		t.Fatalf("unneeded derived events stored: %+v %v", logs, err)
	}
	tx, err := st.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var unneeded int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM gameplay_metrics WHERE event_type IN ('__spending','__active')`).Scan(&unneeded); err != nil || unneeded != 0 {
		t.Fatalf("unneeded metrics updated: %d %v", unneeded, err)
	}
}
