package base

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// RunWithShutdown wires fn as a RunE that respects context cancellation via cmd.Context() and registers a shutdown callback.
// SIGINT/SIGTERM cancels the context and waits for fn to return, so cleanup finishes before shutdown.Wait returns.
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
