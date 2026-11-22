//go:build unit

package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func documentationFixture(t *testing.T) *DocumentationSnapshot {
	t.Helper()
	var snap DocumentationSnapshot
	require.NoError(t, json.Unmarshal(documentationDefaults, &snap))
	return &snap
}

func TestDocumentationDefaultsAndPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "groups.db")
	store := NewGroupCategoryStore(path)
	category, err := store.Create("API groups", "preserved", false)
	require.NoError(t, err)
	require.NoError(t, store.Assign(12, category.ID))
	cap := 2.5
	require.NoError(t, store.SetGroupCap(12, &cap))
	_, err = store.SaveUserMenu(ctx, UserMenuConfig{Sections: []UserMenuSection{{ID: "docs", Title: "Documents", Items: []string{"/docs/creative"}}}})
	require.NoError(t, err)
	snap, err := store.Documentation(ctx)
	require.NoError(t, err)
	require.Len(t, snap.Articles, 10)
	require.Len(t, snap.Categories, 2)
	require.NoError(t, ValidateDocumentation(snap))
	for _, article := range snap.Articles {
		require.Contains(t, article.Content, "```", article.ID)
		require.Contains(t, article.Agent.Template, "{{api_key}}")
		require.Contains(t, article.Agent.Template, "{{document}}")
	}
	snap.Categories = append(snap.Categories, DocumentationCategory{ID: "custom", Name: "Custom docs"})
	snap.Articles[0].Name = "Renamed article"
	snap.Articles[0].CategoryID = "custom"
	snap.Articles[0].Content = "# Changed\n\n## New chapter\n\nContent."
	require.NoError(t, store.SaveDocumentation(ctx, snap))
	require.EqualValues(t, 1, snap.Revision)
	snap.Articles[0].Logo = "grok"
	require.NoError(t, store.SaveDocumentation(ctx, snap))
	require.EqualValues(t, 2, snap.Revision)
	reloaded, err := NewGroupCategoryStore(path).Documentation(ctx)
	require.NoError(t, err)
	require.Equal(t, snap, reloaded)
	groups, err := store.List()
	require.NoError(t, err)
	require.Equal(t, category.ID, groups.Assignments["12"])
	require.Equal(t, cap, groups.GroupCaps["12"])
	menu, err := store.GetUserMenu(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"/docs/creative"}, menu.Sections[0].Items)
	_, err = store.Create("Another group category", "", false)
	require.NoError(t, err)
	reloaded, err = store.Documentation(ctx)
	require.NoError(t, err)
	require.Equal(t, snap, reloaded)
	// An intentional empty library must stay empty after a restart.
	snap.Articles = []DocumentationArticle{}
	snap.Categories = []DocumentationCategory{}
	require.NoError(t, store.SaveDocumentation(ctx, snap))
	reloaded, err = NewGroupCategoryStore(path).Documentation(ctx)
	require.NoError(t, err)
	require.Empty(t, reloaded.Articles)
	require.Empty(t, reloaded.Categories)
}

func TestDocumentationConflictingSaves(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "groups.db")
	first, second := NewGroupCategoryStore(path), NewGroupCategoryStore(path)
	a, err := first.Documentation(ctx)
	require.NoError(t, err)
	b, err := second.Documentation(ctx)
	require.NoError(t, err)
	a.Articles[0].Name = "Winner"
	require.NoError(t, first.SaveDocumentation(ctx, a))
	b.Articles[0].Name = "Stale"
	require.ErrorIs(t, second.SaveDocumentation(ctx, b), ErrDocumentationConflict)
	oldRevision := *a
	a.Articles[0].Name = "Latest"
	require.NoError(t, first.SaveDocumentation(ctx, a))
	require.ErrorIs(t, second.SaveDocumentation(ctx, &oldRevision), ErrDocumentationConflict)
	loaded, err := second.Documentation(ctx)
	require.NoError(t, err)
	require.Equal(t, "Latest", loaded.Articles[0].Name)
	require.EqualValues(t, 2, loaded.Revision)
}

func TestDocumentationValidation(t *testing.T) {
	cases := map[string]func(*DocumentationSnapshot){
		"missing articles":        func(s *DocumentationSnapshot) { s.Articles = nil },
		"missing categories":      func(s *DocumentationSnapshot) { s.Categories = nil },
		"duplicate category":      func(s *DocumentationSnapshot) { s.Categories = append(s.Categories, s.Categories[0]) },
		"duplicate category name": func(s *DocumentationSnapshot) { s.Categories[1].Name = s.Categories[0].Name },
		"duplicate article":       func(s *DocumentationSnapshot) { s.Articles = append(s.Articles, s.Articles[0]) },
		"invalid category":        func(s *DocumentationSnapshot) { s.Articles[0].CategoryID = "missing" },
		"empty content":           func(s *DocumentationSnapshot) { s.Articles[0].Content = " " },
		"large content":           func(s *DocumentationSnapshot) { s.Articles[0].Content = strings.Repeat("x", 1024*1024+1) },
		"invalid logo":            func(s *DocumentationSnapshot) { s.Articles[0].Logo = "../x" },
		"invalid mode":            func(s *DocumentationSnapshot) { s.Articles[0].Agent.KeyMode = "admin" },
		"empty groups":            func(s *DocumentationSnapshot) { s.Articles[0].Agent.KeyMode = "groups" },
		"invalid group":           func(s *DocumentationSnapshot) { s.Articles[0].Agent.GroupIDs = []int64{0} },
		"duplicate group":         func(s *DocumentationSnapshot) { s.Articles[0].Agent.GroupIDs = []int64{1, 1} },
		"unsafe url":              func(s *DocumentationSnapshot) { s.Articles[0].Agent.BaseURL = "javascript:alert(1)" },
		"url credentials":         func(s *DocumentationSnapshot) { s.Articles[0].Agent.BaseURL = "https://user:secret@example.com" },
		"url query":               func(s *DocumentationSnapshot) { s.Articles[0].Agent.BaseURL = "https://example.com?key=x" },
		"unknown variable":        func(s *DocumentationSnapshot) { s.Articles[0].Agent.Template = "{{admin_key}}" },
		"empty prompt":            func(s *DocumentationSnapshot) { s.Articles[0].Agent.Template = "" },
		"invalid revision":        func(s *DocumentationSnapshot) { s.Revision = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := documentationFixture(t)
			mutate(s)
			require.Error(t, ValidateDocumentation(s))
		})
	}
	s := documentationFixture(t)
	s.Articles[0].Agent.Template = "{{ article_name }} {{base_url}} {{api_key}} {{group_name}} {{document}}"
	s.Articles[0].Agent.KeyMode = "groups"
	s.Articles[0].Agent.GroupIDs = []int64{12, 13}
	require.NoError(t, ValidateDocumentation(s))
}

func TestDocumentationStorageErrorDoesNotResetDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.db")
	require.NoError(t, os.WriteFile(path, []byte("not a database"), 0600))
	_, err := NewGroupCategoryStore(path).Documentation(context.Background())
	require.Error(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "not a database", string(raw))
}
