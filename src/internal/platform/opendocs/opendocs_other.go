//go:build !darwin

package opendocs

// Install is a no-op outside macOS: there is no LaunchServices open-documents channel.
func Install(func(string)) {}
