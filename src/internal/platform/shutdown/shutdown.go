// Package shutdown coordinates graceful shutdown on SIGINT/SIGTERM/SIGQUIT, running registered callbacks with a timeout.
package shutdown

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// defaultTimeout is long enough for in-flight GTK/CLI cleanup to settle, short enough that a runaway goroutine cannot block process exit.
var defaultTimeout = 10 * time.Second

var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

// ErrWaitTimeout is returned by Wait when callbacks did not finish within the configured timeout.
// Callers typically map it to exit code 130 (canceled) because the user is presumed to have
// triggered the shutdown via SIGINT and the work couldn't drain in time.
var ErrWaitTimeout = errors.New("gracer waiting timeout")

// ErrForceStopped is returned by Wait when a second signal arrives during callback execution.
// Same exit-code mapping as ErrWaitTimeout: the user really wants to exit now.
var ErrForceStopped = errors.New("gracer force stopped")

// ErrAlreadyWaited is returned by Wait when it has already been invoked once.
// Wait is single-shot: the first call drives the shutdown machinery; subsequent calls
// observe ErrAlreadyWaited without blocking on the trigger channel (which never fires again).
var ErrAlreadyWaited = errors.New("gracer Wait already invoked")

// CallbackFunc is invoked when shutdown begins; returning an error surfaces it to the caller of Wait.
type CallbackFunc func() error

// gracy is package-level so init() can register the signal handler before any caller wires up callbacks.
//
// Two channels with disjoint ownership:
//   - trigger (cap=1) wakes Wait on either a real signal (forwarded from force) or a
//     programmatic GracefulShutdown; non-blocking sends keep the producer side cheap.
//   - force (cap=1) owns the signal.Notify subscription for real signals; the init()
//     forwarding goroutine reads the first one to wake Wait via trigger, and the
//     gracefulShutdownWithContextAndTimeout select reads any further signal for
//     force-stop detection.
//
// waitOnce guards Wait against re-entry so callbacks run exactly once.
type gracer struct {
	trigger   chan struct{}
	force     chan os.Signal
	mu        sync.Mutex
	callbacks []CallbackFunc
	waitOnce  sync.Once
}

var gracy *gracer

func init() {
	gracy = &gracer{
		trigger: make(chan struct{}, 1),
		force:   make(chan os.Signal, 1),
	}
	signal.Notify(gracy.force, defaultSignals...)
	// Forward the first real signal to trigger so Wait wakes once.
	// Subsequent signals during callback execution stay in force and are read
	// by the select in gracefulShutdownWithContextAndTimeout (ErrForceStopped).
	// Capture the gracer locally so this goroutine does not race with any
	// later replacement of the package-level gracy variable.
	g := gracy
	go func() {
		<-g.force

		select {
		case g.trigger <- struct{}{}:
		default:
		}
	}()
}

// AddCallback registers f to run on the next shutdown signal. Safe to call before Wait; once Wait has driven the state machine (one-shot, see ErrAlreadyWaited) any new callbacks are ignored on the wire but still mutate the slice.
func AddCallback(f CallbackFunc) {
	gracy.mu.Lock()
	defer gracy.mu.Unlock()

	gracy.callbacks = append(gracy.callbacks, f)
}

// Wait blocks until a shutdown signal arrives (real or programmatic GracefulShutdown),
// then runs all registered callbacks within defaultTimeout.
// Returns the joined callback errors on completion, or ErrWaitTimeout / ErrForceStopped
// otherwise; callback failures collected before a timeout or force-stop are discarded.
//
// Wait is single-shot: the second call returns ErrAlreadyWaited immediately because the
// underlying state machine has already been driven. Callers needing multiple lifecycle
// passes should restructure to one shutdown package instance per pass.
func Wait() error {
	var (
		err       error
		firstCall bool
	)

	gracy.waitOnce.Do(func() {
		firstCall = true

		<-gracy.trigger

		err = gracefulShutdownWithContextAndTimeout(context.Background(), defaultTimeout)
	})

	if !firstCall {
		return ErrAlreadyWaited
	}

	return err
}

// GracefulShutdown programmatically triggers a shutdown by posting a non-blocking trigger.
// After Wait has already run, or while Wait is blocked, the trigger buffer stays at capacity
// and the send is silently dropped — no real signal is ever lost and the caller never stalls.
func GracefulShutdown() {
	select {
	case gracy.trigger <- struct{}{}:
	default:
	}
}

func gracefulShutdownWithContextAndTimeout(ctx context.Context, timeout time.Duration) error {
	gracy.mu.Lock()
	// Snapshot callbacks while holding the lock so the goroutine below does not race
	// with AddCallback.
	callbacks := gracy.callbacks
	force := gracy.force
	gracy.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	errs := make(chan error, len(callbacks))

	go func() {
		defer close(done)
		defer close(errs)

		for _, f := range callbacks {
			if err := f(); err != nil {
				errs <- err
			}
		}
	}()

	select {
	case <-done:
		return joinErrors(errs)
	case <-force:
		return ErrForceStopped
	case <-ctx.Done():
		return ErrWaitTimeout
	}
}

func joinErrors(errs <-chan error) error {
	var errsSlice []error

	for err := range errs {
		errsSlice = append(errsSlice, err)
	}

	return errors.Join(errsSlice...)
}
