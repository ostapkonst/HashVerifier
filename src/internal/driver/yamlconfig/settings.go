// Package yamlconfig is the YAML-backed implementation of the config port: it loads,
// validates, and persists application settings.
package yamlconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

const (
	appName      = "hashverifier"
	settingsFile = "settings.yaml"

	// saveCoalesceWindow bounds how long the async save worker waits after receiving a
	// snapshot for a newer one to arrive, so bursts of rapid GUI changes (e.g. dragging a
	// tab or a column) collapse into a single disk write. Tighter = more responsive
	// shutdown but more writes on bursts; wider = fewer writes but longer worst-case
	// shutdown delay.
	saveCoalesceWindow = 500 * time.Millisecond
)

// SortOrder is the per-column sort direction persisted to settings (asc | desc).
type SortOrder string

// SortOrder values: asc, or desc.
const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// RestoreMode selects which window dimensions to restore on startup (default | size | position | all).
type RestoreMode string

// RestoreMode values: default, size, position, or all.
const (
	RestoreModeDefault  RestoreMode = "default"
	RestoreModeSize     RestoreMode = "size"
	RestoreModePosition RestoreMode = "position"
	RestoreModeAll      RestoreMode = "all"
)

// WindowState is the maximized/fullscreen state of the main window, persisted across
// sessions (normal | maximized | fullscreen).
type WindowState string

// WindowState values: normal, maximized, or fullscreen.
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

// SettingsBase holds persistable fields and provides synchronous Save.
//
// Concurrency contract: when used by the GUI, every mutable field must be mutated only
// from the GTK main thread (GTK signal handlers run serially there). CLI reads fields as
// defaults and never mutates them. snapshot() takes s.mu and copies a consistent view for
// serialization; write() serializes os.WriteFile calls through s.writeMu so concurrent writers
// can never interleave bytes on the same path.
type SettingsBase struct {
	Window       WindowSettings   `yaml:"window"`
	Generate     GenerateSettings `yaml:"generate"`
	Verify       VerifySettings   `yaml:"verify"`
	Hash         HashSettings     `yaml:"hash"`
	Flatpak      FlatpakSettings  `yaml:"flatpak"`
	mu           sync.Mutex
	loadWarnings []ValidationWarning
	noPersist    bool
	writeMu      sync.Mutex
}

// Settings embeds *SettingsBase and adds async capability.
//
// Constructors that initialize a *Settings via initAsync (Load, NewDefaultsWithWorker)
// guarantee that channels are non-nil and the worker is running, so AsyncSave and Close
// are safe. A *Settings wrapped manually from a SettingsBase (e.g. the CLI ephemeral path
// in loading.go) has nil channels — only call Save / LoadWarnings on those, never AsyncSave.
type Settings struct {
	*SettingsBase
	saveReq   chan saveSnapshot
	stopCh    chan struct{}
	saveDone  chan struct{}
	closeOnce sync.Once
}

// saveSnapshot bundles a marshaled YAML payload with the destination path so the async worker
// does not need to re-marshal or recompute the path under any lock.
type saveSnapshot struct {
	path string
	data []byte
}

// DefaultSettings returns a bare SettingsBase for read-only use (defaults lookup,
// validation, Reset's sync Save). NOT safe for AsyncSave — use Load or
// NewDefaultsWithWorker for that.
func DefaultSettings() *SettingsBase {
	return &SettingsBase{
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
func (s *SettingsBase) GenerateSortableColumns() []string {
	return []string{"idx", "status", "path", "size", "note"}
}

// VerifySortableColumns lists Verify-tab columns the user can sort by; hash and expected_hash are excluded.
func (s *SettingsBase) VerifySortableColumns() []string {
	return []string{"idx", "status", "path", "size", "note"}
}

// getConfigDir returns the per-platform settings directory:
//   - Windows: %APPDATA%\hashverifier
//   - Linux: $XDG_CONFIG_HOME/hashverifier (or $HOME/.config/hashverifier)
//   - macOS: $HOME/Library/Application Support/hashverifier
//   - other: current working directory (last-resort fallback for unknown GOOS)
//
// Returns an error on Linux/macOS when the user's home directory cannot be determined,
// or on Windows when APPDATA is unset.
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
		dir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}

		return dir, nil
	}
}

