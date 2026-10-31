#!/usr/bin/env bash

# Builds the GTK-OSX dependency silo used by the macOS MVP bundle.
# The script must run on macOS; GTK-OSX compiles native libraries and cannot be
# cross-built from Linux.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# shellcheck source=versions.env
source "${ROOT}/build/macos/versions.env"

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "ERROR: macOS bundle preparation must run on macOS." >&2
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

if [[ -f "${PREFIX}/lib/pkgconfig/gtk+-3.0.pc" && "${FORCE_GTK_OSX_REBUILD:-0}" != "1" ]]; then
    echo "GTK-OSX prefix already present: ${PREFIX}"
    exit 0
fi

fetch_pinned "${GTK_OSX_BUILD_REPO}" "${GTK_OSX_DIR}" "${GTK_OSX_BUILD_COMMIT}"

cat >"${JHBUILDRC_CUSTOM}" <<EOF
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

echo "Bootstrapping jhbuild..."
"${DEVPREFIX}/bin/jhbuild" bootstrap-gtk-osx

echo "Building ${GTK_OSX_MODULES}..."
"${DEVPREFIX}/bin/jhbuild" build "${GTK_OSX_MODULES}"

echo "GTK-OSX prefix: ${PREFIX}"
