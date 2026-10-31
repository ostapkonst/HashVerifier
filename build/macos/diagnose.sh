#!/usr/bin/env bash

# Collects the information needed to diagnose a macOS bundle failure.
# Run it on the Mac where the DMG was tested.

set -u

APP="${1:-/Applications/HashVerifier.app}"
BIN="${APP}/Contents/MacOS/HashVerifier"

echo "=== System ==="
sw_vers
uname -m
echo

echo "=== App ==="
echo "Path: ${APP}"
ls -ld "${APP}" 2>&1 || true
plutil -p "${APP}/Contents/Info.plist" 2>&1 || true
echo

echo "=== Code signature ==="
codesign -dv --verbose=4 "${APP}" 2>&1 || true
codesign --verify --deep --strict "${APP}" 2>&1 || true
echo

echo "=== Dynamic links ==="
if [[ -x "${BIN}" ]]; then
    otool -L "${BIN}" 2>&1 || true
else
    echo "Executable not found: ${BIN}"
fi
echo

echo "=== Resources ==="
find "${APP}/Contents/Resources" -maxdepth 4 -type f 2>&1 | head -100 || true
echo

echo "=== Recent application log ==="
log show --last 10m --style compact --predicate 'process == "HashVerifier"' 2>&1 | tail -200 || true
