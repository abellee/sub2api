//go:build unit

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type authMatchingRepo struct {
	service.AccountMatchingRepository
	err             error
	userID, groupID int64
}

func (r *authMatchingRepo) AllowedAccounts(_ context.Context, userID, groupID int64) ([]int64, error) {
	r.userID, r.groupID = userID, groupID
	return []int64{20}, r.err
}

func TestAccountMatchingAuthUsesAuthenticatedUserAndFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, lookupErr := range []error{nil, errors.New("database unavailable")} {
		cfg := &config.Config{RunMode: config.RunModeSimple}
		groupID := int64(10)
		key := &service.APIKey{ID: 1, UserID: 2, Key: "matching-test", Status: service.StatusActive, GroupID: &groupID,
			User:  &service.User{ID: 2, Status: service.StatusActive},
			Group: &service.Group{ID: groupID, Status: service.StatusActive, Hydrated: true},
		}
		repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
		matching := &authMatchingRepo{err: lookupErr}
		svc := service.ProvideAPIKeyService(matching, repo, nil, nil, nil, nil, nil, cfg, nil, nil)
		router := gin.New()
		router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
		entered := false
		router.GET("/t", func(c *gin.Context) {
			entered = true
			require.True(t, service.AccountMatchingAllows(c.Request.Context(), 20))
			require.False(t, service.AccountMatchingAllows(c.Request.Context(), 21))
			c.Status(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodGet, "/t?user_id=99", nil)
		req.Header.Set("Authorization", "Bearer matching-test")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, int64(2), matching.userID)
		require.Equal(t, groupID, matching.groupID)
		if lookupErr == nil {
			require.Equal(t, http.StatusOK, response.Code)
			require.True(t, entered)
		} else {
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
			require.False(t, entered)
		}
	}
}
