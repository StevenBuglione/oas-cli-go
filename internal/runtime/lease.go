package runtime

import (
	"sync"
	"sync/atomic"
	"time"
)

// LeaseConfig configures heartbeat and grace-shutdown behavior for the
// standalone LeaseTracker (used in unit tests and optional embedding).
type LeaseConfig struct {
	// HeartbeatInterval is the expected cadence at which the client sends heartbeats.
	HeartbeatInterval time.Duration
	// MissedHeartbeatLimit is the number of missed beats before the lease is considered stale.
	MissedHeartbeatLimit int
	// GracePeriod is the maximum time to wait for in-flight requests to finish after lease expiry.
	GracePeriod time.Duration
}

// DefaultLeaseConfig returns defaults aligned with the spec (15 s interval, 3 missed beats, 5 s grace).
func DefaultLeaseConfig() LeaseConfig {
	return LeaseConfig{
		HeartbeatInterval:    15 * time.Second,
		MissedHeartbeatLimit: 3,
		GracePeriod:          5 * time.Second,
	}
}

// LeaseTracker monitors heartbeats and coordinates a grace shutdown when the
// lease expires. If a request is in-flight at expiry the tracker waits up to
// GracePeriod for it to complete before triggering shutdown.
type LeaseTracker struct {
	cfg        LeaseConfig
	mu         sync.Mutex
	lastBeat   time.Time
	inflight   atomic.Int64
	shutdownCh chan struct{}
	once       sync.Once
	onShutdown func()
}

// NewLeaseTracker creates a LeaseTracker. onShutdown is called exactly once
// when shutdown is triggered (may be nil). Call Start to begin background
// monitoring.
func NewLeaseTracker(cfg LeaseConfig, onShutdown func()) *LeaseTracker {
	if cfg.MissedHeartbeatLimit <= 0 {
		cfg.MissedHeartbeatLimit = 1
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 15 * time.Second
	}
	if cfg.GracePeriod <= 0 {
		cfg.GracePeriod = 5 * time.Second
	}
	return &LeaseTracker{
		cfg:        cfg,
		lastBeat:   time.Now(),
		shutdownCh: make(chan struct{}),
		onShutdown: onShutdown,
	}
}

// Heartbeat renews the lease; must be called by the client at HeartbeatInterval.
func (lt *LeaseTracker) Heartbeat() {
	lt.mu.Lock()
	lt.lastBeat = time.Now()
	lt.mu.Unlock()
}

// Close terminates the session explicitly and triggers shutdown.
func (lt *LeaseTracker) Close() {
	lt.triggerShutdown()
}

// AcquireInflight increments the in-flight counter. Callers must pair each
// AcquireInflight with a ReleaseInflight.
func (lt *LeaseTracker) AcquireInflight() {
	lt.inflight.Add(1)
}

// ReleaseInflight decrements the in-flight counter.
func (lt *LeaseTracker) ReleaseInflight() {
	lt.inflight.Add(-1)
}

// InflightCount returns the current number of active requests.
func (lt *LeaseTracker) InflightCount() int64 {
	return lt.inflight.Load()
}

// Done returns a channel that is closed when shutdown is triggered.
func (lt *LeaseTracker) Done() <-chan struct{} {
	return lt.shutdownCh
}

// Start begins background lease checking. It stops when stop is closed.
func (lt *LeaseTracker) Start(stop <-chan struct{}) {
	go lt.run(stop)
}

func (lt *LeaseTracker) run(stop <-chan struct{}) {
	interval := lt.cfg.HeartbeatInterval / 2
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-tick.C:
			if lt.isLeaseStale(now) {
				lt.handleExpiry()
				return
			}
		}
	}
}

func (lt *LeaseTracker) isLeaseStale(now time.Time) bool {
	lt.mu.Lock()
	last := lt.lastBeat
	lt.mu.Unlock()
	staleAfter := lt.cfg.HeartbeatInterval * time.Duration(lt.cfg.MissedHeartbeatLimit)
	return now.Sub(last) > staleAfter
}

func (lt *LeaseTracker) handleExpiry() {
	if lt.inflight.Load() == 0 {
		lt.triggerShutdown()
		return
	}
	// Wait up to GracePeriod for in-flight requests to drain.
	deadline := time.Now().Add(lt.cfg.GracePeriod)
	for lt.inflight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	lt.triggerShutdown()
}

func (lt *LeaseTracker) triggerShutdown() {
	lt.once.Do(func() {
		close(lt.shutdownCh)
		if lt.onShutdown != nil {
			lt.onShutdown()
		}
	})
}
