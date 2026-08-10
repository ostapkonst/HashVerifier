package settings

import (
	"fmt"
	"strings"
)

type SettingInfo struct {
	Name        string
	Value       string
	Default     string
	Description string
}

type SettingsSection struct {
	Name     string
	Settings []SettingInfo
}

var descriptionsMap = map[string]string{
	"generate.follow_symbolic_links":   "Follow symbolic links when scanning directories",
	"generate.sort_paths":              "Sort paths before hashing",
	"generate.flat_paths":              "Strip root directory from paths; save checksum file inside source directory",
	"generate.algorithm":               "Default hash algorithm (e.g., .sha256, .md5)",
	"generate.column_order":            "Order of columns in Generate tab",
	"generate.sort_column":             "Column to sort by in Generate tab",
	"generate.sort_order":              "Sort order in Generate tab (asc/desc)",
	"generate.exclude_dialog.width":    "Saved width of the exclude dialog",
	"generate.exclude_dialog.height":   "Saved height of the exclude dialog",
	"verify.verify_on_open":            "Auto-start verification when opening checksum file",
	"verify.column_order":              "Order of columns in Verify tab",
	"verify.sort_column":               "Column to sort by in Verify tab",
	"verify.sort_order":                "Sort order in Verify tab (asc/desc)",
	"hash.algorithms":                  "Default hash algorithms for hash command and GUI (e.g., .sha256, .md5)",
	"hash.hash_on_open":                "Auto-start hashing when opening a file",
	"window.tab_order":                 "Order of tabs in main window",
	"window.current_page":              "Currently active tab",
	"window.restore_mode":              "Restore window size and/or position on startup (default, size, position, all)",
	"window.width":                     "Window width (saved automatically)",
	"window.height":                    "Window height (saved automatically)",
	"window.x_pos":                     "Window X position (saved automatically)",
	"window.y_pos":                     "Window Y position (saved automatically)",
	"window.window_state":              "Window state on exit (normal, maximized, fullscreen) (saved automatically)",
	"flatpak.suppress_sandbox_warning": "Suppress the Flatpak sandbox warning dialog on startup (Flatpak only)",
}

