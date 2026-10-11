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
