# HashVerifier Development Guide

This document contains information for developers and builders of the project.

## Requirements

**For native build:**

- Go 1.25 or later
- GTK3 development libraries (for GUI)

**For Windows builds:**

- For native Windows builds, a special Go compiler with Windows 7 support is used: [go-legacy-win7](https://github.com/thongtech/go-legacy-win7)

**For Docker build:**

- Docker
- docker-compose

## Build from Source

```bash
# Clone repository
git clone https://github.com/ostapkonst/HashVerifier.git
cd HashVerifier

# Choose target and execute it with `make <target>`
make help
```

## Project Structure

```
HashVerifier/
├── .github/workflows/    # GitHub Actions CI/CD
├── build/                # Docker build files (Dockerfile, docker-compose, scripts)
├── docs/                 # Documentation
├── flatpak/              # Required to publish an application on FlatHub
├── src/                  # Go source code
│   ├── main.go           # Application entry point (CGO + dispatch)
│   ├── go.mod            # Go module manifest
│   └── internal/         # All application Go code
│       ├── appmeta/      # Identity (Name/Version/Link) + checksum-file header/footer formatters
│       ├── domain/       # Pure types and algorithms — no I/O (algorithm, exclude, hashfn, parser, result, walk)
│       ├── service/      # Use-case orchestration (generate, verify, hash)
│       ├── adapter/      # User-facing interfaces (Cobra CLI, GTK3 GUI)
│       │   ├── cli/      # One package per subcommand (generate, verify, hash, config) plus base/ for shared helpers
│       │   └── gui/      # Window lifecycle, three tabs (generate, verify, hash), and shared widgets/base helpers
│       ├── driver/       # YAML settings persistence + validation + display (yamlconfig package)
│       └── platform/     # Cross-platform infrastructure (crash, editor, eol, env, errs, fs, shutdown, flatpak, reveal)
├── .dockerignore         # Docker build context exclusions
├── .gitattributes        # Git attributes (line endings, binary files)
├── .gitignore            # Git ignore rules
├── .golangci.yml         # Go linter configuration
├── LICENSE               # MIT License
├── Makefile              # Build automation
├── README.md             # Main documentation
└── THIRD_PARTY_NOTICES   # Third-party software notices
```

### Maintaining this section

The tree above is the canonical map of `src/internal/` and **must stay in sync with the codebase**. Update it in the same commit whenever you:

- **add** a new package → add a line and a one-word role comment if the role is not obvious;
- **rename** a package → update both the directory and the comment;
- **move** files between packages → delete from the old location, add at the new one;
- **remove** a package → delete its line.

## Architecture Layers

The codebase follows a layered architecture:

- **`domain/`** — Pure types and algorithms. No I/O, no framework dependencies.
- **`service/`** — Use-case orchestration. Wires domain types into workflows (generate / verify / hash).
- **`driver/`** — Concrete I/O implementations (YAML persistence lives here).
- **`platform/`** — OS and runtime infrastructure (editor, eol, env, errs, fs, shutdown, flatpak, reveal).
- **`adapter/`** — User-facing interfaces. Each adapter is independently swappable:
  - `adapter/cli` is a Cobra command tree. One package per subcommand (`generate`, `verify`, `hash`, `config`).
  - `adapter/gui` is a GTK3 graphical interface. One package per tab (`generate`, `verify`, `hash`).
  - A future `adapter/http` or `adapter/tui` could be added without touching the layers below.
  - Both adapters share the same logical structure: **entry + `base/` for cross-cutting helpers + one package per use-case**.
- **`appmeta/`** — Application identity (`Name`, `Version`, `Link`) injected via `-ldflags` plus checksum-file header/footer formatters.

## Technologies

- **Language:** Go 1.25
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **GUI Toolkit:** [gotk3](https://github.com/gotk3/gotk3) (GTK3 bindings)
- **Logging:** [zerolog](https://github.com/rs/zerolog)
- **Cryptography:** [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto), [blake3](https://github.com/lukechampine/blake3)

## Crash reporting

Every new goroutine the application spawns must use `crash.Go(name, fn)` instead of bare `go fn`; the main goroutine defers `crash.Recover()`. This routes any recovered panic to the configured Sinks:

- Linux: `journalctl -t HashVerifier` (priority `err`).
- macOS / BSD: traditional syslog under `LOG_USER` (facility `err`); also surfaced by `log show --predicate ...` if unified logging is on.
- Windows: Event Viewer → Windows Logs → Application; entries show under Source `Application` with Event ID `1000`.

A recovered panic triggers `os.Exit(2)` after sinks complete, so stderr carries the metadata plus a footer pointing to the OS log and the issue tracker; the OS log sinks carry the same report including the full goroutine stack. The reporter is always active, even under `HASHVERIFIER_NO_CONFIG=1` — otherwise ephemeral CI runs lose crash detail.

Panics raised on the GTK thread itself (signal handlers, `widgets.IdleAdd` callbacks) unwind through the C frames of `gtk_main()` back into the `gui.gtkMain` goroutine and are still caught by its `crash.Go` wrapper, so sinks are written before `os.Exit(2)` cuts the process short.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Commenting Style

Comments explain the **why** behind non-obvious decisions, not restate what the code already says; anything that adds noise — section markers, English translations of code, multi-paragraph godoc — is best left out.

**Every package** carries one `// Package <name>` line, and **every exported identifier** has a godoc (`golint` requires it; standard interface methods like `error.Error` and `fmt.Stringer.String` are exempt). Godoc answers **why** or non-obvious behavior rather than `// X returns Y.` form.

Keep package godoc to **one line**; function, type and method godoc to **1–2 lines** of roughly **140 chars** each; block comments above functions to **three lines**; inline comments to **one line** — and never separate godoc paragraphs with blank lines.

Existing **"why"** comments (such as the C-macro ordering in `drag_dest_unset.go`, the `component-aware` matching in `exclude.Matcher`, or the `defaultTimeout` trade-off in `shutdown`) are preserved **verbatim** across refactors; the caps above apply to *new* comments.

All comments are in English; see [Notes](#notes) for the spelling convention. Tool directives such as `//nolint:*`, `//go:embed`, `// #cgo`, `// #include`, and C macros are kept **verbatim** since the tooling depends on them.

### Error combining order

When combining two errors via `errors.Join(a, b)` or `fmt.Errorf("%w: %w", a, b)`, place the **first-occurred (root) cause FIRST**. The `platform/errs` package walks the error chain via `Unwrap() []error` and returns the **first** non-nil peer to `UnwrapAndNormalize`, which then formats that text for the user. Concretely:

- `errors.Join(rootCauseErr, laterConsequence)` — user sees `rootCauseErr`
- `fmt.Errorf("%w: %w", underlyingCause, sentinel)` — user sees `underlyingCause`

The "first-occurred" error is what the user typically needs to debug: it's the trigger, and later errors in the chain are its consequences. For peer collections (multiple independent failures joined in a slice), `errors.Join(errs...)` is fine — order is registration/execution order.

## Notes

All user-facing strings, identifiers, and documentation in this project use **American English** spelling (e.g., `canceled`, not `cancelled`; `color`, not `colour`). This convention matches the Go standard library (e.g., `context.Canceled`) and keeps the codebase internally consistent. Please follow the same spelling when contributing new code, messages, or docs.
