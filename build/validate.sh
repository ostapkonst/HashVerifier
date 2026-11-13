#!/usr/bin/env bash

# Shared input validation for the packaging scripts.

validate_version() {
    local version="${1:-}"

    if [[ ! "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "ERROR: VERSION must be X.Y.Z, got '${version}'" >&2
        return 1
    fi
}

validate_arch() {
    local arch="${1:-}"

    shift

    for allowed in "$@"; do
        if [[ "${arch}" == "${allowed}" ]]; then
            return 0
        fi
    done

    echo "ERROR: unsupported architecture '${arch}' (expected: $*)" >&2
    return 1
}

# An empty arch means the host architecture, which is also the only one the
# GTK-OSX toolchain can build natively.
validate_macos_arch() {
    local arch="${1:-}" host

    case "$(uname -m)" in
        arm64)       host="arm64" ;;
        x86_64|i386) host="amd64" ;;
        *)
            echo "ERROR: unsupported host architecture: $(uname -m)" >&2
            return 1
            ;;
    esac

    if [[ -z "${arch}" ]]; then
        arch="${host}"
    fi

    validate_arch "${arch}" arm64 amd64 || return 1

    if [[ "${arch}" != "${host}" ]]; then
        echo "ERROR: ${arch} bundle requires an ${host} macOS runner." >&2
        return 1
    fi
}
