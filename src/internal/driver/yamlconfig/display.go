package yamlconfig

import (
	"fmt"
	"strings"
)

// SettingInfo is a single row rendered by `config show`.
type SettingInfo struct {
	Name        string
	Value       string
	Default     string
	Description string
}

// SettingsSection groups SettingInfo rows for `config show`; Name is the section heading.
type SettingsSection struct {
	Name     string
	Settings []SettingInfo
}

var descriptionsMap = map[string]string{
	"generate.follow_symbolic_links":                "Follow symbolic links when scanning directories",
	"generate.sort_paths":                           "Sort paths before hashing",
	"generate.flat_paths":                           "Strip root directory from paths; save checksum file inside source directory",
	"generate.skip_hidden":                          "Skip hidden files and directories when scanning (dotfiles on Unix, hidden attribute on Windows)",
	"generate.algorithm":                            "Default hash algorithm (e.g., .sha256, .md5)",
	"generate.column_order":                         "Order of columns in Generate tab",
	"generate.sort_column":                          "Column to sort by in Generate tab",
	"generate.sort_order":                           "Sort order in Generate tab (asc/desc)",
	"generate.exclude_dialog.width":                 "Saved width of the exclude dialog",
	"generate.exclude_dialog.height":                "Saved height of the exclude dialog",
	"verify.verify_on_open":                         "Auto-start verification when opening checksum file",
	"verify.column_order":                           "Order of columns in Verify tab",
	"verify.sort_column":                            "Column to sort by in Verify tab",
	"verify.sort_order":                             "Sort order in Verify tab (asc/desc)",
	"hash.algorithms":                               "Default hash algorithms for hash command and GUI (e.g., .sha256, .md5)",
	"hash.hash_on_open":                             "Auto-start hashing when opening a file",
	"discover.column_order":                         "Order of columns in Discover tab",
	"discover.sort_column":                          "Column to sort by in Discover tab",
	"discover.sort_order":                           "Sort order in Discover tab (asc/desc)",
	"discover.sources_dialog.width":                 "Saved width of the discover sources dialog",
	"discover.sources_dialog.height":                "Saved height of the discover sources dialog",
	"discover.sources_dialog.filter":                "File filter for the discover sources dialog (all, with_checksum, without_checksum)",
	"discover.sources_dialog.follow_symbolic_links": "Follow symbolic links when scanning discovered sources",
	"window.tab_order":                              "Order of tabs in main window",
	"window.current_page":                           "Currently active tab",
	"window.restore_mode":                           "Restore window size and/or position on startup (default, size, position, all)",
	"window.width":                                  "Window width (saved automatically)",
	"window.height":                                 "Window height (saved automatically)",
	"window.x_pos":                                  "Window X position (saved automatically; may be negative on multi-monitor setups)",
	"window.y_pos":                                  "Window Y position (saved automatically; may be negative on multi-monitor setups)",
	"window.window_state":                           "Window state on exit (normal, maximized, fullscreen) (saved automatically)",
	"window.position_saved":                         "Whether the window position has been recorded (restore applies X/Y only when true)",
	"flatpak.suppress_sandbox_warning":              "Suppress the Flatpak sandbox warning dialog on startup (Flatpak only)",
}

// GetAllSettingsInfo renders cfg next to defaults for `config show`, grouped by section.
func GetAllSettingsInfo(cfg, defaults *SettingsBase) []SettingsSection {
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
				{
					Name:        "position_saved",
					Value:       formatSettingValue(cfg.Window.PositionSaved),
					Default:     formatSettingValue(defaults.Window.PositionSaved),
					Description: descriptionsMap["window.position_saved"],
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
					Name:        "skip_hidden",
					Value:       formatSettingValue(cfg.Generate.SkipHidden),
					Default:     formatSettingValue(defaults.Generate.SkipHidden),
					Description: descriptionsMap["generate.skip_hidden"],
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
			Name: "Discover",
			Settings: []SettingInfo{
				{
					Name:        "column_order",
					Value:       formatSettingValueSlice(cfg.Discover.ColumnOrder),
					Default:     formatSettingValueSlice(defaults.Discover.ColumnOrder),
					Description: descriptionsMap["discover.column_order"],
				},
				{
					Name:        "sort_column",
					Value:       formatSettingValue(cfg.Discover.SortColumn),
					Default:     formatSettingValue(defaults.Discover.SortColumn),
					Description: descriptionsMap["discover.sort_column"],
				},
				{
					Name:        "sort_order",
					Value:       formatSettingValue(cfg.Discover.SortOrder),
					Default:     formatSettingValue(defaults.Discover.SortOrder),
					Description: descriptionsMap["discover.sort_order"],
				},
				{
					Name:        "sources_dialog.width",
					Value:       formatSettingValue(cfg.Discover.SourcesDialog.Width),
					Default:     formatSettingValue(defaults.Discover.SourcesDialog.Width),
					Description: descriptionsMap["discover.sources_dialog.width"],
				},
				{
					Name:        "sources_dialog.height",
					Value:       formatSettingValue(cfg.Discover.SourcesDialog.Height),
					Default:     formatSettingValue(defaults.Discover.SourcesDialog.Height),
					Description: descriptionsMap["discover.sources_dialog.height"],
				},
				{
					Name:        "sources_dialog.filter",
					Value:       formatSettingValue(cfg.Discover.SourcesDialog.Filter),
					Default:     formatSettingValue(defaults.Discover.SourcesDialog.Filter),
					Description: descriptionsMap["discover.sources_dialog.filter"],
				},
				{
					Name:        "sources_dialog.follow_symbolic_links",
					Value:       formatSettingValue(cfg.Discover.SourcesDialog.FollowSymbolicLinks),
					Default:     formatSettingValue(defaults.Discover.SourcesDialog.FollowSymbolicLinks),
					Description: descriptionsMap["discover.sources_dialog.follow_symbolic_links"],
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