func GetAllSettingsInfo(cfg, defaults *Settings) []SettingsSection {
	return []SettingsSection{
		{
			Name: "Window",
			Settings: []SettingInfo{
				{
					Name:        "tab_order",
					Value:       formatSettingValueSlice(cfg.Window.TabOrder),
					Default:     formatSettingValueSlice(defaults.Window.TabOrder),
					Description: descriptionsMap["window.tab_order"],
				},
				{
					Name:        "current_page",
					Value:       formatSettingValue(cfg.Window.CurrentPage),
					Default:     formatSettingValue(defaults.Window.CurrentPage),
					Description: descriptionsMap["window.current_page"],
				},
				{
					Name:        "restore_mode",
					Value:       formatSettingValue(cfg.Window.RestoreMode),
					Default:     formatSettingValue(defaults.Window.RestoreMode),
					Description: descriptionsMap["window.restore_mode"],
				},
				{
					Name:        "width",
					Value:       formatSettingValue(cfg.Window.Width),
					Default:     formatSettingValue(defaults.Window.Width),
					Description: descriptionsMap["window.width"],
				},
				{
					Name:        "height",
					Value:       formatSettingValue(cfg.Window.Height),
					Default:     formatSettingValue(defaults.Window.Height),
					Description: descriptionsMap["window.height"],
				},
				{
					Name:        "x_pos",
					Value:       formatSettingValue(cfg.Window.X),
					Default:     formatSettingValue(defaults.Window.X),
					Description: descriptionsMap["window.x_pos"],
				},
				{
					Name:        "y_pos",
					Value:       formatSettingValue(cfg.Window.Y),
					Default:     formatSettingValue(defaults.Window.Y),
					Description: descriptionsMap["window.y_pos"],
				},
				{
					Name:        "window_state",
					Value:       formatSettingValue(cfg.Window.WindowState),
					Default:     formatSettingValue(defaults.Window.WindowState),
					Description: descriptionsMap["window.window_state"],
				},
			},
		},
		{
			Name: "Generate",
			Settings: []SettingInfo{
				{
					Name:        "follow_symbolic_links",
					Value:       formatSettingValue(cfg.Generate.FollowSymbolicLinks),
					Default:     formatSettingValue(defaults.Generate.FollowSymbolicLinks),
					Description: descriptionsMap["generate.follow_symbolic_links"],
				},
				{
					Name:        "sort_paths",
					Value:       formatSettingValue(cfg.Generate.SortPaths),
					Default:     formatSettingValue(defaults.Generate.SortPaths),
					Description: descriptionsMap["generate.sort_paths"],
				},
				{
					Name:        "flat_paths",
					Value:       formatSettingValue(cfg.Generate.FlatPaths),
					Default:     formatSettingValue(defaults.Generate.FlatPaths),
					Description: descriptionsMap["generate.flat_paths"],
				},
				{
					Name:        "algorithm",
					Value:       formatSettingValue(cfg.Generate.Algorithm),
					Default:     formatSettingValue(defaults.Generate.Algorithm),
					Description: descriptionsMap["generate.algorithm"],
				},
				{
					Name:        "column_order",
					Value:       formatSettingValueSlice(cfg.Generate.ColumnOrder),
					Default:     formatSettingValueSlice(defaults.Generate.ColumnOrder),
					Description: descriptionsMap["generate.column_order"],
				},
				{
					Name:        "sort_column",
					Value:       formatSettingValue(cfg.Generate.SortColumn),
					Default:     formatSettingValue(defaults.Generate.SortColumn),
					Description: descriptionsMap["generate.sort_column"],
				},
				{
					Name:        "sort_order",
					Value:       formatSettingValue(cfg.Generate.SortOrder),
					Default:     formatSettingValue(defaults.Generate.SortOrder),
					Description: descriptionsMap["generate.sort_order"],
				},
				{
					Name:        "exclude_dialog.width",
					Value:       formatSettingValue(cfg.Generate.ExcludeDialog.Width),
					Default:     formatSettingValue(defaults.Generate.ExcludeDialog.Width),
					Description: descriptionsMap["generate.exclude_dialog.width"],
				},
				{
					Name:        "exclude_dialog.height",
					Value:       formatSettingValue(cfg.Generate.ExcludeDialog.Height),
					Default:     formatSettingValue(defaults.Generate.ExcludeDialog.Height),
					Description: descriptionsMap["generate.exclude_dialog.height"],
				},
			},
		},
		{
			Name: "Verify",
			Settings: []SettingInfo{
				{
					Name:        "verify_on_open",
					Value:       formatSettingValue(cfg.Verify.VerifyOnOpen),
					Default:     formatSettingValue(defaults.Verify.VerifyOnOpen),
					Description: descriptionsMap["verify.verify_on_open"],
				},
				{
					Name:        "column_order",
					Value:       formatSettingValueSlice(cfg.Verify.ColumnOrder),
					Default:     formatSettingValueSlice(defaults.Verify.ColumnOrder),
					Description: descriptionsMap["verify.column_order"],
				},
				{
					Name:        "sort_column",
					Value:       formatSettingValue(cfg.Verify.SortColumn),
					Default:     formatSettingValue(defaults.Verify.SortColumn),
					Description: descriptionsMap["verify.sort_column"],
				},
				{
					Name:        "sort_order",
					Value:       formatSettingValue(cfg.Verify.SortOrder),
					Default:     formatSettingValue(defaults.Verify.SortOrder),
					Description: descriptionsMap["verify.sort_order"],
				},
			},
		},
		{
			Name: "Hash",
			Settings: []SettingInfo{
				{
					Name:        "algorithms",
					Value:       formatSettingValueSlice(cfg.Hash.Algorithms),
					Default:     formatSettingValueSlice(defaults.Hash.Algorithms),
					Description: descriptionsMap["hash.algorithms"],
				},
				{
					Name:        "hash_on_open",
					Value:       formatSettingValue(cfg.Hash.HashOnOpen),
					Default:     formatSettingValue(defaults.Hash.HashOnOpen),
					Description: descriptionsMap["hash.hash_on_open"],
				},
			},
		},
		{
			Name: "Flatpak",
			Settings: []SettingInfo{
				{
					Name:        "suppress_sandbox_warning",
					Value:       formatSettingValue(cfg.Flatpak.SuppressSandboxWarning),
					Default:     formatSettingValue(defaults.Flatpak.SuppressSandboxWarning),
					Description: descriptionsMap["flatpak.suppress_sandbox_warning"],
				},
			},
		},
	}
}

func formatSettingValue(v any) string {
	switch val := v.(type) {
	case bool:
		return fmt.Sprintf("%t", val)
	case string:
		if val == "" {
			return "-"
		}

		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatSettingValueSlice(slice []string) string {
	if len(slice) == 0 {
		return "-"
	}

	return strings.Join(slice, ", ")
}
