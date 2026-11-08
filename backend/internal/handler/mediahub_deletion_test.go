package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestMediaDeletionCallbackKeepsBillingAndOwnershipSeparate(t *testing.T) {
	links, db := mediaLinkFixture(t)
	bills := 0
	h := &MediaHubRelayHandler{links: links, cfg: MediaHubConfig{MediadBaseURL: "https://d.tokenfree.dev"}, billingSink: mediaResultSinkFunc(func(mediahub.SettleRequest) error { bills++; return nil })}
	h.settle = mediahub.NewSettleServer(func() string { return "secret" }, &mediaHubSettleChain{h: h}, time.Hour)
	h.settle.SetFileDeletionSink(links)
	post := func(body []byte, secret string) int {
		r := httptest.NewRequest("POST", "/internal/media/result", bytes.NewReader(body))
		stamp := strconv.FormatInt(time.Now().Unix(), 10)
		r.Header.Set(mediahub.HeaderTimestamp, stamp)
		r.Header.Set(mediahub.HeaderSignature, mediahub.Sign(secret, stamp, "POST", r.URL.Path, body))
		w := httptest.NewRecorder()
		h.settle.ServeHTTP(w, r)
		return w.Code
	}
	completed := []byte(`{"event":"task.completed","task_id":"mtask_delete","user_id":"42","key_id":"7","status":"succeeded","files":[{"file_id":"f0000","download_url":"https://d.tokenfree.dev/downloads/mtask_delete/f0000","size_bytes":5,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","content_type":"video/mp4"}]}`)
	if code := post(completed, "secret"); code != 200 || bills != 1 {
		t.Fatal(code, bills)
	}
	ev := mediahub.FileDeletionEvent{Event: "files.deleted", EventID: "mfd_test", TaskID: "mtask_delete", UserID: "42", KeyID: "7", FileIDs: []string{"f0000"}, DeletedAt: time.Now().UTC(), Reason: "retention_expired"}
	call := func(ev mediahub.FileDeletionEvent, secret string) int {
		body, _ := json.Marshal(ev)
		return post(body, secret)
	}
	if code := call(ev, "wrong"); code != 401 {
		t.Fatal("unsigned deletion accepted", code)
	}
	wrong := ev
	wrong.UserID = "43"
	if code := call(wrong, "secret"); code != 503 {
		t.Fatal("wrong owner accepted", code)
	}
	wrong = ev
	wrong.FileIDs = []string{"f0000", "f0001"}
	if code := call(wrong, "secret"); code != 503 {
		t.Fatal("unknown file accepted", code)
	}
	before, _ := links.List(context.Background(), 42, ev.TaskID)
	if len(before) != 1 || before[0].DeletedAt != nil {
		t.Fatal("partial deletion transaction committed")
	}
	for i := 0; i < 2; i++ {
		if code := call(ev, "secret"); code != 200 {
			t.Fatal("deletion not acknowledged independently of billing dedup", code)
		}
	}
	if bills != 1 {
		t.Fatal("file deletion charged user", bills)
	}
	if err := links.MarkReady(context.Background(), 42, ev.TaskID); err != nil {
		t.Fatal(err)
	}
	list, err := mediahub.NewResultLinkStore(db).List(context.Background(), 42, ev.TaskID)
	if err != nil || len(list) != 1 || list[0].DeletedAt == nil || list[0].Status != "deleted" || list[0].DownloadURL != "" || list[0].DeleteReason != "retention_expired" {
		t.Fatal("deleted state lost or download link exposed", list, err)
	}
	if other, _ := links.List(context.Background(), 43, ev.TaskID); len(other) != 0 {
		t.Fatal("other user sees deleted record")
	}
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer media.Close()
	h.cfg.MediadBaseURL = media.URL
	h.client = media.Client()
	controller := &MediaHubController{relay: h}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7})
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	})
	router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
	router.GET("/v1/videos/:request_id", func(c *gin.Context) { controller.TryStandardVideoTask(c, false) })
	router.GET("/media/tasks/:task_id", func(c *gin.Context) { h.taskStatus(c, 42, 7, 3, c.Param("task_id")) })
	router.GET("/media/files/*file", func(c *gin.Context) { h.serveFiles(c, 42, "/files"+c.Param("file")) })
	for _, path := range []string{"/v1/images/tasks/mtask_delete", "/v1/videos/mtask_delete", "/media/tasks/mtask_delete", "/media/files/" + list[0].ID + "/content"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 410 || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), `"deleted"`) {
			t.Fatal("deleted media still redirected or lost after task GC", path, w.Code, w.Body.String())
		}
	}
}
