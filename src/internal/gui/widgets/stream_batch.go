package widgets

import "time"

// StreamBatchConfig configures batching of channel items for GUI consumers.
//
// Items for which IsProgress returns true are forwarded to OnProgress
// immediately, one call per item. Other items are accumulated and
// delivered to OnBatch as slices. A slice is flushed when it reaches
// FlushSize entries, after FlushInterval has elapsed since the first
// item in the current slice, or when the channel is closed.
//
// If GetError returns a non-nil error for an item, the current batch
// is flushed, OnProgress is invoked for the error item (so the GUI can
// show final stats), and RunStream stops consuming further items.
//
// OnFinish, if set, is invoked exactly once when RunStream terminates:
// with nil on graceful close, or with the triggering error otherwise.
// It is called via a defer, so it always runs — even if the consuming
// goroutine is about to return.
type StreamBatchConfig[T any] struct {
	FlushSize     int
	FlushInterval time.Duration
	IsProgress    func(T) bool
	GetError      func(T) error
	OnProgress    func(T)
	OnBatch       func(items []T)
	OnFinish      func(err error)
}

// RunStream consumes from ch until it is closed or an error item is
// seen, dispatching items to the configured callbacks. OnFinish, if
// set, is invoked exactly once with the final outcome (nil on graceful
// close, the triggering error otherwise) — always via a defer, so it
// runs even if the caller's goroutine panics mid-loop.
func RunStream[T any](ch <-chan T, cfg StreamBatchConfig[T]) {
	var (
		batch        []T
		pendingFlush bool
		timer        *time.Timer
		timerCh      <-chan time.Time
		resultErr    error
	)

	startTimer := func() {
		if cfg.FlushInterval <= 0 {
			return
		}

		if timer == nil {
			timer = time.NewTimer(cfg.FlushInterval)
			timerCh = timer.C
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			timer.Reset(cfg.FlushInterval)
		}

		pendingFlush = true
	}

	stopTimer := func() {
		if timer == nil {
			return
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		timerCh = nil
		pendingFlush = false
	}

	flush := func() {
		if len(batch) == 0 {
			stopTimer()

			return
		}

		items := batch
		batch = nil

		stopTimer()
		cfg.OnBatch(items)
	}

	defer func() {
		if cfg.OnFinish != nil {
			cfg.OnFinish(resultErr)
		}
	}()

	for {
		select {
		case item, ok := <-ch:
			if !ok {
				flush()

				return
			}

			if cfg.GetError != nil {
				if err := cfg.GetError(item); err != nil {
					flush()

					if cfg.IsProgress == nil || cfg.IsProgress(item) {
						if cfg.OnProgress != nil {
							cfg.OnProgress(item)
						}
					}

					resultErr = err

					return
				}
			}

			if cfg.IsProgress != nil && cfg.IsProgress(item) {
				if cfg.OnProgress != nil {
					cfg.OnProgress(item)
				}

				continue
			}

			batch = append(batch, item)
			if cfg.FlushSize > 0 && len(batch) >= cfg.FlushSize {
				flush()

				continue
			}

			if !pendingFlush {
				startTimer()
			}

		case <-timerCh:
			timerCh = nil
			timer = nil
			pendingFlush = false

			if len(batch) > 0 {
				items := batch
				batch = nil

				cfg.OnBatch(items)
			}
		}
	}
}
