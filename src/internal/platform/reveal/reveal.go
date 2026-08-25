// Package reveal opens the OS file manager with a file or directory selected across platforms.
package reveal

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog/log"
)

// Sentinel errors returned by Reveal when no path is supplied or the file manager fails to launch.
var (
	ErrEmptyPath     = errors.New("reveal: empty path")
	ErrCommandFailed = errors.New("reveal: failed to launch file manager")
)

// dbusCallTimeout caps FileManager1.ShowItems so a hung file manager cannot stall the caller indefinitely.
const dbusCallTimeout = 3 * time.Second

func fireAndForget(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(context.WithoutCancel(ctx), name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() { _ = cmd.Wait() }()

	return nil
}

// Reveal asks the OS file manager to show path.
func Reveal(ctx context.Context, path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	if err := highlight(ctx, abs); err != nil {
		log.Warn().Err(err).Msg("Failed to highlight file in file manager, falling back to opening directory")
	} else {
		return nil
	}

	return openOrFail(ctx, filepath.Dir(abs))
}

func openOrFail(ctx context.Context, dir string) error {
	if err := openDirectory(ctx, dir); err != nil {
		return fmt.Errorf("%w: %v", ErrCommandFailed, err)
	}

	return nil
}

// highlight requests that the OS file manager select abs; on failure Reveal falls back to opening the parent directory.
func highlight(ctx context.Context, abs string) error {
	switch runtime.GOOS {
	case "windows":
		return fireAndForget(ctx, "explorer.exe", "/select,", abs)
	case "darwin":
		return fireAndForget(ctx, "open", "-R", abs)
	default:
		return revealViaDbus(ctx, abs)
	}
}

func openDirectory(ctx context.Context, dir string) error {
	switch runtime.GOOS {
	case "windows":
		return fireAndForget(ctx, "explorer.exe", dir)
	case "darwin":
		return fireAndForget(ctx, "open", dir)
	default:
		url := "file://" + dir
		return fireAndForget(ctx, "xdg-open", url)
	}
}

func revealViaDbus(ctx context.Context, abs string) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck

	url := "file://" + abs
	obj := conn.Object("org.freedesktop.FileManager1", "/org/freedesktop/FileManager1")

	callCtx, cancel := context.WithTimeout(ctx, dbusCallTimeout)
	defer cancel()

	call := obj.CallWithContext(callCtx, "org.freedesktop.FileManager1.ShowItems", 0, []string{url}, "")

	return call.Err
}
