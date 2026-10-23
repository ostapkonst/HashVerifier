# HashVerifier Usage Guide

## GUI Mode

The root command is a GUI launcher. **Any single positional argument opens the GUI as a literal path** — even strings that look like CLI commands (`generate`, `verify`, `hash`, `discover`, `config`) or flags (`-v`, `--version`, `-h`, `--help`). With 0–1 args the GUI opens; with 2+ args the first arg is dispatched as a CLI command instead (see [CLI Mode](#cli-mode)).

```bash
# Launch GUI (no path)
./hashverifier

# Open file or directory
./hashverifier /path/to/directory
./hashverifier /path/to/checksum.sha256
./hashverifier /path/to/document.pdf

# Names that look like commands or flags are still paths
./hashverifier ./help
./hashverifier ./generate
./hashverifier ./config
./hashverifier -v
./hashverifier --version

# Ephemeral mode (env var; --no-config is not a root flag)
HASHVERIFIER_NO_CONFIG=1 ./hashverifier ./data
```

> **Path vs. command.** A single argument always opens the GUI on that path. CLI mode triggers when there are 2+ positional arguments — the first is the command name (`generate`, `verify`, `hash`, `discover`, `config`), the rest are forwarded to it. `./hashverifier --no-config ./dir` does not work for GUI ephemeral mode (it would dispatch `--no-config` as a CLI command); use the env var instead.

## CLI Mode

CLI mode is triggered by 2+ positional arguments. The first argument is the command name (`generate`, `verify`, `hash`, `discover`, `config`); the rest are forwarded to the command.

> **Help and version.** There is no `--help` / `--version` at the root — `--help` opens GUI on `./--help` and `--version` opens GUI on `./--version`. Per-subcommand help is available as `<command> --help`; the application version is shown in the GUI Help → About dialog.

> **Defaults and overrides.** Each command loads its applicable defaults from `settings.yaml` (e.g., `generate.*` and `hash.algorithms`). Pass a CLI flag to override the corresponding config setting for a single invocation.

### Generate Checksums

```bash
./hashverifier generate ./data ./data.sha256
./hashverifier generate ./photos ./photos.md5
./hashverifier generate ./data ./data/checksums.sha256 --flat-paths
./hashverifier generate ./data ./data.sha256 --exclude build/ --exclude secrets.env
./hashverifier generate ./data ./data/manifest --algorithm .sha256
./hashverifier generate ./data ./data.sha256 --follow-symbolic-links=false
./hashverifier generate ./data ./data.sha256 --sort-paths=false
./hashverifier generate ./data ./data.sha256 --skip-hidden
./hashverifier generate ./data ./data.sha256 --force
```

Algorithm is determined in this order: `--algorithm` flag → output file extension → `generate.algorithm` config setting.
Settings `generate.follow_symbolic_links`, `generate.sort_paths`, `generate.skip_hidden` and `generate.flat_paths` are loaded from configuration file; their corresponding CLI flags override the config.
An empty flag value (e.g. `--algorithm ""`) is treated as unset and falls back to the next source in the order above.

> **Symbolic link handling.** With `--follow-symbolic-links=true` (default), file symlinks are hashed as regular entries (hash of the target) and directory symlinks are descended into. With `--follow-symbolic-links=false`, symlink entries are excluded entirely: file symlinks are not listed, and directory symlinks are not descended into (like `tar` without `-h`). With following enabled, broken symlinks and symlink loops are reported as per-file failures (`FAILED` rows, counted as `Failures` in the footer, exit code `1`); with following disabled they are excluded silently. Non-regular files (FIFOs, sockets, devices) are always rejected as per-file failures in both modes.

> **Exclude matching and case sensitivity.** `--exclude` matches a top-level tree entry by name — the entry itself, and its contents if it's a directory. Matching is component-aware (`build` excludes `build/x` but not `build-tools/x`). Both `build` and `build/` are accepted and equivalent. A rule with a slash (e.g. `build/out.o`) matches that single rel path; use a top-level directory name (e.g. `build/`) to exclude every file beneath it. On Windows and macOS the default filesystem is case-insensitive, so matching is case-insensitive there; on Linux matching is case-sensitive. Repeat `--exclude` with each casing you need to cover on a case-sensitive filesystem.

> **Skip hidden.** With `--skip-hidden` (or `generate.skip_hidden: true`), entries that the host platform marks hidden are skipped like excluded files: a name starting with `.` on Unix, or the hidden file attribute (on the file or any parent directory) on Windows. Hidden files are counted as **skipped** and are not written to the checksum file; the GUI Exclude dialog hides them too.

By default, `generate` refuses to overwrite an existing output file (exit code `1`). Pass `--force` to overwrite without prompting. The output path must also be a regular file or a symlink whose target is a regular file — directories, FIFOs, sockets, device nodes (`/dev/null`, `/dev/zero`) and symlinks to any of those are refused regardless of `--force` so that the checksum file is never silently dropped or written to an unsafe target. The same rule applies to `--export` in the `hash` command.
SUMS-style filenames are **not** auto-detected for output (unlike `verify`). Without `--algorithm` or a recognized extension, the algorithm falls back silently to `generate.algorithm` in settings. Prefer an explicit extension (e.g., `.sha256`) or `--algorithm` to avoid surprises.

### Verify Files

```bash
./hashverifier verify ./data.sha256
./hashverifier verify ./archive.md5
./hashverifier verify ./checksum.txt --algorithm .sha256
```

Algorithm is determined in this order: `--algorithm` flag → SUMS-style filename → checksum file extension. The `--algorithm` flag requires a leading dot (e.g., `.sha256`).
The checksum file itself must be a regular file; anything else (directory, FIFO, socket, device) is refused with exit code `1`.
SUMS-style filenames (e.g., `SHA256SUMS`, `MD5SUMS`, `BLAKE3SUMS`, `SFVSUMS.TXT`) are detected automatically — both the algorithm prefix (any supported algorithm) and the suffix (`SUMS`, `SUM`, `SUMS.TXT`, `SUM.TXT`) are matched case-insensitively.

### Calculate File Hash

```bash
./hashverifier hash ./document.pdf
./hashverifier hash ./image.png --algorithms .md5,.sha256
./hashverifier hash ./image.png --algorithms .md5 --algorithms .sha256
./hashverifier --no-config hash ./document.pdf --export checksums.sha256
./hashverifier --no-config hash ./document.pdf --export a.sha256 --export b.md5 --force
```

Algorithms are determined from the `hash.algorithms` configuration setting. The `--algorithms` flag overrides the config and accepts a comma-separated list or repeated flags. Each algorithm must include a leading dot (e.g., `.md5`, `.sha256`). An empty `--algorithms ""` is treated as unset and falls back to the `hash.algorithms` setting.
Use `--export` to write a checksum line to file. Repeat the flag to export multiple algorithms at once; the algorithm is inferred from the file extension (`.sha256`, `.md5`, etc.). Pass `--force` to overwrite existing files (refused by default). Each exported algorithm must be listed in `--algorithms` (or the default `hash.algorithms` setting).
Use `--no-config` (or `HASHVERIFIER_NO_CONFIG=1`) for reproducible behavior in scripts and CI — built-in defaults (`md5`, `sha1`, `sha256`) are used instead of the user's `hash.algorithms` from `settings.yaml`.

### Discover Files

```bash
./hashverifier discover ./downloads
./hashverifier discover ./downloads --filter with_checksum
./hashverifier discover ./a.mkv ./b.txt --filter all
./hashverifier discover ./downloads --rename update
./hashverifier discover ./downloads --rename add --dry-run
./hashverifier --no-config discover ./dir --filter without_checksum --rename add
```

Scans files and directories for CRC-32 checksums embedded in file names (e.g. `filename [1a2b3c4d].ext`) and verifies them against the file contents. Comparison happens only when the name carries a checksum; files without one are reported as `NO CHECKSUM` and do not affect the exit code. Sources may be files, directories, or both, mixed.

Each scanned file receives one status:

| Status | Meaning |
|--------|---------|
| `MATCHED` | The embedded checksum equals the measured CRC-32 |
| `MISMATCH` | The embedded checksum differs from the measured CRC-32 |
| `FAILED` | The file could not be read or hashed |
| `NO CHECKSUM` | The name carries no checksum, so no comparison was possible |

`NO CHECKSUM` is informational and never counts as a failure.

`--filter` selects which files are scanned: `all`, `with_checksum`, or `without_checksum`. It defaults to `discover.sources_dialog.filter`.
Symbolic links are followed (broken links are reported as `FAILED`), and hidden files are scanned like any other file. Non-regular files (FIFOs, sockets, devices) are reported as `FAILED`.

`--rename add|remove|update` rewrites names from the computed CRC-32: `add` inserts the checksum into names without one (`frame.txt` → `frame [1a2b3c4d].txt`), `remove` strips it, `update` replaces a stale checksum. Renaming is off by default. Use `--dry-run` with `--rename` to log the planned renames without applying them. Failed files are never renamed. Renaming runs as a cancellable background operation: Stop in the GUI or Ctrl+C in the CLI cancels it, keeping renames already applied. Renaming a symbolic link renames the link itself, not its target.

Exit code is `1` when any file mismatches or fails, when a source is unavailable, when a flag is invalid, or when a rename fails; `NO CHECKSUM` files are not failures. The exit code reflects the discovery verdict before any rename, so re-run the command after `--rename` to re-verify.

## Configuration

See [Configuration Guide](CONFIGURATION.md) for detailed settings documentation.

### Quick Commands

```bash
./hashverifier config show        # View settings
./hashverifier config edit        # Edit settings
./hashverifier config reset       # Reset settings (prompts for confirmation)
./hashverifier config reset --yes # Skip the confirmation prompt (for scripts/CI)
```

## Ephemeral Mode (`--no-config`)

The `--no-config` flag and the `HASHVERIFIER_NO_CONFIG` environment variable let you run HashVerifier without reading or writing `settings.yaml` — see the [Ephemeral Mode](CONFIGURATION.md) section of the Configuration Guide for the accepted env var values and precedence rules.

```bash
./hashverifier --no-config generate ./data ./data.sha256
./hashverifier --no-config hash ./file.png
HASHVERIFIER_NO_CONFIG=1 ./hashverifier   # GUI: title becomes "HashVerifier — Ephemeral Mode"
```

`config edit` and `config reset` are unavailable in this mode and exit with code `78` (see [Exit Codes](#exit-codes-cli-mode)).

## Output Format

### SHA256 Example

```
; Generated at <timestamp> by HashVerifier <version> (https://github.com/ostapkonst/HashVerifier)

# Lines starting with `#` are also treated as comments
a1b2c3d4e5f6... *documents/report.pdf
f6e5d4c3b2a1... *documents/notes.txt
```

> For hash-first formats (`.md5`, `.sha1`, `.sha256`, `.blake3`, …), both `;` and `#` at the start of a line are treated as comments and skipped during verification. CRC-32/SFV files keep strict path-first format and only honor `;` as a comment — a line starting with `#` is treated as a regular path when it matches the SFV `path hash` layout (its path starts with `#`, typically leading to UNREADABLE), and otherwise fails verification with a parse error.

### CRC32/SFV Example

```
; Generated at <timestamp> by HashVerifier <version> (https://github.com/ostapkonst/HashVerifier)

documents/report.pdf a1b2c3d4
documents/notes.txt f6e5d4c3
```

### Footer with Statistics

Appended to all checksum files:

```
; Statistics:
;   Status: success
;   Processed: 2
```

For single-entry exports from the Hash tab the footer is the same shape, with `exported` status and an `Entries: 1` counter (no `Processed` line, since there is no generation run to report on):

```
; Statistics:
;   Status: exported
;   Entries: 1
```

### Status Values

| Status | Description |
|--------|-------------|
| `exported` | Single-entry export written from the Hash tab |
| `success` | All files were hashed successfully |
| `completed with errors` | Some files could not be hashed (e.g., permission denied) |
| `completed with skipped` | Some files were skipped due to unsupported names or user exclusion (see note below) |
| `completed with errors and skipped` | Some files could not be hashed and some were skipped |
| `canceled` | Operation was canceled by the user |
| `failed` | Operation could not complete due to a hard error (e.g., I/O error, unwritable output) |

> **Note:** Files with newlines (`\n`), carriage returns (`\r`), backslashes (`\`, on Linux) in their names, or paths matched by `--exclude` are skipped during generation — skipped files cannot be represented unambiguously in the checksum file format or were deliberately excluded. For CRC-32/SFV output, file names starting with `;` (parsed as a comment) or ending with a space are skipped as well. These files are counted as **skipped** in the statistics and are not written to the checksum file.

## Verification Results

| Status | Description |
|--------|-------------|
| `MATCHED` | File hash matches — integrity confirmed |
| `MISMATCH` | File hash differs — file may be corrupted |
| `UNREADABLE` | File could not be read — missing or permission denied |

## Exit Codes (CLI Mode)

| Code | Meaning | When |
|------|---------|------|
| `0` | Success | All files processed/verified successfully |
| `1` | Any error | Argument errors, refuse overwrite (use `--force`), missing/unreadable files, write failures, partial failures (mismatch/unreadable), permission denied, invalid algorithm, etc. |
| `2` | Panic | Recovered panic from any goroutine; stderr contains the metadata with a "see system log" hint and a link to the issue tracker; OS log (`journalctl`, Event Viewer, syslog) carries the same report including the full stack |
| `78` | Configuration error | `config show` / `config edit`: settings file is corrupt (unparseable YAML); `config edit` only: `--no-config` mode rejected or no text editor configured; `config reset` only: filesystem error (permissions/disk) or `--no-config` mode rejected |
| `130` | Canceled | Operation interrupted by Ctrl+C (SIGINT) |

> **Notes:**
> - Skipped files in `generate` (invalid names for checksum format or user-excluded) do **not** affect the exit code.
> - Argument errors (wrong number of args, unknown flags) return exit code `1`.
