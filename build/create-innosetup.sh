#!/bin/bash

# Creates a Windows installer using Inno Setup via Docker
# Builds a complete .exe installer with all dependencies and file associations

set -euo pipefail

readonly PACKAGE_NAME="hashverifier"
readonly PACKAGE_ARCH="${1:-amd64}"

readonly BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly BUILD_DIR="${BASE_DIR}/build"
readonly DIST_DIR="${BASE_DIR}/dist/windows-${PACKAGE_ARCH}"
readonly WORK_DIR="${BASE_DIR}/.pkg-build"
readonly OUT_DIR="${WORK_DIR}/package"

readonly SOURCE_BINARY="${DIST_DIR}/${PACKAGE_NAME}.exe"
readonly SOURCE_ISS="${BUILD_DIR}/${PACKAGE_NAME}.iss"
readonly SOURCE_ICON="${BASE_DIR}/src/internal/gui/widgets/glade/favicon.ico"
readonly SOURCE_FILETYPE_ICON="${BUILD_DIR}/hashverifier-filetype.ico"
readonly SOURCE_LICENSE="${BASE_DIR}/LICENSE"
readonly SOURCE_THIRD_PARTY="${BASE_DIR}/THIRD_PARTY_NOTICES"

readonly ISS_ARCH="${PACKAGE_ARCH}"
readonly ISS_VERSION="${VERSION#v}"
readonly ISS_OUTPUT_DIR="${WORK_DIR}/dist/innosetup/${PACKAGE_ARCH}"
readonly ISS_PACKAGE_NAME="${PACKAGE_NAME}-${ISS_VERSION}-windows-${PACKAGE_ARCH}.exe"

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

    if ! command -v docker &> /dev/null; then
        log_error "Docker not found. Install Docker to build Inno Setup installers."
        exit 1
    fi
}

validate_source_files() {
    log_info "Validating source files..."

    local missing_files=()

    if [[ ! -f "${SOURCE_BINARY}" ]]; then
        missing_files+=("${SOURCE_BINARY}")
    fi

    if [[ ! -f "${SOURCE_ISS}" ]]; then
        missing_files+=("${SOURCE_ISS}")
    fi

    if [[ ! -f "${SOURCE_ICON}" ]]; then
        missing_files+=("${SOURCE_ICON}")
    fi

    if [[ ! -f "${SOURCE_FILETYPE_ICON}" ]]; then
        missing_files+=("${SOURCE_FILETYPE_ICON}")
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

    # Check that dist directory has DLL files (GTK3 runtime)
    local dll_count
    dll_count=$(find "${DIST_DIR}" -maxdepth 1 -name "*.dll" | wc -l)
    if [[ "${dll_count}" -eq 0 ]]; then
        log_error "No DLL files found in ${DIST_DIR}. Build Windows binary first."
        exit 1
    fi

    log_info "Found ${dll_count} DLL files in distribution directory"
}

cleanup() {
    local exit_code=$?
    if [[ ${exit_code} -ne 0 ]]; then
        log_error "Error occurred. Temporary files preserved in: ${WORK_DIR}"
    fi
    return ${exit_code}
}

print_version() {
    log_info "Processing version: ${ISS_VERSION} (${PACKAGE_ARCH})"
}

prepare_directories() {
    log_info "Creating directory structure..."

    rm -rf "${ISS_OUTPUT_DIR}"

    mkdir -p "${ISS_OUTPUT_DIR}"
    mkdir -p "${OUT_DIR}"

    # Hotfix: The output file appears to be in use (5).
    chmod 777 "${ISS_OUTPUT_DIR}"
}

copy_files_to_staging() {
    log_info "Copying files to staging directory..."

    cp -r "${DIST_DIR}/." "${ISS_OUTPUT_DIR}/"

    cp "${SOURCE_ISS}" "${ISS_OUTPUT_DIR}/"

    cp "${SOURCE_ICON}" "${ISS_OUTPUT_DIR}/"
    cp "${SOURCE_FILETYPE_ICON}" "${ISS_OUTPUT_DIR}/"

    # Create combined license file for Inno Setup (LICENSE + THIRD_PARTY_NOTICES)
    log_info "Creating combined license file..."
    {
        cat "${SOURCE_LICENSE}"
        echo ""
        echo "============================================"
        cat "${SOURCE_THIRD_PARTY}"
    } | sed -e '/./,$!d' -e :a -e '/^\n*$/{$d;N;ba' -e '}' > "${ISS_OUTPUT_DIR}/hashverifier-license.txt"
}

build_installer() {
    log_info "Building Inno Setup installer..."

    log_info "Running Inno Setup compiler..."
    log_info "  Architecture: ${ISS_ARCH}"
    log_info "  Version: ${ISS_VERSION}"
    log_info "  Source: ${ISS_OUTPUT_DIR}"

    # Run Inno Setup compiler via Docker
    # The amake/innosetup image uses iscc as ENTRYPOINT
    # Mount the staging directory as /work (the container's working directory)
    # All files referenced by the .iss script must be under /work
    docker run --rm \
        -v "${ISS_OUTPUT_DIR}:/work" \
        -w /work \
        amake/innosetup:innosetup6 \
        "/DAppVersion=${ISS_VERSION}" \
        "/DAppArch=${ISS_ARCH}" \
        "/O/work" \
        "${PACKAGE_NAME}.iss"

    mv "${ISS_OUTPUT_DIR}/${ISS_PACKAGE_NAME}" "${OUT_DIR}/"
    log_success "Installer created: ${OUT_DIR}/${ISS_PACKAGE_NAME}"
}

show_package_info() {
    local installer_file="${OUT_DIR}/${ISS_PACKAGE_NAME}"

    log_stage "Package Information"

    log_info "Installer file: ${installer_file}"
    log_info "File size: $(du -h "${installer_file}" | cut -f1)"
    file "${installer_file}"
}

main() {
    trap cleanup EXIT

    log_stage "Initialization"

    print_version
    validate_prerequisites
    validate_source_files

    log_stage "Preparation"

    prepare_directories
    copy_files_to_staging

    log_stage "Installer Build"

    build_installer
    show_package_info

    log_stage "Complete"

    log_success "Windows installer created: ${OUT_DIR}/${ISS_PACKAGE_NAME}"
}

main "$@"
