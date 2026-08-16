# HashVerifier Usage Guide

## GUI Mode (Default)

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

**Generate checksums:**

```bash
./hashverifier generate ./data ./data.sha256
./hashverifier generate ./photos ./photos.md5
./hashverifier generate ./data ./data/checksums.sha256 --flat-paths
./hashverifier generate ./data ./data.sha256 --exclude build/ --exclude secrets.env
./hashverifier generate ./data ./data/manifest --algorithm .sha256
./hashverifier generate ./data ./data/manifest --algorithm sha256
./hashverifier generate ./data ./data --follow-symbolic-links=false
./hashverifier generate ./data ./data --sort-paths=false
```

Algorithm is determined in this order: `--algorithm` flag → output file extension → `generate.algorithm` config setting.
Settings `generate.follow_symbolic_links`, `generate.sort_paths` and `generate.flat_paths` are loaded from configuration file; their corresponding CLI flags override the config.

**Verify files:**

```bash
./hashverifier verify ./data.sha256
./hashverifier verify ./archive.md5
./hashverifier verify ./checksum.txt --algorithm .sha256
./hashverifier verify ./checksum.txt --algorithm md5
```

Algorithm is determined in this order: `--algorithm` flag → checksum file extension. The `--algorithm` flag accepts both `.sha256` and `sha256` forms.

**Calculate file hash:**

```bash
./hashverifier hash ./document.pdf
./hashverifier hash ./image.png --algorithms .md5,.sha256
./hashverifier hash ./image.png --algorithms .md5 --algorithms .sha256
```

Algorithms are determined from the `hash.algorithms` configuration setting. The `--algorithms` flag overrides the config and accepts a comma-separated list or repeated flags.

## Configuration

See [Configuration Guide](CONFIGURATION.md) for detailed settings documentation.

**Quick commands:**

```bash
./hashverifier config        # View settings
./hashverifier config edit   # Edit settings
./hashverifier config reset  # Reset to defaults
```

## Output Format

**SHA256 example:**

```
; Generated at <timestamp> by HashVerifier <version>

# Lines starting with `#` are also treated as comments
a1b2c3d4e5f6... *documents/report.pdf
f6e5d4c3b2a1... *documents/notes.txt
```

> For hash-first formats (`.md5`, `.sha1`, `.sha256`, `.blake3`, …), both `;` and `#` at the start of a line are treated as comments and skipped during verification. CRC-32/SFV files keep strict path-first format and only honour `;` as a comment.

**CRC32/SFV example:**

```
; Generated at <timestamp> by HashVerifier <version>

documents/report.pdf a1b2c3d4
documents/notes.txt f6e5d4c3
```

**Footer with statistics (appended to all checksum files):**

```
; Statistics:
;   Status: success
;   Processed: 2
```

**Status values:**

| Status | Description |
|--------|-------------|
| `success` | All files were hashed successfully |
| `completed with errors` | Some files could not be hashed (e.g., permission denied) |
| `completed with skipped` | Some files were skipped due to unsupported names (see note below) |
| `completed with errors and skipped` | Some files could not be hashed and some were skipped |
| `cancelled` | Operation was cancelled by the user |
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
| `130` | Cancelled | Operation interrupted by Ctrl+C (SIGINT) |

> **Notes:**
> - Skipped files in `generate` (invalid names for checksum format or user-excluded) do **not** affect the exit code.
> - Argument errors (wrong number of args, unknown flags) return exit code `1`.
> - GUI mode always exits with `0` on success and `1` on failure.
