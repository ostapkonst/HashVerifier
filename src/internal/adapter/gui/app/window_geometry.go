package app

import (
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

// WindowGeometry tracks and persists window size, position, and state across sessions.
type WindowGeometry struct {
	window       *gtk.Window
	settings     *settings.Settings
	windowState  settings.WindowState
	normalWidth  int
	normalHeight int
	normalX      int
	normalY      int
}

func NewWindowGeometry(window *gtk.Window, settings *settings.Settings) *WindowGeometry {
	return &WindowGeometry{
		window:   window,
		settings: settings,
	}
}

// Restore applies the persisted window size, position, and state from settings per WindowSettings.RestoreMode.
func (wg *WindowGeometry) Restore() {
	if wg.settings.Window.RestoreMode == settings.RestoreModeDefault {
		return
	}

	if (wg.settings.Window.RestoreMode == settings.RestoreModeSize ||
		wg.settings.Window.RestoreMode == settings.RestoreModeAll) &&
		(wg.settings.Window.Width > 0 && wg.settings.Window.Height > 0) {
		wg.window.Resize(wg.settings.Window.Width, wg.settings.Window.Height)
		wg.normalWidth = wg.settings.Window.Width
		wg.normalHeight = wg.settings.Window.Height
	}

	if (wg.settings.Window.RestoreMode == settings.RestoreModePosition ||
		wg.settings.Window.RestoreMode == settings.RestoreModeAll) &&
		(wg.settings.Window.X >= 0 || wg.settings.Window.Y >= 0) {
		wg.window.Move(wg.settings.Window.X, wg.settings.Window.Y)
		wg.normalX = wg.settings.Window.X
		wg.normalY = wg.settings.Window.Y
	}

	if wg.settings.Window.RestoreMode == settings.RestoreModeSize ||
		wg.settings.Window.RestoreMode == settings.RestoreModeAll {
		switch wg.settings.Window.WindowState {
		case settings.WindowStateMaximized:
			wg.window.Maximize()
		case settings.WindowStateFullscreen:
			wg.window.Fullscreen()
		}
	}
}

// Save persists the current window size, position, and state to settings (no-op during destruction).
func (wg *WindowGeometry) Save() {
	if wg.windowState == settings.WindowStateNormal {
		width, height := wg.window.GetSize()
		x, y := wg.window.GetPosition()
		wg.normalWidth = width
		wg.normalHeight = height
		wg.normalX = x
		wg.normalY = y
	}

	if wg.settings.Window.RestoreMode == settings.RestoreModeSize ||
		wg.settings.Window.RestoreMode == settings.RestoreModeAll {
		wg.settings.Window.Width = wg.normalWidth
		wg.settings.Window.Height = wg.normalHeight
		state := wg.windowState
		wg.settings.Window.WindowState = state
	}

	if wg.settings.Window.RestoreMode == settings.RestoreModePosition ||
		wg.settings.Window.RestoreMode == settings.RestoreModeAll {
		wg.settings.Window.X = wg.normalX
		wg.settings.Window.Y = wg.normalY
	}

	if err := wg.settings.Save(); err != nil {
		log.Error().Err(err).Msg("Failed to save window geometry")
	}
}

// ConnectEvents wires configure-event and window-state-event handlers so Save runs on every geometry change.
func (wg *WindowGeometry) ConnectEvents() {
	wg.window.Connect("delete-event", func() {
		wg.Save()
	})
	wg.window.Connect("window-state-event", func(_ *gtk.Window, event *gdk.Event) {
		wsEvent := gdk.EventWindowStateNewFromEvent(event)
		if wsEvent == nil {
			return
		}

		newState := wsEvent.NewWindowState()
		switch {
		case newState&gdk.WINDOW_STATE_FULLSCREEN != 0:
			wg.windowState = settings.WindowStateFullscreen
		case newState&gdk.WINDOW_STATE_MAXIMIZED != 0:
			wg.windowState = settings.WindowStateMaximized
		default:
			wg.windowState = settings.WindowStateNormal
		}
	})
	wg.window.Connect("configure-event", func() {
		if wg.windowState == settings.WindowStateNormal {
			width, height := wg.window.GetSize()
			x, y := wg.window.GetPosition()
			wg.normalWidth = width
			wg.normalHeight = height
			wg.normalX = x
			wg.normalY = y
		}
	})
}
