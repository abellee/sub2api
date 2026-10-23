//go:build unit

package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestUserMenuPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "group_categories.db")
	svc := NewGroupCategoryStore(path)
	category, err := svc.Create("VIP", "preserve category", false)
	require.NoError(t, err)
	require.NoError(t, svc.Assign(12, category.ID))
	cap := 2.5
	require.NoError(t, svc.SetGroupCap(12, &cap))
	empty, err := svc.GetUserMenu(ctx)
	require.NoError(t, err)
	require.NotNil(t, empty.Sections)
	require.Empty(t, empty.Sections)

	input := UserMenuConfig{Sections: []UserMenuSection{
		{ID: " models ", Title: " 模型与服务 ", Items: []string{" /model-plaza ", "/keys", "/custom/docs-1"}},
		{ID: "account", Title: "账户", Items: []string{"/usage", "/profile"}},
		{ID: "empty", Title: "暂未分配"},
	}}
	saved, err := svc.SaveUserMenu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "models", saved.Sections[0].ID)
	require.Equal(t, "模型与服务", saved.Sections[0].Title)
	require.Equal(t, []string{"/model-plaza", "/keys", "/custom/docs-1"}, saved.Sections[0].Items)
	require.NotNil(t, saved.Sections[2].Items)
	svc = NewGroupCategoryStore(path)
	loaded, err := svc.GetUserMenu(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
	snapshot, err := svc.List()
	require.NoError(t, err)
	require.Equal(t, category.ID, snapshot.Assignments["12"])
	require.Equal(t, cap, snapshot.GroupCaps["12"])
	_, err = svc.Create("Regular", "", false)
	require.NoError(t, err)
	loaded, err = NewGroupCategoryStore(path).GetUserMenu(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, loaded, "category edits must preserve menus")

	_, err = svc.SaveUserMenu(ctx, UserMenuConfig{})
	require.NoError(t, err)
	reset, err := svc.GetUserMenu(ctx)
	require.NoError(t, err)
	require.Empty(t, reset.Sections, "removing headings restores the default navigation")
}

func TestUserMenuRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		sections []UserMenuSection
	}{
		{"empty title", []UserMenuSection{{ID: "a", Title: "  "}}},
		{"long title", []UserMenuSection{{ID: "a", Title: strings.Repeat("界", 81)}}},
		{"duplicate ids", []UserMenuSection{{ID: "a", Title: "One"}, {ID: "a", Title: "Two"}}},
		{"duplicate titles", []UserMenuSection{{ID: "a", Title: "One"}, {ID: "b", Title: " One "}}},
		{"invalid id", []UserMenuSection{{ID: "../a", Title: "One"}}},
		{"reserved id", []UserMenuSection{{ID: "__unassigned__", Title: "One"}}},
		{"duplicate path", []UserMenuSection{{ID: "a", Title: "One", Items: []string{"/keys", "/keys"}}}},
		{"multiple assignments", []UserMenuSection{{ID: "a", Title: "One", Items: []string{"/keys"}}, {ID: "b", Title: "Two", Items: []string{"/keys"}}}},
		{"admin route", []UserMenuSection{{ID: "a", Title: "One", Items: []string{"/admin/users"}}}},
		{"external url", []UserMenuSection{{ID: "a", Title: "One", Items: []string{"https://example.com"}}}},
		{"custom traversal", []UserMenuSection{{ID: "a", Title: "One", Items: []string{"/custom/../admin"}}}},
		{"too many headings", make([]UserMenuSection, 33)},
		{"too many items", []UserMenuSection{{ID: "a", Title: "One", Items: make([]string, 129)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewGroupCategoryStore(filepath.Join(t.TempDir(), "group_categories.db"))
			original := UserMenuConfig{Sections: []UserMenuSection{{ID: "original", Title: "Original", Items: []string{"/keys"}}}}
			before, err := svc.SaveUserMenu(context.Background(), original)
			require.NoError(t, err)
			_, err = svc.SaveUserMenu(context.Background(), UserMenuConfig{Sections: tt.sections})
			require.True(t, infraerrors.IsBadRequest(err))
			after, err := svc.GetUserMenu(context.Background())
			require.NoError(t, err)
			require.Equal(t, before, after, "invalid writes must leave saved navigation intact")
		})
	}
}

func TestUserMenuStorageErrors(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "group_categories.db")
	require.NoError(t, os.WriteFile(path, []byte("broken database"), 0o600))
	svc := NewGroupCategoryStore(path)
	_, err := svc.GetUserMenu(ctx)
	require.Error(t, err)
	_, err = svc.SaveUserMenu(ctx, UserMenuConfig{})
	require.Error(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "broken database", string(raw))
}

func TestUserMenuSharesMigratedCategorySQLite(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "group_categories.json")
	raw := `{"categories":[{"id":"vip","name":"VIP","description":"legacy"}],"assignments":{"12":"vip"},"group_caps":{"12":3.5}}`
	require.NoError(t, os.WriteFile(legacy, []byte(raw), 0o600))
	store := NewGroupCategoryStore(legacy)
	menu := UserMenuConfig{Sections: []UserMenuSection{{ID: "models", Title: "模型", Items: []string{"/keys"}}}}
	_, err := store.SaveUserMenu(context.Background(), menu)
	require.NoError(t, err)
	dbRaw, err := os.ReadFile(filepath.Join(dir, "group_categories.db"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(dbRaw), "SQLite format 3"))
	// Later edits of the old JSON must never overwrite the imported database.
	require.NoError(t, os.WriteFile(legacy, []byte(`{"categories":[]}`), 0o600))
	reopened := NewGroupCategoryStore(filepath.Join(dir, "group_categories.db"))
	snapshot, err := reopened.List()
	require.NoError(t, err)
	require.Len(t, snapshot.Categories, 1)
	require.Equal(t, "legacy", snapshot.Categories[0].Description)
	require.Equal(t, "vip", snapshot.Assignments["12"])
	require.Equal(t, 3.5, snapshot.GroupCaps["12"])
	loaded, err := reopened.GetUserMenu(context.Background())
	require.NoError(t, err)
	require.Equal(t, &menu, loaded)
	_, err = os.Stat(legacy)
	require.NoError(t, err)
}

func TestUserMenuRejectsCorruptLegacyCategoryImport(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), "group_categories.json")
	require.NoError(t, os.WriteFile(legacy, []byte("broken JSON"), 0o600))
	store := NewGroupCategoryStore(legacy)
	_, err := store.SaveUserMenu(context.Background(), UserMenuConfig{})
	require.ErrorContains(t, err, "import legacy group categories")
	raw, err := os.ReadFile(legacy)
	require.NoError(t, err)
	require.Equal(t, "broken JSON", string(raw))
}
