// Package crash captures unrecovered panics from any goroutine and writes them
// to stderr plus a per-platform OS log: journald on Linux, syslog on macOS/BSD,
// Windows Event Log on Windows. Configure once via Install, spawn workers via Go,
// and defer Recover at the top of any entry point that already runs on its own goroutine.
//
// # Invariant: the package itself must never panic
//
// This package is the last line of defence for the rest of the program: once
// panic recovery reaches it, no further failure should be able to erase the
// original cause. Every internal recover exists to preserve that cause:
//
//   - If a Sink panics, we log the failure and skip that sink so the remaining
//     sinks still receive the original Event. The original PanicValue is not
//     modified.
//   - handle() preserves the original PanicValue through its exit step:
//     SetExitOnPanic(true) terminates via os.Exit(2) after sinks; the default
//     re-panics so Go runtime prints the stack to stderr and exits with code 2.
//
// Adding a new panic() or recover() to this package requires preserving the
// invariant. The package must not lose, transform, or shadow the originating
// panic value; it may only log, skip, or re-raise.
package crash
