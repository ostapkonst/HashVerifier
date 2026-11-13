#!/usr/bin/env bash

# Builds a self-contained HashVerifier.app and DMG against a prepared GTK-OSX
# prefix. Must run on macOS.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# shellcheck source=versions.env
source "${ROOT}/build/macos/versions.env"
# shellcheck source=../validate.sh
source "${ROOT}/build/validate.sh"

ARCH="arm64"
while [[ $# -gt 0 ]]; do
    case "$1" in
        --arch)
            ARCH="${2:-}"
            shift 2
            ;;
        *)
            echo "Usage: $0 [--arch arm64|amd64]" >&2
            exit 2
            ;;
    esac
done

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "ERROR: the macOS bundle must be built on macOS." >&2
    exit 1
fi

case "${ARCH}" in
    arm64) GOARCH="arm64" ;;
    amd64) GOARCH="amd64" ;;
    *)
        echo "ERROR: unsupported arch: ${ARCH}" >&2
        exit 2
        ;;
esac

validate_macos_arch "${ARCH}" || exit 2

PREFIX="${GTK_OSX_PREFIX:-${HOME}/gtk/inst}"
if [[ ! -f "${PREFIX}/lib/pkgconfig/gtk+-3.0.pc" ]]; then
    echo "ERROR: GTK-OSX prefix not found at ${PREFIX}. Run build/macos/prepare.sh first." >&2
    exit 1
fi

if [[ ! -x "${PREFIX}/bin/glib-compile-schemas" ]]; then
    echo "ERROR: glib-compile-schemas not found at ${PREFIX}/bin." >&2
    exit 1
fi

missing_tools=()
for tool in go git python3 sips iconutil codesign hdiutil; do
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

missing_files=()
for file in \
    "${ROOT}/build/macos/hashverifier.png" \
    "${ROOT}/build/macos/hashverifier-filetype.png" \
    "${ROOT}/build/macos/Info.plist" \
    "${ROOT}/build/macos/hashverifier.bundle" \
    "${ROOT}/build/macos/verify.sh" \
    "${ROOT}/LICENSE" \
    "${ROOT}/THIRD_PARTY_NOTICES"; do
    if [[ ! -f "${file}" ]]; then
        missing_files+=("${file}")
    fi
