package base

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// RunWithShutdown wires fn as a RunE that respects context cancellation via cmd.Context() and registers a shutdown callback.
// SIGINT/SIGTERM cancels the context and waits for fn to return, so cleanup finishes before shutdown.Wait returns.
//
// Exit code mapping (matching docs/USAGE.md):
//   - fn returned nil and shutdown.Wait returned nil → 0
//   - fn returned an error and shutdown.Wait propagated it → 1 (caller wraps in ExitError)
//   - shutdown.Wait returned ErrWaitTimeout (callbacks exceeded the 10s budget) → 130
//   - shutdown.Wait returned ErrForceStopped (user pressed Ctrl+C twice) → 130
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

	return MapShutdownError(shutdown.Wait())
}
