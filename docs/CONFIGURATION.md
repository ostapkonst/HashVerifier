# HashVerifier Configuration

HashVerifier stores user preferences in a YAML settings file. You can view and edit settings via the command line or by directly editing the settings file.

For runtime usage (CLI invocation patterns, output formats, exit codes) see [USAGE Guide](USAGE.md).

## Settings Location

| Platform | Path |
|----------|------|
| Linux   | `$XDG_CONFIG_HOME/hashverifier/settings.yaml` (fallback: `~/.config/hashverifier/settings.yaml`) |
| macOS   | `~/Library/Application Support/hashverifier/settings.yaml` |
| Windows | `%APPDATA%\hashverifier\settings.yaml` |
| Other   | `./settings.yaml` in the current working directory (last-resort fallback) |

The file is created on demand: `config edit` seeds it with the defaults when it is missing, `config reset` rewrites it, and the GUI writes it on the first automatic save. `config show` never creates or modifies it.

## CLI Commands

**View settings:**

```bash
./hashverifier config show
```

**Edit settings:**

```bash
./hashverifier config edit
```

Opens the settings file in your default text editor. The editor is taken from `$VISUAL`, then `$EDITOR`, then — if neither is set — the first editor found on `PATH`: `notepad.exe`, `code` or `notepad++.exe` on Windows; `vim`, `nano` or `vi` elsewhere. When no editor is available the command exits with code `78`. A configured editor that cannot be parsed, cannot be started, or exits non-zero exits with code `1`.

**Reset settings:**

```bash
./hashverifier config reset
```

Pass `--yes` to skip the confirmation prompt (scripts/CI).

## Ephemeral Mode (`--no-config`)

Pass `--no-config` to any CLI subcommand, or set `HASHVERIFIER_NO_CONFIG=1`, to skip reading and writing `settings.yaml`. HashVerifier runs with built-in defaults; any toggle, edit, or reset that would touch the file becomes a no-op (with `config edit` and `config reset` returning a non-zero exit code).

GUI mode honors the same env var — the GUI path is chosen before flag parsing and does not parse `--no-config`, so set `HASHVERIFIER_NO_CONFIG=1` instead. When it is set, the window title becomes `HashVerifier — Ephemeral Mode` so the mode is unmistakable. `config show` stays available and prints the built-in defaults together with the path of the settings file, but neither reads nor writes that file.

**Truthy values for the env var:** `1`, `true`, `yes`, `on`, `y`, `t` (case-insensitive; surrounding whitespace is ignored). Anything else (including empty and `0`) is treated as false. The CLI flag takes precedence over the environment variable.

Typical use cases: CI/automation, ephemeral containers, Flatpak or sandbox runs that should not pollute the host profile, sharing a reproducible environment.

## Available Settings

> **Value case.** Values in `settings.yaml` are matched exactly as listed below and are case-sensitive: `ALL` is not `all`, and an unknown spelling is reset to the default with a repair warning. The corresponding CLI flags accept the same values, spelled exactly as listed.

### Window Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `window.tab_order` | `generate, verify, hash, discover` | Order of tabs in main window |
| `window.current_page` | `0` | Currently active tab (saved automatically) |
| `window.restore_mode` | `all` | Restore window size/position on startup — see values below |
| `window.width` | `0` | Window width (saved automatically) |
| `window.height` | `0` | Window height (saved automatically) |
| `window.x_pos` | `0` | Window X position (saved automatically; may be negative on multi-monitor setups) |
| `window.y_pos` | `0` | Window Y position (saved automatically; may be negative on multi-monitor setups) |
| `window.window_state` | `normal` | Window state on exit (saved automatically) |
| `window.position_saved` | `false` | Whether the window position has been recorded (saved automatically; restore applies X/Y only when true) |

**Restore mode values:**

| Value | Behavior |
|-------|----------|
| `default` | Use default window size from UI definition |
| `size` | Restore window size (width and height) and apply last maximized/fullscreen state |
| `position` | Restore only window position (X and Y), applied only when `position_saved` is true |
| `all` | Restore window size, position, and apply last maximized/fullscreen state |

**Window state values:**

| Value | Behavior |
|-------|----------|
| `normal` | Standard windowed mode |
| `maximized` | Window fills the screen but keeps the title bar |
| `fullscreen` | Window covers the entire screen without decorations |

### Generate Tab Settings

| Setting | Default | CLI Flag | Description |
|---------|---------|----------|-------------|
| `generate.follow_symbolic_links` | `true` | `--follow-symbolic-links` | Follow symbolic links when scanning directories |
| `generate.sort_paths` | `true` | `--sort-paths` | Sort paths before hashing |
| `generate.flat_paths` | `false` | `--flat-paths` | Strip root directory from paths; the checksum file should be saved inside the source directory (e.g., as `checksums.<ext>`) |
| `generate.skip_hidden` | `false` | `--skip-hidden` | Skip hidden files and directories (names starting with `.` on Unix, the Windows hidden attribute) |
| `generate.algorithm` | `.md5` | `--algorithm` | Default hash algorithm |
| `generate.column_order` | `idx, status, path, size, hash, note` | — | Order of columns in Generate tab |
| `generate.sort_column` | `idx` | — | Column to sort by in Generate tab |
| `generate.sort_order` | `asc` | — | Sort order in Generate tab (`asc` / `desc`) |
| `generate.exclude_dialog.width` | `0` | — | Saved width of the exclude dialog (saved automatically) |
| `generate.exclude_dialog.height` | `0` | — | Saved height of the exclude dialog (saved automatically) |

