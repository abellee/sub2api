package iqdetectd

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPickCandidateNeverTestedFirst(t *testing.T) {
	now := time.Now()
	// Every position matters: a zero timestamp used to reset the minimum
	// when followed by an already-tested account.
	for unseen := int64(1); unseen <= 4; unseen++ {
		candidates := []candidate{{id: 1}, {id: 2}, {id: 3}, {id: 4}}
		history := map[int64]time.Time{}
		for _, c := range candidates {
			if c.id != unseen {
				history[c.id] = now.Add(time.Duration(c.id) * time.Second)
			}
		}
		if got := pickCandidate(candidates, history); got.id != unseen {
			t.Fatalf("unseen %d: picked %d", unseen, got.id)
		}
	}
	if got := pickCandidate([]candidate{{id: 1}, {id: 2}}, map[int64]time.Time{1: now, 2: now.Add(-time.Hour)}); got.id != 2 {
		t.Fatalf("picked %d instead of oldest account 2", got.id)
	}
}

func TestScheduledRotationCoversEveryAccountPerGroupAndModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rotation.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	if _, err := store.ReplaceDetectionModels(ctx, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	groups := []SelectionGroup{{ID: 1}, {ID: 2}}
	for i := range groups {
		groups[i].Accounts = []SelectionAccount{
			{ID: 1, Models: []string{"a", "b"}},
			{ID: 2, Models: []string{"a", "b"}},
			{ID: 3, Models: []string{"a", "b"}},
			{ID: 1, Models: []string{"a", "b"}}, // duplicate selection
			{ID: 4, Models: []string{"a", "b"}}, // missing credentials
			{ID: 5, Models: []string{"other"}},  // different model
		}
	}
	creds := map[int64]TestRunCredential{}
	for _, id := range []int64{1, 2, 3, 5} {
		creds[id] = TestRunCredential{AccountID: id, APIURL: "https://unused.invalid", APIKey: "test"}
	}
	// Distinct scopes deliberately prefer different next accounts. Account 1
	// was tested here, but 2 and 3 were tested only in other groups/models.
	for _, row := range []TestResult{
		{GroupID: 1, Model: "a", AccountID: 1},
		{GroupID: 2, Model: "a", AccountID: 2},
		{GroupID: 1, Model: "b", AccountID: 3},
	} {
		if _, err := store.InsertRunningTestResult(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	for cycle := 0; cycle < 3; cycle++ {
		seen := map[[2]int64]map[int64]bool{}
		for round := 0; round < 3; round++ {
			// Reopen SQLite each round: rotation must survive service restarts.
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			store, err = OpenStore(path)
			if err != nil {
				t.Fatal(err)
			}
			runner := NewTestRunner(store, 0, nil)
			for _, group := range groups {
				_, jobs, err := runner.buildGroupJobs(ctx, group, creds)
				if err != nil || len(jobs) != 2 {
					t.Fatalf("group %d jobs = %+v, %v", group.ID, jobs, err)
				}
				for mi, job := range jobs {
					key := [2]int64{group.ID, int64(mi)}
					if seen[key] == nil {
						seen[key] = map[int64]bool{}
					}
					if seen[key][job.accountID] {
						t.Fatalf("cycle %d round %d group %d model %s repeated %d before covering all accounts", cycle, round, group.ID, job.model, job.accountID)
					}
					seen[key][job.accountID] = true
					if cycle == 0 && round == 0 && ((group.ID == 1 && mi == 0 && job.accountID == 1) || (group.ID == 2 && mi == 0 && job.accountID == 2) || (group.ID == 1 && mi == 1 && job.accountID == 3)) {
						t.Fatal("tested account picked before an untested one in its scope")
					}
					id, err := store.InsertRunningTestResult(ctx, TestResult{GroupID: group.ID, Model: job.model, AccountID: job.accountID})
					if err != nil {
						t.Fatal(err)
					}
					// Failures/stops must consume a slot too, otherwise a broken
					// account could monopolize every subsequent test.
					if err := store.CompleteTestResult(ctx, id, TestResult{Status: RunStatusStopped, PelicanError: "stopped", CandyError: "stopped"}); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
}

type rotationTransport func(*http.Request) (*http.Response, error)

func (f rotationTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func waitRotationIdle(t *testing.T, runner *TestRunner, groupID int64) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for runner.isGroupRunning(groupID) {
		select {
		case <-deadline.C:
			t.Fatal("test group did not finish")
		case <-tick.C:
		}
	}
}

func TestManualRotationReservesGroupBeforeReturningPlan(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if _, err := store.ReplaceDetectionModels(ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertDetectionPrompts(ctx, DetectionPrompts{PelicanPrompt: "test", CandyPrompt: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveSelection(ctx, []SelectionGroup{{ID: 1, IntervalMinutes: 1, Accounts: []SelectionAccount{
		{ID: 1, Models: []string{"a"}}, {ID: 2, Models: []string{"a"}}, {ID: 3, Models: []string{"a"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	creds := []TestRunCredential{}
	for _, id := range []int64{1, 2, 3} {
		creds = append(creds, TestRunCredential{AccountID: id, APIURL: "https://unused.invalid", APIKey: "test"})
	}
	runner := NewTestRunner(store, 30*time.Second, nil)
	gate := make(chan struct{})
	started := make(chan struct{}, 2)
	var calls atomic.Int64
	runner.Client = &http.Client{Transport: rotationTransport(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-gate:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"test"}}]}`))}, nil
	})}
	var wg sync.WaitGroup
	plans := make(chan *TestRunPlan, 16)
	errors := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Go(func() {
			plan, err := runner.TriggerTestRun(ctx, creds, 1)
			plans <- plan
			errors <- err
		})
	}
	wg.Wait()
	close(plans)
	close(errors)
	// The scheduler must see the same reservation while probes are blocked.
	runner.scheduleTick(ctx)
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("mock probes did not start")
		}
	}
	if stopped := runner.StopTestRuns(); stopped != 1 {
		t.Fatalf("stopped %d groups, want 1", stopped)
	}
	waitRotationIdle(t, runner, 1)
	close(gate)
	var stopped int
	if err := store.db.QueryRowContext(ctx, "SELECT count(*) FROM test_results WHERE status = 'stopped'").Scan(&stopped); err != nil || stopped != 1 {
		t.Fatalf("stopped rows = %d, %v", stopped, err)
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int64]bool{}
	for plan := range plans {
		if plan.Triggered > 0 {
			if len(seen) > 0 {
				t.Fatal("concurrent requests dispatched more than one group run")
			}
			seen[plan.Items[0].AccountID] = true
		}
	}
	if len(seen) != 1 || calls.Load() != 2 {
		t.Fatalf("first round: accounts=%v calls=%d", seen, calls.Load())
	}
	for i := 0; i < 2; i++ {
		plan, err := runner.TriggerTestRun(ctx, creds, 1)
		if err != nil || plan.Triggered != 1 {
			t.Fatalf("manual plan = %+v, %v", plan, err)
		}
		waitRotationIdle(t, runner, 1)
		id := plan.Items[0].AccountID
		if seen[id] {
			t.Fatalf("manual rotation repeated %d before covering all accounts", id)
		}
		seen[id] = true
	}
}
