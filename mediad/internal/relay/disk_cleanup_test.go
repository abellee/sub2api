package relay

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func diskCleanupFixture(t *testing.T) (*MemoryStore, string) {
	t.Helper()
	root := t.TempDir()
	store, err := NewDiskStore(filepath.Join(root, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "media")
	if err := os.MkdirAll(media, 0700); err != nil {
		t.Fatal(err)
	}
	return store, media
}

func addDiskCleanupTask(t *testing.T, store *MemoryStore, root, id string, age time.Duration, status, callback string, contentTypes ...string) *Task {
	t.Helper()
	completed := time.Now().Add(-age)
	task := &Task{ID: id, UserID: "2", KeyID: "7", Status: status, CallbackState: callback, CompletedAt: &completed, Response: []byte(`{"data":[{"b64_json":"private"}]}`)}
	for i, ct := range contentTypes {
		file := ResultFile{ID: []string{"f0000", "f0001", "f0002"}[i], ContentType: ct}
		task.Files = append(task.Files, file)
		if err := os.MkdirAll(filepath.Join(root, id), 0700); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{"", ".json"} {
			if err := os.WriteFile(filepath.Join(root, id, file.ID+suffix), []byte("media"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestDiskCleanupUsesCompletionOrderAndStopsAfterEnoughSpace(t *testing.T) {
	store, media := diskCleanupFixture(t)
	old := addDiskCleanupTask(t, store, media, "mtask_old", 2*time.Minute, StatusSucceeded, CallbackConfirmed, "image/png", "video/mp4", "image/jpeg")
	addDiskCleanupTask(t, store, media, "mtask_new", time.Minute, StatusSucceeded, CallbackConfirmed, "image/png")
	addDiskCleanupTask(t, store, media, "mtask_unconfirmed", time.Hour, StatusSucceeded, CallbackPending, "image/png")
	addDiskCleanupTask(t, store, media, "mtask_active", time.Hour, StatusDownloading, CallbackConfirmed, "image/png")
	addDiskCleanupTask(t, store, media, "mtask_failed", time.Hour, StatusFailed, CallbackConfirmed, "image/png")
	addDiskCleanupTask(t, store, media, "mtask_video", time.Hour, StatusSucceeded, CallbackConfirmed, "video/mp4")
	unfinished := addDiskCleanupTask(t, store, media, "mtask_unfinished", time.Hour, StatusSucceeded, CallbackConfirmed, "image/png")
	unfinished.CompletedAt = nil
	if err := store.Update(unfinished); err != nil {
		t.Fatal(err)
	}
	usage := func(path string) (float64, error) {
		if path != media {
			t.Fatal("wrong filesystem measured", path)
		}
		used := 92.0
		for _, id := range []string{"f0000", "f0002"} {
			if _, err := os.Stat(filepath.Join(media, old.ID, id)); os.IsNotExist(err) {
				used -= 5
			}
		}
		return used, nil
	}
	if n, err := store.gcImagesForDisk(85, media, usage); err != nil || n != 2 {
		t.Fatalf("cleanup: %d %v", n, err)
	}
	for _, id := range []string{"f0000", "f0002"} {
		for _, suffix := range []string{"", ".json"} {
			if _, err := os.Stat(filepath.Join(media, old.ID, id+suffix)); !os.IsNotExist(err) {
				t.Fatal("old image remains", id, suffix)
			}
		}
	}
	for _, id := range []string{"mtask_new", "mtask_unconfirmed", "mtask_active", "mtask_failed", "mtask_video", "mtask_unfinished"} {
		if _, err := os.Stat(filepath.Join(media, id, "f0000")); err != nil {
			t.Fatal("protected media removed", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(media, old.ID, "f0001")); err != nil {
		t.Fatal("video in mixed task removed", err)
	}
	if err := store.Update(old); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewDiskStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := reopened.Get(old.ID)
	if len(current.Files) != 1 || current.Files[0].ContentType != "video/mp4" || current.FilesAcknowledged || current.FilesDeletedAt != nil || current.ResponseData != nil || len(current.Response) != 0 {
		t.Fatal("stale update/restart restored deleted images or expired the remaining video")
	}
	notices := reopened.deletions.pending(time.Now())
	if len(notices) != 2 {
		t.Fatal("deletion notifications were not persisted", len(notices))
	}
	for _, notice := range notices {
		if notice.Event.Reason != "disk_pressure" || notice.Event.TaskID != old.ID {
			t.Fatal("wrong deletion event", notice.Event)
		}
	}
}

func TestDiskCleanupRejectsUnsafePathsBeforeDeletion(t *testing.T) {
	store, media := diskCleanupFixture(t)
	for _, ids := range [][2]string{{"../outside", "f0000"}, {"mtask_safe", "../outside"}} {
		event := FileDeletionEvent{TaskID: ids[0], FileIDs: []string{ids[1]}}
		if err := store.deleteDiskImage(media, event); err == nil {
			t.Fatal("unsafe cleanup path accepted", ids)
		}
	}
	if len(store.deletions.items) != 0 {
		t.Fatal("unsafe deletion was queued")
	}
}

func TestDiskCleanupDoesNotDeleteWhenDisabledOrUsageCannotBeTrusted(t *testing.T) {
	for _, tc := range []struct {
		name      string
		threshold int
		used      float64
		err       error
		wantError bool
	}{
		{"disabled", 0, 99, nil, false},
		{"below", 85, 84, nil, false},
		{"equal", 85, 85, nil, false},
		{"read failed", 85, 0, errors.New("disk read failed"), true},
		{"invalid measurement", 85, math.NaN(), nil, true},
		{"invalid threshold", 101, 99, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, media := diskCleanupFixture(t)
			addDiskCleanupTask(t, store, media, "mtask_image", time.Minute, StatusSucceeded, CallbackConfirmed, "image/png")
			usage := func(string) (float64, error) {
				if tc.threshold == 0 {
					t.Fatal("disabled cleanup measured disk")
				}
				return tc.used, tc.err
			}
			n, err := store.gcImagesForDisk(tc.threshold, media, usage)
			if n != 0 || (err != nil) != tc.wantError {
				t.Fatal(n, err)
			}
			if _, err := os.Stat(filepath.Join(media, "mtask_image", "f0000")); err != nil {
				t.Fatal("file deleted", err)
			}
		})
	}
}

func TestDiskCleanupIntentPreventsLossAndResumesAfterRestart(t *testing.T) {
	store, media := diskCleanupFixture(t)
	task := addDiskCleanupTask(t, store, media, "mtask_resume", time.Minute, StatusSucceeded, CallbackConfirmed, "image/png")
	queueRoot := store.deletions.root
	blocked := filepath.Join(t.TempDir(), "not-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	store.deletions.root = blocked
	if _, err := store.gcImagesForDisk(85, media, func(string) (float64, error) { return 99, nil }); err == nil {
		t.Fatal("failed intent write ignored")
	}
	file := filepath.Join(media, task.ID, "f0000")
	if _, err := os.Stat(file); err != nil {
		t.Fatal("image removed before intent was saved")
	}
	store.deletions.root = queueRoot
	event := newDeletionEvent(task, []string{"f0000"}, time.Now(), "disk_pressure")
	if err := store.deletions.put(event, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewDiskStore(store.root)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := reopened.gcImagesForDisk(0, media, func(string) (float64, error) { t.Fatal("disabled cleanup read disk"); return 0, nil }); err != nil || n != 1 {
		t.Fatal("interrupted deletion did not finish", n, err)
	}
	current, _ := reopened.Get(task.ID)
	if len(current.Files) != 0 || !current.FilesAcknowledged || current.FilesDeletedAt == nil || len(reopened.deletions.pending(time.Now())) != 1 {
		t.Fatal("interrupted deletion lost metadata or notification")
	}
}

func TestDiskUsageMeasuresMediaFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("disk measurement is supported on deployment and development platforms")
	}
	root := t.TempDir()
	used, err := diskUsedPercent(root)
	if err != nil || used < 0 || used > 100 || math.IsNaN(used) {
		t.Fatal(used, err)
	}
	if _, err := diskUsedPercent(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing media filesystem accepted")
	}
}
