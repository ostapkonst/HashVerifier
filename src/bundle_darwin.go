//go:build darwin

package main

import (
	"os"
	"path/filepath"
)

// setupMacOSBundleEnvironment points GTK at the resources shipped inside the
// .app bundle, overriding any inherited values so bundled schemas and module
// caches are always used. It is a no-op when the executable is not in a bundle.
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

	set := func(key, value string) {
		_ = os.Setenv(key, value)
	}

	set("XDG_DATA_DIRS", filepath.Join(resourcesDir, "share"))
	set("GTK_DATA_PREFIX", resourcesDir)
	set("GTK_EXE_PREFIX", resourcesDir)
	set("GTK_PATH", resourcesDir)
	set("GSETTINGS_SCHEMA_DIR", filepath.Join(resourcesDir, "share", "glib-2.0", "schemas"))

	if cache := firstMatchingFile(resourcesDir, "lib/gdk-pixbuf-2.0/*/loaders.cache"); cache != "" {
		set("GDK_PIXBUF_MODULE_FILE", cache)
	}

	if cache := firstMatchingFile(resourcesDir, "lib/gtk-3.0/*/immodules.cache"); cache != "" {
		set("GTK_IM_MODULE_FILE", cache)
	}
}

func firstMatchingFile(root, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}

	return matches[0]
}
