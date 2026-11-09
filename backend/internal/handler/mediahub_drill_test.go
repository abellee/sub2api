package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func mediaLinkFixture(t *testing.T) (*mediahub.ResultLinkStore, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE users(id BIGINT PRIMARY KEY); INSERT INTO users(id) VALUES(42),(43);`); err != nil {
		t.Fatal(err)
	}
	raw, err := migrations.FS.ReadFile("250_media_generation_files.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.ReplaceAll(strings.Split(string(raw), "COMMENT ON")[0], "TIMESTAMPTZ", "TIMESTAMP")
	if _, err = db.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	raw, err = migrations.FS.ReadFile("251_media_file_deletion.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(strings.ReplaceAll(strings.ReplaceAll(string(raw), " IF NOT EXISTS", ""), "TIMESTAMPTZ", "TIMESTAMP")); err != nil {
		t.Fatal(err)
	}
	return mediahub.NewResultLinkStore(db), db
}

func TestMediaHubStoresUserLinksWithoutDownloadingAndRetries(t *testing.T) {
	links, db := mediaLinkFixture(t)
	ctx := context.Background()
	var requests, bills int
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "must not download or acknowledge", 500)
	}))
	defer remote.Close()
	h := &MediaHubRelayHandler{links: links, cfg: MediaHubConfig{MediadBaseURL: remote.URL}}
	h.billingSink = mediaResultSinkFunc(func(mediahub.SettleRequest) error {
		bills++
		if bills == 1 {
			return errors.New("billing unavailable")
		}
		return nil
	})
	req := mediahub.SettleRequest{UserID: 42, APIKeyID: 7, TaskID: "mtask_links", Status: "succeeded", Files: []mediahub.ResultFile{{ID: "f0000", DownloadURL: "/downloads/mtask_links/f0000?token=private", Size: 10, SHA256: strings.Repeat("a", 64), ContentType: "video/mp4"}}}
	chain := &mediaHubSettleChain{h: h}
	if chain.Settle(req) == nil {
		t.Fatal("billing failure ignored")
	}
	if list, err := links.List(ctx, 42, ""); err != nil || len(list) != 0 {
		t.Fatal("unbilled download exposed", err)
	}
	if err := chain.Settle(req); err != nil {
		t.Fatal(err)
	}
	list, err := links.List(ctx, 42, req.TaskID)
	if err != nil || len(list) != 1 || list[0].DownloadURL != remote.URL+req.Files[0].DownloadURL {
		t.Fatal("link not persisted", list, err)
	}
	if requests != 0 {
		t.Fatal("main site downloaded file bytes or sent destructive ACK")
	}
	if other, err := links.List(ctx, 43, ""); err != nil || len(other) != 0 {
		t.Fatal("another user can list files")
	}
	if _, err := links.Get(ctx, 43, list[0].ID); err != sql.ErrNoRows {
		t.Fatal("another user can get a file")
	}
	// Re-create the store to model a main-site restart; records remain in the table.
	restarted := mediahub.NewResultLinkStore(db)
	if again, err := restarted.List(ctx, 42, ""); err != nil || len(again) != 1 {
		t.Fatal("download records lost after restart")
	}
	conflicting := req
	conflicting.UserID = 43
	if links.Prepare(ctx, conflicting, remote.URL) == nil {
		t.Fatal("existing result owner overwritten")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM media_generation_files`).Scan(&count); err != nil || count != 1 {
		t.Fatal("retry duplicated file records")
	}
}

func TestMediaHubCallbackAndUserDownloadRedirect(t *testing.T) {
	links, _ := mediaLinkFixture(t)
	gin.SetMode(gin.TestMode)
	var bills int
	h := &MediaHubRelayHandler{links: links, cfg: MediaHubConfig{MediadBaseURL: "https://media.example.com"}, billingSink: mediaResultSinkFunc(func(mediahub.SettleRequest) error { bills++; return nil })}
	h.settle = mediahub.NewSettleServer(func() string { return "callback-secret" }, &mediaHubSettleChain{h: h}, time.Hour)
	body := []byte(`{"event":"task.completed","task_id":"mtask_callback","user_id":"42","key_id":"7","status":"succeeded","files":[{"file_id":"f0000","download_url":"https://media.example.com/downloads/mtask_callback/f0000?token=abc","size_bytes":5,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","content_type":"image/png"}]}`)
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/internal/media/result", bytes.NewReader(body))
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		r.Header.Set(mediahub.HeaderTimestamp, ts)
		r.Header.Set(mediahub.HeaderSignature, mediahub.Sign("callback-secret", ts, "POST", r.URL.Path, body))
		w := httptest.NewRecorder()
		h.settle.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	if bills != 1 {
		t.Fatal("duplicate notification billed more than once")
	}
	router := gin.New()
	router.GET("/files/*path", func(c *gin.Context) {
		user := int64(42)
		if c.GetHeader("X-Test-User") == "43" {
			user = 43
		}
		h.serveFiles(c, user, "/files"+c.Param("path"))
	})
	r := httptest.NewRequest("GET", "/files/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	var out struct {
		Data []mediahub.ResultLink `json:"data"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Data) != 1 {
		t.Fatal(w.Body.String())
	}
	endpoint := "/files/" + out.Data[0].ID + "/content"
	for _, user := range []string{"42", "43"} {
		r = httptest.NewRequest("GET", endpoint, nil)
		r.Header.Set("X-Test-User", user)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if user == "42" {
			if w.Code != 307 || w.Header().Get("Location") != out.Data[0].DownloadURL {
				t.Fatal("owner did not receive direct download redirect", w.Code)
			}
		} else if w.Code != 404 {
			t.Fatal("another user accessed download")
		}
	}
}