done
if [[ ${#missing_files[@]} -gt 0 ]]; then
    echo "ERROR: missing required files:" >&2
    for file in "${missing_files[@]}"; do
        echo "  - ${file}" >&2
    done
    exit 1
fi

VERSION="${VERSION:-0.0.0}"
if [[ ! "${VERSION}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "ERROR: VERSION must be X.Y.Z: ${VERSION}" >&2
    exit 2
fi
PLIST_VERSION="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
BUILD_NUMBER="${BUILD_NUMBER:-1}"
if [[ ! "${BUILD_NUMBER}" =~ ^[0-9]+(\.[0-9]+){0,2}$ ]]; then
    echo "ERROR: BUILD_NUMBER must be one to three dot-separated integers: ${BUILD_NUMBER}" >&2
    exit 2
fi
BUILD_DIR="${ROOT}/.pkg-build/macos/${ARCH}"
STAGE="${BUILD_DIR}/stage"
APP="${STAGE}/HashVerifier.app"
APP_BINARY="${BUILD_DIR}/hashverifier"
TOOLS_DIR="${ROOT}/.pkg-build/macos/tools"
BUNDLER_DIR="${TOOLS_DIR}/gtk-mac-bundler"
OUT_DIR="${ROOT}/.pkg-build/package"

mkdir -p "${BUILD_DIR}" "${OUT_DIR}"

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

mkdir -p "${TOOLS_DIR}"
fetch_pinned "${GTK_MAC_BUNDLER_REPO}" "${BUNDLER_DIR}" "${GTK_MAC_BUNDLER_COMMIT}"

rm -rf "${STAGE}"
mkdir -p "${STAGE}"

sed -e "s/__VERSION__/${PLIST_VERSION}/g" -e "s/__BUILD__/${BUILD_NUMBER}/g" "${ROOT}/build/macos/Info.plist" >"${BUILD_DIR}/Info.plist"
cp "${ROOT}/build/macos/hashverifier.bundle" "${BUILD_DIR}/hashverifier.bundle"

make_icns() {
    local source="$1"
    local output="$2"
    local tmpdir iconset
    tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/hashverifier.XXXXXX")"
    iconset="${tmpdir}/HashVerifier.iconset"
    mkdir -p "${iconset}"

    for size in 16 32 128 256 512; do
        sips -z "${size}" "${size}" "${source}" --out "${iconset}/icon_${size}x${size}.png" >/dev/null
        sips -z "$((size * 2))" "$((size * 2))" "${source}" --out "${iconset}/icon_${size}x${size}@2x.png" >/dev/null
    done

    iconutil -c icns "${iconset}" -o "${output}"
    rm -rf "${tmpdir}"
}

make_icns "${ROOT}/build/macos/hashverifier.png" "${BUILD_DIR}/HashVerifier.icns"
make_icns "${ROOT}/build/macos/hashverifier-filetype.png" "${BUILD_DIR}/hashverifier-filetype.icns"

echo "Building HashVerifier ${VERSION} for macOS/${ARCH}..."
(
    cd "${ROOT}/src"
    env \
        GOOS=darwin \
        GOARCH="${GOARCH}" \
        CGO_ENABLED=1 \
        CGO_LDFLAGS="-Wl,-headerpad_max_install_names" \
        MACOSX_DEPLOYMENT_TARGET="${MACOS_DEPLOYMENT_TARGET}" \
        PATH="${PREFIX}/bin:${PATH}" \
        PKG_CONFIG_PATH="${PREFIX}/lib/pkgconfig:${PREFIX}/share/pkgconfig" \
        go build \
        -trimpath \
        -ldflags="-s -w -X 'github.com/ostapkonst/HashVerifier/internal/appmeta.Version=v${VERSION}'" \
        -o "${APP_BINARY}" \
        .
)

echo "Bundling with gtk-mac-bundler..."
(
    cd "${BUILD_DIR}"
    env \
        GTK_OSX_PREFIX="${PREFIX}" \
        JHBUILD_PREFIX="${PREFIX}" \
        MACOS_BUNDLE_DEST="${STAGE}" \
        MACOS_APP_BINARY="${APP_BINARY}" \
        PATH="${PREFIX}/bin:${PATH}" \
        PKG_CONFIG_PATH="${PREFIX}/lib/pkgconfig:${PREFIX}/share/pkgconfig" \
        PYTHONPATH="${BUNDLER_DIR}" \
        python3 - "${BUILD_DIR}/hashverifier.bundle" <<'PY'
import sys
from bundler.main import main
main(sys.argv[1:])
PY
)

if [[ ! -d "${APP}" ]]; then
    echo "ERROR: gtk-mac-bundler did not create ${APP}" >&2
    exit 1
fi

cp "${ROOT}/LICENSE" "${APP}/Contents/Resources/LICENSE"
cp "${ROOT}/THIRD_PARTY_NOTICES" "${APP}/Contents/Resources/THIRD_PARTY_NOTICES"

if [[ -d "${APP}/Contents/Resources/share/glib-2.0/schemas" ]]; then
    "${PREFIX}/bin/glib-compile-schemas" "${APP}/Contents/Resources/share/glib-2.0/schemas"
else
    echo "ERROR: GSettings schemas were not copied into the bundle." >&2
    exit 1
fi

echo "Signing bundle (ad-hoc)..."
while IFS= read -r -d '' file; do
    codesign --force --sign - "${file}" >/dev/null
done < <(find "${APP}/Contents" -type f \( -perm -111 -o -name '*.dylib' -o -name '*.so' \) -print0)
codesign --force --sign - "${APP}" >/dev/null

DMG="${OUT_DIR}/hashverifier-${PLIST_VERSION}-macos-${ARCH}.dmg"
ln -sfn /Applications "${STAGE}/Applications"
rm -f "${DMG}"

echo "Creating DMG..."
hdiutil create \
    -volname "HashVerifier ${PLIST_VERSION}" \
    -srcfolder "${STAGE}" \
    -ov \
    -format UDZO \
    "${DMG}" >/dev/null
hdiutil verify "${DMG}" >/dev/null

echo "Verifying bundle..."
bash "${ROOT}/build/macos/verify.sh" --app "${APP}" --dmg "${DMG}"

echo "App: ${APP}"
echo "DMG: ${DMG}"
