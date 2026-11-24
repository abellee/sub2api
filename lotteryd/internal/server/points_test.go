package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMyPointsRequireLoginAndValidatePagination(t *testing.T) {
	s := New(nil, "points-test", "test").Handler()
	for _, path := range []string{"/v1/me/points", "/v1/me/points/transactions"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	for _, query := range []string{"page=0", "page=-1", "page=1000001", "page=abc", "page_size=0", "page_size=51", "kind=invalid"} {
		r := httptest.NewRequest(http.MethodGet, "/v1/me/points/transactions?"+query, nil)
		r.Header.Set("Authorization", "Bearer "+signJWT("points-test", 2, "user"))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code, query)
	}
}

func TestMyPointsArePrivatePaginatedAndPreservePrecision(t *testing.T) {
	e := newTestEnv(t)
	defer e.srv.Close()
	st := e.app.Store
	_, _, err := st.GrantPoints(2, 100, "test", "", "", "first grant", map[string]string{"secret": "internal-only"})
	require.NoError(t, err)
	_, err = st.SpendPoints(2, 30, "test", "", "", "used points", nil)
	require.NoError(t, err)
	_, _, err = st.GrantPoints(2, 50, "test", "", "", "last grant", nil)
	require.NoError(t, err)
	_, _, err = st.GrantPoints(3, 9007199254740993, "test", "", "", "other user's record", nil)
	require.NoError(t, err)
	token := signJWT(e.secret, 2, "user")
	summaryResponse := e.req(t, "GET", "/v1/me/points?user_id=3", token, nil)
	require.Equal(t, "no-store", summaryResponse.Header.Get("Cache-Control"))
	summary := mustJSON(t, summaryResponse)
	require.Equal(t, "120", summary["balance"])
	require.Equal(t, "150", summary["total_earned"])
	require.Equal(t, "30", summary["total_spent"])

	page := mustJSON(t, e.req(t, "GET", "/v1/me/points/transactions?user_id=3&page_size=1", token, nil))
	require.EqualValues(t, 3, page["total"])
	rows := page["items"].([]any)
	require.Len(t, rows, 1)
	require.Equal(t, "last grant", rows[0].(map[string]any)["note"])
	page = mustJSON(t, e.req(t, "GET", "/v1/me/points/transactions?page=2&page_size=1", token, nil))
	require.Equal(t, "-30", page["items"].([]any)[0].(map[string]any)["delta"])

	spent := mustJSON(t, e.req(t, "GET", "/v1/me/points/transactions?kind=spend", token, nil))
	require.EqualValues(t, 1, spent["total"])
	require.Len(t, spent["items"], 1)
	require.Equal(t, "70", spent["items"].([]any)[0].(map[string]any)["balance_after"])
	grants := mustJSON(t, e.req(t, "GET", "/v1/me/points/transactions?kind=grant", token, nil))
	require.EqualValues(t, 2, grants["total"])
	encoded, err := json.Marshal(grants)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(encoded), "internal-only"))
	require.False(t, strings.Contains(string(encoded), "ext_json"))
	require.False(t, strings.Contains(string(encoded), "user_id"))

	other := mustJSON(t, e.req(t, "GET", "/v1/me/points", signJWT(e.secret, 3, "user"), nil))
	require.Equal(t, "9007199254740993", other["balance"])
	emptyToken := signJWT(e.secret, 4, "user")
	empty := mustJSON(t, e.req(t, "GET", "/v1/me/points", emptyToken, nil))
	require.Equal(t, "0", empty["balance"])
	emptyHistory := mustJSON(t, e.req(t, "GET", "/v1/me/points/transactions", emptyToken, nil))
	require.EqualValues(t, 0, emptyHistory["total"])
	require.Equal(t, []any{}, emptyHistory["items"])
}
