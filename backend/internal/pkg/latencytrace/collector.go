// Package latencytrace records bounded, payload-free request timing diagnostics.
package latencytrace

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const MaxRecords = 10000

var (
	ErrInvalidScope = errors.New("user_id, group_id and account_id must be positive and exist")
	ErrActiveLimit  = errors.New("collection already active for this scope, or 10 active collections reached")
	ErrNotFound     = errors.New("collection not found")
)

type Scope struct {
	UserID    int64 `json:"user_id"`
	GroupID   int64 `json:"group_id"`
	AccountID int64 `json:"account_id"`
}

type Session struct {
	Sequence int64  `json:"sequence"`
	ID       string `json:"id"`
	Scope
	CreatedBy   int64      `json:"created_by"`
	StartedAt   time.Time  `json:"started_at"`
	StoppedAt   *time.Time `json:"stopped_at"`
	StopReason  string     `json:"stop_reason"`
	RecordCount int        `json:"record_count"`
}

type Event struct {
	Name       string   `json:"name"`
	AtMS       float64  `json:"at_ms"`
	DurationMS *float64 `json:"duration_ms,omitempty"`
	Attempt    int      `json:"attempt,omitempty"`
}

type Attempt struct {
	Number       int    `json:"number"`
	AccountID    int64  `json:"account_id"`
	Transport    string `json:"transport"`
	RequestBytes int64  `json:"request_bytes"`
	Status       int    `json:"status,omitempty"`
	Failed       bool   `json:"failed"`
	Reused       *bool  `json:"reused,omitempty"`
}

type Record struct {
	ID              int64     `json:"id"`
	CollectionID    string    `json:"collection_id"`
	RequestID       string    `json:"request_id"`
	ClientRequestID string    `json:"client_request_id"`
	StartedAt       time.Time `json:"started_at"`
	Status          int       `json:"status"`
	RequestBytes    int64     `json:"request_bytes"`
	DurationMS      float64   `json:"duration_ms"`
	Events          []Event   `json:"events"`
	Attempts        []Attempt `json:"attempts"`
	Truncated       bool      `json:"truncated"`
}

type Repository interface {
	CreateLatencySession(context.Context, *Session) error
	StopLatencySession(context.Context, string) error
	ListLatencySessions(context.Context, string, int64) ([]Session, error)
	ActiveLatencySessions(context.Context) ([]Session, error)
	InsertLatencyRecord(context.Context, *Record) error
	ListLatencyRecords(context.Context, string, string, int64) ([]Record, error)
}

type snapshot struct {
	sessions []Session
	loaded   time.Time
}
type Health struct {
	QueueDepth    int    `json:"queue_depth"`
	Dropped       uint64 `json:"dropped"`
	WriteFailed   uint64 `json:"write_failed"`
	RefreshFailed uint64 `json:"refresh_failed"`
	Ready         bool   `json:"ready"`
}

type Collector struct {
	repo          Repository
	cache         atomic.Pointer[snapshot]
	mu            sync.Mutex
	queue         chan Record
	dropped       atomic.Uint64
	writeFailed   atomic.Uint64
	refreshFailed atomic.Uint64
	cancel        context.CancelFunc
	done          chan struct{}
}

func New(repo Repository) *Collector { return &Collector{repo: repo, queue: make(chan Record, 256)} }

func (c *Collector) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return
	}
	ctx, c.cancel = context.WithCancel(ctx)
	c.done = make(chan struct{})
	c.refresh(ctx)
	go c.run(ctx)
}

func (c *Collector) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	cancel, done := c.cancel, c.done
	c.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (c *Collector) refresh(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	sessions, err := c.repo.ActiveLatencySessions(ctx)
	if err != nil {
		c.refreshFailed.Add(1)
		return
	}
	c.cache.Store(&snapshot{sessions: sessions, loaded: time.Now()})
}

