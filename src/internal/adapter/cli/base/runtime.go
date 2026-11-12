package base

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// RunWithShutdown wraps fn in a child context of cmd.Context() and registers a shutdown callback that cancels it on SIGINT/SIGTERM/SIGQUIT.
// SIGINT/SIGTERM/SIGQUIT cancels the context and waits for fn to return, so cleanup finishes before shutdown.Wait returns.
//
// Exit code mapping (matching docs/USAGE.md):
//   - fn returned nil and shutdown.Wait returned nil → 0
//   - fn returned an error → propagated as-is; an *ExitError keeps its own code (1 on failure,
//     130 when canceled), a plain error exits 1
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

	crash.Go("cli.runtime.worker", func() {
		done <- fn(ctx)

		shutdown.GracefulShutdown()
	})

	return MapShutdownError(shutdown.Wait())
}

// RunCommand adapts a subcommand's exec function to cobra's RunE signature through RunWithShutdown.
func RunCommand(cmd *cobra.Command, args []string, exec func(context.Context, *cobra.Command, []string) error) error {
	return RunWithShutdown(cmd, func(ctx context.Context) error {
		return exec(ctx, cmd, args)
	})
}
