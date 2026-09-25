package widgets

import (
	"sync"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// ProgressSink coalesces a burst of progress values into a single high-priority GTK idle
// callback. GtkTreeView schedules row validation at priority 125, above the default idle
// priority used by IdleAdd (200); posting progress at PRIORITY_HIGH_IDLE (100) keeps the bars
// and captions updating during heavy streaming instead of being starved until the run ends.
type ProgressSink[T any] struct {
	window *gtk.Window
	apply  func(T)

	mu     sync.Mutex
	latest T
	queued bool
	gen    uint64
}

// NewProgressSink binds a sink to window; apply runs on the GTK thread with the latest value.
func NewProgressSink[T any](window *gtk.Window, apply func(T)) *ProgressSink[T] {
	return &ProgressSink[T]{window: window, apply: apply}
}

// Post records v as the latest value and schedules one apply if none is already pending, so a
// burst of updates never queues stale redraws.
func (s *ProgressSink[T]) Post(v T) {
	s.mu.Lock()

	s.latest = v
	if s.queued {
		s.mu.Unlock()

		return
	}

	s.queued = true
	s.gen++
	gen := s.gen
	s.mu.Unlock()

	glib.IdleAddPriority(glib.PRIORITY_HIGH_IDLE, func() {
		if !IsAlive(s.window) {
			return
		}

		s.mu.Lock()
		if gen != s.gen {
			s.mu.Unlock()

			return
		}

		v := s.latest
		s.queued = false
		s.mu.Unlock()

		s.apply(v)
	})
}

// Latest returns the most recently posted value (the zero value before the first Post). Safe
// from any goroutine; used by finish handlers to read the terminal snapshot without racing Post.
func (s *ProgressSink[T]) Latest() T {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.latest
}

// Stop discards any queued apply so a finished run cannot overwrite the final UI state.
func (s *ProgressSink[T]) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gen++
	s.queued = false
}
