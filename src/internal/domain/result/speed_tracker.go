package result

import (
	"sync"
	"time"
)

// SpeedTracker accumulates hashed bytes for live throughput display (rolling bytes/sec).
type SpeedTracker struct {
	mu        sync.Mutex
	bytes     int64
	startTime time.Time
	speed     float64
}

// NewSpeedTracker returns an empty SpeedTracker ready for AddBytes calls.
func NewSpeedTracker() *SpeedTracker {
	return &SpeedTracker{}
}

// AddBytes accumulates n bytes (no-op when n <= 0, guard against misuse).
// The first call after Reset sets startTime; subsequent calls recompute speed = bytes / elapsed.
func (t *SpeedTracker) AddBytes(n int64) {
	if n <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.bytes += n

	if t.startTime.IsZero() {
		t.startTime = time.Now()
		return
	}

	elapsed := time.Since(t.startTime).Seconds()
	if !(elapsed > 0) {
		return
	}

	t.speed = float64(t.bytes) / elapsed
}

// Speed returns the rolling throughput in bytes per second since the first AddBytes call after Reset.
func (t *SpeedTracker) Speed() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.speed
}

// Reset zeros the byte counter, speed, and start time; the next AddBytes starts a new measurement window.
func (t *SpeedTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.bytes = 0
	t.speed = 0
	t.startTime = time.Time{}
}