> **CLI flag override.** Each flag listed above takes precedence over its corresponding setting when the flag is passed explicitly. Without the flag, the value comes from `settings.yaml` — except `generate.algorithm`, which is consulted only when the output file name carries no recognized algorithm extension (`--algorithm` → output extension → `generate.algorithm`).

### Verify Tab Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `verify.verify_on_open` | `true` | Auto-start verification when opening checksum file |
| `verify.close_on_success` | `false` | Close the application after a successful verification |
| `verify.column_order` | `idx, status, path, size, hash, expected_hash, note` | Order of columns in Verify tab |
| `verify.sort_column` | `status` | Column to sort by in Verify tab |
| `verify.sort_order` | `desc` | Sort order in Verify tab (`asc` / `desc`) |

### Hash Tab Settings

| Setting | Default | CLI Flag | Description |
|---------|---------|----------|-------------|
| `hash.algorithms` | `.md5, .sha1, .sha256` | `--algorithms` | Default hash algorithms for `hash` command and GUI (e.g., `.md5`, `.sha256`) |
| `hash.hash_on_open` | `true` | — | Auto-start hashing when opening a file |

> **CLI flag override.** Each flag listed above takes precedence over its corresponding setting when the flag is passed explicitly. Without the flag, the value comes from `settings.yaml`.

### Discover Tab Settings

| Setting | Default | CLI Flag | Description |
|---------|---------|----------|-------------|
| `discover.column_order` | `idx, status, path, size, hash, expected_hash, note` | — | Order of columns in Discover tab |
| `discover.sort_column` | `status` | — | Column to sort by in Discover tab |
| `discover.sort_order` | `desc` | — | Sort order in Discover tab (`asc` / `desc`) |
| `discover.sources_dialog.follow_symbolic_links` | `true` | `--follow-symbolic-links` | Follow symbolic links when scanning sources |
| `discover.sources_dialog.skip_hidden` | `false` | `--skip-hidden` | Skip hidden files and directories when scanning sources |
| `discover.sources_dialog.files_to_scan` | `with_checksum` | `--files-to-scan` | Which files a discovery run processes — see values below |
| `discover.sources_dialog.action` | `report_only` | `--action` | What a discovery run does with in-scope files — see values below |
| `discover.sources_dialog.width` | `0` | — | Saved width of the sources dialog (saved automatically) |
| `discover.sources_dialog.height` | `0` | — | Saved height of the sources dialog (saved automatically) |

> **CLI flag override.** Each flag listed above takes precedence over its corresponding setting when the flag is passed explicitly. Without the flag, the value comes from `settings.yaml`.

**Files to scan values:**

| Value | Selected files |
|-------|----------------|
| `with_checksum` | Names that embed a CRC-32 |
| `all` | Every file |
| `without_checksum` | Names that do not embed a CRC-32 |

**Action values:**

| Value | Behavior |
|-------|----------|
| `report_only` | Verify checksums; never rename files |
| `add_missing` | Add checksums to names without one; existing checksums are verified, not fixed |
| `add_or_fix` | Add missing checksums and replace mismatched ones |
| `remove` | Remove embedded checksums without reading the file contents |

### Flatpak Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `flatpak.suppress_sandbox_warning` | `false` | Suppress the Flatpak sandbox warning dialog on startup |

> **Note:** Flatpak settings only apply when running the application as a Flatpak package.

## Invalid Settings

A value of the right type but outside the allowed set — an unknown `restore_mode`, `window_state`, `sort_order`, `files_to_scan` or `action`, an unknown algorithm, a negative size, a broken `column_order` or `sort_column` — is reset to its default when the file is loaded, and the repair is reported: `config show` prints a `Repairs applied (N)` block for the file as it was loaded, `config edit` prints it for the file as the editor left it, and `generate`, `hash` and `discover` log a warning (`verify` does not read the settings at all). `hash.algorithms` is the exception — only the unrecognized entries are dropped and the recognized ones are kept; an explicitly empty list is accepted as it is and leaves the `hash` command without an algorithm to compute.

The load never rewrites the file. `config show` only reads; `config edit` opens the file in your editor (and creates it with the defaults when it is missing), so the file changes only if you save it; the GUI rewrites it on the next settings change, which persists the repair. Unknown keys are ignored, and a value of the wrong YAML type is not repaired — the parse fails first, so `config show` and `config edit` report the file as corrupt and exit with code `78`, while the other commands and the GUI fall back to the built-in defaults (the GUI then overwrites the unreadable file on its next save).
