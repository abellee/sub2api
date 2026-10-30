//go:build unit

package service

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupDocumentationPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.db")
	store := NewGroupCategoryStore(path)
	category, err := store.Create("Images", "keep me", false)
	require.NoError(t, err)
	require.NoError(t, store.Assign(10, category.ID))
	cap := 2.5
	require.NoError(t, store.SetGroupCap(10, &cap))
	for _, id := range []string{"image-1k", "banana", "image-az", "image-adobe", "image-official", "image2", "seedream", "mj-v8.2", "video", "sd-video"} {
		require.NoError(t, store.SetGroupDocumentation(10, id))
		snapshot, err := NewGroupCategoryStore(path).List()
		require.NoError(t, err)
		require.Equal(t, id, snapshot.Documentation["10"])
		require.Equal(t, category.ID, snapshot.Assignments["10"])
		require.Equal(t, cap, snapshot.GroupCaps["10"])
	}
	require.Error(t, store.SetGroupDocumentation(10, "unknown"))
	require.Error(t, store.SetGroupDocumentation(0, "banana"))
	snapshot, err := store.List()
	require.NoError(t, err)
	require.Equal(t, "sd-video", snapshot.Documentation["10"])
	require.NoError(t, store.SetGroupDocumentation(11, "banana"))
	require.NoError(t, store.SetGroupDocumentation(10, ""))
	snapshot, err = NewGroupCategoryStore(path).List()
	require.NoError(t, err)
	require.NotContains(t, snapshot.Documentation, "10")
	require.Equal(t, "banana", snapshot.Documentation["11"])
	require.Equal(t, category.ID, snapshot.Assignments["10"])
}
