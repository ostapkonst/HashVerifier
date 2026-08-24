.DEFAULT_GOAL := help

.PHONY: build run clean help
.PHONY: linux-amd64 linux-arm64 windows-amd64 windows-i686 deb-amd64 deb-arm64 rpm-amd64 rpm-arm64 appimage-amd64 appimage-arm64
.PHONY: innosetup-amd64 innosetup-i686
.PHONY: flatpak flatpak-run flatpak-validate
.PHONY: lint lint-install lint-fix format
.PHONY: third-party-notices reset-config

GO_LICENSES_VERSION   := v2.0.1
GOLANGCI_LINT_VERSION := v2.12.2
VERSION               ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")

build:
	cd src && \
	go build -trimpath -ldflags="-s -w -X 'github.com/ostapkonst/HashVerifier/internal/appmeta.Version=$(VERSION)'" -o ../hashverifier .

run:
	cd src && \
	go run .

linux-amd64:
	@echo "Building for Linux/amd64..."
	@mkdir -p dist/linux-amd64
	VERSION=$(VERSION) docker compose -f build/docker-compose.dist.yml run --rm --build linux-amd64
	@echo "✓ Linux/amd64 binary: dist/linux-amd64/hashverifier"

linux-arm64:
	@echo "Building for Linux/arm64 (aarch64)..."
	@mkdir -p dist/linux-arm64
	VERSION=$(VERSION) docker compose -f build/docker-compose.dist.yml run --rm --build linux-arm64
	@echo "✓ Linux/arm64 binary: dist/linux-arm64/hashverifier"

windows-amd64:
	@echo "Building for Windows/amd64..."
	@mkdir -p dist/windows-amd64
	VERSION=$(VERSION) docker compose -f build/docker-compose.dist.yml run --rm --build windows-amd64
	@echo "✓ Windows/amd64 binary: dist/windows-amd64/hashverifier.exe"

windows-i686:
	@echo "Building for Windows/i686..."
	@mkdir -p dist/windows-i686
	VERSION=$(VERSION) docker compose -f build/docker-compose.dist.yml run --rm --build windows-i686
	@echo "✓ Windows/i686 binary: dist/windows-i686/hashverifier.exe"

appimage-amd64:
	@echo "Building AppImage package for AMD64..."
	@mkdir -p .pkg-build/dist/appimage/amd64 .pkg-build/package dist/linux-amd64
	VERSION=$(VERSION) docker compose -f build/docker-compose.appimage.yml run --rm appimage-builder /app/build/create-appimage.sh
	@echo "✓ AppImage package (amd64): .pkg-build/package/*.AppImage"

appimage-arm64:
	@echo "Building AppImage package for ARM64..."
	@mkdir -p .pkg-build/dist/appimage/arm64 .pkg-build/package dist/linux-arm64
	VERSION=$(VERSION) docker compose -f build/docker-compose.appimage.yml run --rm appimage-builder-arm64 /app/build/create-appimage.sh
	@echo "✓ AppImage package (arm64): .pkg-build/package/*.AppImage"

flatpak: flatpak-validate
	@echo "Building Flatpak package..."
	@mkdir -p .pkg-build/flatpak/build-dir
	@flatpak install --user -y flathub org.gnome.Sdk//50 org.freedesktop.Sdk.Extension.golang//25.08
	@cd flatpak && flatpak-builder --user --force-clean ../.pkg-build/flatpak/build-dir io.github.ostapkonst.HashVerifier.yml
	@echo "✓ Flatpak package: .pkg-build/flatpak/build-dir"

flatpak-run:
	@echo "Running HashVerifier Flatpak..."
	@mkdir -p .pkg-build/flatpak/repo
	@flatpak build-export .pkg-build/flatpak/repo .pkg-build/flatpak/build-dir
	@flatpak remote-delete --user --force local-repo > /dev/null 2>&1 || true
	@flatpak remote-add --user --if-not-exists --no-gpg-verify local-repo .pkg-build/flatpak/repo
	@flatpak install --user --reinstall -y local-repo io.github.ostapkonst.HashVerifier
	@flatpak run --user io.github.ostapkonst.HashVerifier

flatpak-validate:
	@echo "Validating Flatpak manifest and metainfo files..."
	@echo ""
	@echo "Checking io.github.ostapkonst.HashVerifier.yml syntax..."
	@if command -v python3 >/dev/null 2>&1; then \
		python3 -c "import yaml; yaml.safe_load(open('flatpak/io.github.ostapkonst.HashVerifier.yml'))" && echo "  ✓ YAML syntax is valid" || echo "  ✗ YAML syntax error"; \
	else \
		echo "  Note: Python3 not available, skipping YAML syntax check"; \
	fi
	@echo ""
	@echo "Validating io.github.ostapkonst.HashVerifier.metainfo.xml..."
	@if command -v python3 >/dev/null 2>&1; then \
		python3 -c "import xml.etree.ElementTree as ET; ET.parse('flatpak/io.github.ostapkonst.HashVerifier.metainfo.xml')" && echo "  ✓ XML syntax is valid" || echo "  ✗ XML syntax error"; \
	else \
		echo "  Note: Python3 not available, skipping XML syntax check"; \
	fi
	@echo ""
	@echo "✓ Flatpak validation complete"

deb-amd64:
	@echo "Building DEB package for AMD64..."
	@mkdir -p .pkg-build/dist/deb/amd64 .pkg-build/package dist/linux-amd64
	VERSION=$(VERSION) docker compose -f build/docker-compose.packages.yml run --rm package-builder /app/build/create-deb.sh
	@echo "✓ DEB package (amd64): .pkg-build/package/*.deb"

