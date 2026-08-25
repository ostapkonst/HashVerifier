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
│       │   ├── cli/         # Cobra command tree
│       │   │   ├── base/       # ExitError, ReportError, RunWithShutdown, flag/loading helpers
│       │   │   ├── generate/   # generate subcommand
│       │   │   ├── verify/     # verify subcommand
│       │   │   ├── hash/       # hash subcommand
│       │   │   └── config/     # config (show | edit | reset) subcommand
│       │   └── gui/         # GTK3 GUI
│       │       ├── app/         # Lifecycle + window geometry + tab manager
│       │       ├── base/        # TabBase, ProgressTracker, SetStatLabel, ErrTabBusy
│       │       ├── widgets/     # Reusable GTK widgets + dialogs + ColumnConfig
│       │       ├── generate/    # GenerateTab
│       │       ├── verify/      # VerifyTab
│       │       └── hash/        # HashTab
│       ├── driver/yamlconfig/   # YAML settings persistence + validation + display
│       └── platform/     # Cross-platform infrastructure (editor, eol, env, errs, fs, shutdown, flatpak, reveal)
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

The tree **never expands past 5 levels of nesting from the repo root** (`HashVerifier/` is level 0).

## Architecture Layers

The codebase follows a layered architecture:

- **`domain/`** — Pure types and algorithms. No I/O, no framework dependencies. Safe to unit-test.
- **`service/`** — Use-case orchestration. Wires domain types into workflows (generate / verify / hash).
- **`driver/`** — Concrete I/O implementations (YAML persistence lives here).
- **`platform/`** — OS and runtime infrastructure (filesystem, environment, signals, Flatpak, editor).
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

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Commenting Style

Comments are expected to explain the **why** behind non-obvious decisions, not restate what the code already says. Anything that just adds noise — section markers, English translations of code, multi-paragraph godoc — is best left out.

All comments are written in English (see [Notes](#notes) for the spelling convention). Tool directives such as `//nolint:*`, `//go:embed`, `// #cgo`, `// #include`, and C macros are kept verbatim since the tooling depends on them.

## Notes

All user-facing strings, identifiers, and documentation in this project use **American English** spelling (e.g., `canceled`, not `cancelled`; `color`, not `colour`). This convention matches the Go standard library (e.g., `context.Canceled`) and keeps the codebase internally consistent. Please follow the same spelling when contributing new code, messages, or docs.
