package gracer

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const defaultTimeout = 10 * time.Second

var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

type CallbackFunc func() error

var gracy *gracer

type gracer struct {
	stop      chan os.Signal
	mu        sync.RWMutex
	callbacks []CallbackFunc
}

func init() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, defaultSignals...)

	gracy = &gracer{stop: stop}
}

func AddCallback(f CallbackFunc) {
	gracy.mu.Lock()
	defer gracy.mu.Unlock()

	gracy.callbacks = append(gracy.callbacks, f)
}

func Wait() error {
	<-gracy.stop

	return gracefulShutdownWithContextAndTimeout(context.Background(), defaultTimeout)
}

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
	if len(errs) == 0 {
		return nil
	}

	errsSlice := []error{}

	for err := range errs {
		errsSlice = append(errsSlice, err)
	}

	return errors.Join(errsSlice...)
}
