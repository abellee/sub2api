package events

import (
	"context"
	"testing"
	"time"
)

func TestDispatcherDoesNotBlockBusinessWhenReceiverStalls(t *testing.T) {
	started := make(chan struct{}, 1)
	d := NewDispatcher("test", 1, 2, func(ctx context.Context, e Event) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	})
	defer d.Stop()
	d.Submit(Event{Type: "announcement.read", UserID: 2})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("sender did not start")
	}
	start := time.Now()
	for i := 0; i < 10000; i++ {
		d.Submit(Event{Type: "announcement.read", UserID: 2})
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatal("event queue blocked the caller")
	}
	if d.Dropped.Load() == 0 {
		t.Fatal("full queue was not counted")
	}
}

func TestEmitSnapshotsDataAndIsolatesSenderPanic(t *testing.T) {
	var got Event
	SetSink(func(e Event) { got = e })
	defer SetSink(nil)
	data := map[string]string{"model": "before"}
	Publish("billing.completed", 2, "request-1", data)
	data["model"] = "after"
	if got.Data["model"] != "before" {
		t.Fatal("event retained mutable business data")
	}
	SetSink(func(Event) { panic("sender stopped") })
	Publish("billing.completed", 2, "request-2", nil)
}
