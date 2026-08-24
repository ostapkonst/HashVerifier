// Package reveal opens the OS file manager with a file or directory selected. Behavior is OS-specific: Explorer on Windows, D-Bus org.freedesktop.FileManager1 on Linux, "open -R" on macOS.
package reveal

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/godbus/dbus/v5"
)

var (
	ErrEmptyPath     = errors.New("reveal: empty path")
	ErrCommandFailed = errors.New("reveal: failed to launch file manager")
)

const dbusCallTimeout = 2 * time.Second

func fireAndForget(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(context.WithoutCancel(ctx), name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() { _ = cmd.Wait() }()

	return nil
}

// Reveal asks the OS file manager to highlight path. Returns ErrEmptyPath for "" and ErrCommandFailed if the launcher fails.
func Reveal(ctx context.Context, path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	if _, err := os.Stat(abs); err == nil {
		if rerr := revealFile(ctx, abs); rerr == nil {
			return nil
		}
	}

	switch runtime.GOOS {
	case "windows":
		if err := fireAndForget(ctx, "explorer", "/select,"+abs); err != nil {
			return fmt.Errorf("%w: %v", ErrCommandFailed, err)
		}
	case "darwin":
		if err := fireAndForget(ctx, "open", "-R", abs); err != nil {
			return fmt.Errorf("%w: %v", ErrCommandFailed, err)
		}
	default:
		if err := revealViaDbus(ctx, abs); err != nil {
			if err := fireAndForget(ctx, "xdg-open", "file://"+url.PathEscape(abs)); err != nil {
				return fmt.Errorf("%w: %v", ErrCommandFailed, err)
			}
		}
	}

	return nil
}

func revealFile(ctx context.Context, abs string) error {
	switch runtime.GOOS {
	case "windows":
		return fireAndForget(ctx, "explorer", "/select,"+abs)
	case "darwin":
		return fireAndForget(ctx, "open", "-R", abs)
	default:
		return revealViaDbus(ctx, abs)
	}
}

func revealViaDbus(ctx context.Context, abs string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	uri := "file://" + url.PathEscape(abs)
	obj := conn.Object("org.freedesktop.FileManager1", "/org/freedesktop/FileManager1")

	callCtx, cancel := context.WithTimeout(ctx, dbusCallTimeout)
	defer cancel()

	call := obj.CallWithContext(callCtx, "org.freedesktop.FileManager1.ShowItems", 0, []string{uri}, "")
	if call.Err != nil {
		return call.Err
	}

	return nil
}
