package platform

import (
	"context"

	"github.com/ostapkonst/HashVerifier/internal/platform/flatpak"
	"github.com/ostapkonst/HashVerifier/internal/platform/reveal"
)

var (
	ErrEmptyPath     = reveal.ErrEmptyPath
	ErrCommandFailed = reveal.ErrCommandFailed
)

func IsRunningInFlatpak() bool {
	return flatpak.IsRunningInFlatpak()
}

func GetFlatpakFilesystems() []string {
	return flatpak.GetFilesystems()
}

func RevealFile(ctx context.Context, path string) error {
	return reveal.Reveal(ctx, path)
}
