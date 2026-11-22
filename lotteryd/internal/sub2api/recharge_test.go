package sub2api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchTotalRecharged(t *testing.T) {
	for _, tc := range []struct {
		data   string
		status int
		want   float64
		fails  bool
	}{
		{`{"total_recharged":12.34,"balance":0,"items":[{"value":1}]}`, 200, 12.34, false},
		{`{"total_recharged":0}`, 200, 0, false},
		{`{}`, 200, 0, true}, {`{"total_recharged":-1}`, 200, 0, true},
		{`{"total_recharged":null}`, 200, 0, true}, {`{}`, 503, 0, true},
	} {
		t.Run(tc.data+fmt.Sprint(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/v1/admin/users/7/balance-history", r.URL.Path)
				require.Equal(t, "admin-key", r.Header.Get("x-api-key"))
				require.Equal(t, "1", r.URL.Query().Get("page_size"))
				require.Equal(t, "balance", r.URL.Query().Get("type"))
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"code":0,"data":%s}`, tc.data)
			}))
			defer srv.Close()
			amount, err := NewClient(srv.URL, "admin-key").FetchTotalRecharged(context.Background(), 7)
			if tc.fails {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, amount)
			}
		})
	}
}
