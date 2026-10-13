package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetPublicResultDetailUsesInlinePelicanHTMLWithoutLength(t *testing.T) {
	const page = "<!doctype html><html><body><main><h1>coast</h1></main></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/test-results/4":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": 4,
				"group_id": 3,
				"group_name": "GPT Plus",
				"model": "gpt-6-astra",
				"status": "completed",
				"pelican_html": "` + page + `",
				"pelican_error": "",
				"candy_answer": "<final_answer>29</final_answer>",
				"candy_correct": true
			}`))
		case "/v1/selection":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"groups":[{"id":3,"name":"GPT Plus","interval_minutes":10}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := &IQDetectService{baseURL: srv.URL, client: srv.Client()}
	detail, err := svc.GetPublicResultDetail(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PelicanHTML != page {
		t.Fatalf("pelican html = %q", detail.PelicanHTML)
	}
	if !detail.PelicanOK {
		t.Fatal("expected pelican_ok")
	}
	if detail.CandyAnswer != "29" {
		t.Fatalf("candy answer = %q", detail.CandyAnswer)
	}
	if detail.CandyReview != "pending" || detail.PelicanReview != "pending" {
		t.Fatalf("reviews = %s / %s", detail.CandyReview, detail.PelicanReview)
	}
}

func TestGetPublicResultDetailFetchesPelicanWhenBodyMissing(t *testing.T) {
	const page = "<!doctype html><html><body><main><p>from-pelican</p></main></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/test-results/4":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": 4,
				"group_id": 3,
				"model": "gpt-6-astra",
				"status": "completed",
				"pelican_error": ""
			}`))
		case "/v1/test-results/4/pelican":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(page))
		case "/v1/selection":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"groups":[{"id":3,"interval_minutes":10}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := &IQDetectService{
		baseURL: srv.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
	detail, err := svc.GetPublicResultDetail(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PelicanHTML != page {
		t.Fatalf("pelican html = %q", detail.PelicanHTML)
	}
	if !detail.PelicanOK {
		t.Fatal("expected pelican_ok")
	}
}

func TestGetPublicOverviewReportsPendingReviewAndPelican(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/selection":
			_, _ = w.Write([]byte(`{"groups":[{"id":3,"name":"GPT Plus","interval_minutes":10}]}`))
		case "/v1/detection-models":
			_, _ = w.Write([]byte(`{"items":[{"name":"gpt-6-astra"}],"reasoning_effort":"high"}`))
		case "/v1/test-results":
			_, _ = w.Write([]byte(`{"items":[{
				"id": 5,
				"group_id": 3,
				"model": "gpt-6-astra",
				"status": "completed",
				"candy_correct": true,
				"pelican_html_len": 1200,
				"pelican_error": ""
			},{
				"id": 4,
				"group_id": 3,
				"model": "gpt-6-astra",
				"status": "completed",
				"candy_correct": false,
				"candy_review": "rejected",
				"pelican_review": "approved",
				"pelican_html_len": 80
			}],"total":2}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := &IQDetectService{baseURL: srv.URL, client: srv.Client()}
	overview, err := svc.GetPublicOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Groups) != 1 || len(overview.Groups[0].Results) != 2 {
		t.Fatalf("groups = %+v", overview.Groups)
	}
	pending := overview.Groups[0].Results[0]
	if !pending.PelicanOK {
		t.Fatal("expected pelican_ok from html length")
	}
	if pending.CandyReview != "pending" || pending.PelicanReview != "pending" {
		t.Fatalf("reviews = %s / %s", pending.CandyReview, pending.PelicanReview)
	}
	if pending.CandyCorrect == nil || !*pending.CandyCorrect {
		t.Fatal("automatic grade should still be returned")
	}
	reviewed := overview.Groups[0].Results[1]
	if reviewed.CandyReview != "rejected" || reviewed.PelicanReview != "approved" {
		t.Fatalf("reviews = %s / %s", reviewed.CandyReview, reviewed.PelicanReview)
	}
}
