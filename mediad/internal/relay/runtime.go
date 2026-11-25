package relay

import (
	"context"
	"sync"
	"time"
)

// Lowering the limit drains existing work without canceling paid requests.
type taskSlots struct {
	mu             sync.Mutex
	limit, running int
	changed        chan struct{}
}

func (s *taskSlots) signal() {
	if s.changed != nil {
		close(s.changed)
	}
	s.changed = make(chan struct{})
}
func (s *taskSlots) setLimit(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limit = n
	s.signal()
}
func (s *taskSlots) acquire(ctx context.Context) error {
	for {
		s.mu.Lock()
		if err := ctx.Err(); err != nil {
			s.mu.Unlock()
			return err
		}
		limit := s.limit
		if limit <= 0 {
			limit = 8
		}
		if s.running < limit {
			s.running++
			s.mu.Unlock()
			return nil
		}
		if s.changed == nil {
			s.changed = make(chan struct{})
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}
func (s *taskSlots) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running--
	s.signal()
}

func (t *Tracker) creationTimeout() time.Duration {
	if t.runtimeSettings != nil {
		return time.Duration(t.runtimeSettings.Active().MaxSyncSeconds) * time.Second
	}
	return t.createTimeout
}
func (t *Tracker) pollSettings() (time.Duration, time.Duration) {
	if t.runtimeSettings != nil {
		s := t.runtimeSettings.Active()
		return time.Duration(s.PollIntervalSeconds) * time.Second, time.Duration(s.MaxPollSeconds) * time.Second
	}
	return t.interval, t.maxPoll
}
func (t *Tracker) stagingAttempts() int {
	if t.runtimeSettings != nil {
		return t.runtimeSettings.Active().DownloadAttempts
	}
	return t.downloadAttempts
}
func (t *Tracker) downloadBaseURL() string {
	if t.runtimeSettings != nil {
		return t.runtimeSettings.Active().DownloadBaseURL
	}
	return t.downloadBase
}
func (t *Tracker) resultStager() *ResultStager {
	if t.stager == nil || t.runtimeSettings == nil {
		return t.stager
	}
	// Never mutate a client that is serving an existing download.
	s := *t.stager
	c := *s.Client
	runtime := t.runtimeSettings.Active()
	c.Timeout = time.Duration(runtime.DownloadTimeoutSeconds) * time.Second
	s.Client = &c
	s.MaxBytes = int64(runtime.MaxFileMiB) << 20
	return &s
}
func (h *RelayHandler) relaySecret() string {
	if h.tracker != nil && h.tracker.runtimeSettings != nil {
		if secret := h.tracker.runtimeSettings.Active().RelaySecret; secret != "" {
			return secret
		}
	}
	return h.secret
}
func (h *ManagementHandler) managementToken() string {
	if h.Settings != nil {
		if token := h.Settings.Active().ManagementToken; token != "" {
			return token
		}
	}
	return h.token
}
