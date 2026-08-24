package result

import (
	"sync"
	"time"
)

type SpeedTracker struct {
	mu        sync.Mutex
	bytes     int64
	startTime time.Time
	speed     float64
}

func NewSpeedTracker() *SpeedTracker {
	return &SpeedTracker{}
}

func (t *SpeedTracker) AddBytes(n int64) {
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

func (t *SpeedTracker) Speed() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.speed
}

func (t *SpeedTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.bytes = 0
	t.speed = 0
	t.startTime = time.Time{}
}
