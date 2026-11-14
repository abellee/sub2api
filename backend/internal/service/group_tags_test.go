//go:build unit

package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupTagsPersistAndPreserveConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "group_categories.db")
	store := NewGroupCategoryStore(path)
	category, err := store.Create("分类", "说明", false)
	require.NoError(t, err)
	require.NoError(t, store.Assign(42, category.ID))
	capRate := 0.8
	require.NoError(t, store.SetGroupCap(42, &capRate))
	require.NoError(t, store.SetGroupTags(42, []GroupTag{{Name: " 快速 ", Description: " 优先处理 "}, {Name: "稳定"}}))
	require.NoError(t, store.SetGroupTags(43, []GroupTag{{Name: "其它组"}}))
	store = NewGroupCategoryStore(path)
	snapshot, err := store.List()
	require.NoError(t, err)
	require.Equal(t, []GroupTag{{Name: "快速", Description: "优先处理"}, {Name: "稳定"}}, snapshot.GroupTags["42"])
	require.Equal(t, capRate, snapshot.GroupCaps["42"])
	require.Equal(t, category.ID, snapshot.Assignments["42"])
	require.Len(t, snapshot.Categories, 1)

	// Other configuration writers must keep tags as well.
	require.NoError(t, store.SetGroupCap(42, nil))
	require.NoError(t, store.Reorder([]string{category.ID}))
	snapshot, err = store.List()
	require.NoError(t, err)
	require.Len(t, snapshot.GroupTags["42"], 2)
	require.NoError(t, store.SetGroupTags(42, []GroupTag{{Name: "更新"}}))
	require.NoError(t, store.SetGroupTags(42, []GroupTag{}))
	snapshot, err = NewGroupCategoryStore(path).List()
	require.NoError(t, err)
	require.NotContains(t, snapshot.GroupTags, "42")
	require.Equal(t, []GroupTag{{Name: "其它组"}}, snapshot.GroupTags["43"])
	require.Equal(t, category.ID, snapshot.Assignments["42"])
}

func TestGroupTagsRejectInvalidInputWithoutOverwriting(t *testing.T) {
	store := NewGroupCategoryStore(filepath.Join(t.TempDir(), "config.db"))
	original := []GroupTag{{Name: "保留"}}
	require.NoError(t, store.SetGroupTags(42, original))
	for _, tags := range [][]GroupTag{
		{{Name: " "}},
		{{Name: strings.Repeat("名", 51)}},
		{{Name: "名称", Description: strings.Repeat("文", 201)}},
		make([]GroupTag, 21),
	} {
		require.Error(t, store.SetGroupTags(42, tags))
	}
	require.Error(t, store.SetGroupTags(0, original))
	snapshot, err := store.List()
	require.NoError(t, err)
	require.Equal(t, original, snapshot.GroupTags["42"])
}
