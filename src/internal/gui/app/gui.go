package app

import (
	"context"
	"fmt"
	"runtime"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/envutil"
	"github.com/ostapkonst/HashVerifier/internal/gui/tabs"
	"github.com/ostapkonst/HashVerifier/internal/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/settings"
	"github.com/ostapkonst/HashVerifier/internal/system"
	"github.com/ostapkonst/HashVerifier/utils/gracer"
)

type App struct {
	window       *gtk.Window
	builder      *gtk.Builder
	generateTab  *tabs.GenerateTab
	verifyTab    *tabs.VerifyTab
	hashTab      *tabs.HashTab
	icon         *gdk.Pixbuf
	ctx          context.Context
	settings     *settings.Settings
	notebook     *gtk.Notebook
	tabManager   *TabManager
	windowGeom   *WindowGeometry
	pathResolver *PathResolver
	dragAndDrop  *DragAndDrop
}

func Run(path string) error {
	readyToStartGTKLoop := make(chan error, 1)

	go func() {
		// Без этого может возникнуть такая ошибка:
		// NSInternalInconsistencyException: 'NSWindow should only be instantiated on the main thread!'
		runtime.LockOSThread()

		gtk.Init(nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		app := &App{ctx: ctx}
		if err := app.initUI(); err != nil {
			readyToStartGTKLoop <- fmt.Errorf("failed to initialize UI: %w", err)
			return
		}

		app.window.Show()

		glib.IdleAdd(func() {
			app.showFlatpakWarningIfNeeded()
			app.fillTabAndSwitch(path)
		})

		gracer.AddCallback(func() error {
			cancel()
			app.generateTab.Wait()
			app.verifyTab.Wait()
			app.hashTab.Wait()
			gtk.MainQuit()

			return nil
		})

		readyToStartGTKLoop <- nil

		gtk.Main()
	}()

	err := <-readyToStartGTKLoop
	if err != nil {
		return err
	}

	return gracer.Wait()
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
			return
		}

		a.tabManager.ApplySelectedPage(a.tabManager.GetTabNumberByName("generate"))
	case PathTypeFile:
		_, errAlgFromExt := checksum.AlgorithmFromExtension(resolvedPath)
		_, errAlgFromSums := checksum.AlgorithmFromAllSumsFiles(resolvedPath)

		if errAlgFromExt == nil || errAlgFromSums == nil {
			if err := a.verifyTab.Fill(resolvedPath); err != nil {
				return
			}

			a.tabManager.ApplySelectedPage(a.tabManager.GetTabNumberByName("verify"))
		} else {
			if err := a.hashTab.Fill(resolvedPath); err != nil {
				return
			}

			a.tabManager.ApplySelectedPage(a.tabManager.GetTabNumberByName("hash"))
		}
	}
}

func (a *App) initUI() error {
	noConfig := envutil.Bool("HASHVERIFIER_NO_CONFIG")

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
		gracer.GracefulShutdown()
	})

	if err := a.connectAboutButton(); err != nil {
		return fmt.Errorf("failed to connect about button: %w", err)
	}

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

	a.generateTab = tabs.NewGenerateTab(a.ctx, a.builder, a.window, a.settings)
	a.verifyTab = tabs.NewVerifyTab(a.ctx, a.builder, a.window, a.settings)
	a.hashTab = tabs.NewHashTab(a.ctx, a.builder, a.window, a.settings)
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

func (a *App) connectAboutButton() error {
	obj, err := a.builder.GetObject("main_about")
	if err != nil {
		return fmt.Errorf("failed to get about button: %w", err)
	}

	aboutBtn, ok := obj.(*gtk.Button)
	if !ok {
		return fmt.Errorf("object is not a GtkButton")
	}

	aboutBtn.Connect("clicked", func() {
		widgets.ShowAboutDialog(a.window, a.icon)
	})

	return nil
}

func (a *App) showFlatpakWarningIfNeeded() {
	if a.settings.Flatpak.SuppressSandboxWarning {
		return
	}

	if !system.IsRunningInFlatpak() {
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
