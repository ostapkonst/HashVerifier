#!/bin/bash

# Creates a DEB package for Debian/Ubuntu systems
# Builds a complete DEB package with all dependencies and desktop integration

set -euo pipefail

readonly PACKAGE_NAME="hashverifier"
readonly PACKAGE_SECTION="utils"
readonly PACKAGE_PRIORITY="optional"
readonly PACKAGE_ARCH="${PACKAGE_ARCH:-amd64}"
readonly PACKAGE_MAINTAINER="ostapkonst"
readonly PACKAGE_DESCRIPTION="Cross-platform checksum generation and verification tool"
readonly PACKAGE_HOMEPAGE="https://github.com/ostapkonst/HashVerifier"

readonly PACKAGE_DEPENDS=(
    "libgtk-3-0 (>= 3.24)"
    "libc6 (>= 2.31)"
)

DEB_VERSION="${VERSION#v}"  # Без этого: version number does not start with digit
readonly DEB_VERSION="${DEB_VERSION//-/\~}"

readonly BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly BUILD_DIR="${BASE_DIR}/build"
readonly DIST_DIR="${BASE_DIR}/dist/linux-${PACKAGE_ARCH}"
readonly WORK_DIR="${BASE_DIR}/.pkg-build"
readonly OUT_DIR="${WORK_DIR}/package"

readonly SOURCE_BINARY="${DIST_DIR}/${PACKAGE_NAME}"
readonly SOURCE_DESKTOP="${BUILD_DIR}/${PACKAGE_NAME}.desktop"
readonly SOURCE_MIME="${BUILD_DIR}/${PACKAGE_NAME}-mime.xml"
readonly SOURCE_ICON="${BUILD_DIR}/${PACKAGE_NAME}.svg"
readonly SOURCE_FILETYPE_ICON="${BUILD_DIR}/${PACKAGE_NAME}-filetype.svg"
readonly SOURCE_LICENSE="${BASE_DIR}/LICENSE"
readonly SOURCE_THIRD_PARTY="${BASE_DIR}/THIRD_PARTY_NOTICES"

readonly DEB_ROOT="${WORK_DIR}/dist/deb/${PACKAGE_ARCH}"
readonly DEB_DIR="${DEB_ROOT}/DEBIAN"
readonly DEB_BIN_DIR="${DEB_ROOT}/usr/bin"
readonly DEB_DESKTOP_DIR="${DEB_ROOT}/usr/share/applications"
readonly DEB_MIME_DIR="${DEB_ROOT}/usr/share/mime/packages"
readonly DEB_DOC_DIR="${DEB_ROOT}/usr/share/doc/${PACKAGE_NAME}"
readonly DEB_ICON_DIR="${DEB_ROOT}/usr/share/icons/hicolor/scalable/apps"
readonly DEB_MIMETYPE_ICON_DIR="${DEB_ROOT}/usr/share/icons/hicolor/scalable/mimetypes"

readonly DEB_PACKAGE_NAME="${PACKAGE_NAME}_${DEB_VERSION}_${PACKAGE_ARCH}.deb"

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

    if ! command -v dpkg-deb &> /dev/null; then
        log_error "dpkg-deb not found. Install dpkg-dev."
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

    if [[ ! -f "${SOURCE_MIME}" ]]; then
        missing_files+=("${SOURCE_MIME}")
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

print_version() {
    log_info "Processing version: ${DEB_VERSION}"
}

prepare_directories() {
    log_info "Creating directory structure..."

    rm -rf "${DEB_ROOT}"

    mkdir -p "${DEB_DIR}"
    mkdir -p "${DEB_BIN_DIR}"
    mkdir -p "${DEB_DESKTOP_DIR}"
    mkdir -p "${DEB_MIME_DIR}"
    mkdir -p "${DEB_DOC_DIR}"
    mkdir -p "${DEB_ICON_DIR}"
    mkdir -p "${DEB_MIMETYPE_ICON_DIR}"
    mkdir -p "${OUT_DIR}"
}

