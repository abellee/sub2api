package relay

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetainedDownloadRequiresConfirmationAndPrivateToken(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "tasks")
	files := filepath.Join(root, "media")
	store, err := NewDiskStore(state)
	if err != nil {
		t.Fatal(err)
	}
	task := &Task{ID: "mtask_download", Status: StatusSucceeded, UserID: "42", KeyID: "7", CreatedAt: time.Now(), CallbackState: CallbackPending, Files: []ResultFile{{ID: "f0000", Size: 5, ContentType: "image/png"}, {ID: "f0001", Size: 5, ContentType: "image/png"}}}
	if err = store.Create(task); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(files, task.ID), 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"f0000", "f0001"} {
		if err = os.WriteFile(filepath.Join(files, task.ID, id), []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	tracker := NewTracker(store, func(string) (*Schema, bool) { return nil, false }, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(files, 1024)
	tracker.ConfigureDownloads("https://media.example.com")
	if err = tracker.prepareDownloadLinks(task); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(task.Files[0].DownloadURL)
	if !strings.HasSuffix(u.Path, "/f0000.png") {
		t.Fatal("download URL has no real extension", u.Path)
	}
	request := func(s *MemoryStore, path string) *httptest.ResponseRecorder {
		tr := NewTracker(s, func(string) (*Schema, bool) { return nil, false }, nil, nil)
		defer tr.Stop()
		tr.ConfigureFiles(files, 1024)
		mux := http.NewServeMux()
		NewRelayHandler(nil, s, tr, "relay", nil).Register(mux)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	if w := request(store, u.RequestURI()); w.Code != 404 {
		t.Fatal("unconfirmed download exposed")
	}
	task.CallbackState = CallbackConfirmed
	if err = store.Update(task); err != nil {
		t.Fatal(err)
	}
	if w := request(store, u.RequestURI()); w.Code != 200 || w.Body.String() != "image" {
		t.Fatal("confirmed download unavailable", w.Code)
	}
	if w := request(store, u.RequestURI()); !strings.Contains(w.Header().Get("Content-Disposition"), "f0000.png") {
		t.Fatal("download filename has no extension")
	}
	legacy := strings.Replace(u.RequestURI(), "f0000.png", "f0000", 1)
	if w := request(store, legacy); w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "f0000.png") {
		t.Fatal("old download broken", w.Code)
	}
	if w := request(store, strings.Replace(u.RequestURI(), "f0000.png", "f0000.gif", 1)); w.Code != 404 {
		t.Fatal("wrong extension accepted")
	}
	if w := request(store, u.Path); w.Code != 404 {
		t.Fatal("missing token accepted")
	}
	if w := request(store, strings.Replace(u.RequestURI(), "f0000", "f0001", 1)); w.Code != 404 {
		t.Fatal("token reused for another file")
	}
	reopened, err := NewDiskStore(state)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(reopened, u.RequestURI()); w.Code != 200 {
		t.Fatal("restart invalidated permanent link")
	}
	if n := reopened.GC(time.Nanosecond); n != 0 {
		t.Fatal("retained media task was collected")
	}
	if _, err = os.Stat(filepath.Join(files, task.ID, "f0000")); err != nil {
		t.Fatal("media file removed after result notification")
	}
	legacyMux := http.NewServeMux()
	NewRelayHandler(nil, store, tracker, relaySecret, nil).Register(legacyMux)
	ack := signedRequest(t, "POST", "/relay/tasks/"+task.ID+"/files/ack", nil, map[string]string{HeaderUserID: "42", HeaderKeyID: "7"})
	w := httptest.NewRecorder()
	legacyMux.ServeHTTP(w, ack)
	if w.Code != 410 {
		t.Fatal("legacy acknowledgement accepted", w.Code)
	}
	if _, err = os.Stat(filepath.Join(files, task.ID, "f0000")); err != nil {
		t.Fatal("legacy acknowledgement deleted retained file")
	}
}

func TestDownloadFilenameFormats(t *testing.T) {
	for ct, ext := range map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/svg+xml": ".svg", "image/webp": ".webp", "video/mp4": ".mp4", "application/octet-stream": ".bin"} {
		file := ResultFile{ID: "f0000", ContentType: ct, DownloadURL: "https://d.tokenfree.dev/downloads/task/f0000?token=original"}
		if downloadFilename(file) != "f0000"+ext {
			t.Fatal(ct)
		}
		u, _ := url.Parse(formattedDownloadURL(file))
		if !strings.HasSuffix(u.Path, ext) || u.Query().Get("token") != "original" {
			t.Fatal("URL or signature changed", ct)
		}
		file.DownloadURL = u.String()
		if formattedDownloadURL(file) != file.DownloadURL {
			t.Fatal("duplicate extension")
		}
	}
}
