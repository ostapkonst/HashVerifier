// Package platform aggregates cross-platform glue: editor detection, Flatpak detection, and "show in file manager".
package platform

import (
	"context"

	"github.com/ostapkonst/HashVerifier/internal/platform/flatpak"
	"github.com/ostapkonst/HashVerifier/internal/platform/reveal"
)

// ErrEmptyPath is returned by RevealFile when given a zero-length path.
var ErrEmptyPath = reveal.ErrEmptyPath

// ErrCommandFailed is returned by RevealFile when the file-manager launcher exits non-zero.
var ErrCommandFailed = reveal.ErrCommandFailed

// IsRunningInFlatpak reports whether the process is sandboxed via Flatpak.
func IsRunningInFlatpak() bool {
	return flatpak.IsRunningInFlatpak()
}

// GetFlatpakFilesystems lists the filesystem paths exposed to the Flatpak sandbox via [Context] filesystems=.
func GetFlatpakFilesystems() []string {
	return flatpak.GetFilesystems()
}

// RevealFile asks the OS file manager to highlight path. Behavior is OS-specific.
func RevealFile(ctx context.Context, path string) error {
	return reveal.Reveal(ctx, path)
}
