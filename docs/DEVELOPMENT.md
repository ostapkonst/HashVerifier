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
│   └── internal/
│       ├── appmeta/      # App identity (Name/Version/Link) + checksum file header/footer formatters
│       ├── domain/       # ═══ PURE DOMAIN LAYER (no I/O, no frameworks) ═══
│       │   ├── algorithm/   # Algorithm enum + registry + factory
│       │   ├── exclude/     # Path-based exclusion matcher
│       │   ├── hashfn/      # Streaming hash primitives + path validation errors
│       │   ├── parser/      # SFV/*SUMS checksum file parser
│       │   ├── result/      # Status enums, result/stats structs, speed tracker
│       │   └── walk/        # Directory walker + checksum line formatter
│       ├── service/      # ═══ USE-CASE / APPLICATION LOGIC LAYER ═══
│       │   ├── generate/    # Walk + hash + write checksum file
│       │   ├── verify/      # Read checksum file + verify
│       │   └── hash/        # Single-file multi-algo hash
│       ├── adapter/      # ═══ INTERFACE ADAPTERS ═══
│       │   ├── cli/         # Cobra wiring (generate, verify, hash, config subcommands)
│       │   └── gui/         # GTK3 graphical interface
│       │       ├── app/     # Lifecycle + window geometry + tab manager
│       │       ├── tabs/    # Generate / Verify / Hash tabs + shared base
│       │       └── widgets/ # Reusable GTK widgets, dialogs, list-store helpers
│       ├── driver/       # ═══ CONCRETE DRIVERS ═══
│       │   └── yamlconfig/  # YAML-backed settings persistence + validation + display
│       └── platform/     # ═══ INFRASTRUCTURE LAYER ═══
│           ├── platform.go  # Root façade: IsRunningInFlatpak, RevealFile
│           ├── eol/         # Platform-correct line endings
│           ├── env/         # Env-var helpers (HASHVERIFIER_NO_CONFIG)
│           ├── errs/        # Error-chain unwrap + normalization
│           ├── fs/          # Filesystem helpers (overwrite guard)
│           ├── shutdown/    # Signal-based graceful shutdown
│           ├── flatpak/     # Flatpak runtime detection + filesystems
│           └── reveal/      # Cross-platform "show in file manager"
├── .dockerignore         # Docker build context exclusions
├── .gitattributes        # Git attributes (line endings, binary files)
├── .gitignore            # Git ignore rules
├── .golangci.yml         # Go linter configuration
├── LICENSE               # MIT License
├── Makefile              # Build automation
├── README.md             # Main documentation
└── THIRD_PARTY_NOTICES   # Third-party software notices
```

## Architecture Layers

The codebase follows a layered architecture:

- **`domain/`** — Pure types and algorithms. No I/O, no framework dependencies. Safe to unit-test.
- **`service/`** — Use-case orchestration. Wires domain types into workflows (generate / verify / hash).
- **`driver/`** — Concrete I/O implementations (YAML persistence lives here).
- **`platform/`** — OS and runtime infrastructure (filesystem, environment, signals, Flatpak).
- **`adapter/`** — User-facing interfaces. Each adapter is independently swappable:
  - `adapter/cli` is a Cobra command tree.
  - `adapter/gui` is a GTK3 graphical interface.
  - A future `adapter/http` or `adapter/tui` could be added without touching the layers below.
- **`appmeta/`** — Application identity (`Name`, `Version`, `Link`) injected via `-ldflags` plus checksum-file header/footer formatters.

## Technologies

- **Language:** Go 1.25
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **GUI Toolkit:** [gotk3](https://github.com/gotk3/gotk3) (GTK3 bindings)
- **Logging:** [zerolog](https://github.com/rs/zerolog)
- **Cryptography:** [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto), [blake3](https://github.com/lukechampine/blake3)

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## Notes

All user-facing strings, identifiers, and documentation in this project use **American English** spelling (e.g., `canceled`, not `cancelled`; `color`, not `colour`). This convention matches the Go standard library (e.g., `context.Canceled`) and keeps the codebase internally consistent. Please follow the same spelling when contributing new code, messages, or docs.
