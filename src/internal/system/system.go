package system

import (
	"context"

	"github.com/ostapkonst/HashVerifier/internal/system/flatpak"
	"github.com/ostapkonst/HashVerifier/internal/system/reveal"
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
