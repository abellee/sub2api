//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDocumentationPublishedAndCategoryFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewGroupCategoryStore(filepath.Join(t.TempDir(), "config.db"))
	snap, err := store.Documentation(context.Background())
	require.NoError(t, err)
	snap.Articles[0].Published = false
	snap.Articles[0].Content = "PRIVATE DRAFT"
	snap.Categories = append(snap.Categories, service.DocumentationCategory{ID: "draft-only", Name: "Draft only"})
	snap.Categories = append(snap.Categories, service.DocumentationCategory{ID: "empty", Name: "Empty category"})
	snap.Articles[0].CategoryID = "draft-only"
	require.NoError(t, store.SaveDocumentation(context.Background(), snap))
	h := NewDocumentationHandler(store)
	r := gin.New()
	r.GET("/documentation", h.ListPublished)
	r.GET("/categories", h.Categories)
	r.GET("/admin", h.Get)
	for _, tc := range []struct {
		path  string
		count int
		draft bool
	}{
		{"/documentation", 9, false}, {"/documentation?category=image", 7, false}, {"/documentation?category=video", 2, false}, {"/documentation?category=draft-only", 0, false}, {"/documentation?category=empty", 0, false}, {"/documentation?category=missing", 0, false}, {"/admin", 10, true},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		require.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Data service.DocumentationSnapshot `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Len(t, body.Data.Articles, tc.count)
		if tc.path == "/documentation?category=video" {
			for _, article := range body.Data.Articles {
				require.Equal(t, "video", article.CategoryID)
				require.NotEmpty(t, article.Content)
				require.NotEmpty(t, article.Agent.Template)
			}
		}
		if !tc.draft {
			require.NotContains(t, w.Body.String(), "PRIVATE DRAFT")
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/categories", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data []service.DocumentationCategory `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, snap.Categories, body.Data, "return all configured categories in order, including empty categories")
	require.NotContains(t, w.Body.String(), "PRIVATE DRAFT")
}

func TestDocumentationUpdateValidationAndConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewGroupCategoryStore(filepath.Join(t.TempDir(), "config.db"))
	h := NewDocumentationHandler(store)
	r := gin.New()
	r.PUT("/admin", h.Update)
	snap, err := store.Documentation(context.Background())
	require.NoError(t, err)
	raw, err := json.Marshal(snap)
	require.NoError(t, err)
	for _, tc := range []struct {
		body   []byte
		status int
	}{
		{[]byte(`{}`), http.StatusBadRequest}, {[]byte(`{"articles":[]}`), http.StatusBadRequest},
		{raw, http.StatusOK}, {raw, http.StatusConflict},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/admin", bytes.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code, w.Body.String())
	}
}
