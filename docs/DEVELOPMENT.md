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
│   ├── cmd/              # CLI subcommand wiring (cobra factories)
│   ├── internal/         # Application internals
│   │   ├── envutil/       # Env-var parser (truthy values for ephemeral mode)
│   │   ├── header/        # Checksum-file header constants
│   │   ├── action/        # generate / verify / hash services
│   │   ├── checksum/      # algorithms and parsers
│   │   ├── gui/           # GTK3 graphical interface
│   │   └── settings/      # YAML settings persistence
│   ├── utils/            # Shared utilities (graceful shutdown, eof, error unwrap)
│   ├── main.go           # Application entry point
│   └── go.mod            # Go module manifest
├── .dockerignore         # Docker build context exclusions
├── .gitattributes        # Git attributes (line endings, binary files)
├── .gitignore            # Git ignore rules
├── .golangci.yml         # Go linter configuration
├── LICENSE               # MIT License
├── Makefile              # Build automation
├── README.md             # Main documentation
└── THIRD_PARTY_NOTICES   # Third-party software notices
```

## Technologies

- **Language:** Go 1.25
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra)
- **GUI Toolkit:** [gotk3](https://github.com/gotk3/gotk3) (GTK3 bindings)
- **Logging:** [zerolog](https://github.com/rs/zerolog)
- **Cryptography:** [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto), [blake3](https://github.com/lukechampine/blake3)

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.
