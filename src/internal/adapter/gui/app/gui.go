// Package app is the GTK3 lifecycle layer: it initializes the main window and wires the three tabs.
package app

import (
	"context"
	"fmt"
	"runtime"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/generate"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/hash"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/verify"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/flatpak"
	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// App is the GTK application: top-level window, three tabs, and the lifecycle helpers
// (drag-drop, tab manager, window geometry, path resolver).
type App struct {
	window       *gtk.Window
	builder      *gtk.Builder
	generateTab  *generate.GenerateTab
	verifyTab    *verify.VerifyTab
	hashTab      *hash.HashTab
	icon         *gdk.Pixbuf
	ctx          context.Context
	settings     *settings.Settings
	notebook     *gtk.Notebook
	tabManager   *TabManager
	windowGeom   *WindowGeometry
	pathResolver *PathResolver
	dragAndDrop  *DragAndDrop
	noConfig     bool
}

// Run starts GTK, builds the window, and blocks until shutdown.
//
// path is the literal string passed by the CLI dispatcher (root-level, any 0–1 positional args).
// It is intended to be a file or directory to autofill, but the dispatcher does no flag parsing, so
// any user-supplied string reaches this function — flags like "--version" or names that happen to
// match a CLI command name (e.g. "generate") all land here. PathResolver rejects unstatable values
// and the GUI opens normally without autofill.
//
// noConfig enables ephemeral mode (settings are neither read nor written). Callers are expected to
// resolve noConfig against the env themselves (env.NoConfig) before calling Run.
func Run(path string, noConfig bool) error {
	readyToStartGTKLoop := make(chan error, 1)

	crash.Go("gui.gtkMain", func() {
		// GTK3 requires GUI calls on the OS thread that called gtk.Init; required on every platform, not just macOS.
		runtime.LockOSThread()

		gtk.Init(nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		app := &App{ctx: ctx, noConfig: noConfig}
		if err := app.initUI(); err != nil {
			readyToStartGTKLoop <- fmt.Errorf("failed to initialize UI: %w", err)
			return
		}

		app.window.Show()

		// IdleAdd fires inside gtk.Main iteration; keeping the warning + autofill off the init path
		// also lets widgets.IdleAdd drop the callback if the window is already in destruction.
		widgets.IdleAdd(app.window, func() {
			app.showFlatpakWarningIfNeeded()
			app.fillTabAndSwitch(path)
		})

		shutdown.AddCallback(func() error {
			cancel()
			app.generateTab.Wait()
			app.verifyTab.Wait()
			app.hashTab.Wait()
			// Drain any pending AsyncSave so the last user-driven change lands on disk
			// before gtk_main quits; writeMu serializes against Sync Save() from CLI hooks.
			app.settings.Close()
			// gtk.MainQuit must post onto the GTK main loop; calling it directly here would run
			// on the shutdown goroutine, off the locked GTK thread.
			glib.IdleAdd(gtk.MainQuit)

			return nil
		})

		readyToStartGTKLoop <- nil

		gtk.Main()
	})

	err := <-readyToStartGTKLoop
	if err != nil {
		return err
	}

	return base.MapShutdownError(shutdown.Wait())
}

func (a *App) fillTabAndSwitch(path string) {
	pathType, resolvedPath, err := a.pathResolver.Resolve(path)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fill tab because of invalid path")
		return
	}

	switch pathType {
	case PathTypeDirectory:
		if err := a.generateTab.Fill(resolvedPath); err != nil {
			log.Warn().Err(err).Str("path", resolvedPath).Msg("Failed to fill Generate tab")
		}

		a.tabManager.ApplySelectedPage(a.tabManager.GetTabNumberByName("generate"))
	case PathTypeFile:
		_, errAlgFromExt := algorithm.AlgorithmFromExtension(resolvedPath)
		_, errAlgFromSums := algorithm.AlgorithmFromAllSumsFiles(resolvedPath)

		if errAlgFromExt == nil || errAlgFromSums == nil {
			if err := a.verifyTab.Fill(resolvedPath); err != nil {
				log.Warn().Err(err).Str("path", resolvedPath).Msg("Failed to fill Verify tab")
			}

			a.tabManager.ApplySelectedPage(a.tabManager.GetTabNumberByName("verify"))
		} else {
			if err := a.hashTab.Fill(resolvedPath); err != nil {
				log.Warn().Err(err).Str("path", resolvedPath).Msg("Failed to fill Hash tab")
			}

			a.tabManager.ApplySelectedPage(a.tabManager.GetTabNumberByName("hash"))
		}
	}
}

func (a *App) initUI() error {
	noConfig := a.noConfig

	builder, err := widgets.GetMainForm()
	if err != nil {
		return fmt.Errorf("failed to get main form: %w", err)
	}

	a.builder = builder

	favIcon, err := widgets.GetMainIcon()
	if err != nil {
		return fmt.Errorf("failed to get main icon: %w", err)
	}

	a.icon = favIcon

	window, err := widgets.GetMainWindow(builder)
	if err != nil {
		return fmt.Errorf("failed to get main window: %w", err)
	}

	a.window = window
	if noConfig {
		window.SetTitle("HashVerifier — Ephemeral Mode")
	}

	window.SetIcon(favIcon)
	window.Connect("destroy", func() {
		shutdown.GracefulShutdown()
	})

	a.connectAboutButton()

	if noConfig {
		a.settings = settings.NewDefaultsWithWorker(true)
	} else {
		a.settings, err = settings.Load()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load settings, using defaults")

			a.settings = settings.NewDefaultsWithWorker(false)
		}
	}

	for _, w := range a.settings.LoadWarnings() {
		log.Warn().
			Str("field", w.Field).
			Str("invalid_value", w.Value).
			Str("default", w.Default).
			Msg("Invalid settings value, replaced with default")
	}

	a.generateTab = generate.NewGenerateTab(a.ctx, a.builder, a.window, a.settings)
	a.verifyTab = verify.NewVerifyTab(a.ctx, a.builder, a.window, a.settings)
	a.hashTab = hash.NewHashTab(a.ctx, a.builder, a.window, a.settings)
	a.notebook = widgets.GetNotebook(a.builder, "notebook")
	a.tabManager = NewTabManager(a.notebook, a.window, a.settings)
	a.windowGeom = NewWindowGeometry(a.window, a.settings)
	a.pathResolver = NewPathResolver()
	a.dragAndDrop = NewDragAndDrop(a.window, a.pathResolver, a.fillTabAndSwitch)
	a.tabManager.ApplyTabOrder()
	a.tabManager.ApplyCurrentPage()
	a.windowGeom.Restore()
	a.tabManager.ConnectReorderHandler()
	a.tabManager.ConnectSwitchHandler()
	a.windowGeom.ConnectEvents()
	a.dragAndDrop.Setup()
	a.dragAndDrop.DisableDropOnInputWidgets(a.window)

	return nil
}

func (a *App) connectAboutButton() {
	aboutBtn := widgets.GetButton(a.builder, "main_about")

	aboutBtn.Connect("clicked", func() {
		widgets.ShowAboutDialog(a.window, a.icon)
	})
}

func (a *App) showFlatpakWarningIfNeeded() {
	if a.settings.Flatpak.SuppressSandboxWarning {
		return
	}

	if !flatpak.IsRunningInFlatpak() {
		return
	}

	suppress := widgets.ShowFlatpakSandboxWarningDialog(a.window)
	if !suppress {
		return
	}

	a.settings.Flatpak.SuppressSandboxWarning = true
	if err := a.settings.AsyncSave(); err != nil {
		log.Error().Err(err).Msg("Failed to save Flatpak warning suppression setting")
	}
}
