package crash

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// Reporter collects runtime crashes and routes them to one or more Sinks.
// Constructed by Install and consulted via the package-level helpers (Go, Recover,
// SetExitOnPanic, SetOnGUI).
type Reporter struct {
	app, version string
	sinks        []Sink
	onGUI        atomic.Pointer[func(Event)]
	exitOnPanic  atomic.Bool
	inHandler    atomic.Bool
}

// Options configures Install. App and Version are written into every Event so
// crash reports self-identify.
type Options struct {
	App, Version string
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

	r := &Reporter{app: opts.App, version: opts.Version}

	r.sinks = append(r.sinks, &stderrSink{})
	if s, err := newPlatformSink(); err == nil && s != nil {
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

// SetExitOnPanic toggles whether the Reporter terminates the process via os.Exit
// after sinks complete. CLI defaults to false (re-panic lets Go runtime print
// the stack to stderr and exit with code 2); GUI should set true to exit cleanly
// with code 1 after displaying the error dialog.
func SetExitOnPanic(b bool) {
	if r := global.Load(); r != nil {
		r.exitOnPanic.Store(b)
	}
}

// SetOnGUI installs a closure that runs after sinks complete; typically used by
// the GUI adapter to display a GTK error dialog on the GTK main thread. Safe to
// call from any goroutine; safe to call multiple times (last write wins).
func SetOnGUI(fn func(Event)) {
	if r := global.Load(); r != nil {
		r.onGUI.Store(&fn)
	}
}

// Recover returns a deferred function suitable for `defer r.Recover()` at the
// top of any goroutine entry point. The recovered panic is funneled through
// handle, which writes to every Sink in order and then either re-panics or
// exits per the current exitOnPanic setting.
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

	onGUIPtr := r.onGUI.Load()
	if onGUIPtr != nil {
		func() {
			// OnGUI recover: the callback is user-supplied code that runs on
			// the GTK main thread. If it panics, the surrounding handle()
			// must still reach os.Exit(1) to honor the documented exit code.
			defer func() {
				if v := recover(); v != nil {
					log.Warn().Interface("panic", v).Msg("OnGUI callback panicked")
				}
			}()

			(*onGUIPtr)(ev)
		}()
	}

	if r.exitOnPanic.Load() {
		code := 2 // bare exit (CLI safety net or future use without a dialog).
		if onGUIPtr != nil {
			code = 1 // GUI: clean exit so no Go-runtime stack dump pollutes stderr.
		}

		os.Exit(code) //nolint:gocritic // the deferred inHandler.Store(false) is irrelevant once the process exits
	}

	panic(ev.PanicValue)
}
