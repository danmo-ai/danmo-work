#!/usr/bin/env bash
# Linux Tauri desktop build — AppImage + .deb (x86_64)
# Cargo -> out/desktop/cargo, bundles -> out/desktop/bundle
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

APP_NAME="${DQ_APP_NAME:-danmo-work}"
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-$DQ_DESKTOP_CARGO}"
dq_ensure_out_layout

if [[ "$(uname -s)" != Linux ]]; then
  echo "pack-linux-desktop must run on Linux" >&2
  exit 1
fi

ARCH="$(uname -m)"
if [[ "$ARCH" != "x86_64" ]]; then
  echo "pack-linux-desktop currently supports x86_64 only (got $ARCH)" >&2
  exit 1
fi

cd "$DQ_ROOT/desktop"
if [[ ! -d node_modules ]]; then
  npm install
fi

# Desktop app needs to know the backend API URL
export VITE_API_BASE_URL="http://127.0.0.1:${DQ_BACKEND_PORT:-7801}"

if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" && -z "${TAURI_SIGNING_PRIVATE_KEY_PATH:-}" && -f "$DQ_ROOT/desktop/src-tauri/keys/updater.key" ]]; then
  export TAURI_SIGNING_PRIVATE_KEY_PATH="$DQ_ROOT/desktop/src-tauri/keys/updater.key"
  export TAURI_SIGNING_PRIVATE_KEY_PASSWORD="${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"
fi

# tauri build bundler only reads TAURI_SIGNING_PRIVATE_KEY (not PATH).
if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" && -n "${TAURI_SIGNING_PRIVATE_KEY_PATH:-}" && -f "${TAURI_SIGNING_PRIVATE_KEY_PATH}" ]]; then
  TAURI_SIGNING_PRIVATE_KEY="$(tr -d '\r\n' < "$TAURI_SIGNING_PRIVATE_KEY_PATH")"
  export TAURI_SIGNING_PRIVATE_KEY
fi
unset TAURI_SIGNING_PRIVATE_KEY_PATH

has_tauri_signing_key() {
  [[ -n "${TAURI_SIGNING_PRIVATE_KEY:-}" ]]
}

# Build Go backend as Tauri sidecar binary
echo "==> Building backend sidecar..."
"$SCRIPT_DIR/build_sidecar.sh"

# Ensure only the target-tripled sidecar exists in bin/ to avoid duplicates in the bundle
BIN_DIR="$DQ_ROOT/desktop/src-tauri/bin"
rm -f "$BIN_DIR"/danmo-work-backend
rm -f "$BIN_DIR"/danmo-work-backend.exe

echo "==> Tauri build ($APP_NAME) -> $CARGO_TARGET_DIR"
# AppImage (portable + updater) and .deb (apt-style install)
if has_tauri_signing_key; then
  npm run tauri build -- -b appimage -b deb
else
  echo "WARNING: no Tauri signing key — building without updater artifacts"
  npm run tauri build -- -b appimage -b deb --config '{"bundle":{"createUpdaterArtifacts":false}}'
fi

BUNDLE_SRC=""
for candidate in \
  "$CARGO_TARGET_DIR/release/bundle" \
  "$CARGO_TARGET_DIR/x86_64-unknown-linux-gnu/release/bundle" \
  "$DQ_ROOT/desktop/src-tauri/target/release/bundle"; do
  if [[ -d "$candidate" ]]; then
    BUNDLE_SRC="$candidate"
    break
  fi
done

if [[ -z "$BUNDLE_SRC" ]]; then
  echo "Tauri bundle not found under $CARGO_TARGET_DIR" >&2
  exit 1
fi

rm -rf "$DQ_DESKTOP_BUNDLE"/*
mkdir -p "$DQ_DESKTOP_BUNDLE"
cp -R "$BUNDLE_SRC"/* "$DQ_DESKTOP_BUNDLE/"

APPIMAGE="$(find "$DQ_DESKTOP_BUNDLE" -type f -name '*.AppImage' | head -1 || true)"
DEB="$(find "$DQ_DESKTOP_BUNDLE" -type f -name '*.deb' | head -1 || true)"
if [[ -z "$APPIMAGE" || -z "$DEB" ]]; then
  echo "ERROR: expected both AppImage and .deb under $DQ_DESKTOP_BUNDLE" >&2
  find "$DQ_DESKTOP_BUNDLE" -type f 2>/dev/null || true
  exit 1
fi

echo "==> Desktop bundle -> $DQ_DESKTOP_BUNDLE"
echo "    AppImage: $APPIMAGE"
echo "    deb:      $DEB"
if has_tauri_signing_key; then
  SIG="$(find "$DQ_DESKTOP_BUNDLE" -type f -name '*.AppImage.sig' | head -1 || true)"
  if [[ -n "$SIG" ]]; then
    echo "    updater:  $SIG"
  else
    echo "WARNING: no *.AppImage.sig — updater latest.json will skip linux-x86_64" >&2
  fi
fi
