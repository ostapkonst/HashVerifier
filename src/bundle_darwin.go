//go:build darwin

package main

import (
	"os"
	"path/filepath"
)

// setupMacOSBundleEnvironment points GTK at the resources shipped inside the
// .app bundle. It is a no-op when the executable is not inside a bundle, and
// leaves explicitly configured environment variables untouched.
func setupMacOSBundleEnvironment() {
	executable, err := os.Executable()
	if err != nil {
		return
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	macOSDir := filepath.Dir(executable)
	contentsDir := filepath.Dir(macOSDir)
	if filepath.Base(macOSDir) != "MacOS" || filepath.Base(contentsDir) != "Contents" {
		return
	}

	resourcesDir := filepath.Join(contentsDir, "Resources")
	if info, err := os.Stat(resourcesDir); err != nil || !info.IsDir() {
		return
	}

	setIfEmpty := func(key, value string) {
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}

	setIfEmpty("XDG_DATA_DIRS", filepath.Join(resourcesDir, "share"))
	setIfEmpty("GTK_DATA_PREFIX", resourcesDir)
	setIfEmpty("GTK_EXE_PREFIX", resourcesDir)
	setIfEmpty("GTK_PATH", resourcesDir)
	setIfEmpty("GSETTINGS_SCHEMA_DIR", filepath.Join(resourcesDir, "share", "glib-2.0", "schemas"))

	if cache := firstMatchingFile(resourcesDir, "lib/gdk-pixbuf-2.0/*/loaders.cache"); cache != "" {
		setIfEmpty("GDK_PIXBUF_MODULE_FILE", cache)
	}

	if cache := firstMatchingFile(resourcesDir, "lib/gtk-3.0/*/immodules.cache"); cache != "" {
		setIfEmpty("GTK_IM_MODULE_FILE", cache)
	}
}

func firstMatchingFile(root, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}

	return matches[0]
}
