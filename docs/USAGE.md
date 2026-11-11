# HashVerifier Usage Guide

## GUI Mode

The root command is a GUI launcher. **Any single command-line token opens the GUI as a literal path** — even strings that look like CLI commands (`generate`, `verify`, `hash`, `discover`, `config`) or flags (`-v`, `--version`, `-h`, `--help`). The mode is chosen by token count before any flag parsing: with 0–1 tokens the GUI opens; with 2+ tokens the CLI is used, a leading `--no-config` is parsed as the global flag, and the first non-flag token is the command name (see [CLI Mode](#cli-mode)).

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

# Ephemeral mode (env var; the GUI path does not parse --no-config)
HASHVERIFIER_NO_CONFIG=1 ./hashverifier ./data
```

> **Path vs. command.** A single token always opens the GUI, and the token is treated as a literal path: an existing file or directory is opened in the matching tab, while a token that does not exist on disk leaves the GUI on its last tab with nothing prefilled. CLI mode is selected by token count, not by positional arguments: 2+ command-line tokens enter the CLI, where the root persistent flag `--no-config` is recognized before or after the subcommand. As a result, `./hashverifier --no-config ./dir` (two tokens) enters CLI mode, leaves `./dir` as the only positional argument, and fails with `unknown command: ./dir`; it does not enable GUI ephemeral mode. Use `HASHVERIFIER_NO_CONFIG=1` for the GUI instead.

The window has four tabs:

- **Generate** — pick an input directory and output checksum file; the options cover the same ground as the `generate` CLI flags (follow symlinks, sort paths, flat paths, skip hidden, algorithm, exclude), though algorithm selection and exclude granularity differ (see the notes below); an overwrite always asks for confirmation.
- **Verify** — pick a checksum file and verify the files it references.
- **Hash** — hash a single file with one or more algorithms, with optional export. Every supported algorithm is listed in the table; the checked ones come from the `hash.algorithms` setting. A file whose name carries an embedded CRC-32 (e.g. `movie [1a2b3c4d].mkv`) has the CRC-32 row checked automatically when the file is picked with *Browse*, dropped onto the window, or opened from the launcher — the choice is saved, so it stays checked afterwards. Paste the checksum from the name into the search field to compare — the field turns green when a computed hash matches it and red when hashes were computed but none of them matches.
- **Discover** — scan files and directories for CRC-32 checksums embedded in names, then report, add, fix, or remove them.

The active tab, window geometry, and column layout are saved automatically. Tab order is read from `window.tab_order` in `settings.yaml`.

> **Version.** The application version is shown in the GUI's *About* dialog, opened from the *About* button at the right end of the tab strip.

> **Algorithm selection differs from the CLI.** In the GUI the algorithm always follows the output file's extension: the *Algorithm* selector rewrites the output checksum file name to match, and an extension that is still unrecognized when the run starts is refused ("Checksum file has an unsupported extension"). The CLI resolves it as `--algorithm` flag → output file extension → `generate.algorithm` setting, so it can pick an algorithm that differs from the output extension and falls back to the setting when the extension is unrecognized. The `generate.algorithm` setting pre-selects the GUI selector.

> **The Verify algorithm selector is locked for known extensions.** On the Verify tab the selector is editable only when the checksum file name carries no recognized algorithm extension — `SHA256SUMS` or `archive.sha256.txt`, for example. A name like `checksums.sha256` locks the selector to that extension, so the algorithm cannot be changed there. The CLI's `--algorithm` overrides the file name in every case.

> **Exclude granularity differs from the CLI.** The GUI *Exclude* dialog lists the top-level entries of the source directory and excludes them by name, so it cannot express a nested rule such as `build/out.o`. The CLI `--exclude` accepts those — see [Exclude matching and case sensitivity](#generate-checksums).

> **GUI interactions.** All four tabs share the same mouse actions:
>
> - **Drag and drop** — a dropped directory opens in Generate, a file whose name resolves to an algorithm opens in Verify, anything else opens in Hash; only the first path of a multi-file drop is used. The Discover sources dialog also accepts dropped files and folders.
> - **Context menu** — right-click a row in Generate, Verify, or Discover to *Show in Explorer* or to copy its full path, directory, file name, or any single column. The Hash tab's menu offers *Export hash* plus per-column copies instead.
> - **Discover sources dialog** — add files or folders, remove the selected sources, and choose *Follow symlinks*, *Skip hidden*, *Files to scan*, and *Checksum actions*; the confirm button shows how many sources are selected, and confirming with none clears the results.
> - **Exclude dialog** — *Exclude All* and *Keep All* set every checkbox at once; the confirm button shows how many entries will be excluded.
> - **Overwrite prompt** — Generate and the Hash tab's export ask for confirmation before replacing an existing file.
> - **Flatpak** — when running as a Flatpak package, a startup warning explains the sandbox limits and can be suppressed with *Don't show this warning again*.

## CLI Mode

CLI mode is triggered by 2+ command-line tokens (flags count). A leading `--no-config` is parsed as a global persistent flag; the first non-flag token is the command name (`generate`, `verify`, `hash`, `discover`, `config`, plus Cobra's built-in `help` and `completion`), and the rest are forwarded to it.

> **Windows has no CLI.** The Windows package ships the GUI only: every invocation starts the GUI, and only the first argument is used — as a literal path to open. `HASHVERIFIER_NO_CONFIG=1` still selects ephemeral mode there.

> **Help.** Per-subcommand help is available as `<command> --help`. A single `--help` or `--version` token never reaches the CLI — the GUI dispatcher takes it as a path instead (see [GUI Mode](#gui-mode)).

> **Defaults and overrides.** Each command loads its applicable defaults from `settings.yaml` (e.g., `generate.*` and `hash.algorithms`). Pass a CLI flag to override the corresponding config setting for a single invocation. The `verify` command has no settings-backed options — its algorithm comes from the flag, the file name, or the file extension.

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

> **Symbolic link handling.** With `--follow-symbolic-links=true` (default), file symlinks are hashed as regular entries (hash of the target's content; the recorded path is the symlink's own name) and directory symlinks are descended into. With `--follow-symbolic-links=false`, symlink entries are excluded entirely: file symlinks are not listed, and directory symlinks are not descended into (like `tar` without `-h`). With following enabled, broken symlinks are reported as per-file failures (`FAILED` rows, counted as `Failures` in the footer, exit code `1`); with following disabled they are excluded silently. Non-regular files (FIFOs, sockets, devices) are always rejected as per-file failures in both modes.
>
> **Symlink loops are not detected.** A directory symlink that points into its own tree is descended into again, so the files below it are listed once per path. With two or more such symlinks the number of paths grows without bound and the run never finishes.

> **Exclude matching and case sensitivity.** `--exclude` matches a top-level tree entry by name — the entry itself, and its contents if it's a directory. Matching is component-aware (`build` excludes `build/x` but not `build-tools/x`). Both `build` and `build/` are accepted and equivalent. A rule with a slash (e.g. `build/out.o`) matches that single rel path; use a top-level directory name (e.g. `build/`) to exclude every file beneath it. On Windows and macOS the default filesystem is case-insensitive, so matching is case-insensitive there; on Linux matching is case-sensitive. Repeat `--exclude` with each casing you need to cover on a case-sensitive filesystem.

> **Skip hidden.** With `--skip-hidden` (or `generate.skip_hidden: true`), entries that the host platform marks hidden are skipped like excluded files: a name starting with `.` on Unix (on the entry itself or any parent directory below the source), or the hidden file attribute (on the file or any parent directory below the source) on Windows. Hidden files are counted as **skipped** and are not written to the checksum file; the GUI Exclude dialog hides them too when *Skip hidden* is enabled. The source directory itself is never treated as hidden, so pointing the generator at a hidden directory still hashes what it holds.

By default, `generate` refuses to overwrite an existing output file (exit code `1`). Pass `--force` to overwrite without prompting. The output path must also be a regular file or a symlink whose target is a regular file — directories, FIFOs, sockets, device nodes (`/dev/null`, `/dev/zero`) and symlinks to any of those are refused regardless of `--force` so that the checksum file is never silently dropped or written to an unsafe target. The same rule applies to `--export` in the `hash` command, which additionally refuses an output path that resolves to the file being hashed (including hard-link and symlink aliases) so the source is never overwritten.
SUMS-style filenames are **not** auto-detected for output (unlike `verify`). Without `--algorithm` or a recognized extension, the algorithm falls back silently to `generate.algorithm` in settings. Prefer an explicit extension (e.g., `.sha256`) or `--algorithm` to avoid surprises.

### Verify Files

```bash
./hashverifier verify ./data.sha256
./hashverifier verify ./archive.md5
./hashverifier verify ./checksum.txt --algorithm .sha256
```

Algorithm is determined in this order: `--algorithm` flag → SUMS-style filename → checksum file extension. The `--algorithm` flag requires a leading dot (e.g., `.sha256`).
The checksum file itself must be a regular file; anything else (directory, FIFO, socket, device) is refused with exit code `1`.
Relative entries are resolved against the directory that holds the checksum file — the path as given, so a relative checksum-file path resolves against the current working directory. Absolute entries are used as they are. For a symlinked checksum file the base is the link's own directory, not the target's.
SUMS-style filenames (e.g., `SHA256SUMS`, `MD5SUMS`, `BLAKE3SUMS`, `SFVSUMS.TXT`) are detected automatically — both the algorithm prefix (any supported algorithm) and the suffix (`SUMS`, `SUM`, `SUMS.TXT`, `SUM.TXT`) are matched case-insensitively. A trailing `.txt` after the algorithm extension is ignored too (`archive.sha256.txt` resolves to SHA-256).

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
./hashverifier discover "./movie [1a2b3c4d].mkv"
./hashverifier discover ./downloads --files-to-scan all
./hashverifier discover ./downloads --files-to-scan without_checksum --action add_missing
./hashverifier discover ./downloads --action add_or_fix
./hashverifier discover ./downloads --action remove
./hashverifier discover ./downloads --follow-symbolic-links=false --skip-hidden
./hashverifier --no-config discover ./dir
```

Scans files and directories for CRC-32 checksums embedded in file names (e.g. `filename [1a2b3c4d].ext`). Every file found under the sources is listed; `--files-to-scan` selects the files the run processes and `--action` selects what it does with them. The defaults (`with_checksum`, `report_only`) verify checksum-bearing names and report everything else as `SKIPPED`. For a symlink the checksum is read from the link's own name — the name you point at — never the target's name. Sources may be files, directories, or both, mixed.

**Files to scan:**

| Value | Selected files |
|-------|----------------|
| `with_checksum` | Names that embed a CRC-32 |
| `all` | Every file |
| `without_checksum` | Names that do not embed a CRC-32 |

`--files-to-scan` defaults to `discover.sources_dialog.files_to_scan` (`with_checksum`).

**Checksum actions:**

| Value | Behavior |
|-------|----------|
| `report_only` | Verify checksums; never rename files |
| `add_missing` | Add a checksum to names without one; existing checksums are verified, not fixed |
| `add_or_fix` | Add missing checksums and replace mismatched ones |
| `remove` | Remove embedded checksums without reading the file contents |

`--action` defaults to `discover.sources_dialog.action` (`report_only`).

**Combined behavior.** Every file is always listed; the scope changes its status, and the action changes whether it is renamed:

| Scope | `report_only` | `add_missing` | `add_or_fix` | `remove` |
|-------|---------------|---------------|--------------|----------|
| `with_checksum` | verify; names without a checksum are `SKIPPED` | verify only; mismatches stay `MISMATCH` | fix mismatches | remove checksums; names without one are `SKIPPED` |
| `all` | verify; names without a checksum are `SKIPPED` | add to names without one; existing checksums are verified only | add or fix | remove checksums; names without one are `SKIPPED` |
| `without_checksum` | all `SKIPPED` | add to every selected name; names with a checksum are `SKIPPED` | same as `add_missing` | all `SKIPPED` |

In `report_only`, `with_checksum` and `all` produce the same statuses: only names with an embedded checksum can be verified, and names without one are `SKIPPED` without hashing. The scopes differ for the renaming actions.

Each file receives one status:

| Status | Meaning |
|--------|---------|
| `SUCCESS` | Desired final state reached: a checksum is present and correct, or (for `remove`) the checksum was removed |
| `MISMATCH` | A checksum remains but differs from the measured CRC-32 (`report_only`, or `add_missing`, which does not fix existing checksums) |
| `FAILED` | The file could not be read, hashed, or renamed, or the target name already exists |
| `SKIPPED` | The file is hidden (`--skip-hidden`), outside the selected scope, has no checksum where the action needs one, or there is nothing to do |

`SKIPPED` is informational and never counts as a failure.

**Renaming.** `add_missing` inserts ` [crc]` before the final extension (`clip.mkv` → `clip [1a2b3c4d].mkv`; a dotfile such as `.hidden` is treated as extensionless). `add_or_fix` replaces the checksum in place (`clip [deadbeef].mkv` → `clip [1a2b3c4d].mkv`). `remove` deletes the checksum and the spaces before it (`clip [1a2b3c4d].mkv` → `clip.mkv`). A successful rename updates the listed path; the `Note` column records `added [crc]`, `fixed [old] → [new]`, or `removed [old]`. `add_missing` on an existing mismatch reports `MISMATCH` with the note `mismatch not fixed (Add missing)`.

Rewrites never overwrite an existing name: if the target already exists — including a name created earlier in the same run — the file is `FAILED`. Renames happen while the run streams; canceling the run leaves already renamed files renamed and does not roll back. `remove` only checks that the path is a regular file and does not read its contents, so its `Hash` and `Expected Hash` columns are `00000000`; the removed checksum is shown in the note.

`--follow-symbolic-links` defaults to `discover.sources_dialog.follow_symbolic_links` (`true`); pass `--follow-symbolic-links=false` to ignore symbolic links. With following enabled, broken links are reported as `FAILED`.
`--skip-hidden` defaults to `discover.sources_dialog.skip_hidden` (`false`); when set, hidden files and directories (dot-prefixed on Unix, the hidden attribute on Windows) are not scanned and are reported as `SKIPPED`. Otherwise hidden files are scanned like any other file. Non-regular files (FIFOs, sockets, devices) are reported as `FAILED` whenever the action needs to read or rename them; a file skipped without being read is `SKIPPED`.

> **Explicit sources.** `--skip-hidden` and `--follow-symbolic-links` govern entries discovered while walking a directory source, not the source itself. A hidden file passed directly is scanned like any other file, and a symlink passed directly is followed and its target's content is hashed regardless of `--follow-symbolic-links`; the embedded checksum is still read from the symlink's own name, never the target's, so a link whose own name carries no checksum is reported as `SKIPPED`. A source that cannot be stat-ed (including a broken symlink) fails the command with exit code `1` as an unavailable source; it is not reported as a per-file `FAILED` row.

> **Hidden files and several sources.** A file is classified against the source that found it: it is hidden when its path below that source runs through a hidden entry — a dot-prefixed name on Unix, the hidden attribute on Windows. A source that is itself hidden does not hide its own contents, so pointing at a hidden directory scans what it holds. When several sources find the same file, it is reported as `SKIPPED` only when every one of them considers it hidden: a source listed explicitly wins over a broader source that hides the file, and the outcome does not depend on the order of the sources.

> **Symlink loops.** `discover` walks directory sources with the same traversal as `generate`, so the symlink-loop limitation described under [Generate Checksums](#generate-checksums) applies here as well.

Exit code is `1` when any file mismatches or fails, when a source is unavailable, or when a flag is invalid; `SKIPPED` entries are not failures.

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

The `--no-config` flag and the `HASHVERIFIER_NO_CONFIG` environment variable let you run HashVerifier without reading or writing `settings.yaml` — see the [Ephemeral Mode](CONFIGURATION.md#ephemeral-mode---no-config) section of the Configuration Guide for the accepted env var values and precedence rules.

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

a1b2c3d4e5f6... *documents/report.pdf
f6e5d4c3b2a1... *documents/notes.txt
```

> For hash-first formats (`.md5`, `.sha1`, `.sha256`, `.blake3`, …), both `;` and `#` at the start of a line (after any leading whitespace) are treated as comments and skipped during verification. CRC-32/SFV files keep strict path-first format and only honor `;` as a comment — a line starting with `#` is treated as a regular path when it matches the SFV `path hash` layout (its path starts with `#`, typically leading to UNREADABLE), and otherwise fails verification with a parse error.

### CRC32/SFV Example

```
; Generated at <timestamp> by HashVerifier <version> (https://github.com/ostapkonst/HashVerifier)

documents/report.pdf a1b2c3d4
documents/notes.txt f6e5d4c3
```

### Footer with Statistics

Appended to all checksum files, separated from the checksum entries by a blank line:

```
; Statistics:
;   Status: success
;   Processed: 2
```

Each counter line is written only when it is non-zero, so a run with failures and skips reports all of them:

```
; Statistics:
;   Status: completed with errors and skipped
;   Processed: 2
;   Failures: 1
;   Skipped: 3
;   Pending: 4
```

| Counter | Written when | Meaning |
|---------|--------------|---------|
| `Processed` | at least one file was hashed | Files hashed successfully |
| `Failures` | at least one file failed | Files that could not be hashed |
| `Skipped` | at least one file was skipped | Files that were deliberately not hashed (see the note below) |
| `Pending` | files are still unaccounted for | Files that were neither hashed, failed, nor skipped |

`Pending` is non-zero only when the run stopped early — a cancellation, or a write error that ended the generator before the whole tree was processed.

For single-entry exports — the Hash tab's export action and the CLI's `hash --export` — the footer is the same shape, with `exported` status and an `Entries: 1` counter (no `Processed` line, since there is no generation run to report on):

```
; Statistics:
;   Status: exported
;   Entries: 1
```

### Status Values

| Status | Description |
|--------|-------------|
| `exported` | Single-entry export written from the Hash tab or by `hash --export` |
| `success` | Every file the walk listed for hashing was hashed successfully |
| `completed with errors` | Some files could not be hashed (e.g., permission denied) |
| `completed with skipped` | Some files were skipped (hidden, user-excluded, or unsupported names — see note below) |
| `completed with errors and skipped` | Some files could not be hashed and some were skipped |
| `canceled` | Operation was canceled by the user |
| `failed` | Operation could not complete due to a hard error (e.g., I/O error, unwritable output). A hard walk error still writes the footer; when the output file itself fails, the buffered writer's sticky error can drop the footer, so the file may end without the statistics block |

> **Note:** Files with newlines (`\n`), carriage returns (`\r`), or backslashes (`\`, on Unix-like hosts) in their names, and paths matched by `--exclude`, are skipped during generation — skipped files cannot be represented unambiguously in the checksum file format or were deliberately excluded. For CRC-32/SFV output, a path starting with `;` (the line would be parsed as a comment) or ending with a space (the trailing space is eaten by the line parser) is skipped as well. These files are counted as **skipped** in the statistics and are not written to the checksum file.

The check runs on the whole path as it is written into the checksum file, so a backslash in a directory name skips every file below it. With the default layout the path starts with a prefix — the source directory's name when the checksum file is written as a sibling of it (in the source directory's parent directory), and the source directory's absolute path in every other case. Save the checksum file next to the source directory to keep the short `name/path` prefix; saving it inside the source directory yields absolute paths in the file. The `;` rule can therefore only be triggered by the source directory's own name, and never by an absolute prefix, which starts with the path separator (or a drive letter on Windows); the trailing-space rule depends on the file name alone. With `--flat-paths` there is no prefix, so the relative path is checked and a top-level entry whose name starts with `;` triggers it.

A file that **failed** to hash is a different case: it is still written to the checksum file, with an all-zero digest of the algorithm's width (`0000…0000 *path`, or `path 00000000` for CRC-32/SFV). The failure is counted in `Failures` and the command exits with code `1`.

## Verification Results

| Status | Description |
|--------|-------------|
| `MATCHED` | File hash matches — integrity confirmed |
| `MISMATCH` | File hash differs — file may be corrupted |
| `UNREADABLE` | File could not be read — missing or permission denied |

Verifying a file that was recorded with an all-zero digest reports `MISMATCH` (the digest cannot match) or `UNREADABLE` (the file is gone or unreadable).

## Exit Codes (CLI Mode)

| Code | Meaning | When |
|------|---------|------|
| `0` | Success | All files processed/verified successfully |
| `1` | Any error | Argument errors, refuse overwrite (use `--force`), missing/unreadable files, write failures, partial failures (mismatch/unreadable), permission denied, invalid algorithm, `config edit` could not parse, start, or run the configured editor, etc. |
| `2` | Panic | Recovered panic from a goroutine wrapped by the crash reporter (`crash.Go` / `crash.Recover`); stderr contains the metadata with a "see system log" hint and a link to the issue tracker; OS log (`journalctl`, Unified Log, Event Viewer, syslog) carries the same report including the full stack |
| `78` | Configuration error | `config show`: settings file is corrupt (unparseable YAML) or unreadable; `config edit` only: `--no-config` mode rejected, no text editor configured, the settings file could not be prepared (permissions/disk), or the edited file turned out to be corrupt or unreadable; `config reset` only: filesystem error (permissions/disk) or `--no-config` mode rejected |
| `130` | Canceled | Operation canceled by a termination signal (SIGINT/SIGTERM/SIGQUIT), a second signal during shutdown, or a shutdown timeout |

> **Notes:**
> - Skipped files in `generate` (hidden, invalid names for the checksum format, or user-excluded) do **not** affect the exit code.
> - A directory the walk could not read (permission denied) is only logged as a warning: its contents are missing from the checksum file, and neither the status nor the exit code changes.
> - Argument errors (wrong number of args, unknown flags) return exit code `1`.
