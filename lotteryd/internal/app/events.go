package app

import (
	"context"
	"log/slog"
	"lotteryd/api/events"
	"time"
)

// RunEvents owns a separate queue and worker; no request or settlement worker waits for it.
func (ap *App) RunEvents(ctx context.Context) {
	d := events.NewDispatcher("lotteryd", 2, 2048, ap.Store.ReceiveEvent)
	events.SetSink(d.Submit)
	defer func() { events.SetSink(nil); d.Stop() }()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	nextClock := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if ap.Cfg.DisableSettlement {
			continue
		}
		if time.Now().After(nextClock) {
			clockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := ap.Store.EmitEventClock(clockCtx, time.Now()); err != nil {
				slog.Warn("gameplay clock failed", "error", err)
			}
			cancel()
			nextClock = time.Now().Add(time.Minute)
		}
		for i := 0; i < 100; i++ {
			workCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			worked, err := ap.Store.ProcessNextEvent(workCtx)
			cancel()
			if err != nil {
				slog.Warn("gameplay event processing failed", "error", err)
				break
			}
			if !worked {
				break
			}
		}
	}
}
