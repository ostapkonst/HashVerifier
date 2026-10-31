// Package shutdown coordinates graceful shutdown on SIGINT/SIGTERM/SIGQUIT, running registered callbacks with a timeout.
package shutdown

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// defaultTimeout is long enough for in-flight GTK/CLI cleanup to settle, short enough that a
// runaway goroutine cannot block process exit.
var defaultTimeout = 10 * time.Second

var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

// ErrWaitTimeout is returned by Wait when callbacks did not finish within the configured timeout.
// Callers typically map it to exit code 130 (canceled) because the user is presumed to have
// triggered the shutdown via a signal and the work couldn't drain in time.
var ErrWaitTimeout = errors.New("gracer waiting timeout")

// ErrForceStopped is returned by Wait when a second signal arrives during callback execution.
// Same exit-code mapping as ErrWaitTimeout: the user really wants to exit now.
var ErrForceStopped = errors.New("gracer force stopped")

// ErrAlreadyWaited is returned by Wait when it has already been invoked once.
// Wait is single-shot: the first call drives the shutdown machinery; subsequent calls
// (including concurrent ones) observe ErrAlreadyWaited without blocking.
var ErrAlreadyWaited = errors.New("gracer Wait already invoked")

// CallbackFunc is invoked when shutdown begins; returning an error surfaces it to the caller of Wait.
type CallbackFunc func() error

// gracer holds the shutdown state: two buffered channels (trigger, force), a mutex-protected
// callbacks slice, and atomic guards against re-entry. The single instance is package-level
// (var gracy) so arm() can install the signal handler before any caller wires up callbacks.
//
//   - trigger (cap=1) wakes Wait on either a real signal (forwarded from force) or a
//     programmatic GracefulShutdown; non-blocking sends keep the producer side cheap.
//   - force (cap=1) owns the signal.Notify subscription for real signals; the arm()
//     forwarding goroutine reads the first one to wake Wait via trigger, and the
//     gracefulShutdownWithContextAndTimeout select reads any further signal for
//     force-stop detection.
type gracer struct {
	trigger     chan struct{}
	force       chan os.Signal
	mu          sync.Mutex
	callbacks   []CallbackFunc
	armOnce     sync.Once
	waitStarted atomic.Bool
}

var gracy *gracer

func init() {
	gracy = &gracer{
		trigger: make(chan struct{}, 1),
		force:   make(chan os.Signal, 1),
	}
}

// arm installs the process-wide signal handler on first use. Installing it lazily instead of
// in init() preserves the default signal disposition for invocations that never participate
// in graceful shutdown (e.g. `config show/edit/reset`): signal.Notify suppresses
// SIGINT/SIGTERM/SIGQUIT termination, so a non-participating command would otherwise become
// uninterruptible.
func (g *gracer) arm() {
	g.armOnce.Do(func() {
		signal.Notify(g.force, defaultSignals...)
		// Forward the first real signal to trigger so Wait wakes once; subsequent signals
		// stay in force for the select in gracefulShutdownWithContextAndTimeout.
		go func() {
			<-g.force

			select {
			case g.trigger <- struct{}{}:
			default:
			}
		}()
	})
}

// AddCallback registers f to run on the next shutdown signal; safe to call before Wait.
// It also arms the signal handler, since registering a callback is the point at which the
// caller opts into graceful shutdown.
// After Wait has driven the state machine (one-shot, see ErrAlreadyWaited), new callbacks
// are ignored on the wire but still mutate the slice.
func AddCallback(f CallbackFunc) {
	gracy.arm()

	gracy.mu.Lock()
	defer gracy.mu.Unlock()

	gracy.callbacks = append(gracy.callbacks, f)
}

// Wait blocks until a shutdown signal arrives (real or programmatic GracefulShutdown),
// then runs all registered callbacks within defaultTimeout.
// Returns the joined callback errors on completion, or ErrWaitTimeout / ErrForceStopped
// otherwise; callback failures collected before a timeout or force-stop are discarded.
//
// Wait is single-shot: the first call drives the state machine; any second call — sequential
// or concurrent — returns ErrAlreadyWaited immediately without blocking on the first caller.
// Callers needing multiple lifecycle passes should restructure to one shutdown package
// instance per pass.
func Wait() error {
	gracy.arm()

	if !gracy.waitStarted.CompareAndSwap(false, true) {
		return ErrAlreadyWaited
	}

	<-gracy.trigger

	return gracefulShutdownWithContextAndTimeout(context.Background(), defaultTimeout)
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
	// Snapshot callbacks under the lock so the goroutine below does not race with AddCallback.
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
