package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mediad/internal/config"
)

func TestLibraryDeletesOnlyOwnedFilesAndSurvivesStaleUpdatesAndRestart(t *testing.T) {
	root := t.TempDir()
	store, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	completed := time.Now().Add(-10 * time.Minute)
	task := &Task{ID: "mtask_library", UserID: "42", Status: StatusSucceeded, CallbackState: CallbackConfirmed, CompletedAt: &completed, DownloadToken: "private", Response: []byte(`{"data":[{"b64_json":"private"}]}`), Files: []ResultFile{{ID: "f0000", ContentType: "image/png"}, {ID: "f0001", ContentType: "image/png"}}}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "media")
	if err := os.MkdirAll(filepath.Join(media, task.ID), 0700); err != nil {
		t.Fatal(err)
	}
	for _, f := range task.Files {
		for _, suffix := range []string{"", ".json"} {
			if err := os.WriteFile(filepath.Join(media, task.ID, f.ID+suffix), []byte("media"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	tracker := NewTracker(store, nil, nil, nil)
	defer tracker.Stop()
	tracker.ConfigureFiles(media, 1024)
	h := NewManagementHandler(nil, store, tracker, "management")
	settings := config.Config{DataDir: root, ResultDir: media, PersistCredentials: true}
	h.Settings, err = config.OpenSettings(filepath.Join(root, "settings.json"), settings)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	request := func(path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	body := `{"user_id":42,"files":[{"task_id":"mtask_library","file_id":"f0000"}]}`
	if w := request("/management/files/delete", body, "wrong"); w.Code != 401 {
		t.Fatal("unprotected deletion", w.Code)
	}
	if w := request("/management/files/delete", strings.Replace(body, "42", "43", 1), "management"); strings.Contains(w.Body.String(), `"deleted":true`) {
		t.Fatal("cross-user deletion")
	}
	w := request("/management/files/status", body, "management")
	var status struct {
		Files []struct {
			Available bool      `json:"available"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"files"`
	}
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || len(status.Files) != 1 || !status.Files[0].Available || !status.Files[0].ExpiresAt.Equal(completed.Add(time.Hour)) {
		t.Fatal("wrong retention", w.Body.String())
	}
	if w := request("/management/files/delete", body, "management"); !strings.Contains(w.Body.String(), `"deleted":true`) {
		t.Fatal(w.Body.String())
	}
	for _, suffix := range []string{"", ".json"} {
		if _, err := os.Stat(filepath.Join(media, task.ID, "f0000"+suffix)); !os.IsNotExist(err) {
			t.Fatal("deleted file remains")
		}
	}
	if _, err := os.Stat(filepath.Join(media, task.ID, "f0001")); err != nil {
		t.Fatal("other result removed")
	}
	if err := store.Update(task); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	current, _ := reopened.Get(task.ID)
	if len(current.Files) != 1 || current.Files[0].ID != "f0001" || len(current.Response) != 0 || len(current.DeletedFileIDs) != 1 {
		t.Fatal("stale update restored deleted content")
	}
	result := httptest.NewRecorder()
	(&RelayHandler{tracker: tracker}).handleStandardResult(result, httptest.NewRequest("GET", "/", nil), current, "gemini")
	var remaining struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					FileData struct {
						URI string `json:"fileUri"`
					} `json:"fileData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if result.Code != 200 || json.Unmarshal(result.Body.Bytes(), &remaining) != nil || len(remaining.Candidates) != 1 || len(remaining.Candidates[0].Content.Parts) != 1 || remaining.Candidates[0].Content.Parts[0].FileData.URI != formattedDownloadURL(current.Files[0]) {
		t.Fatal("remaining Gemini image unavailable", result.Code, result.Body.String())
	}
	if w := request("/management/files/delete", body, "management"); !strings.Contains(w.Body.String(), `"deleted":true`) {
		t.Fatal("retry not idempotent", w.Body.String())
	}
	if err := store.deleteResultFile(task.ID, "../outside", "42", media); err == nil {
		t.Fatal("unsafe file accepted")
	}
	if err := store.deleteResultFile(task.ID, "f0001", "42", media); err != nil {
		t.Fatal(err)
	}
	current, _ = store.Get(task.ID)
	if !current.FilesAcknowledged || len(current.Files) != 0 {
		t.Fatal("last deletion not persisted")
	}
}

func TestLibraryRejectsOversizedBatch(t *testing.T) {
	h := NewManagementHandler(nil, nil, nil, "management")
	mux := http.NewServeMux()
	h.Register(mux)
	body := `{"user_id":42,"files":[` + strings.TrimSuffix(strings.Repeat(`{"task_id":"mtask_x","file_id":"f0000"},`, 51), ",") + `]}`
	r := httptest.NewRequest("POST", "/management/files/delete", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer management")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("oversized batch accepted", w.Code)
	}
}