// getSettingsPath returns the absolute path to settings.yaml for the current platform.
// Wraps getConfigDir to prepend the directory; propagates errors from there.
func getSettingsPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}

	return filepath.Join(configDir, settingsFile), nil
}

// ensureConfigDir creates the per-platform settings directory if it does not yet exist.
// Idempotent: returns nil immediately if the directory already exists and is accessible.
func ensureConfigDir() error {
	configDir, err := getConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if _, err := os.Stat(configDir); err != nil {
		if !os.IsNotExist(err) {
			// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
			return err
		}

		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	return nil
}

// Load reads settings.yaml and returns Settings with the worker started for persistent
// mode. Returns an error if the file is missing read permissions, unreadable, or
// contains unparseable YAML; a missing file is not an error (first run). AsyncSave and
// Close are safe to use on the returned value.
func Load() (*Settings, error) {
	s := &Settings{SettingsBase: DefaultSettings()}

	err := s.readFromDisk()
	s.loadWarnings = s.Validate()

	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	s.initAsync()

	return s, nil
}

// LoadEphemeral returns a SettingsBase for noPersist mode (--no-config / HASHVERIFIER_NO_CONFIG=1).
// No worker, no channels; AsyncSave is a no-op, Close is a no-op. Cannot fail.
// For the async type (Settings with worker started, e.g. for GUI tabs in noPersist mode),
// pass true to NewDefaultsWithWorker.
func LoadEphemeral() *SettingsBase {
	s := DefaultSettings()
	s.noPersist = true

	return s
}

// NewDefaultsWithWorker returns Settings with default values and worker started. Use:
//   - As in-process fallback when Load() fails (e.g. corrupt YAML or unwritable HOME),
//     typically with noPersist=false in persistent mode and noPersist=true in ephemeral mode.
//   - For GUI noPersist mode: NewDefaultsWithWorker(true) gives *Settings with worker started
//     and AsyncSave/Save no-op'd by noPersist, suitable for tabs that need the async type.
//
// Cannot fail.
func NewDefaultsWithWorker(noPersist bool) *Settings {
	s := &Settings{SettingsBase: DefaultSettings()}
	s.noPersist = noPersist
	s.initAsync()

	return s
}

// initAsync allocates channels and starts the background save worker. Always called from
// constructors that return *Settings; the caller is guaranteed that AsyncSave/Close work.
func (s *Settings) initAsync() {
	s.saveReq = make(chan saveSnapshot, 1)
	s.stopCh = make(chan struct{})

	s.saveDone = make(chan struct{})

	go s.saveLoop()
}

// readFromDisk populates s from settings.yaml. Returns nil error if the file does not
// exist (first run); returns an error for read or YAML parse failures.
func (s *SettingsBase) readFromDisk() error {
	if s.noPersist {
		return nil
	}

	settingsPath, err := getSettingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
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

// snapshot copies every persisted field under s.mu, then marshals outside the lock so the
// yaml.Marshal cost (still small but non-trivial for big settings) does not block signal
// handlers that want to mutate fields. The pointer is passed to yaml.Marshal so it does not
// copy the snapshot a second time.
func (s *SettingsBase) snapshot() (string, []byte, error) {
	settingsPath, err := getSettingsPath()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get settings path: %w", err)
	}

	s.mu.Lock()
	snap := &SettingsBase{
		Window:   s.Window,
		Generate: s.Generate,
		Verify:   s.Verify,
		Hash:     s.Hash,
		Flatpak:  s.Flatpak,
	}
	s.mu.Unlock()

	data, err := yaml.Marshal(snap)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal settings: %w", err)
	}

	return settingsPath, data, nil
}

