package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// SyncIsolationWindow is persisted per user/source group, shared by all keys and servers.
type SyncIsolationWindow struct {
	Isolated bool                 `json:"isolated"`
	Events   []SyncIsolationEvent `json:"events"`
}

type SyncIsolationEvent struct {
	At     int64 `json:"at"`
	Stream bool  `json:"stream"`
}

func (w *SyncIsolationWindow) Observe(now time.Time, stream bool, enabled bool) bool {
	cutoff := now.Add(-time.Minute).UnixMilli()
	first := 0
	for first < len(w.Events) && w.Events[first].At <= cutoff {
		first++
	}
	w.Events = append(w.Events[first:], SyncIsolationEvent{At: now.UnixMilli(), Stream: stream})
	before := w.Isolated
	if !enabled {
		w.Isolated = false
	} else if !w.Isolated {
		if stream {
			w.Events = nil
		} else if len(w.Events) > 10 {
			w.Isolated = true
		}
	} else {
		streams := 0
		for _, event := range w.Events {
			if event.Stream {
				streams++
			}
		}
		if streams*10 > len(w.Events)*9 {
			w.Isolated = false
		}
	}
	if before != w.Isolated {
		w.Events = []SyncIsolationEvent{{At: now.UnixMilli(), Stream: stream}}
		if !w.Isolated && stream {
			w.Events = nil
		}
	}
	return before != w.Isolated
}

// PolicyGroupID keeps subscriptions, user pricing, RPM and matching rules shared
// across an automatic isolation move instead of granting a fresh allowance.
func (g *Group) PolicyGroupID() int64 {
	if g.SyncIsolationSourceGroupID > 0 {
		return g.SyncIsolationSourceGroupID
	}
	return g.ID
}

type SyncIsolationRepository interface {
	ObserveSyncIsolation(ctx context.Context, keyID, userID, groupID int64, stream bool, observedAt time.Time) (int64, bool, error)
}

const syncIsolationWorkerCount = 16

type syncIsolationObservation struct {
	keyID, userID, groupID int64
	stream                 bool
	at                     time.Time
}

type syncIsolationWorkers struct {
	mu      sync.Mutex
	queues  [syncIsolationWorkerCount]syncIsolationQueue
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped bool
}

type syncIsolationQueue struct {
	pending []syncIsolationObservation
	wake    chan struct{}
}

// QueueSyncIsolation never waits for database work and never retains a request,
// its body, credentials or cached auth objects. A user's keys share one FIFO.
func (s *APIKeyService) QueueSyncIsolation(key *APIKey, stream bool) {
	if s == nil || key == nil || key.Group == nil || (!key.Group.SyncIsolationEnabled && key.Group.SyncIsolationSourceGroupID == 0) {
		return
	}
	repo, ok := s.groupRepo.(SyncIsolationRepository)
	if !ok {
		return
	}
	w := &s.syncIsolation
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return
	}
	if w.cancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		w.cancel = cancel
		for i := range w.queues {
			w.queues[i].wake = make(chan struct{}, 1)
			w.wg.Add(1)
			go func(queue *syncIsolationQueue) {
				defer w.wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case <-queue.wake:
						w.mu.Lock()
						batch := queue.pending
						queue.pending = nil
						w.mu.Unlock()
						for _, event := range batch {
							if ctx.Err() != nil {
								return
							}
							s.processSyncIsolation(ctx, repo, event)
						}
					}
				}
			}(&w.queues[i])
		}
	}
	event := syncIsolationObservation{keyID: key.ID, userID: key.UserID, groupID: key.Group.ID, stream: stream, at: time.Now()}
	queue := &w.queues[uint64(key.UserID)%syncIsolationWorkerCount]
	// Only a rolling minute is relevant, including when database work stalls.
	first := 0
	for first < len(queue.pending) && event.at.Sub(queue.pending[first].at) >= time.Minute {
		first++
	}
	queue.pending = append(queue.pending[first:], event)
	select {
	case queue.wake <- struct{}{}:
	default:
	}
}

func (s *APIKeyService) processSyncIsolation(parent context.Context, repo SyncIsolationRepository, event syncIsolationObservation) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("sync isolation background panic", "user_id", event.userID, "group_id", event.groupID, "panic", recovered)
		}
	}()
	if time.Since(event.at) >= time.Minute {
		return
	}
	groupID, changed, err := repo.ObserveSyncIsolation(ctx, event.keyID, event.userID, event.groupID, event.stream, event.at)
	if err != nil {
		if parent.Err() == nil {
			slog.Warn("sync isolation background update failed", "user_id", event.userID, "group_id", event.groupID, "error", err)
		}
		return
	}
	if changed || groupID != event.groupID {
		s.InvalidateAuthCacheByUserID(ctx, event.userID)
	}
}

func (s *APIKeyService) StopSyncIsolation() {
	if s == nil {
		return
	}
	w := &s.syncIsolation
	w.mu.Lock()
	w.stopped = true
	if w.cancel != nil {
		w.cancel()
	}
	for i := range w.queues {
		w.queues[i].pending = nil
	}
	w.mu.Unlock()
	w.wg.Wait()
}
