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

	return openDir(ctx, filepath.Dir(abs))
}

func revealFile(ctx context.Context, path string) error {
	switch runtime.GOOS {
	case "windows":
		return fireAndForget(ctx, "explorer.exe", "/select,", path)
	case "darwin":
		return fireAndForget(ctx, "open", "-R", path)
	default:
		return tryFileManager1(ctx, path)
	}
}

func tryFileManager1(ctx context.Context, path string) error {
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()

	callCtx, cancel := context.WithTimeout(ctx, dbusCallTimeout)
	defer cancel()

	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("connect session bus: %w", err)
	}

	defer func() { _ = conn.Close() }()

	obj := conn.Object("org.freedesktop.FileManager1", dbus.ObjectPath("/org/freedesktop/FileManager1"))
	call := obj.CallWithContext(callCtx, "org.freedesktop.FileManager1.ShowItems", 0, []string{uri}, "")

	return call.Err
}

func openDir(ctx context.Context, dir string) error {
	var name string

	var args []string

	switch runtime.GOOS {
	case "windows":
		name, args = "explorer.exe", []string{dir}
	case "darwin":
		name, args = "open", []string{dir}
	default:
		name, args = "xdg-open", []string{dir}
	}

	if err := fireAndForget(ctx, name, args...); err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrCommandFailed, name, dir, err)
	}

	return nil
}
