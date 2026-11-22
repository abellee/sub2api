//go:build unit

package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResponsesUpstreamFailoverRounds(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(map[bool]string{false: "final retry exhausted", true: "success stops immediately"}[success], func(t *testing.T) {
			_, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "all_429")
			defer cleanup()
			template := repo.accounts[1]
			repo.accounts = nil
			upstream.rateLimitIDs = nil
			upstream.failureStatus = map[int64]int{}
			for i, host := range []string{"one", "one", "two", "two", "two", "two", "three", "three"} {
				account := template
				account.ID = int64(1001 + i)
				account.Priority = i
				account.Credentials = cloneCredentialMap(template.Credentials)
				account.Credentials["base_url"] = "https://" + host + ".example/v1"
				repo.accounts = append(repo.accounts, account)
				upstream.failureStatus[account.ID] = http.StatusBadGateway
			}
			if success {
				delete(upstream.failureStatus, 1007)
			}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", bytes.NewBufferString(`{"model":"grok","input":"hello","stream":false}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)
			if success {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, []int64{1001, 1003, 1007}, upstream.accountHits())
			} else {
				require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
				require.Equal(t, []int64{1001, 1003, 1007, 1002, 1004, 1008, 1005}, upstream.accountHits())
			}
			for _, account := range repo.accounts {
				require.Equal(t, service.StatusActive, account.Status)
			}
		})
	}
}
