#!/usr/bin/env bash

# Structural verification for the macOS MVP bundle. Runs on macOS and catches
# missing resources or accidental links to build-host libraries before the DMG
# is uploaded for manual testing.

set -euo pipefail

APP=""
DMG=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --app)
            APP="${2:-}"
            shift 2
            ;;
        --dmg)
            DMG="${2:-}"
            shift 2
            ;;
        *)
            echo "Usage: $0 --app <HashVerifier.app> [--dmg <file.dmg>]" >&2
            exit 2
            ;;
    esac
done

if [[ -z "${APP}" || ! -d "${APP}" ]]; then
    echo "ERROR: app bundle not found: ${APP}" >&2
    exit 1
fi

BIN="${APP}/Contents/MacOS/HashVerifier"
PLIST="${APP}/Contents/Info.plist"
RESOURCES="${APP}/Contents/Resources"

for required in \
    "${BIN}" \
    "${PLIST}" \
    "${RESOURCES}/HashVerifier.icns" \
    "${RESOURCES}/hashverifier-filetype.icns" \
    "${RESOURCES}/share/glib-2.0/schemas/gschemas.compiled" \
    "${RESOURCES}/share/icons/Adwaita"; do
    if [[ ! -e "${required}" ]]; then
        echo "ERROR: missing bundle entry: ${required}" >&2
        exit 1
    fi
done

if ! grep -q 'CFBundleDocumentTypes' "${PLIST}" \
    || ! grep -q 'UTExportedTypeDeclarations' "${PLIST}" \
    || ! grep -q 'hashverifier-filetype' "${PLIST}"; then
    echo "ERROR: Info.plist document type declarations are incomplete." >&2
    exit 1
fi

if ! find "${RESOURCES}/lib/gdk-pixbuf-2.0" -name loaders.cache -print -quit | grep -q .; then
    echo "ERROR: gdk-pixbuf loaders.cache is missing." >&2
    exit 1
fi

plutil -lint "${PLIST}"

external_links="$(mktemp "${TMPDIR:-/tmp}/hashverifier-otool.XXXXXX")"
trap 'rm -f "${external_links}"' EXIT

while IFS= read -r -d '' file; do
    otool -L "${file}" | tail -n +2 >>"${external_links}"
done < <(find "${APP}/Contents" -type f \( -perm -111 -o -name '*.dylib' -o -name '*.so' \) -print0)

if grep -Eq '(/opt/homebrew|/usr/local|/Users/|/private/tmp/|/tmp/hashverifier|/gtk/inst)' "${external_links}"; then
    echo "ERROR: bundle references build-host libraries:" >&2
    grep -E '(/opt/homebrew|/usr/local|/Users/|/private/tmp/|/tmp/hashverifier|/gtk/inst)' "${external_links}" >&2
    exit 1
fi

codesign --verify --deep --strict "${APP}"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/hashverifier-cli.XXXXXX")"
trap 'rm -rf "${tmpdir}"; rm -f "${external_links}"' EXIT
printf 'hashverifier-cli check' >"${tmpdir}/sample.bin"

"${BIN}" --no-config hash "${tmpdir}/sample.bin" --algorithms .md5 >/dev/null 2>&1
"${BIN}" generate --help >/dev/null 2>&1

if [[ -n "${DMG}" ]]; then
    if [[ ! -f "${DMG}" ]]; then
        echo "ERROR: DMG not found: ${DMG}" >&2
        exit 1
    fi

    hdiutil verify "${DMG}" >/dev/null
fi

echo "Bundle verification passed: ${APP}"