// writeFile centralizes "ensure config dir + write" under s.writeMu so both the
// synchronous Save path and the async worker can create the config directory on
// demand; without it a first-run GUI write fails ENOENT and preferences silently
// never persist.
func (s *SettingsBase) writeFile(path string, data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := ensureConfigDir(); err != nil {
		return fmt.Errorf("failed to ensure config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// write serializes the os.WriteFile call against Save() and any other writer through s.writeMu,
// so two callers can never interleave bytes on the same path even when one is async.
func (s *SettingsBase) write(snap saveSnapshot) {
	if err := s.writeFile(snap.path, snap.data); err != nil {
		log.Error().Err(err).Str("path", snap.path).Msg("Failed to write settings")
	}
}

// Save writes s to settings.yaml synchronously; no-ops when noPersist is true (ephemeral
// mode). Creates the config directory on demand. Used by CLI subcommands (config reset)
// where the caller wants confirmation that the file landed before returning; Settings has a
// non-blocking AsyncSave for GUI signal handlers.
func (s *SettingsBase) Save() error {
	if s.noPersist {
		return nil
	}

	settingsPath, data, err := s.snapshot()
	if err != nil {
		return err
	}

	return s.writeFile(settingsPath, data)
}

// Close is a no-op on SettingsBase (no async infrastructure to tear down). Settings.Close
// shadows it via method promotion through pointer embedding.
func (s *SettingsBase) Close() {}

// AsyncSave snapshots the current state and enqueues it for the background worker. Multiple
// calls within a saveCoalesceWindow coalesce into one disk write; later snapshots always
// win because the worker drains the channel before picking up the latest payload. The call
// returns immediately so HDD latency is hidden behind the GTK main loop. Safe to call on
// any Settings with initialized channels; no-op when noPersist is true.
func (s *Settings) AsyncSave() error {
	if s.noPersist {
		return nil
	}

	settingsPath, data, err := s.snapshot()
	if err != nil {
		return err
	}

	snap := saveSnapshot{path: settingsPath, data: data}

	// Latest-wins queueing: the channel holds at most one pending snapshot, so when it is
	// full the worker is already draining or writing the prior one. Drain it and push the
	// fresh payload so the worker eventually writes our state, not the stale one.
	select {
	case s.saveReq <- snap:
	default:
		select {
		case <-s.saveReq:
		default:
		}

		s.saveReq <- snap
	}

	return nil
}

// saveLoop drains saveReq with a saveCoalesceWindow coalesce. On stop it flushes the
// most recent snapshot before returning. A deferred recover turns an internal panic into a
// logged error rather than a process exit so the next Sync Save() still works.
func (s *Settings) saveLoop() {
	defer close(s.saveDone)
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).
				Msg("settings save worker panicked; async saves disabled until restart")
		}
	}()

	for {
		var snap saveSnapshot

		select {
		case <-s.stopCh:
			select {
			case snap = <-s.saveReq:
				s.write(snap)
			default:
			}

			return
		case snap = <-s.saveReq:
		}

		// Coalesce: wait up to saveCoalesceWindow for one newer snapshot. With channel cap=1
		// at most one can arrive; further ones will be picked up by the next outer iteration.
		select {
		case newer := <-s.saveReq:
			snap = newer
		case <-time.After(saveCoalesceWindow):
		case <-s.stopCh:
			// Newer in the channel is always fresher than snap, so write whichever is
			// more recent; channel-empty means no async save raced past us and we use snap.
			select {
			case newer := <-s.saveReq:
				s.write(newer)
			default:
				s.write(snap)
			}

			return
		}

		s.write(snap)
	}
}

// Close signals the save worker to drain pending snapshots and exit, then blocks until it
// has done so. Safe to call multiple times; subsequent calls are no-ops. Must be invoked
// from the GUI shutdown path before gtk_main quits to guarantee the last user-driven change
// is persisted.
func (s *Settings) Close() {
	s.closeOnce.Do(func() {
		close(s.stopCh)
		<-s.saveDone
	})
}

// GetSettingsPath lets callers show users where settings live or hand the file to $EDITOR for `config edit`.
func GetSettingsPath() (string, error) {
	path, err := getSettingsPath()
	if err != nil {
		return "", fmt.Errorf("get settings path: %w", err)
	}

	return path, nil
}

// Reset overwrites settings.yaml with DefaultSettings and returns any write error.
// Callers must gate this on the --no-config flag to honor ephemeral mode.
func Reset() error {
	return DefaultSettings().Save()
}

// LoadWarnings returns the list of fields that Validate reset to defaults during the last
// Load (e.g. unknown enum values).
func (s *SettingsBase) LoadWarnings() []ValidationWarning {
	return s.loadWarnings
}
