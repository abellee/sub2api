package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestMediaLibraryIsolationPaginationAndDeletion(t *testing.T) {
	links, db := mediaLinkFixture(t)
	ctx := context.Background()
	for i := 0; i < 205; i++ {
		req := mediahub.SettleRequest{UserID: 42, APIKeyID: 7, TaskID: fmt.Sprintf("mtask_library_%d", i), Files: []mediahub.ResultFile{{ID: "f0000", DownloadURL: "https://media.example/download", Size: 1, SHA256: strings.Repeat("a", 64), ContentType: "image/png"}}}
		if i%4 == 0 {
			req.Files[0].ContentType = "video/mp4"
		}
		if err := links.Prepare(ctx, req, "https://media.example"); err != nil {
			t.Fatal(err)
		}
		if err := links.MarkReady(ctx, 42, req.TaskID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE media_generation_files SET created_at=$1 WHERE task_id=$2`, time.Date(2026, 11, 9, 0, 0, i/2, 0, time.UTC), req.TaskID); err != nil {
			t.Fatal(err)
		}
	}
	var deletedRefs int
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Error("missing management credential")
		}
		var in struct {
			UserID int64 `json:"user_id"`
			Files  []struct {
				TaskID string `json:"task_id"`
				FileID string `json:"file_id"`
			} `json:"files"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil || in.UserID != 42 || len(in.Files) > 50 {
			t.Error("invalid owner or batch")
		}
		states := make([]libraryFileState, 0, len(in.Files))
		for _, file := range in.Files {
			expiry := time.Now().Add(time.Hour)
			states = append(states, libraryFileState{TaskID: file.TaskID, FileID: file.FileID, Available: true, ExpiresAt: &expiry, Deleted: true})
		}
		if r.URL.Path == "/management/files/delete" {
			deletedRefs += len(in.Files)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": states})
	}))
	defer remote.Close()
	defaults := DefaultMediaHubSettings()
	defaults.Enabled = false
	defaults.MediadBaseURL = remote.URL
	defaults.ManagementToken = "private"
	defaults.StorePath = filepath.Join(t.TempDir(), "bindings.json")
	h, err := NewMediaHubController(filepath.Join(t.TempDir(), "settings.json"), defaults, nil, links)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	user := router.Group("", func(c *gin.Context) {
		if c.GetHeader("Authorization") != "" {
			id := int64(42)
			if c.GetHeader("Authorization") == "other" {
				id = 43
			}
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: id})
		}
	})
	h.RegisterUser(user)
	request := func(method, path, body, auth string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/media-files", "", ""); w.Code != 401 {
		t.Fatal("anonymous list", w.Code)
	}
	ids := []string{}
	cursor := ""
	for {
		w := request("GET", "/media-files?after="+cursor, "", "owner")
		var out struct {
			Data struct {
				Items []struct {
					ID        string     `json:"id"`
					ExpiresAt *time.Time `json:"expires_at"`
				} `json:"items"`
				Next string `json:"next_cursor"`
			} `json:"data"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal(w.Body.String())
		}
		for _, item := range out.Data.Items {
			if item.ExpiresAt == nil {
				t.Fatal("missing countdown")
			}
			ids = append(ids, item.ID)
		}
		cursor = out.Data.Next
		if cursor == "" {
			break
		}
	}
	if len(ids) != 205 {
		t.Fatal("library truncated", len(ids))
	}
	seen := map[string]bool{}
	var previous *mediahub.ResultLink
	for _, id := range ids {
		record, err := links.Get(ctx, 42, id)
		if err != nil || seen[id] || (previous != nil && (record.CreatedAt.After(previous.CreatedAt) || (record.CreatedAt.Equal(previous.CreatedAt) && record.ID > previous.ID))) {
			t.Fatal("pagination skipped, repeated or reordered results", id, err)
		}
		seen[id], previous = true, record
	}
	videoCount, after := 0, ""
	for {
		w := request("GET", "/media-files?kind=video&after="+after, "", "owner")
		var out struct {
			Data struct {
				Items []mediahub.ResultLink `json:"items"`
				Next  string                `json:"next_cursor"`
			} `json:"data"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal(w.Body.String())
		}
		for _, item := range out.Data.Items {
			if item.ContentType != "video/mp4" {
				t.Fatal("wrong filtered media type")
			}
		}
		videoCount += len(out.Data.Items)
		after = out.Data.Next
		if after == "" {
			break
		}
	}
	if videoCount != 52 {
		t.Fatal("filtered pagination truncated", videoCount)
	}
	if w := request("GET", "/media-files?kind=invalid", "", "owner"); w.Code != 400 {
		t.Fatal("invalid filter accepted")
	}
	body := fmt.Sprintf(`{"ids":[%q]}`, ids[0])
	if w := request("POST", "/media-files/delete", body, "other"); w.Code != 200 || deletedRefs != 0 {
		t.Fatal("cross-user deletion reached media service")
	}
	if w := request("POST", "/media-files/delete", body, "owner"); w.Code != 200 || deletedRefs != 1 {
		t.Fatal("file not deleted", w.Body.String())
	}
	if err := links.MarkReady(ctx, 42, "mtask_library_0"); err != nil {
		t.Fatal(err)
	}
	if record, err := links.Get(ctx, 42, ids[0]); err != nil || record.Status != "deleted" || record.DownloadURL != "" || record.DeleteReason != "user_deleted" {
		t.Fatal("deleted record visible")
	}
	if w := request("POST", "/media-files/delete", body, "owner"); w.Code != 200 || deletedRefs != 1 {
		t.Fatal("retry was not idempotent")
	}
	var tombstones int
	if err := db.QueryRow(`SELECT COUNT(*) FROM media_generation_files WHERE deleted_at IS NOT NULL`).Scan(&tombstones); err != nil || tombstones != 1 {
		t.Fatal("deletion marker missing")
	}
	large, _ := json.Marshal(map[string]any{"ids": ids[:51]})
	if w := request("POST", "/media-files/delete", string(large), "owner"); w.Code != 400 {
		t.Fatal("oversized batch accepted")
	}
}
