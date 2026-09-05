package crash

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// Reporter collects runtime crashes and routes them to one or more Sinks.
// Constructed by Install and consulted via the package-level helpers (Go, Recover,
// SetExitOnPanic).
type Reporter struct {
	app, version, link string
	sinks              []Sink
	exitOnPanic        atomic.Bool
	inHandler          atomic.Bool
}

// Options configures Install. App, Version and Link are written into every
// Event and used to compose the formatted crash report; crash reports
// self-identify and point readers at the project's issue tracker.
type Options struct {
	App, Version, Link string
}

// global holds the installed Reporter; nil before Install or after a failed Install.
var global atomic.Pointer[Reporter]

// Install creates the global Reporter with the always-on stderr Sink plus a
// best-effort platform OS log Sink. First call wins; subsequent calls return the
// existing Reporter and ignore opts, so library init order cannot surprise main.
func Install(opts Options) *Reporter {
	if existing := global.Load(); existing != nil {
		return existing
	}

	r := &Reporter{app: opts.App, version: opts.Version, link: opts.Link}

	r.sinks = append(r.sinks, &stderrSink{r: r})
	if s, err := newPlatformSink(r); err == nil {
		r.sinks = append(r.sinks, s)
	}

	global.Store(r)

	return r
}

// Recover returns a function for `defer crash.Recover()` at the top of main or
// any entry point that runs on its own goroutine. If Install has not been called,
// the returned function still recovers and prints to stderr before exiting 2.
func Recover() func() {
	r := global.Load()
	if r == nil {
		return func() {
			v := recover()
			if v == nil {
				return
			}

			fmt.Fprintln(os.Stderr, "panic (no crash reporter installed):", v)
			os.Exit(2)
		}
	}

	return r.Recover()
}

// Go runs fn in a new goroutine; any panic is recovered and reported under name.
// If Install has not been called, falls back to bare `go fn` so legacy code paths
// retain their original crash-to-runtime behavior.
func Go(name string, fn func()) {
	r := global.Load()
	if r == nil {
		go fn()

		return
	}

	r.Go(name, fn)
}

// SetExitOnPanic controls how handle() terminates the process after sinks complete.
// true exits with os.Exit(2); false re-panics so Go runtime prints the stack to
// stderr and exits with code 2. main.go calls this with true at startup.
func SetExitOnPanic(b bool) {
	if r := global.Load(); r != nil {
		r.exitOnPanic.Store(b)
	}
}

// Recover returns a deferred function suitable for `defer r.Recover()` at the
// top of any goroutine entry point. The recovered panic is funneled through
// handle, which writes to every Sink in order and then terminates the process
// per the current exitOnPanic setting.
func (r *Reporter) Recover() func() {
	return func() {
		v := recover()
		if v == nil {
			return
		}

		r.handle(buildEvent(r, v, "main"))
	}
}

// Go runs fn in a new goroutine; any panic is recovered and reported under name.
// The wrapper defer is per-goroutine so re-panics from handle escape the wrapper
// cleanly and reach the Go runtime (CLI exit 2 with stack) without double-reporting.
func (r *Reporter) Go(name string, fn func()) {
	go func() {
		defer func() {
			v := recover()
			if v == nil {
				return
			}

			r.handle(buildEvent(r, v, name))
		}()

		fn()
	}()
}

func (r *Reporter) handle(ev Event) {
	if !r.inHandler.CompareAndSwap(false, true) {
		fmt.Fprintln(os.Stderr, "panic-in-crash-handler:", ev.PanicValue)
		os.Exit(2)
	}
	defer r.inHandler.Store(false)

	for _, s := range r.sinks {
		func(s Sink) {
			// Per-sink recover: a faulty sink must not block the rest of the
			// chain. We log the secondary failure and let the original Event
			// (with its original PanicValue) reach the remaining sinks.
			defer func() {
				if v := recover(); v != nil {
					log.Warn().Str("sink", s.Name()).Interface("panic", v).Msg("crash sink panicked")
				}
			}()

			if err := s.Send(ev); err != nil {
				log.Warn().Err(err).Str("sink", s.Name()).Msg("crash sink failed")
			}
		}(s)
	}

	if r.exitOnPanic.Load() {
		os.Exit(2) //nolint:gocritic // deferred inHandler.Store(false) is irrelevant after os.Exit
	}

	panic(ev.PanicValue)
}
