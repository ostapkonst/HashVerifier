# HashVerifier Usage Guide

## GUI Mode

```bash
# Launch GUI
./hashverifier

# Open directory (Generate tab)
./hashverifier /path/to/directory

# Open checksum file (Verify tab)
./hashverifier /path/to/checksum.sha256

# Open regular file (Hash tab)
./hashverifier /path/to/document.pdf
```

## CLI Mode

> **Defaults and overrides.** Each command loads its applicable defaults from `settings.yaml` (e.g., `generate.*` and `hash.algorithms`). Pass a CLI flag to override the corresponding config setting for a single invocation.

### Generate Checksums

```bash
./hashverifier generate ./data ./data.sha256
./hashverifier generate ./photos ./photos.md5
./hashverifier generate ./data ./data/checksums.sha256 --flat-paths
./hashverifier generate ./data ./data.sha256 --exclude build/ --exclude secrets.env
./hashverifier generate ./data ./data/manifest --algorithm .sha256
./hashverifier generate ./data ./data --follow-symbolic-links=false
./hashverifier generate ./data ./data --sort-paths=false
```

Algorithm is determined in this order: `--algorithm` flag → output file extension → `generate.algorithm` config setting.
Settings `generate.follow_symbolic_links`, `generate.sort_paths` and `generate.flat_paths` are loaded from configuration file; their corresponding CLI flags override the config.

### Verify Files

```bash
./hashverifier verify ./data.sha256
./hashverifier verify ./archive.md5
./hashverifier verify ./checksum.txt --algorithm .sha256
```

Algorithm is determined in this order: `--algorithm` flag → checksum file extension. The `--algorithm` flag requires a leading dot (e.g., `.sha256`).
SUMS-style filenames (e.g., `SHA256SUMS`, `MD5SUMS`, `BLAKE3SUMS`, `SFVSUMS.TXT`) are detected automatically — both the algorithm prefix (any supported algorithm) and the suffix (`SUMS`, `SUM`, `SUMS.TXT`, `SUM.TXT`) are matched case-insensitively.

### Calculate File Hash

```bash
./hashverifier hash ./document.pdf
./hashverifier hash ./image.png --algorithms .md5,.sha256
./hashverifier hash ./image.png --algorithms .md5 --algorithms .sha256
```

Algorithms are determined from the `hash.algorithms` configuration setting. The `--algorithms` flag overrides the config and accepts a comma-separated list or repeated flags. Each algorithm must include a leading dot (e.g., `.md5`, `.sha256`).

## Configuration

See [Configuration Guide](CONFIGURATION.md) for detailed settings documentation.

### Quick Commands

```bash
./hashverifier config        # View settings
./hashverifier config edit   # Edit settings
./hashverifier config reset  # Reset to defaults
```

## Ephemeral Mode (`--no-config`)

The `--no-config` flag and the `HASHVERIFIER_NO_CONFIG` environment variable let you run HashVerifier without reading or writing `settings.yaml`. Useful for CI, sandboxed environments, or one-shot use where the user's profile must remain untouched.

```bash
./hashverifier --no-config generate ./data ./data.sha256
./hashverifier --no-config hash ./file.png
HASHVERIFIER_NO_CONFIG=1 ./hashverifier   # GUI: title becomes "HashVerifier — Ephemeral Mode"
```

Accepted truthy env var values: `1`, `true`, `yes`, `on`, `y`, `t` (case-insensitive). Anything else, including empty and `0`, is treated as false.

CLI flag wins over the env var. `config edit` and `config reset` are unavailable in this mode and exit with a non-zero code.

## Output Format

### SHA256 Example

```
; Generated at <timestamp> by HashVerifier <version>

# Lines starting with `#` are also treated as comments
a1b2c3d4e5f6... *documents/report.pdf
f6e5d4c3b2a1... *documents/notes.txt
```

> For hash-first formats (`.md5`, `.sha1`, `.sha256`, `.blake3`, …), both `;` and `#` at the start of a line are treated as comments and skipped during verification. CRC-32/SFV files keep strict path-first format and only honour `;` as a comment — lines starting with `#` are treated as regular paths.

### CRC32/SFV Example

```
; Generated at <timestamp> by HashVerifier <version>

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
| `completed with skipped` | Some files were skipped due to unsupported names (see note below) |
| `completed with errors and skipped` | Some files could not be hashed and some were skipped |
| `canceled` | Operation was canceled by the user |
| `failed` | Operation could not complete due to a hard error (e.g., I/O error, symlink loop) |

> **Note:** Files with newlines (`\n`), carriage returns (`\r`), or backslashes (`\`, on Linux) in their names are skipped during generation — they cannot be represented unambiguously in the checksum file format. These files are counted as **skipped** in the statistics and are not written to the checksum file.

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
| `1` | Partial failure | `verify`: mismatch or unreadable files detected; `generate`: some files failed to hash |
| `2` | Hard error | File or directory not found, unreadable checksum file, write failure, etc. |
| `78` | Configuration error | `config show` / `config edit` / `config reset`: `--no-config` mode rejected, settings file is corrupt (unparseable YAML), or no text editor configured |
| `130` | Canceled | Operation interrupted by Ctrl+C (SIGINT) |

> **Notes:**
> - Skipped files in `generate` (invalid names for checksum format or user-excluded) do **not** affect the exit code.
> - Argument errors (wrong number of args, unknown flags) return exit code `1`.
> - GUI mode always exits with `0` on success and `1` on failure.
