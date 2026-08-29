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
const defaultTimeout = 10 * time.Second

var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

// CallbackFunc is invoked when shutdown begins; returning an error surfaces it to the caller of Wait.
type CallbackFunc func() error

// gracy is package-level so init() can register the signal handler before any caller wires up callbacks.
var gracy *gracer

type gracer struct {
	stop      chan os.Signal
	mu        sync.RWMutex
	callbacks []CallbackFunc
}

func init() {
	stop := make(chan os.Signal, 2)
	signal.Notify(stop, defaultSignals...)

	gracy = &gracer{stop: stop}
}

// AddCallback registers f to run on the next shutdown signal.
func AddCallback(f CallbackFunc) {
	gracy.mu.Lock()
	defer gracy.mu.Unlock()

	gracy.callbacks = append(gracy.callbacks, f)
}

// Wait blocks until a shutdown signal arrives, then runs all registered callbacks within defaultTimeout.
// Returns the joined callback errors on completion, or the lone "waiting timeout" / "force stopped" error
// otherwise; callback failures collected before a timeout or force-stop are discarded.
func Wait() error {
	<-gracy.stop

	return gracefulShutdownWithContextAndTimeout(context.Background(), defaultTimeout)
}

// GracefulShutdown programmatically triggers a shutdown by sending SIGTERM to the registered channel.
func GracefulShutdown() {
	gracy.stop <- syscall.SIGTERM
}

func gracefulShutdownWithContextAndTimeout(ctx context.Context, timeout time.Duration) error {
	gracy.mu.Lock()
	defer gracy.mu.Unlock()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, defaultSignals...)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	errs := make(chan error, len(gracy.callbacks))

	go func() {
		defer close(done)
		defer close(errs)

		for _, f := range gracy.callbacks {
			if err := f(); err != nil {
				errs <- err
			}
		}
	}()

	select {
	case <-done:
		return joinErrors(errs)
	case <-stop:
		return errors.New("gracer force stopped")
	case <-ctx.Done():
		return errors.New("gracer waiting timeout")
	}
}

func joinErrors(errs <-chan error) error {
	var errsSlice []error

	for err := range errs {
		errsSlice = append(errsSlice, err)
	}

	return errors.Join(errsSlice...)
}
