// Package widgets is the home of reusable GTK widgets, dialogs, and helpers shared by every tab.
package widgets

import "time"

// StreamBatchConfig configures batching of channel items for the GTK run-stream loop. Items matching IsProgress are forwarded to OnProgress immediately; other items are accumulated and flushed to OnBatch by size, time, or channel close. OnFinish is invoked exactly once via defer when RunStream terminates.
type StreamBatchConfig[T any] struct {
	FlushSize     int
	FlushInterval time.Duration
	IsProgress    func(T) bool
	GetError      func(T) error
	OnProgress    func(T)
	OnBatch       func(items []T)
	OnFinish      func(err error)
}

// RunStream consumes from ch, batching items per cfg and dispatching via the callbacks; stops on the first error item.
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

		timer = time.NewTimer(cfg.FlushInterval)
		timerCh = timer.C
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

		timer = nil
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

		if cfg.OnBatch != nil {
			cfg.OnBatch(items)
		}
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

			items := batch
			batch = nil

			if cfg.OnBatch != nil {
				cfg.OnBatch(items)
			}
		}
	}
}
