package iqdetectd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPublicIQDetectionOverviewAndDetail(t *testing.T) {
	srv, store := newTestServer(t)
	ctx := context.Background()
	visible := false
	if _, err := store.SaveSelection(ctx, []SelectionGroup{
		{
			ID: 3, Name: "GPT Plus", IntervalMinutes: 10,
			Accounts: []SelectionAccount{{ID: 9, Name: "SECRET-ACCOUNT"}},
		},
		{ID: 4, Name: "manual only", IntervalMinutes: 0},
		{ID: 5, Name: "not public", IntervalMinutes: 10, Visible: &visible},
	}); err != nil {
		t.Fatalf("save selection: %v", err)
	}
	if _, err := store.ReplaceDetectionModels(ctx, []string{"gpt-6-astra"}); err != nil {
		t.Fatalf("replace models: %v", err)
	}
	if err := store.UpsertDetectionPrompts(ctx, DetectionPrompts{
		ReasoningEffort: "high",
		CandyPrompt:     "SECRET-PROMPT",
		CandyAnswer:     "SECRET-ANSWER",
		QuietWindows:    []QuietWindow{{Start: "23:00", End: "07:00"}, {Start: "12:00", End: "13:00"}},
	}); err != nil {
		t.Fatalf("save prompts: %v", err)
	}

	correct := true
	var ids []int64
	for i := 0; i < 49; i++ {
		id, err := store.InsertRunningTestResult(ctx, TestResult{
			GroupID: 3, GroupName: "GPT Plus", AccountID: 9, AccountName: "SECRET-ACCOUNT",
			Model: "gpt-6-astra",
		})
		if err != nil {
			t.Fatalf("insert result: %v", err)
		}
		ids = append(ids, id)
		done := TestResult{
			PelicanHTML:      "<p>PELICAN-BODY-" + strconv.Itoa(i) + "</p>",
			PelicanLatencyMS: 1500,
			CandyAnswer:      "work <final_answer>21</final_answer>",
			CandyCorrect:     &correct,
			CandyLatencyMS:   800,
		}
		if err := store.CompleteTestResult(ctx, id, done); err != nil {
			t.Fatalf("complete result: %v", err)
		}
	}
	if err := store.UpdateTestResultReview(ctx, ids[48], "candy", "rejected"); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := store.UpdateTestResultReview(ctx, ids[48], "pelican", "approved"); err != nil {
		t.Fatalf("review: %v", err)
	}
	hiddenID, err := store.InsertRunningTestResult(ctx, TestResult{
		GroupID: 4, GroupName: "manual only", AccountID: 9, AccountName: "SECRET-ACCOUNT",
		Model: "gpt-6-astra",
	})
	if err != nil {
		t.Fatalf("insert hidden: %v", err)
	}
	if err := store.CompleteTestResult(ctx, hiddenID, TestResult{
		PelicanHTML: "<p>HIDDEN</p>",
		CandyAnswer: "<final_answer>9</final_answer>",
	}); err != nil {
		t.Fatalf("complete hidden: %v", err)
	}

	handler := srv.Handler()
	overviewRec := httptest.NewRecorder()
	overviewReq := httptest.NewRequest(http.MethodGet, "/iq-detection/overview", nil)
	handler.ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("overview status = %d body=%s", overviewRec.Code, overviewRec.Body.String())
	}
	if got := overviewRec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("overview CORS = %q", got)
	}
	var overviewEnv struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    publicOverview `json:"data"`
	}
	if err := json.Unmarshal(overviewRec.Body.Bytes(), &overviewEnv); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if overviewEnv.Code != 0 || overviewEnv.Message != "success" {
		t.Fatalf("overview envelope = %+v", overviewEnv)
	}
	rawOverview := overviewRec.Body.String()
	for _, secret := range []string{"SECRET-ACCOUNT", "SECRET-PROMPT", "SECRET-ANSWER", "PELICAN-BODY", "account_id", "final_answer"} {
		if strings.Contains(rawOverview, secret) {
			t.Fatalf("overview leaked %s", secret)
		}
	}
	if len(overviewEnv.Data.Settings.Models) != 1 || overviewEnv.Data.Settings.Models[0] != "gpt-6-astra" {
		t.Fatalf("models = %#v", overviewEnv.Data.Settings.Models)
	}
	if overviewEnv.Data.Settings.ReasoningEffort != "high" {
		t.Fatalf("reasoning = %q", overviewEnv.Data.Settings.ReasoningEffort)
	}
	if !reflect.DeepEqual(overviewEnv.Data.Settings.QuietWindows, []QuietWindow{{Start: "23:00", End: "07:00"}, {Start: "12:00", End: "13:00"}}) {
		t.Fatalf("quiet windows = %+v", overviewEnv.Data.Settings.QuietWindows)
	}
	if len(overviewEnv.Data.Groups) != 1 {
		t.Fatalf("groups = %#v", overviewEnv.Data.Groups)
	}
	group := overviewEnv.Data.Groups[0]
	if group.ID != 3 || group.Name != "GPT Plus" || group.IntervalMinutes != 10 {
		t.Fatalf("group = %+v", group)
	}
	if len(group.Results) != publicOverviewResults {
		t.Fatalf("result count = %d", len(group.Results))
	}
	if group.Results[0].ID != ids[48] {
		t.Fatalf("newest id = %d, want %d", group.Results[0].ID, ids[48])
	}
	for _, row := range group.Results {
		if row.ID == ids[0] {
			t.Fatal("oldest result was not trimmed from the overview")
		}
	}
	newest := group.Results[0]
	if newest.CandyReview != "rejected" || newest.CandyCorrect == nil || !*newest.CandyCorrect {
		t.Fatalf("newest meta = %+v", newest)
	}
	if !newest.PelicanOK || newest.PelicanReview != "approved" {
		t.Fatalf("pelican meta = %+v", newest)
	}
	older := group.Results[1]
	if older.CandyReview != "pending" {
		t.Fatalf("unreviewed meta = %+v", older)
	}

	detailRec := httptest.NewRecorder()
	handler.ServeHTTP(detailRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/"+strconv.FormatInt(ids[1], 10), nil))
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detailEnv struct {
		Code int          `json:"code"`
		Data publicResult `json:"data"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detailEnv); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detailEnv.Code != 0 {
		t.Fatalf("detail code = %d", detailEnv.Code)
	}
	detail := detailEnv.Data
	if detail.CandyAnswer != "21" || detail.PelicanHTML != "<p>PELICAN-BODY-1</p>" || !detail.PelicanOK {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.CandyReview != "pending" || detail.PelicanReview != "pending" {
		t.Fatalf("detail review = %+v", detail)
	}
	reviewedRec := httptest.NewRecorder()
	handler.ServeHTTP(reviewedRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/"+strconv.FormatInt(ids[48], 10), nil))
	if reviewedRec.Code != http.StatusOK {
		t.Fatalf("reviewed detail status = %d", reviewedRec.Code)
	}
	if err := json.Unmarshal(reviewedRec.Body.Bytes(), &detailEnv); err != nil {
		t.Fatal(err)
	}
	if detailEnv.Data.CandyReview != newest.CandyReview || detailEnv.Data.PelicanReview != newest.PelicanReview {
		t.Fatalf("overview and detail review differ: %+v / %+v", newest, detailEnv.Data)
	}
	rawDetail := detailRec.Body.String()
	for _, secret := range []string{"SECRET-ACCOUNT", "SECRET-PROMPT", "SECRET-ANSWER", "account_id", "account_name"} {
		if strings.Contains(rawDetail, secret) {
			t.Fatalf("detail leaked %s", secret)
		}
	}

	hiddenRec := httptest.NewRecorder()
	handler.ServeHTTP(hiddenRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/"+strconv.FormatInt(hiddenID, 10), nil))
	if hiddenRec.Code != http.StatusNotFound {
		t.Fatalf("hidden status = %d", hiddenRec.Code)
	}
	notPublicID, err := store.InsertRunningTestResult(ctx, TestResult{GroupID: 5, Model: "gpt-6-astra"})
	if err != nil {
		t.Fatal(err)
	}
	notPublicRec := httptest.NewRecorder()
	handler.ServeHTTP(notPublicRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/"+strconv.FormatInt(notPublicID, 10), nil))
	if notPublicRec.Code != http.StatusNotFound {
		t.Fatalf("unchecked group detail status = %d", notPublicRec.Code)
	}
	// Hiding a group must take effect on existing results immediately.
	if _, err := store.SaveSelection(ctx, []SelectionGroup{{ID: 3, Name: "GPT Plus", IntervalMinutes: 10, Visible: &visible}}); err != nil {
		t.Fatal(err)
	}
	uncheckedRec := httptest.NewRecorder()
	handler.ServeHTTP(uncheckedRec, overviewReq)
	if err := json.Unmarshal(uncheckedRec.Body.Bytes(), &overviewEnv); err != nil {
		t.Fatal(err)
	}
	if uncheckedRec.Code != http.StatusOK || len(overviewEnv.Data.Groups) != 0 {
		t.Fatal("unchecked group remains in overview")
	}
	uncheckedDetailRec := httptest.NewRecorder()
	handler.ServeHTTP(uncheckedDetailRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/"+strconv.FormatInt(ids[48], 10), nil))
	if uncheckedDetailRec.Code != http.StatusNotFound {
		t.Fatalf("hidden historical detail status = %d", uncheckedDetailRec.Code)
	}
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/999999", nil))
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", missingRec.Code)
	}
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, httptest.NewRequest(http.MethodGet, "/iq-detection/results/nope", nil))
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad id status = %d", badRec.Code)
	}

	optRec := httptest.NewRecorder()
	handler.ServeHTTP(optRec, httptest.NewRequest(http.MethodOptions, "/iq-detection/overview", nil))
	if optRec.Code != http.StatusNoContent {
		t.Fatalf("options status = %d", optRec.Code)
	}
	if got := optRec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("options CORS = %q", got)
	}
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, httptest.NewRequest(http.MethodPost, "/iq-detection/overview", strings.NewReader("{}")))
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post status = %d", postRec.Code)
	}
}

func TestPublicOverviewWithoutQuietWindows(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/iq-detection/overview", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"quiet_windows":[]`) {
		t.Fatalf("unset quiet windows should be an empty array: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicLimiterBlocksAfterBudget(t *testing.T) {
	var limiter publicLimiter
	now := time.Now()
	for i := 0; i < publicRatePerMinute; i++ {
		if !limiter.allow("203.0.113.5", now) {
			t.Fatalf("request %d blocked", i)
		}
	}
	if limiter.allow("203.0.113.5", now) {
		t.Fatal("expected the next request to be blocked")
	}
	if !limiter.allow("203.0.113.9", now) {
		t.Fatal("a different address was blocked")
	}
	if !limiter.allow("203.0.113.5", now.Add(time.Minute)) {
		t.Fatal("the window did not reset")
	}
}

func TestPublicClientIPTrustsForwardedHeaderFromLoopbackOnly(t *testing.T) {
	local := httptest.NewRequest(http.MethodGet, "/iq-detection/overview", nil)
	local.RemoteAddr = "127.0.0.1:4321"
	local.Header.Set("X-Forwarded-For", "203.0.113.8, 10.0.0.1")
	if got := publicClientIP(local); got != "203.0.113.8" {
		t.Fatalf("loopback client = %q", got)
	}
	remote := httptest.NewRequest(http.MethodGet, "/iq-detection/overview", nil)
	remote.RemoteAddr = "198.51.100.8:4321"
	remote.Header.Set("X-Forwarded-For", "203.0.113.8")
	if got := publicClientIP(remote); got != "198.51.100.8" {
		t.Fatalf("remote client = %q", got)
	}
}
