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

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/generate"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/hash"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/verify"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/env"
	"github.com/ostapkonst/HashVerifier/internal/platform/flatpak"
	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// App is the GTK application: top-level window, three tabs, and the lifecycle helpers (drag-drop, tab manager, window geometry).
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

// Run starts GTK, builds the window, and blocks until shutdown; path is an optional CLI-supplied file or directory to autofill,
// noConfig enables ephemeral mode (settings are neither read nor written). noConfig is OR'ed with HASHVERIFIER_NO_CONFIG inside initUI.
func Run(path string, noConfig bool) error {
	readyToStartGTKLoop := make(chan error, 1)

	go func() {
		// GTK on macOS requires GUI calls on the main OS thread.
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

		// Runs on the GTK thread once the main loop starts (IdleAdd fires inside
		// gtk.Main iteration); keeps the startup warning + autofill off the init path.
		// widgets.IdleAdd drops the callback if the window is already in destruction.
		widgets.IdleAdd(app.window, func() {
			app.showFlatpakWarningIfNeeded()
			app.fillTabAndSwitch(path)
		})

		shutdown.AddCallback(func() error {
			cancel()
			app.generateTab.Wait()
			app.verifyTab.Wait()
			app.hashTab.Wait()
			// gtk.MainQuit must be posted onto the GTK main loop; calling it directly
			// here would run it on the shutdown goroutine, off the locked GTK thread.
			glib.IdleAdd(gtk.MainQuit)

			return nil
		})

		readyToStartGTKLoop <- nil

		gtk.Main()
	}()

	err := <-readyToStartGTKLoop
	if err != nil {
		return err
	}

	return shutdown.Wait()
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
	// CLI flag takes precedence over the env var, mirroring base.LoadNoConfig in the CLI adapter.
	noConfig := a.noConfig || env.Bool("HASHVERIFIER_NO_CONFIG")

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

	a.settings, err = settings.Load(noConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load settings, using defaults")

		a.settings = settings.DefaultSettings()
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
	if err := a.settings.Save(); err != nil {
		log.Error().Err(err).Msg("Failed to save Flatpak warning suppression setting")
	}
}
