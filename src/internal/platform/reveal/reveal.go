// Package reveal opens the OS file manager with a file or directory selected across platforms.
package reveal

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
)

// Sentinel errors for the failure modes Reveal can classify on its own:
// ErrEmptyPath on empty input, ErrCommandFailed when the fallback open also
// fails. Highlight failures are logged and silently fall back to opening the
// parent directory; they do not surface as ErrCommandFailed. filepath.Abs /
// highlight / openOrFail failures are returned wrapped, not as bare sentinels.
var (
	ErrEmptyPath     = errors.New("empty path")
	ErrCommandFailed = errors.New("failed to launch file manager")
)

// dbusCallTimeout caps FileManager1.ShowItems so a hung file manager cannot stall the caller indefinitely.
const dbusCallTimeout = 3 * time.Second

func fireAndForget(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(context.WithoutCancel(ctx), name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}

	crash.Go("reveal.cmdWait", func() { _ = cmd.Wait() })

	return nil
}

// Reveal asks the OS file manager to select path; falls back to opening the parent directory
// when highlighting fails. Returns ErrEmptyPath on empty input, a wrapped filepath.Abs error on
// path resolution failure, or an error only when the fallback also fails.
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

	err = openOrFail(ctx, filepath.Dir(abs))
	if err != nil {
		return fmt.Errorf("open containing directory: %w", err)
	}

	return nil
}

func openOrFail(ctx context.Context, dir string) error {
	if err := openDirectory(ctx, dir); err != nil {
		return fmt.Errorf("%w: %w", err, ErrCommandFailed)
	}

	return nil
}

// highlight requests that the OS file manager select abs; on failure Reveal falls back to opening the parent directory.
func highlight(ctx context.Context, abs string) error {
	var err error

	switch runtime.GOOS {
	case "windows":
		err = fireAndForget(ctx, "explorer.exe", "/select,", abs)
	case "darwin":
		err = fireAndForget(ctx, "open", "-R", abs)
	default:
		err = revealViaDbus(ctx, abs)
	}

	if err != nil {
		return fmt.Errorf("highlight file in file manager: %w", err)
	}

	return nil
}

// fileURI builds a percent-encoded file:// URI from an absolute path so desktop file managers
// parse it per RFC 8089 instead of interpreting literal `%`, spaces, or non-ASCII bytes as URI
// metacharacters.
func fileURI(abs string) string {
	return (&url.URL{Scheme: "file", Path: abs}).String()
}

func openDirectory(ctx context.Context, dir string) error {
	var err error

	switch runtime.GOOS {
	case "windows":
		err = fireAndForget(ctx, "explorer.exe", dir)
	case "darwin":
		err = fireAndForget(ctx, "open", dir)
	default:
		err = fireAndForget(ctx, "xdg-open", fileURI(dir))
	}

	if err != nil {
		return fmt.Errorf("open directory in file manager: %w", err)
	}

	return nil
}

// revealViaDbus opens a private session-bus connection for each call. SessionBus() returns a
// process-wide shared connection that godbus forbids closing and that a concurrent Reveal
// would tear down for everyone; ConnectSessionBus() gives a connection this call owns, so
// closing it is correct and cannot abort another reveal's in-flight ShowItems.
func revealViaDbus(ctx context.Context, abs string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect to session bus: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("Failed to close D-Bus session bus connection")
		}
	}()

	url := fileURI(abs)
	obj := conn.Object("org.freedesktop.FileManager1", "/org/freedesktop/FileManager1")

	callCtx, cancel := context.WithTimeout(ctx, dbusCallTimeout)
	defer cancel()

	call := obj.CallWithContext(callCtx, "org.freedesktop.FileManager1.ShowItems", 0, []string{url}, "")

	if call.Err != nil {
		return fmt.Errorf("call FileManager1.ShowItems: %w", call.Err)
	}

	return nil
}
