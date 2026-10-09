//go:build unit

package service

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupCategoryStoreReorderDescending(t *testing.T) {
	dir := t.TempDir()
	store := NewGroupCategoryStore(filepath.Join(dir, "group_categories.json"))

	first, err := store.Create("Alpha", "alpha desc", false)
	require.NoError(t, err)
	require.Equal(t, "alpha desc", first.Description)

	second, err := store.Create("Beta", "", false)
	require.NoError(t, err)

	require.NoError(t, store.Reorder([]string{second.ID, first.ID}))

	snap, err := store.List()
	require.NoError(t, err)
	require.Len(t, snap.Categories, 2)
	require.Equal(t, second.ID, snap.Categories[0].ID)
	require.Greater(t, snap.Categories[0].SortOrder, snap.Categories[1].SortOrder)
}

func TestGroupCategoryStoreAssignAndDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewGroupCategoryStore(filepath.Join(dir, "group_categories.json"))

	cat, err := store.Create("VIP", "vip users", false)
	require.NoError(t, err)
	require.Equal(t, "vip users", cat.Description)
	require.NoError(t, store.Assign(12, cat.ID))

	desc := "updated note"
	updated, err := store.Update(cat.ID, "VIP", &desc, nil)
	require.NoError(t, err)
	require.Equal(t, "updated note", updated.Description)

	snap, err := store.List()
	require.NoError(t, err)
	require.Equal(t, cat.ID, snap.Assignments["12"])

	require.NoError(t, store.Delete(cat.ID))
	snap, err = store.List()
	require.NoError(t, err)
	require.Empty(t, snap.Categories)
	_, ok := snap.Assignments["12"]
	require.False(t, ok)
}