rpm-amd64:
	@echo "Building RPM package for AMD64..."
	@mkdir -p .pkg-build/dist/rpm/x86_64 .pkg-build/package dist/linux-amd64
	VERSION=$(VERSION) docker compose -f build/docker-compose.packages.yml run --rm package-builder /app/build/create-rpm.sh
	@echo "✓ RPM package (amd64): .pkg-build/package/*.rpm"

deb-arm64:
	@echo "Building DEB package for ARM64..."
	@mkdir -p .pkg-build/dist/deb/arm64 .pkg-build/package dist/linux-arm64
	VERSION=$(VERSION) docker compose -f build/docker-compose.packages.yml run --rm deb-arm64 /app/build/create-deb.sh
	@echo "✓ DEB package (arm64): .pkg-build/package/*.deb"

rpm-arm64:
	@echo "Building RPM package for ARM64..."
	@mkdir -p .pkg-build/dist/rpm/aarch64 .pkg-build/package dist/linux-arm64
	VERSION=$(VERSION) docker compose -f build/docker-compose.packages.yml run --rm rpm-arm64 /app/build/create-rpm.sh
	@echo "✓ RPM package (arm64): .pkg-build/package/*.rpm"

innosetup-amd64:
	@echo "Building Windows installer for AMD64..."
	@mkdir -p .pkg-build/dist/innosetup/amd64 .pkg-build/package dist/windows-amd64
	VERSION=$(VERSION) bash build/create-innosetup.sh amd64
	@echo "✓ Windows installer (amd64): .pkg-build/package/*.exe"

innosetup-i686:
	@echo "Building Windows installer for i686..."
	@mkdir -p .pkg-build/dist/innosetup/i686 .pkg-build/package dist/windows-i686
	VERSION=$(VERSION) bash build/create-innosetup.sh i686
	@echo "✓ Windows installer (i686): .pkg-build/package/*.exe"

clean:
	@if [ -d dist ]; then \
		echo "Removing dist/ (may require sudo)..."; \
		sudo rm -rf dist/; \
	fi
	@if [ -d .pkg-build ]; then \
		echo "Removing .pkg-build/ (may require sudo)..."; \
		sudo rm -rf .pkg-build/; \
	fi
	@rm -f hashverifier hashverifier.exe
	@echo "✓ Cleaned build artifacts"

reset-config:
	@echo "Resetting user settings to defaults..."
	cd src && go run . config reset -y
	@echo "✓ User settings reset to defaults"

lint-install:
	@if [ -f .bin/golangci-lint ]; then \
		echo "✓ golangci-lint already installed in .bin/"; \
	else \
		echo "Installing golangci-lint..."; \
		mkdir -p .bin; \
		curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b .bin $(GOLANGCI_LINT_VERSION); \
		echo "✓ golangci-lint installed to .bin/"; \
	fi

lint: lint-install
	cd src && ../.bin/golangci-lint run --config ../.golangci.yml

lint-fix: lint-install
	cd src && ../.bin/golangci-lint run --fix --config ../.golangci.yml

format: lint-install
	cd src && ../.bin/golangci-lint fmt --config ../.golangci.yml

third-party-notices:
	@if [ -f .bin/go-licenses ]; then \
		echo "✓ go-licenses already installed in .bin/"; \
	else \
		echo "Installing go-licenses..."; \
		mkdir -p .bin; \
		GOBIN=$$(pwd)/.bin go install github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION); \
		echo "✓ go-licenses installed to .bin/"; \
	fi
	@echo "Generating THIRD_PARTY_NOTICES..."
	cd build && go run generate-notices.go
	@echo "✓ THIRD_PARTY_NOTICES generated"

help:
	@echo "HashVerifier Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Native Build Targets:"
	@echo "  build                Build for current platform (native)"
	@echo "  run                  Build and run the application"
	@echo ""
	@echo "Binary Distribution Targets:"
	@echo "  linux-amd64          Build Linux/amd64 binary using Docker"
	@echo "  linux-arm64          Build Linux/arm64 binary using Docker"
	@echo "  windows-amd64        Build Windows/amd64 binary using Docker"
	@echo "  windows-i686         Build Windows/i686 binary using Docker"
	@echo ""
	@echo "Package Distribution Targets:"
	@echo "  deb-amd64            Build DEB package (Debian/Ubuntu) for amd64"
	@echo "  deb-arm64            Build DEB package (Debian/Ubuntu) for arm64"
	@echo "  rpm-amd64            Build RPM package (Fedora/RHEL) for x86_64"
	@echo "  rpm-arm64            Build RPM package (Fedora/RHEL) for aarch64"
	@echo "  appimage-amd64       Build AppImage package (universal Linux) for amd64"
	@echo "  appimage-arm64       Build AppImage package (universal Linux) for arm64"
	@echo "  innosetup-amd64      Build Windows installer (Inno Setup) for amd64"
	@echo "  innosetup-i686       Build Windows installer (Inno Setup) for i686"
	@echo ""
	@echo "Flatpak Targets:"
	@echo "  flatpak              Build Flatpak package (requires flatpak-builder)"
	@echo "  flatpak-run          Build and run Flatpak package"
	@echo "  flatpak-validate     Validate Flatpak manifest and metainfo files"
	@echo ""
	@echo "Maintenance Targets:"
	@echo "  clean                Remove build artifacts"
	@echo "  reset-config         Reset user settings to defaults"
	@echo "  lint-install         Install golangci-lint ($(GOLANGCI_LINT_VERSION)) into .bin/"
	@echo "  lint                 Run golangci-lint from .bin/"
	@echo "  lint-fix             Run golangci-lint with auto-fix"
	@echo "  format               Format code with golangci-lint"
	@echo "  third-party-notices  Generate THIRD_PARTY_NOTICES"
	@echo "  help                 Show this help message"
