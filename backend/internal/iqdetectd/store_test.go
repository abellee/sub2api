package iqdetectd

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "iqdetect.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreModelsCRUD(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	created, err := store.CreateModel(ctx, CreateModelInput{Name: "gpt-test", Label: "Test Model"})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	if !created.Enabled {
		t.Error("new model should default to enabled")
	}
	if _, err := store.CreateModel(ctx, CreateModelInput{Name: "gpt-test"}); err == nil {
		t.Error("duplicate model should fail")
	}

	enabled := false
	updated, err := store.UpdateModel(ctx, created.ID, CreateModelInput{Enabled: &enabled})
	if err != nil {
		t.Fatalf("update model: %v", err)
	}
	if updated.Enabled {
		t.Error("model should be disabled after update")
	}

	if err := store.DeleteModel(ctx, created.ID); err != nil {
		t.Fatalf("delete model: %v", err)
	}
	if _, err := store.GetModelByID(ctx, created.ID); err == nil {
		t.Error("deleted model should not be found")
	}
}

func TestStoreRunsAndScores(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if _, err := store.CreateModel(ctx, CreateModelInput{Name: "m-a"}); err != nil {
		t.Fatalf("create model: %v", err)
	}

	run, err := store.CreateRun(ctx, []string{"m-a"}, 3)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.Status != RunStatusPending {
		t.Errorf("run status = %q, want pending", run.Status)
	}

	active, err := store.HasActiveRun(ctx)
	if err != nil || !active {
		t.Fatalf("HasActiveRun = %v, %v; want true", active, err)
	}

	if err := store.UpdateRunStatus(ctx, run.ID, RunStatusRunning, ""); err != nil {
		t.Fatalf("update run status: %v", err)
	}

	_, err = store.InsertResult(ctx, ModelResult{
		RunID: run.ID, Model: "m-a", IQ: 90, Correct: 5, Total: 10,
		Details: []ProbeOutcome{{ProbeID: "arith-001", Correct: true}},
	})
	if err != nil {
		t.Fatalf("insert result: %v", err)
	}

	results, err := store.ListResultsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("list results: %v", err)
	}
	if len(results) != 1 || results[0].Details[0].ProbeID != "arith-001" {
		t.Errorf("unexpected results: %+v", results)
	}

	if err := store.UpdateRunStatus(ctx, run.ID, RunStatusCompleted, ""); err != nil {
		t.Fatalf("complete run: %v", err)
	}
	active, err = store.HasActiveRun(ctx)
	if err != nil || active {
		t.Fatalf("HasActiveRun after completion = %v, %v; want false", active, err)
	}

	scores, err := store.ListScores(ctx, 10)
	if err != nil {
		t.Fatalf("list scores: %v", err)
	}
	if len(scores) != 1 {
		t.Fatalf("scores len = %d, want 1", len(scores))
	}
	s := scores[0]
	if s.Model != "m-a" || s.LatestIQ != 90 || s.BestIQ != 90 || s.Runs != 1 || s.LastRunAt == nil {
		t.Errorf("unexpected score summary: %+v", s)
	}
	if len(s.History) != 1 || s.History[0].RunID != run.ID {
		t.Errorf("unexpected history: %+v", s.History)
	}
}

func TestStoreAccountCredentials(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	created, err := store.UpsertAccountCredential(ctx, UpsertAccountCredentialInput{
		AccountID: 7, APIURL: "https://api.example.com", APIKey: "sk-secret",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !created.HasAPIKey || created.APIURL != "https://api.example.com" {
		t.Errorf("unexpected credential: %+v", created)
	}

	// 空 apiKey 保留旧 key；api_url 可单独更新。
	updated, err := store.UpsertAccountCredential(ctx, UpsertAccountCredentialInput{
		AccountID: 7, APIURL: "https://api2.example.com",
	})
	if err != nil {
		t.Fatalf("upsert without key: %v", err)
	}
	if !updated.HasAPIKey {
		t.Error("empty api_key should keep the stored key")
	}
	if updated.APIURL != "https://api2.example.com" {
		t.Errorf("api_url = %q", updated.APIURL)
	}

	items, err := store.ListAccountCredentials(ctx)
	if err != nil || len(items) != 1 || items[0].AccountID != 7 {
		t.Fatalf("list = %v, %v", items, err)
	}

	if _, err := store.UpsertAccountCredential(ctx, UpsertAccountCredentialInput{AccountID: 0}); err == nil {
		t.Error("missing account_id should fail")
	}
}

func TestStoreDetectionSelection(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// 从未保存过：UpdatedAt 为 nil、空列表
	empty, err := store.GetSelection(ctx)
	if err != nil {
		t.Fatalf("get empty selection: %v", err)
	}
	if empty.UpdatedAt != nil || len(empty.Groups) != 0 {
		t.Errorf("unexpected empty selection: %+v", empty)
	}

	groups := []SelectionGroup{
		{ID: 2, Name: "福利", RateMultiplier: 0.08, Accounts: []SelectionAccount{
			{ID: 154, Name: "acc", Models: []string{"gpt-4o", "gpt-4o-mini"}},
		}},
	}
	saved, err := store.SaveSelection(ctx, groups)
	if err != nil {
		t.Fatalf("save selection: %v", err)
	}
	if saved.UpdatedAt == nil || len(saved.Groups) != 1 || saved.Groups[0].Accounts[0].Models[0] != "gpt-4o" {
		t.Errorf("unexpected saved selection: %+v", saved)
	}

	loaded, err := store.GetSelection(ctx)
	if err != nil || len(loaded.Groups) != 1 || loaded.Groups[0].Name != "福利" {
		t.Fatalf("reload = %+v, %v", loaded, err)
	}

	// 保存空选择应保留记录（区分"从未保存"）
	if _, err := store.SaveSelection(ctx, nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	empty2, err := store.GetSelection(ctx)
	if err != nil || empty2.UpdatedAt == nil || len(empty2.Groups) != 0 {
		t.Errorf("empty save = %+v, %v", empty2, err)
	}
}

func TestStoreDetectionModels(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	items, err := store.ListDetectionModels(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("initial list = %v, %v", items, err)
	}

	items, err = store.ReplaceDetectionModels(ctx, []string{"gpt-4o", " GPT-4O ", "claude-x", ""})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(items) != 2 || items[0].Name != "gpt-4o" || items[1].Name != "claude-x" {
		t.Errorf("unexpected items: %+v", items)
	}

	items, err = store.ReplaceDetectionModels(ctx, []string{"only-one"})
	if err != nil || len(items) != 1 || items[0].Name != "only-one" {
		t.Errorf("replace again = %+v, %v", items, err)
	}
}
