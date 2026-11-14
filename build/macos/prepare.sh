#!/usr/bin/env bash

# Builds the GTK-OSX dependency silo used by the macOS MVP bundle.
# The script must run on macOS; GTK-OSX compiles native libraries and cannot be
# cross-built from Linux.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# shellcheck source=versions.env
source "${ROOT}/build/macos/versions.env"
# shellcheck source=../validate.sh
source "${ROOT}/build/validate.sh"

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "ERROR: macOS bundle preparation must run on macOS." >&2
    exit 1
fi

ARCH=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --arch)
            if [[ $# -lt 2 ]]; then
                echo "Usage: $0 [--arch arm64|amd64]" >&2
                exit 2
            fi

            ARCH="$2"
            shift 2
            ;;
        *)
            echo "Usage: $0 [--arch arm64|amd64]" >&2
            exit 2
            ;;
    esac
done

validate_macos_arch "${ARCH}" || exit 2

missing_tools=()
for tool in git python3; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
        missing_tools+=("${tool}")
    fi
done
if [[ ${#missing_tools[@]} -gt 0 ]]; then
    echo "ERROR: missing required tools:" >&2
    for tool in "${missing_tools[@]}"; do
        echo "  - ${tool}" >&2
    done
    exit 1
fi

TOOLS_DIR="${ROOT}/.pkg-build/macos/tools"
CONFIG_DIR="${ROOT}/.pkg-build/macos/config"
GTK_OSX_DIR="${TOOLS_DIR}/gtk-osx-build"
JHBUILDRC_CUSTOM="${CONFIG_DIR}/jhbuildrc-custom"
PREFIX="${GTK_OSX_PREFIX:-${HOME}/gtk/inst}"

mkdir -p "${TOOLS_DIR}" "${CONFIG_DIR}"

fetch_pinned() {
    local repo="$1"
    local dir="$2"
    local commit="$3"

    if [[ ! -d "${dir}/.git" ]]; then
        git clone "${repo}" "${dir}"
    fi

    git -C "${dir}" fetch --depth 1 origin "${commit}" ||
        git -C "${dir}" fetch origin
    git -C "${dir}" checkout --force "${commit}"
}

TOOLCHAIN_ID="${GTK_OSX_BUILD_COMMIT}|${JHBUILD_COMMIT}|${MACOS_DEPLOYMENT_TARGET}|${GTK_OSX_MODULES}|${PYTHON_VERSION}"
STAMP="${PREFIX}/.hashverifier-toolchain"

if [[ -f "${PREFIX}/lib/pkgconfig/gtk+-3.0.pc" && "${FORCE_GTK_OSX_REBUILD:-0}" != "1" ]]; then
    if [[ -f "${STAMP}" && "$(cat "${STAMP}")" == "${TOOLCHAIN_ID}" ]]; then
        echo "GTK-OSX prefix already present: ${PREFIX}"
        exit 0
    fi

    if [[ -n "${CI:-}" && "${PREFIX}" == "${HOME}/gtk/inst" ]]; then
        echo "Removing stale GTK-OSX prefix in CI: ${PREFIX}"
        rm -rf "${HOME}/gtk"
    else
        echo "ERROR: GTK-OSX prefix exists but its toolchain stamp is missing or stale: ${PREFIX}" >&2
        echo "Remove ${PREFIX} or set FORCE_GTK_OSX_REBUILD=1 to rebuild." >&2
        exit 1
    fi
fi

if ! perl -MXML::Parser -e 1 >/dev/null 2>&1; then
    echo "ERROR: Perl module XML::Parser is required by intltool." >&2
    echo "Install it for the active Perl: brew install perl-xml-parser" >&2
    exit 1
fi

fetch_pinned "${GTK_OSX_BUILD_REPO}" "${GTK_OSX_DIR}" "${GTK_OSX_BUILD_COMMIT}"

cat >"${JHBUILDRC_CUSTOM}" <<EOF
prefix = "${PREFIX}"
moduleset = "${GTK_OSX_DIR}/modulesets-stable/gtk-osx.modules"
modulesets_dir = "${GTK_OSX_DIR}/modulesets-stable"
modules = ["${GTK_OSX_MODULES}"]
use_local_modulesets = True
setup_sdk(target="${MACOS_DEPLOYMENT_TARGET}")
EOF

export XDG_CONFIG_HOME="${CONFIG_DIR}"
export JHBUILDRC_CUSTOM
export PYTHON_VERSION
export DEVROOT="${DEVROOT:-${HOME}}"
export DEVPREFIX="${DEVPREFIX:-${DEVROOT}/.new_local}"
export DEV_SRC_ROOT="${DEV_SRC_ROOT:-${DEVROOT}/Source}"
export PYTHONUSERBASE="${PYTHONUSERBASE:-${DEVPREFIX}}"
export PYENV_ROOT="${PYENV_ROOT:-${DEVPREFIX}/share/pyenv}"
export PYENV_INSTALL_ROOT="${PYENV_INSTALL_ROOT:-${DEV_SRC_ROOT}/pyenv}"
export PIP_CONFIG_DIR="${PIP_CONFIG_DIR:-${HOME}/.config/pip}"

echo "Running gtk-osx-setup.sh..."
"${GTK_OSX_DIR}/gtk-osx-setup.sh"

# gtk-osx-setup.sh downloads its base rc from GitLab master. Replace it with
# the configuration from the pinned checkout so the moduleset selection and
# deployment target stay reproducible.
mkdir -p "${XDG_CONFIG_HOME}"
cp "${GTK_OSX_DIR}/jhbuildrc-gtk-osx" "${XDG_CONFIG_HOME}/jhbuildrc"

# gtk-osx-setup.sh clones jhbuild at master. Pin the checkout used by the
# generated jhbuild wrapper before building anything.
if [[ -d "${DEV_SRC_ROOT}/jhbuild/.git" ]]; then
    git -C "${DEV_SRC_ROOT}/jhbuild" fetch --depth 1 origin "${JHBUILD_COMMIT}" ||
        git -C "${DEV_SRC_ROOT}/jhbuild" fetch origin
    git -C "${DEV_SRC_ROOT}/jhbuild" checkout --force "${JHBUILD_COMMIT}"
fi

export PATH="${DEVPREFIX}/bin:${PATH}"

# gtk-osx-setup.sh skips "cargo install cargo-c" when rustup is found outside
# DEVPREFIX (e.g. preinstalled on CI runners), so the cargo-cbuild required by
# librsvg's meson would be missing. Install it with whatever cargo is on hand.
if ! command -v cargo-cbuild >/dev/null 2>&1; then
    CARGO_BIN="${DEVPREFIX}/bin/cargo"
    if [[ ! -x "${CARGO_BIN}" ]]; then
        CARGO_BIN="$(command -v cargo || true)"
    fi
    if [[ -z "${CARGO_BIN}" ]]; then
        echo "ERROR: cargo not found; cannot install cargo-c (required by librsvg)." >&2
        exit 1
    fi
    echo "Installing cargo-c (required by librsvg)..."
    "${CARGO_BIN}" install cargo-c --features=vendored-openssl
fi

echo "Bootstrapping jhbuild..."
"${DEVPREFIX}/bin/jhbuild" bootstrap-gtk-osx

echo "Building ${GTK_OSX_MODULES}..."
"${DEVPREFIX}/bin/jhbuild" build "${GTK_OSX_MODULES}"

mkdir -p "${PREFIX}"
printf '%s\n' "${TOOLCHAIN_ID}" >"${STAMP}"

echo "GTK-OSX prefix: ${PREFIX}"
