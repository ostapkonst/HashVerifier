// Package yamlconfig is the YAML-backed implementation of the config port: it loads, validates, and persists application settings.
package yamlconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	appName      = "hashverifier"
	settingsFile = "settings.yaml"
)

// SortOrder is the per-column sort direction persisted to settings (asc | desc).
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// RestoreMode selects which window dimensions to restore on startup (default | size | position | all).
type RestoreMode string

const (
	RestoreModeDefault  RestoreMode = "default"
	RestoreModeSize     RestoreMode = "size"
	RestoreModePosition RestoreMode = "position"
	RestoreModeAll      RestoreMode = "all"
)

// WindowState is the maximized/fullscreen state persisted across sessions.
type WindowState string

const (
	WindowStateNormal     WindowState = "normal"
	WindowStateMaximized  WindowState = "maximized"
	WindowStateFullscreen WindowState = "fullscreen"
)

// ExcludeDialogSettings persists the size of the exclude dialog between opens.
type ExcludeDialogSettings struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

// GenerateSettings is the persisted "generate" subsection of settings.yaml.
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

// VerifySettings is the persisted "verify" subsection of settings.yaml.
type VerifySettings struct {
	VerifyOnOpen bool      `yaml:"verify_on_open"`
	ColumnOrder  []string  `yaml:"column_order"`
	SortColumn   string    `yaml:"sort_column"`
	SortOrder    SortOrder `yaml:"sort_order"`
}

// HashSettings is the persisted "hash" subsection of settings.yaml.
type HashSettings struct {
	Algorithms []string `yaml:"algorithms"`
	HashOnOpen bool     `yaml:"hash_on_open"`
}

// WindowSettings persists main-window geometry, tab order, and currently active tab.
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

// FlatpakSettings holds preferences that only take effect when running under Flatpak.
type FlatpakSettings struct {
	SuppressSandboxWarning bool `yaml:"suppress_sandbox_warning"`
}

// Settings is the top-level settings document persisted as settings.yaml.
type Settings struct {
	Window       WindowSettings   `yaml:"window"`
	Generate     GenerateSettings `yaml:"generate"`
	Verify       VerifySettings   `yaml:"verify"`
	Hash         HashSettings     `yaml:"hash"`
	Flatpak      FlatpakSettings  `yaml:"flatpak"`
	mu           sync.Mutex
	loadWarnings []ValidationWarning
	noPersist    bool
}

// DefaultSettings returns the baseline used by Load and Reset; no values are read from disk.
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

// GenerateSortableColumns lists Generate-tab columns the user can sort by; the hash column is excluded.
func (s *Settings) GenerateSortableColumns() []string {
	return []string{"idx", "status", "path", "size", "note"}
}

// VerifySortableColumns lists Verify-tab columns the user can sort by; hash and expected_hash are excluded.
func (s *Settings) VerifySortableColumns() []string {
	return []string{"idx", "status", "path", "size", "note"}
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

	if _, err := os.Stat(configDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat config directory: %w", err)
		}

		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	return nil
}

// Load reads settings.yaml into a Settings; noPersist=true returns an ephemeral instance Save will skip.
func Load(noPersist bool) (*Settings, error) {
	s := DefaultSettings()
	s.noPersist = noPersist

	err := s.readFromDisk()
	s.loadWarnings = s.Validate()

	return s, err
}

func (s *Settings) readFromDisk() error {
	if s.noPersist {
		return nil
	}

	settingsPath, err := getSettingsPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to read settings file: %w", err)
	}

	if err := yaml.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to parse settings file: %w", err)
	}

	return nil
}

// Save writes s to settings.yaml; no-ops when constructed via Load(noPersist=true) (ephemeral mode).
// Creates the config directory on demand and is safe to call from a window-close handler.
func (s *Settings) Save() error {
	if s.noPersist {
		return nil
	}

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

// GetSettingsPath lets callers show users where settings live or hand the file to $EDITOR for `config edit`.
func GetSettingsPath() (string, error) {
	return getSettingsPath()
}

// Reset overwrites settings.yaml with DefaultSettings and returns the write error (no-op under --no-config).
func Reset() error {
	defaultSettings := DefaultSettings()
	return defaultSettings.Save()
}

// LoadWarnings returns the list of fields that Validate reset to defaults during the last Load (e.g. unknown enum values).
func (s *Settings) LoadWarnings() []ValidationWarning {
	return s.loadWarnings
}
