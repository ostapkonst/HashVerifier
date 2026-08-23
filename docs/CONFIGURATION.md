# HashVerifier Configuration

HashVerifier stores user preferences in a YAML settings file. You can view and edit settings via the command line or by directly editing the settings file.

For runtime usage (CLI invocation patterns, output formats, exit codes) see [USAGE Guide](USAGE.md).

## Settings Location

| Platform | Path |
|----------|------|
| Linux   | `~/.config/hashverifier/settings.yaml` |
| macOS   | `~/Library/Application Support/hashverifier/settings.yaml` |
| Windows | `%APPDATA%\hashverifier\settings.yaml` |

## CLI Commands

**View settings:**

```bash
./hashverifier config
./hashverifier config show
```

**Edit settings:**

```bash
./hashverifier config edit
```

Opens the settings file in your default text editor (`$VISUAL` or `$EDITOR`).

**Reset settings:**

```bash
./hashverifier config reset
```

## Ephemeral Mode (`--no-config`)

Pass `--no-config` to any CLI subcommand, or set `HASHVERIFIER_NO_CONFIG=1`, to skip reading and writing `settings.yaml`. HashVerifier runs with built-in defaults; any toggle, edit, or reset that would touch the file becomes a no-op (with `config edit` and `config reset` returning a non-zero exit code).

GUI mode follows the same rule: when `HASHVERIFIER_NO_CONFIG` is set, the window title becomes `HashVerifier — Ephemeral Mode` so the mode is unmistakable.

**Truthy values for the env var:** `1`, `true`, `yes`, `on`, `y`, `t` (case-insensitive). Anything else (including empty and `0`) is treated as false. The CLI flag takes precedence over the environment variable.

Typical use cases: CI/automation, ephemeral containers, Flatpak or sandbox runs that should not pollute the host profile, sharing a reproducible environment.

## Available Settings

### Window Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `window.tab_order` | `generate, verify, hash` | Order of tabs in main window |
| `window.current_page` | `0` | Currently active tab (saved automatically) |
| `window.restore_mode` | `all` | Restore window size/position on startup — see values below |
| `window.width` | `0` | Window width (saved automatically) |
| `window.height` | `0` | Window height (saved automatically) |
| `window.x_pos` | `-1` | Window X position (saved automatically, `-1` = not yet recorded) |
| `window.y_pos` | `-1` | Window Y position (saved automatically, `-1` = not yet recorded) |
| `window.window_state` | `normal` | Window state on exit (saved automatically) |

**Restore mode values:**

| Value | Behavior |
|-------|----------|
| `default` | Use default window size from UI definition |
| `size` | Restore only window size (width and height) |
| `position` | Restore only window position (X and Y) |
| `all` | Restore both window size and position |

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
| `generate.flat_paths` | `false` | `--flat-paths` | Strip root directory from paths; save checksum file inside source directory as `checksums.<ext>` |
| `generate.algorithm` | `.md5` | `--algorithm` | Default hash algorithm |
| `generate.column_order` | `idx, status, path, size, hash, note` | — | Order of columns in Generate tab |
| `generate.sort_column` | `idx` | — | Column to sort by in Generate tab |
| `generate.sort_order` | `asc` | — | Sort order in Generate tab (`asc` / `desc`) |
| `generate.exclude_dialog.width` | `0` | — | Saved width of the exclude dialog (saved automatically) |
| `generate.exclude_dialog.height` | `0` | — | Saved height of the exclude dialog (saved automatically) |

> **CLI flag override.** Each flag listed above takes precedence over its corresponding setting when the flag is passed explicitly. Without the flag, the value comes from `settings.yaml`.

### Verify Tab Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `verify.verify_on_open` | `true` | Auto-start verification when opening checksum file |
| `verify.column_order` | `idx, status, path, size, hash, expected_hash, note` | Order of columns in Verify tab |
| `verify.sort_column` | `status` | Column to sort by in Verify tab |
| `verify.sort_order` | `desc` | Sort order in Verify tab (`asc` / `desc`) |

### Hash Tab Settings

| Setting | Default | CLI Flag | Description |
|---------|---------|----------|-------------|
| `hash.algorithms` | `.md5, .sha1, .sha256` | `--algorithms` | Default hash algorithms for `hash` command and GUI (e.g., `.md5`, `.sha256`) |
| `hash.hash_on_open` | `true` | — | Auto-start hashing when opening a file |

> **CLI flag override.** Each flag listed above takes precedence over its corresponding setting when the flag is passed explicitly. Without the flag, the value comes from `settings.yaml`.

### Flatpak Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `flatpak.suppress_sandbox_warning` | `false` | Suppress the Flatpak sandbox warning dialog on startup |

> **Note:** Flatpak settings only apply when running the application as a Flatpak package.
