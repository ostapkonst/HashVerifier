package app

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
)

// guiErrorDialogTimeout caps the GTK liveness check: if the main loop has not
// dispatched our idle callback within this window, GTK is broken and we give
// up on the dialog. Once the callback runs and the dialog is visible, we wait
// indefinitely so the user can read and dismiss at their own pace.
const guiErrorDialogTimeout = 3 * time.Second

// showGUIErrorDialog returns a crash.Event handler that posts a modal error on
// the GTK main loop and waits for the user to dismiss it.
//
// This recover in the GTK-thread callback is a deliberate exception to the
// "no recover in user code" rule (widgets.IdleAdd deliberately panics on errors
// instead). The crash subsystem is the last line of defence: if widgets.ShowError
// itself panics, the GTK main thread dies, handle() never reaches os.Exit(1) and
// the process exits with code 2 (panic) instead of 1 (GUI error). Recovering here
// keeps the original exit path intact, logs the secondary failure, and still
// triggers close(done) so the outer goroutine proceeds to os.Exit(1).
func showGUIErrorDialog(window *gtk.Window) func(crash.Event) {
	return func(ev crash.Event) {
		var callbackStarted atomic.Bool

		done := make(chan struct{})

		glib.IdleAdd(func() {
			defer close(done)

			defer func() {
				if v := recover(); v != nil {
					log.Error().Interface("panic", v).Msg("GTK-thread ShowError panicked")
				}
			}()

			callbackStarted.Store(true)

			if !widgets.IsAlive(window) {
				return
			}

			widgets.ShowError(window, "Unexpected Error",
				fmt.Sprintf("HashVerifier encountered an unexpected error:\n\n%v\n\nDetails have been written to the system log.", ev.PanicValue))
		})

		// Phase 1: GTK liveness check. Wait up to guiErrorDialogTimeout for the
		// idle callback to be dispatched. A timer (not time.After) lets us stop
		// it cleanly on the happy path.
		timer := time.NewTimer(guiErrorDialogTimeout)
		select {
		case <-done:
			timer.Stop()
			return
		case <-timer.C:
		}

		// Phase 2: GTK did not dispatch the callback — broken init or
		// teardown. Bail out so handle() can reach os.Exit(1).
		if !callbackStarted.Load() {
			log.Warn().Dur("timeout", guiErrorDialogTimeout).Msg("GUI error dialog timed out; GTK did not dispatch callback")
			return
		}

		// Phase 3: GTK is alive and the dialog is on screen. Block until the
		// user clicks OK; no timeout is acceptable here.
		<-done
	}
}
