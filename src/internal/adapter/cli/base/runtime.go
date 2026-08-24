package base

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// RunWithShutdown wires a RunE that respects context cancellation via cmd.Context() and registers a shutdown callback so SIGINT/SIGTERM triggers graceful cleanup before Wait returns.
func RunWithShutdown(cmd *cobra.Command, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	done := make(chan error, 1)

	shutdown.AddCallback(func() error {
		cancel()
		return <-done
	})

	go func() {
		done <- fn(ctx)

		shutdown.GracefulShutdown()
	}()

	return shutdown.Wait()
}
