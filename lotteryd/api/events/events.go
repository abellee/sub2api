// Package events contains only event snapshots and a bounded, best-effort sender.
// Business hooks never wait for a receiver, read settings, or perform I/O.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Source     string            `json:"source"`
	UserID     int64             `json:"user_id"`
	Reference  string            `json:"reference"`
	OccurredAt time.Time         `json:"occurred_at"`
	Data       map[string]string `json:"data"`
}

type sink struct{ submit func(Event) }

var current atomic.Pointer[sink]

// SetSink is called once at service startup. A nil sink makes every hook inert.
func SetSink(submit func(Event)) {
	if submit == nil {
		current.Store(nil)
	} else {
		current.Store(&sink{submit})
	}
}
func Enabled() bool { return current.Load() != nil }

// Emit copies the small snapshot; it must never retain request objects or bodies.
func Emit(e Event) {
	defer func() { _ = recover() }()
	s := current.Load()
	if s == nil {
		return
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	data := make(map[string]string, len(e.Data))
	for k, v := range e.Data {
		if len(data) >= 48 {
			break
		}
		if len(k) <= 80 && len(v) <= 1024 {
			data[k] = v
		}
	}
	e.Data = data
	s.submit(e)
}

func Publish(kind string, userID int64, reference string, data map[string]string) {
	Emit(Event{Type: kind, UserID: userID, Reference: reference, Data: data})
}
func S(v any) string          { return fmt.Sprint(v) }
func Amount(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

type Dispatcher struct {
	queue   chan Event
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	send    func(context.Context, Event) error
	source  string
	Dropped atomic.Uint64
	Failed  atomic.Uint64
	Sent    atomic.Uint64
}

func NewDispatcher(source string, workers, capacity int, send func(context.Context, Event) error) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{queue: make(chan Event, capacity), ctx: ctx, cancel: cancel, send: send, source: source}
	for i := 0; i < workers; i++ {
		d.wg.Add(1)
		go d.run()
	}
	return d
}
func (d *Dispatcher) Submit(e Event) {
	select {
	case <-d.ctx.Done():
		return
	default:
	}
	select {
	case d.queue <- e:
	default:
		d.Dropped.Add(1)
	}
}
func (d *Dispatcher) Stop() { d.cancel(); d.wg.Wait() }
func (d *Dispatcher) run() {
	defer d.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var reported uint64
	for {
		select {
		case <-d.ctx.Done():
			return
		case e := <-d.queue:
			d.deliver(e)
		case <-ticker.C:
			if dropped := d.Dropped.Load(); dropped > reported {
				slog.Warn("gameplay event queue full", "source", d.source, "dropped_total", dropped)
				reported = dropped
			}
		}
	}
}
func (d *Dispatcher) deliver(e Event) {
	defer func() {
		if r := recover(); r != nil {
			d.Failed.Add(1)
			slog.Error("gameplay event sender panic", "panic", r)
		}
	}()
	if e.Source == "" {
		e.Source = d.source
	}
	if e.ID == "" {
		if e.Reference != "" {
			e.ID = e.Source + ":" + e.Type + ":" + e.Reference
		} else {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				d.Failed.Add(1)
				return
			}
			e.ID = hex.EncodeToString(b[:])
		}
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(d.ctx, 2*time.Second)
		err := d.send(ctx, e)
		cancel()
		if err == nil {
			d.Sent.Add(1)
			return
		}
		if d.ctx.Err() != nil {
			return
		}
		if attempt == 2 {
			n := d.Failed.Add(1)
			if n == 1 || n%100 == 0 {
				slog.Warn("gameplay event delivery failed", "type", e.Type, "failed_total", n, "error", err)
			}
			return
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 200 * time.Millisecond)
		select {
		case <-d.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
