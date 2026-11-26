package relay

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGeminiResultPreservesLargeSignaturesFromDisk(t *testing.T) {
	for _, signatureKey := range []string{"thoughtSignature", "thought_signature"} {
		for _, restart := range []bool{false, true} {
			t.Run(signatureKey+map[bool]string{false: "/current", true: "/restart"}[restart], func(t *testing.T) {
				signature := strings.Repeat("signature/+=", 110000)
				body, err := json.Marshal(map[string]any{
					"candidates": []any{map[string]any{
						"content": map[string]any{"role": "model", "parts": []any{
							map[string]any{"text": "generated image"},
							map[string]any{"inlineData": map[string]any{"mimeType": "image/jpeg", "data": "aW1hZ2U="}, signatureKey: signature},
						}}, "finishReason": "STOP",
					}}, "usageMetadata": map[string]any{"totalTokenCount": 1431},
				})
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				store, err := NewDiskStore(root)
				if err != nil {
					t.Fatal(err)
				}
				task := &Task{ID: "mtask_signature", UserID: "2", KeyID: "2", CreatedAt: time.Now(),
					Status: StatusSucceeded, CallbackState: CallbackConfirmed, Response: body,
					Files: []ResultFile{{ID: "f0000", ContentType: "image/jpeg", DownloadURL: "https://media.example/downloads/mtask_signature/f0000.jpg?token=test"}},
				}
				if err = store.Create(task); err != nil {
					t.Fatal(err)
				}
				if restart {
					store, err = NewDiskStore(root)
					if err != nil {
						t.Fatal(err)
					}
				}
				task, _ = store.Get(task.ID)
				if task.ResponseData == nil || len(task.Response) != 0 || len(task.ResponseSummary) != 0 {
					t.Fatal("large response must stay on disk without enlarging the cached summary")
				}
				tracker := NewTracker(store, nil, nil, nil)
				defer tracker.Stop()
				handler := NewRelayHandler(nil, store, tracker, relaySecret, nil)
				w := httptest.NewRecorder()
				handler.handleStandardResult(w, httptest.NewRequest("GET", "/", nil), task, "gemini")
				if w.Code != 200 {
					t.Fatalf("result returned %d: %s", w.Code, w.Body.String())
				}
				var got struct {
					Candidates []struct {
						Content      struct{ Parts []map[string]json.RawMessage }
						FinishReason string
					}
					UsageMetadata struct{ TotalTokenCount int }
					DownloadURLs  []string `json:"download_urls"`
				}
				if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if len(got.Candidates) != 1 || len(got.Candidates[0].Content.Parts) != 2 || len(got.DownloadURLs) != 1 {
					t.Fatal("Gemini candidates, text, or downloads lost")
				}
				parts := got.Candidates[0].Content.Parts
				var savedSignature string
				if err = json.Unmarshal(parts[1][signatureKey], &savedSignature); err != nil || savedSignature != signature {
					t.Fatal("thought signature changed")
				}
				if string(parts[0]["text"]) != `"generated image"` || got.Candidates[0].FinishReason != "STOP" || got.UsageMetadata.TotalTokenCount != 1431 {
					t.Fatal("text, finish reason, or usage changed")
				}
				var file struct{ FileURI, MimeType string }
				if err = json.Unmarshal(parts[1]["fileData"], &file); err != nil || file.FileURI != task.Files[0].DownloadURL || file.MimeType != "image/jpeg" {
					t.Fatal("image download link changed")
				}
				if bytes.Contains(w.Body.Bytes(), []byte("inlineData")) || bytes.Contains(w.Body.Bytes(), []byte("aW1hZ2U=")) {
					t.Fatal("image body was not replaced with the download link")
				}
				if len(task.Response) != 0 || len(task.ResponseSummary) != 0 {
					t.Fatal("serving the result retained the large response")
				}
			})
		}
	}
}