copy_files() {
    log_info "Copying package files..."

    cp "${SOURCE_BINARY}" "${DEB_BIN_DIR}/"
    chmod +x "${DEB_BIN_DIR}/${PACKAGE_NAME}"

    cp "${SOURCE_DESKTOP}" "${DEB_DESKTOP_DIR}/"

    cp "${SOURCE_MIME}" "${DEB_MIME_DIR}/${PACKAGE_NAME}-mime.xml"

    cp "${SOURCE_LICENSE}" "${DEB_DOC_DIR}/LICENSE"
    cp "${SOURCE_THIRD_PARTY}" "${DEB_DOC_DIR}/THIRD_PARTY_NOTICES"

    cp "${SOURCE_ICON}" "${DEB_ICON_DIR}/hashverifier.svg"

    cp "${SOURCE_FILETYPE_ICON}" "${DEB_MIMETYPE_ICON_DIR}/application-x-hashverifier-filetype.svg"
}

calculate_installed_size() {
    log_info "Calculating installed size..."

    local total_size
    total_size=$(du -sk "${DEB_ROOT}" | cut -f1)

    INSTALLED_SIZE="${total_size}"

    log_info "Installed-Size: ${INSTALLED_SIZE} KB"
}

create_control_file() {
    log_info "Creating control file..."

    local dependencies
    dependencies=$(IFS=', '; echo "${PACKAGE_DEPENDS[*]}")

    # Use calculated installed size, or estimate if not available
    local installed_size="${INSTALLED_SIZE:-0}"

    cat > "${DEB_DIR}/control" << EOF
Package: ${PACKAGE_NAME}
Version: ${DEB_VERSION}
Section: ${PACKAGE_SECTION}
Priority: ${PACKAGE_PRIORITY}
Architecture: ${PACKAGE_ARCH}
Depends: ${dependencies}
Installed-Size: ${installed_size}
Maintainer: ${PACKAGE_MAINTAINER}
Description: ${PACKAGE_DESCRIPTION}
 HashVerifier is a cross-platform checksum generation and verification tool
 with both CLI and GTK3 graphical interface.
 .
 Features:
  - Checksum generation for entire directories
  - File verification against checksum files
  - Support for 13 hash algorithms (CRC32, MD4, MD5, SHA1, SHA256, SHA384, SHA512,
    SHA3-256, SHA3-384, SHA3-512, BLAKE3, XXH3, XXH128)
  - CLI and GUI interfaces
  - File associations for all checksum formats
Homepage: ${PACKAGE_HOMEPAGE}
EOF
}

create_postinst_script() {
    log_info "Creating postinst script..."

    cat > "${DEB_DIR}/postinst" << 'EOF'
#!/bin/bash
set -e

update-desktop-database /usr/share/applications 2>/dev/null || true
update-mime-database /usr/share/mime 2>/dev/null || true
gtk-update-icon-cache -f /usr/share/icons/hicolor 2>/dev/null || true
EOF

    chmod +x "${DEB_DIR}/postinst"
}

create_postrm_script() {
    log_info "Creating postrm script..."

    cat > "${DEB_DIR}/postrm" << 'EOF'
#!/bin/bash
set -e

update-desktop-database /usr/share/applications 2>/dev/null || true
update-mime-database /usr/share/mime 2>/dev/null || true
gtk-update-icon-cache -f /usr/share/icons/hicolor 2>/dev/null || true
EOF

    chmod +x "${DEB_DIR}/postrm"
}

build_package() {
    log_info "Building DEB package..."

    cd "${WORK_DIR}/dist/deb"

    dpkg-deb --build "${PACKAGE_ARCH}" 2>&1 | { grep -v "^dpkg-deb:" || true; }

    if [[ ! -f "${PACKAGE_ARCH}.deb" ]]; then
        log_error "DEB package was not created"
        exit 1
    fi

    mv "${PACKAGE_ARCH}.deb" "${OUT_DIR}/${DEB_PACKAGE_NAME}"
}

show_package_info() {
    local deb_file="${OUT_DIR}/${DEB_PACKAGE_NAME}"

    log_stage "Package Information"

    log_info "DEB package details:"
    dpkg-deb -I "${deb_file}"

    echo ""
    log_info "DEB package files:"
    dpkg-deb -c "${deb_file}"
}

main() {
    trap cleanup EXIT

    log_stage "Initialization"

    print_version
    validate_prerequisites
    validate_source_files

    log_stage "Preparation"

    prepare_directories
    copy_files
    calculate_installed_size

    log_stage "Package Build"

    create_control_file
    create_postinst_script
    create_postrm_script
    build_package
    show_package_info

    log_stage "Complete"

    log_success "DEB package created: ${OUT_DIR}/${DEB_PACKAGE_NAME}"
}

main "$@"
