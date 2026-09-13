package appcatalog

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreCreateListDelete(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	app, err := store.CreateApp(ctx, CreateAppInput{OfficialURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if app.ID == 0 || app.Name == "" {
		t.Fatalf("unexpected app: %+v", app)
	}
	items, err := store.ListApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d apps", len(items))
	}
	if err := store.DeleteApp(ctx, app.ID); err != nil {
		t.Fatal(err)
	}
	items, err = store.ListApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d", len(items))
	}
}

func TestApplyFetchFillEmptyKeepsManualFields(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	app, err := store.CreateApp(ctx, CreateAppInput{
		OfficialURL: "https://example.com",
		Name:        "Manual",
		IconURL:     "https://cdn.example/icon.png",
		Description: "typed intro",
		Screenshots: []string{"https://cdn.example/a.png"},
		Changelog:   "notes",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ApplyFetch(ctx, app.ID, FetchResult{
		Name:            "Fetched",
		IconURL:         "https://cdn.example/other.png",
		Description:     "from web",
		DownloadPageURL: "https://example.com/download",
		Screenshots:     []string{"https://cdn.example/b.png"},
		Changelog:       "release",
		Version:         "1.2.0",
	}, nil, fetchFillEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Manual" || got.IconURL != "https://cdn.example/icon.png" || got.Description != "typed intro" {
		t.Fatalf("identity fields should stay manual: %+v", got)
	}
	if len(got.Screenshots) != 1 || got.Screenshots[0] != "https://cdn.example/a.png" {
		t.Fatalf("screenshots should stay manual: %+v", got.Screenshots)
	}
	if got.Changelog != "notes" {
		t.Fatalf("changelog should stay manual: %q", got.Changelog)
	}
	if got.Version != "1.2.0" || got.DownloadPageURL != "https://example.com/download" {
		t.Fatalf("empty release fields should fill: %+v", got)
	}
}

func TestApplyFetchFillsMissingSourceURLs(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	app, err := store.CreateApp(ctx, CreateAppInput{GitHubURL: "https://github.com/acme/demo"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ApplyFetch(ctx, app.ID, FetchResult{
		OfficialURL: "https://demo.example",
		GitHubURL:   "https://github.com/acme/demo",
	}, nil, fetchFillEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if got.OfficialURL != "https://demo.example" {
		t.Fatalf("official=%q", got.OfficialURL)
	}
}

func TestUpdateAppPersistsManualFields(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	app, err := store.CreateApp(ctx, CreateAppInput{OfficialURL: "https://example.com", Name: "Old"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.UpdateApp(ctx, app.ID, CreateAppInput{
		OfficialURL: "https://example.com",
		GitHubURL:   "https://github.com/acme/demo",
		Name:        "Edited",
		IconURL:     "https://cdn.example/icon.png",
		Description: "intro",
		Screenshots: []string{"https://cdn.example/a.png"},
		Changelog:   "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Edited" || got.Description != "intro" || got.Changelog != "v1" || got.GitHubURL != "https://github.com/acme/demo" {
		t.Fatalf("update did not persist: %+v", got)
	}
}
