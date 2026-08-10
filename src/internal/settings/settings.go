package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	appName      = "hashverifier"
	settingsFile = "settings.yaml"
)

type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

type RestoreMode string

const (
	RestoreModeDefault  RestoreMode = "default"
	RestoreModeSize     RestoreMode = "size"
	RestoreModePosition RestoreMode = "position"
	RestoreModeAll      RestoreMode = "all"
)

type WindowState string

const (
	WindowStateNormal     WindowState = "normal"
	WindowStateMaximized  WindowState = "maximized"
	WindowStateFullscreen WindowState = "fullscreen"
)

type ExcludeDialogSettings struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

type GenerateSettings struct {
	FollowSymbolicLinks bool                  `yaml:"follow_symbolic_links"`
	SortPaths           bool                  `yaml:"sort_paths"`
	FlatPaths           bool                  `yaml:"flat_paths"`
	Algorithm           string                `yaml:"algorithm"`
	ColumnOrder         []string              `yaml:"column_order"`
	SortColumn          string                `yaml:"sort_column"`
	SortOrder           SortOrder             `yaml:"sort_order"`
	ExcludeDialog       ExcludeDialogSettings `yaml:"exclude_dialog"`
}

type VerifySettings struct {
	VerifyOnOpen bool      `yaml:"verify_on_open"`
	ColumnOrder  []string  `yaml:"column_order"`
	SortColumn   string    `yaml:"sort_column"`
	SortOrder    SortOrder `yaml:"sort_order"`
}

type HashSettings struct {
	Algorithms []string `yaml:"algorithms"`
	HashOnOpen bool     `yaml:"hash_on_open"`
}

type WindowSettings struct {
	TabOrder    []string    `yaml:"tab_order"`
	CurrentPage int         `yaml:"current_page"`
	RestoreMode RestoreMode `yaml:"restore_mode"`
	Width       int         `yaml:"width"`
	Height      int         `yaml:"height"`
	X           int         `yaml:"x_pos"`
	Y           int         `yaml:"y_pos"`
	WindowState WindowState `yaml:"window_state"`
}

type FlatpakSettings struct {
	SuppressSandboxWarning bool `yaml:"suppress_sandbox_warning"`
}

type Settings struct {
	Window   WindowSettings   `yaml:"window"`
	Generate GenerateSettings `yaml:"generate"`
	Verify   VerifySettings   `yaml:"verify"`
	Hash     HashSettings     `yaml:"hash"`
	Flatpak  FlatpakSettings  `yaml:"flatpak"`
	mu       sync.Mutex
}

func DefaultSettings() *Settings {
	return &Settings{
		Window: WindowSettings{
			TabOrder:    []string{"generate", "verify", "hash"},
			CurrentPage: 0,
			RestoreMode: RestoreModeAll,
			Width:       0,
			Height:      0,
			X:           -1,
			Y:           -1,
			WindowState: WindowStateNormal,
		},
		Generate: GenerateSettings{
			FollowSymbolicLinks: true,
			SortPaths:           true,
			FlatPaths:           false,
			Algorithm:           ".md5",
			ColumnOrder:         []string{"idx", "status", "path", "size", "hash", "note"},
			SortColumn:          "idx",
			SortOrder:           SortOrderAsc,
			ExcludeDialog: ExcludeDialogSettings{
				Width:  0,
				Height: 0,
			},
		},
		Verify: VerifySettings{
			VerifyOnOpen: true,
			ColumnOrder:  []string{"idx", "status", "path", "size", "hash", "expected_hash", "note"},
			SortColumn:   "status",
			SortOrder:    SortOrderDesc,
		},
		Hash: HashSettings{
			Algorithms: []string{".md5", ".sha1", ".sha256"},
			HashOnOpen: true,
		},
		Flatpak: FlatpakSettings{
			SuppressSandboxWarning: false,
		},
	}
}

func getConfigDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}

		return filepath.Join(appData, appName), nil

	case "linux":
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			return filepath.Join(xdgConfig, appName), nil
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}

		return filepath.Join(home, ".config", appName), nil

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}

		return filepath.Join(home, "Library", "Application Support", appName), nil

	default:
		return os.Getwd()
	}
}

func getSettingsPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, settingsFile), nil
}

func ensureConfigDir() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	return nil
}

func Load() (*Settings, error) {
	settingsPath, err := getSettingsPath()
	if err != nil {
		return DefaultSettings(), err
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}

		return DefaultSettings(), fmt.Errorf("failed to read settings file: %w", err)
	}

	settings := DefaultSettings()
	if err := yaml.Unmarshal(data, settings); err != nil {
		return DefaultSettings(), fmt.Errorf("failed to parse settings file: %w", err)
	}

	settings.fixColumnOrder()
	settings.fixTabOrder()

	return settings, nil
}

func (s *Settings) fixColumnOrder() {
	defaultSettings := DefaultSettings()

	for _, col := range defaultSettings.Generate.ColumnOrder {
		if !slices.Contains(s.Generate.ColumnOrder, col) {
			s.Generate.ColumnOrder = defaultSettings.Generate.ColumnOrder
			break
		}
	}

	for _, col := range defaultSettings.Verify.ColumnOrder {
		if !slices.Contains(s.Verify.ColumnOrder, col) {
			s.Verify.ColumnOrder = defaultSettings.Verify.ColumnOrder
			break
		}
	}
}

func (s *Settings) fixTabOrder() {
	defaultSettings := DefaultSettings()

	for _, tab := range defaultSettings.Window.TabOrder {
		if !slices.Contains(s.Window.TabOrder, tab) {
			s.Window.TabOrder = defaultSettings.Window.TabOrder
			break
		}
	}
}

func (s *Settings) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ensureConfigDir(); err != nil {
		return fmt.Errorf("failed to ensure config directory: %w", err)
	}

	settingsPath, err := getSettingsPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func GetSettingsPath() (string, error) {
	return getSettingsPath()
}

func Reset() error {
	defaultSettings := DefaultSettings()
	return defaultSettings.Save()
}
