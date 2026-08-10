# HashVerifier Configuration

HashVerifier stores user preferences in a YAML settings file. You can view and edit settings via the command line or by directly editing the settings file.

## Settings Location

| Platform | Path |
|----------|------|
| Linux | `~/.config/hashverifier/settings.yaml` |
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

## Available Settings

### Window Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `window.tab_order` | `generate, verify, hash` | Order of tabs in main window |
| `window.current_page` | `0` | Currently active tab |
| `window.restore_mode` | `all` | Restore window size/position on startup (`default`, `size`, `position`, `all`) |
| `window.width` | `0` | Window width (saved automatically) |
| `window.height` | `0` | Window height (saved automatically) |
| `window.x_pos` | `-1` | Window X position (saved automatically) |
| `window.y_pos` | `-1` | Window Y position (saved automatically) |
| `window.window_state` | `normal` | Window state on exit (`normal`, `maximized`, `fullscreen`) (saved automatically) |

**Restore mode values:**

| Value | Behavior |
|-------|----------|
| `default` | Use default window size from UI definition |
| `size` | Restore only window size (width and height) |
| `position` | Restore only window position (X and Y) |
| `all` | Restore both window size and position |

### Generate Tab Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `generate.follow_symbolic_links` | `true` | Follow symbolic links when scanning directories |
| `generate.sort_paths` | `true` | Sort paths before hashing |
| `generate.flat_paths` | `false` | Strip root directory from paths; save checksum file inside source directory |
| `generate.algorithm` | `.md5` | Default hash algorithm |
| `generate.column_order` | `idx, status, path, size, hash, note` | Order of columns in Generate tab |
| `generate.sort_column` | `idx` | Column to sort by in Generate tab |
| `generate.sort_order` | `asc` | Sort order in Generate tab (asc/desc) |
| `generate.exclude_dialog.width` | `0` | Saved width of the exclude dialog (saved automatically) |
| `generate.exclude_dialog.height` | `0` | Saved height of the exclude dialog (saved automatically) |

### Verify Tab Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `verify.verify_on_open` | `true` | Auto-start verification when opening checksum file |
| `verify.column_order` | `idx, status, path, size, hash, expected_hash, note` | Order of columns in Verify tab |
| `verify.sort_column` | `status` | Column to sort by in Verify tab |
| `verify.sort_order` | `desc` | Sort order in Verify tab (asc/desc) |

### Hash Tab Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `hash.algorithms` | `.md5, .sha1, .sha256` | Default hash algorithms for `hash` command and GUI (e.g., `.md5`,`.sha256`) |
| `hash.hash_on_open` | `true` | Auto-start hashing when opening a file |

### Flatpak Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `flatpak.suppress_sandbox_warning` | `false` | Suppress the Flatpak sandbox warning dialog on startup |

> **Note:** Flatpak settings only apply when running the application as a Flatpak package.
