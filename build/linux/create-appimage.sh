#!/bin/bash

# Creates an AppImage for universal Linux distribution
# Builds a portable AppImage with all dependencies included

set -euo pipefail

readonly PACKAGE_NAME="hashverifier"
readonly PACKAGE_ARCH="${PACKAGE_ARCH:-x86_64}"

readonly BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly BUILD_DIR="${BASE_DIR}/build"

case "${PACKAGE_ARCH}" in
    x86_64)  BUILD_ARCH="amd64" ;;
    aarch64) BUILD_ARCH="arm64" ;;
    *)       BUILD_ARCH="${PACKAGE_ARCH}" ;;
esac

readonly DIST_DIR="${BASE_DIR}/dist/linux-${BUILD_ARCH}"
readonly WORK_DIR="${BASE_DIR}/.pkg-build"
readonly OUT_DIR="${WORK_DIR}/package"

APPIMAGE_VERSION="${VERSION#v}"
readonly APPIMAGE_VERSION="${APPIMAGE_VERSION//-/\~}"

readonly SOURCE_BINARY="${DIST_DIR}/${PACKAGE_NAME}"
readonly SOURCE_DESKTOP="${BUILD_DIR}/linux/${PACKAGE_NAME}.desktop"
readonly SOURCE_ICON="${BUILD_DIR}/linux/${PACKAGE_NAME}.svg"
readonly SOURCE_LICENSE="${BASE_DIR}/LICENSE"
readonly SOURCE_THIRD_PARTY="${BASE_DIR}/THIRD_PARTY_NOTICES"

readonly APPDIR_ROOT="${WORK_DIR}/dist/appimage/${BUILD_ARCH}"
readonly APPDIR_BIN="${APPDIR_ROOT}/usr/bin"
readonly APPDIR_DESKTOP="${APPDIR_ROOT}/usr/share/applications"
readonly APPDIR_DOC="${APPDIR_ROOT}/usr/share/doc/${PACKAGE_NAME}"
readonly APPDIR_ICON_DIR="${APPDIR_ROOT}/usr/share/icons/hicolor/scalable/apps"

readonly DESKTOP_NAME="HashVerifier"

readonly ACTUAL_APPIMAGE_NAME="${DESKTOP_NAME}-${APPIMAGE_VERSION}-${PACKAGE_ARCH}.AppImage"
readonly APPIMAGE_OUTPUT="${OUT_DIR}/${ACTUAL_APPIMAGE_NAME}"

log_info() {
    echo "[INFO] $*"
}

log_stage() {
    echo ""
    echo "========================================"
    echo "  STAGE: $*"
    echo "========================================"
}

log_error() {
    echo "[ERROR] $*" >&2
}

log_success() {
    echo "[SUCCESS] $*"
}

log_warn() {
    echo "[WARNING] $*"
}

validate_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v linuxdeploy &> /dev/null; then
        log_error "linuxdeploy not found. Install linuxdeploy."
        exit 1
    fi
}

validate_source_files() {
    log_info "Validating source files..."

    local missing_files=()

    if [[ ! -f "${SOURCE_BINARY}" ]]; then
        missing_files+=("${SOURCE_BINARY}")
    fi

    if [[ ! -f "${SOURCE_DESKTOP}" ]]; then
        missing_files+=("${SOURCE_DESKTOP}")
    fi

    if [[ ! -f "${SOURCE_LICENSE}" ]]; then
        missing_files+=("${SOURCE_LICENSE}")
    fi

    if [[ ! -f "${SOURCE_THIRD_PARTY}" ]]; then
        missing_files+=("${SOURCE_THIRD_PARTY}")
    fi

    if [[ ${#missing_files[@]} -gt 0 ]]; then
        log_error "Missing required files:"
        for file in "${missing_files[@]}"; do
            log_error "  - ${file}"
        done
        exit 1
    fi

    if [[ -f "${SOURCE_BINARY}" && ! -x "${SOURCE_BINARY}" ]]; then
        log_warn "Binary is not executable, fixing permissions..."
        chmod +x "${SOURCE_BINARY}"
    fi
}

cleanup() {
    local exit_code=$?
    if [[ ${exit_code} -ne 0 ]]; then
        log_error "Error occurred. Temporary files preserved in: ${WORK_DIR}"
    fi
    return ${exit_code}
}

prepare_appdir() {
    log_info "Creating AppDir structure..."

    rm -rf "${APPDIR_ROOT}"

    mkdir -p "${APPDIR_ROOT}"
    mkdir -p "${APPDIR_BIN}"
    mkdir -p "${APPDIR_DESKTOP}"
    mkdir -p "${APPDIR_DOC}"
    mkdir -p "${APPDIR_ICON_DIR}"
    mkdir -p "${OUT_DIR}"
}

copy_files() {
    log_info "Copying files to AppDir..."

    cp "${SOURCE_BINARY}" "${APPDIR_BIN}/"
    chmod +x "${APPDIR_BIN}/${PACKAGE_NAME}"

    sed 's|Exec=/usr/bin/hashverifier|Exec=hashverifier|' "${SOURCE_DESKTOP}" > "${APPDIR_DESKTOP}/${PACKAGE_NAME}.desktop"
    cp "${APPDIR_DESKTOP}/${PACKAGE_NAME}.desktop" "${APPDIR_ROOT}/"

    cp "${SOURCE_LICENSE}" "${APPDIR_DOC}/LICENSE"
    cp "${SOURCE_THIRD_PARTY}" "${APPDIR_DOC}/THIRD_PARTY_NOTICES"

    cp "${SOURCE_ICON}" "${APPDIR_ICON_DIR}/hashverifier.svg"
}

deploy_dependencies() {
    log_info "Deploying GTK3 dependencies with linuxdeploy-plugin-gtk..."

    export LINUXDEPLOY_OUTPUT_VERSION="${APPIMAGE_VERSION}"

    cd "${OUT_DIR}"

    linuxdeploy \
        --appdir "${APPDIR_ROOT}" \
        --executable "${APPDIR_BIN}/${PACKAGE_NAME}" \
        --desktop-file="${APPDIR_DESKTOP}/${PACKAGE_NAME}.desktop" \
        --icon-file="${APPDIR_ICON_DIR}/hashverifier.svg" \
        --plugin gtk \
        --output appimage
}

verify_appimage() {
    log_info "Verifying AppImage..."

    local created_appimage="${OUT_DIR}/${ACTUAL_APPIMAGE_NAME}"

    if [[ ! -f "${created_appimage}" ]]; then
        log_error "AppImage was not created: ${created_appimage}"
        exit 1
    fi

    chmod +x "${created_appimage}"

    local size
    size=$(du -h "${created_appimage}" | cut -f1)
    log_success "AppImage created: ${created_appimage} (${size})"
}

main() {
    trap cleanup EXIT

    log_stage "Initialization"

    validate_prerequisites
    validate_source_files

    log_stage "Preparation"

    prepare_appdir
    copy_files

    log_stage "Build"

    deploy_dependencies
    verify_appimage

    log_stage "Complete"

    log_success "AppImage created: ${APPIMAGE_OUTPUT}"
}

main "$@"