func (c *Collector) run(ctx context.Context) {
	defer close(c.done)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.cache.Store(nil)
			// Bounded shutdown: never delay service shutdown for a diagnostic backlog.
			c.dropped.Add(uint64(len(c.queue)))
			return
		case <-ticker.C:
			c.mu.Lock()
			c.refresh(ctx)
			c.mu.Unlock()
		case record := <-c.queue:
			writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := c.repo.InsertLatencyRecord(writeCtx, &record)
			cancel()
			if err != nil {
				c.writeFailed.Add(1)
			}
		}
	}
}

func (c *Collector) Create(ctx context.Context, scope Scope, actor int64) (*Session, error) {
	if scope.UserID <= 0 || scope.GroupID <= 0 || scope.AccountID <= 0 || actor <= 0 {
		return nil, ErrInvalidScope
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s := &Session{ID: uuid.NewString(), Scope: scope, CreatedBy: actor}
	if err := c.repo.CreateLatencySession(ctx, s); err != nil {
		return nil, err
	}
	c.refresh(ctx)
	return s, nil
}

func (c *Collector) Stop(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.repo.StopLatencySession(ctx, id); err != nil {
		return err
	}
	// Remove locally even if the refresh fails. Other instances converge in 2s;
	// the repository's row lock prevents all writes after Stop has returned.
	if old := c.cache.Load(); old != nil {
		next := &snapshot{loaded: old.loaded}
		for _, s := range old.sessions {
			if s.ID != id {
				next.sessions = append(next.sessions, s)
			}
		}
		c.cache.Store(next)
	}
	return nil
}

func (c *Collector) Sessions(ctx context.Context, id string, before int64) ([]Session, error) {
	return c.repo.ListLatencySessions(ctx, id, before)
}
func (c *Collector) Records(ctx context.Context, id, requestID string, before int64) ([]Record, error) {
	return c.repo.ListLatencyRecords(ctx, id, requestID, before)
}
func (c *Collector) Health() Health {
	s := c.cache.Load()
	return Health{len(c.queue), c.dropped.Load(), c.writeFailed.Load(), c.refreshFailed.Load(), s != nil && time.Since(s.loaded) < 10*time.Second}
}

type collectorKey struct{}
type traceKey struct{}

func WithCollector(ctx context.Context, c *Collector) context.Context {
	return context.WithValue(ctx, collectorKey{}, c)
}
func From(ctx context.Context) *Trace {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(traceKey{}).(*Trace)
	return t
}

// Begin runs after authentication, before the handler reads the body. It only
// allocates for the requested user/group; account matching happens on selection.
func Begin(ctx context.Context, user, group int64, requestID string, start time.Time) (context.Context, *Trace) {
	if t := From(ctx); t != nil {
		return ctx, t
	}
	c, _ := ctx.Value(collectorKey{}).(*Collector)
	if c == nil {
		return ctx, nil
	}
	s := c.cache.Load()
	if s == nil || time.Since(s.loaded) >= 10*time.Second {
		return ctx, nil
	}
	var candidates []Session
	for _, session := range s.sessions {
		if session.UserID == user && session.GroupID == group && !start.Before(session.StartedAt) {
			candidates = append(candidates, session)
		}
	}
	if len(candidates) == 0 {
		return ctx, nil
	}
	t := &Trace{collector: c, candidates: candidates, record: Record{RequestID: requestID, StartedAt: start, Events: make([]Event, 0, 32), Attempts: make([]Attempt, 0, 4)}, matched: make(map[string]bool)}
	t.Mark("auth_complete")
	return context.WithValue(ctx, traceKey{}, t), t
}

type Trace struct {
	mu            sync.Mutex
	collector     *Collector
	candidates    []Session
	matched       map[string]bool
	record        Record
	closed        bool
	attemptNumber int
}

func (t *Trace) active(id string) bool {
	s := t.collector.cache.Load()
	if s == nil || time.Since(s.loaded) >= 10*time.Second {
		return false
	}
	for _, session := range s.sessions {
		if session.ID == id {
			return true
		}
	}
	return false
}

func (t *Trace) event(name string, attempt int, duration *float64) {
	if t.closed {
		return
	}
	if attempt > 32 {
		t.record.Truncated = true
		return
	}
	active := false
	for _, s := range t.candidates {
		if t.active(s.ID) {
			active = true
			break
		}
	}
	if !active {
		return
	}
	// Only the first observation per stage/attempt is needed, even on long streams.
	for _, e := range t.record.Events {
		if e.Name == name && e.Attempt == attempt {
			return
		}
	}
	if len(t.record.Events) >= 256 {
		t.record.Truncated = true
		return
	}
	t.record.Events = append(t.record.Events, Event{name, float64(time.Since(t.record.StartedAt).Microseconds()) / 1000, duration, attempt})
}
func (t *Trace) Mark(name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.event(name, 0, nil)
}
func Mark(ctx context.Context, name string) { From(ctx).Mark(name) }
func (t *Trace) Duration(name string, ms float64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.event(name, t.attemptNumber, &ms)
}
func (t *Trace) SelectAccount(account int64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.candidates {
		if s.AccountID == account {
			t.matched[s.ID] = true
		}
	}
	t.event("account_selected", t.attemptNumber+1, nil)
}
func (t *Trace) AddRequestBytes(n int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.record.RequestBytes += int64(n)
	}
}
func (t *Trace) Finish(clientID string, status int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.event("request_complete", 0, nil)
	t.closed = true
	t.record.ClientRequestID = clientID
	t.record.Status = status
	t.record.DurationMS = float64(time.Since(t.record.StartedAt).Microseconds()) / 1000
	for id := range t.matched {
		if !t.active(id) {
			continue
		}
		r := t.record
		r.CollectionID = id
		select {
		case t.collector.queue <- r:
		default:
			t.collector.dropped.Add(1)
		}
	}
}

