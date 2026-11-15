package relay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCompletedRequestsReleasedWithoutLosingRecoveryOrResults(t *testing.T) {
	for _, status := range []string{StatusQueued, StatusSubmitting, StatusProcessing, StatusDownloading, StatusRecoveryRequired, StatusInterrupted, StatusSucceeded, StatusFailed} {
		for _, operation := range []string{"create", "update", "legacy_restart"} {
			t.Run(status+"/"+operation, func(t *testing.T) {
				root := t.TempDir()
				store, err := NewDiskStore(root)
				if err != nil {
					t.Fatal(err)
				}
				task := &Task{ID: "mtask_memory", UserID: "2", KeyID: "test-key", Status: status,
					CallbackState: CallbackPending, Credential: "recovery-credential", Fingerprint: "same-request", IdempotencyKey: "same-key",
					Request:  &UpstreamRequest{Method: "POST", URL: "https://upstream.example/images", Header: http.Header{"Authorization": {"Bearer recovery-credential"}}, Body: bytes.Repeat([]byte("reference-image"), 1<<16)},
					Response: []byte(`{"data":[{"url":"https://result.example/image.png"}]}`),
					Result:   map[string]any{"url": "https://result.example/image.png"}, Usage: &Usage{Quantity: 1, Unit: UnitImage},
					Files: []ResultFile{{ID: "f0000", DownloadURL: "https://media.example/download"}}}
				want := cloneTask(task)
				if status == StatusSucceeded || status == StatusFailed {
					want.Request = nil
				}
				switch operation {
				case "create":
					err = store.Create(task)
				case "update":
					task.Status = StatusQueued
					if err = store.Create(task); err != nil {
						t.Fatal(err)
					}
					task.Status = status
					err = store.Update(task)
				case "legacy_restart":
					err = WriteState(filepath.Join(root, task.ID+".json"), task)
				}
				if err != nil {
					t.Fatal(err)
				}
				if operation != "legacy_restart" {
					got, _ := store.Get(task.ID)
					if !reflect.DeepEqual(got, want) {
						t.Fatal("in-memory request retention changed recovery data or results")
					}
				}
				reopened, err := NewDiskStore(root)
				if err != nil {
					t.Fatal(err)
				}
				got, _ := reopened.Get(task.ID)
				if !reflect.DeepEqual(got, want) {
					t.Fatal("restart changed recovery data or results")
				}
				data, err := os.ReadFile(filepath.Join(root, task.ID+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var saved Task
				if err := json.Unmarshal(data, &saved); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(&saved, want) {
					t.Fatal("disk still retains a completed request or lost recovery data")
				}
				duplicate, ok := reopened.FindByIdempotencyKey(task.IdempotencyKey)
				if !ok || duplicate.Fingerprint != want.Fingerprint {
					t.Fatal("duplicate request protection lost")
				}
			})
		}
	}
}
