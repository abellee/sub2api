package iqdetectd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestResultDetailPreservesStoredHTMLAndListLength(t *testing.T) {
	for _, html := range []string{"", "<html>鹈鹕 🐦</html>"} {
		t.Run(html, func(t *testing.T) {
			srv, store := newTestServer(t)
			ctx := context.Background()
			id, err := store.InsertRunningTestResult(ctx, TestResult{AccountID: 1, Model: "test-model"})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.CompleteTestResult(ctx, id, TestResult{PelicanHTML: html}); err != nil {
				t.Fatal(err)
			}
			handler := srv.Handler()
			detailResponse := httptest.NewRecorder()
			handler.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/v1/test-results/"+strconv.FormatInt(id, 10), nil))
			if detailResponse.Code != http.StatusOK {
				t.Fatalf("detail status = %d: %s", detailResponse.Code, detailResponse.Body.String())
			}
			var detail TestResult
			if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			listResponse := httptest.NewRecorder()
			handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/v1/test-results", nil))
			if listResponse.Code != http.StatusOK {
				t.Fatalf("list status = %d: %s", listResponse.Code, listResponse.Body.String())
			}
			var list struct {
				Items []TestResult `json:"items"`
			}
			if err := json.Unmarshal(listResponse.Body.Bytes(), &list); err != nil {
				t.Fatal(err)
			}
			if len(list.Items) != 1 {
				t.Fatalf("list count = %d, want 1", len(list.Items))
			}
			if detail.PelicanHTML != html || detail.PelicanHTMLLen != int64(utf8.RuneCountInString(html)) {
				t.Fatalf("detail HTML = %q, length = %d", detail.PelicanHTML, detail.PelicanHTMLLen)
			}
			if detail.PelicanHTMLLen != list.Items[0].PelicanHTMLLen {
				t.Fatalf("detail length %d differs from list length %d", detail.PelicanHTMLLen, list.Items[0].PelicanHTMLLen)
			}
		})
	}
}

func newTestServer(t *testing.T) (*Server, *Store) {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "iqdetect.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	runner := NewRunner(store, "http://127.0.0.1:1", "", time.Second, nil)
	srv := &Server{Store: store, Runner: runner, StartedAt: time.Now().UTC(), Version: "test"}
	return srv, store
}

func TestServerHealthAndModels(t *testing.T) {
	srv, store := newTestServer(t)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/models", strings.NewReader(`{"name":"m-1","label":"M1"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create model status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created MonitoredModel
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created model: %v", err)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list models status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/models/"+strconv.FormatInt(created.ID, 10), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete model status = %d", rec.Code)
	}
	_ = store
}

func TestServerProbesAndScores(t *testing.T) {
	srv, _ := newTestServer(t)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/probes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("probes status = %d", rec.Code)
	}
	var payload struct {
		Items []ProbeInfo `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode probes: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected probes")
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/scores", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scores status = %d", rec.Code)
	}
}

func TestServerTriggerRunRequiresConfig(t *testing.T) {
	srv, _ := newTestServer(t)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("trigger run without config status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

// TestRunnerExecuteAgainstStubUpstream drives a full run against a fake
// OpenAI-compatible endpoint that always answers "42".
func TestRunnerExecuteAgainstStubUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "42"}},
			},
		})
	}))
	defer upstream.Close()

	store, err := OpenStore(filepath.Join(t.TempDir(), "iqdetect.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	if _, err := store.CreateModel(ctx, CreateModelInput{Name: "stub-model"}); err != nil {
		t.Fatalf("create model: %v", err)
	}

	runner := NewRunner(store, upstream.URL, "sk-test", 5*time.Second, nil)
	run, err := runner.TriggerRun(ctx, TriggerRunInput{})
	if err != nil {
		t.Fatalf("trigger run: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		detail, err := store.GetRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if detail.Status == RunStatusCompleted || detail.Status == RunStatusFailed {
			if detail.Status != RunStatusCompleted {
				t.Fatalf("run status = %s, err=%s", detail.Status, detail.Error)
			}
			results, err := store.ListResultsByRun(ctx, run.ID)
			if err != nil {
				t.Fatalf("list results: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("results = %d, want 1", len(results))
			}
			res := results[0]
			if res.Model != "stub-model" || res.Total != len(builtinProbes()) {
				t.Errorf("unexpected result: %+v", res)
			}
			if res.Correct < 1 || res.IQ <= 40 {
				t.Errorf("expected at least one correct answer, got %+v", res)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for run to finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
