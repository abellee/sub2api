//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/iqdetectd"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestIQDetectGroupVisibility(t *testing.T) {
	ctx := context.Background()
	store, err := iqdetectd.OpenStore(filepath.Join(t.TempDir(), "iqdetect.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	sidecar := &iqdetectd.Server{Store: store}
	server := httptest.NewServer(sidecar.Handler())
	t.Cleanup(server.Close)
	svc := &IQDetectService{baseURL: server.URL, client: server.Client()}

	visible, hidden := true, false
	groups := []iqdetectd.SelectionGroup{
		{ID: 1, Name: "legacy", IntervalMinutes: 30},
		{ID: 2, Name: "hidden", IntervalMinutes: 30, Visible: &hidden,
			Accounts: []iqdetectd.SelectionAccount{{ID: 42, Models: []string{"test-model"}}}},
		{ID: 3, Name: "visible", IntervalMinutes: 30, Visible: &visible},
		{ID: 4, Name: "manual-only", Visible: &visible},
	}
	_, err = svc.SaveDetectionSelection(ctx, groups)
	require.NoError(t, err)
	selection, err := svc.GetDetectionSelection(ctx)
	require.NoError(t, err)
	require.Len(t, selection.Groups, 4, "hidden groups remain in the admin selection")
	require.Nil(t, selection.Groups[0].Visible, "legacy visibility remains backward compatible")
	require.NotNil(t, selection.Groups[1].Visible)
	require.False(t, *selection.Groups[1].Visible)
	require.Equal(t, 30, selection.Groups[1].IntervalMinutes, "hiding must preserve scheduling")
	require.Equal(t, groups[1].Accounts, selection.Groups[1].Accounts)

	ids := make([]int64, len(groups))
	for i, group := range groups {
		ids[i], err = store.InsertRunningTestResult(ctx, iqdetectd.TestResult{
			GroupID: group.ID, GroupName: group.Name, AccountID: 42, Model: "test-model",
		})
		require.NoError(t, err)
	}

	overview, err := svc.GetPublicOverview(ctx)
	require.NoError(t, err)
	require.Len(t, overview.Groups, 2)
	require.Equal(t, int64(1), overview.Groups[0].ID)
	require.Equal(t, int64(3), overview.Groups[1].ID)
	require.Equal(t, ids[0], overview.Groups[0].Results[0].ID)
	require.Equal(t, ids[2], overview.Groups[1].Results[0].ID)

	for i, group := range groups {
		detail, detailErr := svc.GetPublicResultDetail(ctx, ids[i])
		if i == 1 || i == 3 {
			require.Nil(t, detail)
			require.True(t, infraerrors.IsNotFound(detailErr), "group %d should be hidden", group.ID)
		} else {
			require.NoError(t, detailErr)
			require.Equal(t, group.ID, detail.GroupID)
		}
	}
	adminResults, total, err := svc.ListTestResults(ctx, 48, 0, 2)
	require.NoError(t, err)
	require.Equal(t, 1, total, "hidden results remain available to admins")
	require.Equal(t, ids[1], adminResults[0].ID)

	groups[1].Visible = &visible
	_, err = svc.SaveDetectionSelection(ctx, groups)
	require.NoError(t, err)
	overview, err = svc.GetPublicOverview(ctx)
	require.NoError(t, err)
	require.Len(t, overview.Groups, 3, "checking visibility republishes existing results")
	detail, err := svc.GetPublicResultDetail(ctx, ids[1])
	require.NoError(t, err)
	require.Equal(t, int64(2), detail.GroupID)
}