type AttemptTrace struct {
	trace  *Trace
	number int
}

func (t *Trace) StartAttempt(account int64, transport string, size int64) *AttemptTrace {
	if t == nil {
		return nil
	}
	t.SelectAccount(account)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.attemptNumber++
	if t.attemptNumber > 32 {
		t.record.Truncated = true
		return nil
	}
	n := t.attemptNumber
	t.record.Attempts = append(t.record.Attempts, Attempt{Number: n, AccountID: account, Transport: transport, RequestBytes: size})
	t.event("upstream_start", n, nil)
	return &AttemptTrace{t, n}
}
func (a *AttemptTrace) Mark(name string) {
	if a == nil {
		return
	}
	a.trace.mu.Lock()
	defer a.trace.mu.Unlock()
	a.trace.event(name, a.number, nil)
}
func (a *AttemptTrace) Done(status int, failed bool) {
	if a == nil {
		return
	}
	a.trace.mu.Lock()
	defer a.trace.mu.Unlock()
	if a.trace.closed {
		return
	}
	item := &a.trace.record.Attempts[a.number-1]
	item.Status = status
	item.Failed = failed
	name := "upstream_headers"
	if failed {
		name = "upstream_error"
	}
	a.trace.event(name, a.number, nil)
}
func (a *AttemptTrace) Reused(reused bool) {
	if a == nil {
		return
	}
	a.trace.mu.Lock()
	defer a.trace.mu.Unlock()
	if !a.trace.closed {
		a.trace.record.Attempts[a.number-1].Reused = &reused
	}
}
func (a *AttemptTrace) RequestSize(size int64) {
	if a == nil {
		return
	}
	a.trace.mu.Lock()
	defer a.trace.mu.Unlock()
	if !a.trace.closed {
		a.trace.record.Attempts[a.number-1].RequestBytes = size
	}
}
func (t *Trace) MarkCurrent(name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.event(name, t.attemptNumber, nil)
}
func (t *Trace) MarkNext(name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.event(name, t.attemptNumber+1, nil)
}
