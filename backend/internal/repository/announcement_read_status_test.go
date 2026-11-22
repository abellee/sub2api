package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementReadStatusPaginationAndCount(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()
	ann, err := client.Announcement.Create().SetTitle("Notice").SetContent("Content").Save(ctx)
	require.NoError(t, err)
	other, err := client.Announcement.Create().SetTitle("Other").SetContent("Other").Save(ctx)
	require.NoError(t, err)
	reads := NewAnnouncementReadRepository(client)
	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com", "d@example.com", "deleted@example.com"} {
		u, err := client.User.Create().SetEmail(email).SetPasswordHash("hash").Save(ctx)
		require.NoError(t, err)
		announcementID := ann.ID
		if email == "a@example.com" {
			announcementID = other.ID
		}
		if email != "c@example.com" {
			require.NoError(t, reads.MarkRead(ctx, announcementID, u.ID, time.Now()))
		}
		if email == "deleted@example.com" {
			require.NoError(t, client.User.UpdateOneID(u.ID).SetDeletedAt(time.Now()).Exec(ctx))
		}
	}
	params := pagination.PaginationParams{Page: 1, PageSize: 1, SortBy: "email", SortOrder: "asc"}
	filters := service.UserListFilters{ReadAnnouncementID: ann.ID}
	users, page, err := repo.ListWithFilters(ctx, params, filters)
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Len(t, users, 1)
	require.Equal(t, "b@example.com", users[0].Email)
	params.Page = 2
	users, page, err = repo.ListWithFilters(ctx, params, filters)
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Len(t, users, 1)
	require.Equal(t, "d@example.com", users[0].Email)
	params.Page = 1
	filters.Search = "d@example.com"
	users, page, err = repo.ListWithFilters(ctx, params, filters)
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, "d@example.com", users[0].Email)
	filters.Search = "a@example.com"
	users, page, err = repo.ListWithFilters(ctx, params, filters)
	require.NoError(t, err)
	require.Empty(t, users)
	require.Zero(t, page.Total)
	count, err := reads.CountByAnnouncementID(ctx, ann.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	_, page, err = repo.ListWithFilters(ctx, params, service.UserListFilters{})
	require.NoError(t, err)
	require.Equal(t, int64(4), page.Total)
}
