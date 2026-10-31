//go:build !darwin

package main

// setupMacOSBundleEnvironment is only meaningful in a macOS .app bundle.
func setupMacOSBundleEnvironment() {}
