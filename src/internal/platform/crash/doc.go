// Package crash captures unrecovered panics from any goroutine and writes them
// to stderr plus a per-platform OS log (journald on Linux, syslog on macOS/BSD,
// Windows Event Log). Configure via Install, spawn workers via Go, defer Recover.
//
// # Invariant: the package itself must never panic.
//
// The package is the last line of defense: once recovery reaches it, no failure
// may erase or transform the originating PanicValue. A Sink panic is logged and
// skipped so other sinks still see the original Event; handle() preserves it
// through both os.Exit(2) and re-panic paths.
package crash
