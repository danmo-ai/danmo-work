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

# Tauri names from productName ("Danmo Work_…"); rename to Danmo.Work_* for
# URL-safe Release assets (matches macOS DMG / Windows setup naming).
VERSION="${RELEASE_VERSION:-$(git -C "$DQ_ROOT" describe --tags --always --dirty 2>/dev/null || echo 0.0.0)}"
VERSION="${VERSION#v}"
SAFE_PREFIX="Danmo.Work_${VERSION}_amd64"

APPIMAGE="$(find "$DQ_DESKTOP_BUNDLE" -type f -name '*.AppImage' ! -name '*.sig' | head -1 || true)"
DEB="$(find "$DQ_DESKTOP_BUNDLE" -type f -name '*.deb' | head -1 || true)"
if [[ -z "$APPIMAGE" || -z "$DEB" ]]; then
  echo "ERROR: expected both AppImage and .deb under $DQ_DESKTOP_BUNDLE" >&2
  find "$DQ_DESKTOP_BUNDLE" -type f 2>/dev/null || true
  exit 1
fi

APPIMAGE_DIR="$(dirname "$APPIMAGE")"
DEB_DIR="$(dirname "$DEB")"
FINAL_APPIMAGE="$APPIMAGE_DIR/${SAFE_PREFIX}.AppImage"
FINAL_DEB="$DEB_DIR/${SAFE_PREFIX}.deb"
mv -f "$APPIMAGE" "$FINAL_APPIMAGE"
mv -f "$DEB" "$FINAL_DEB"

# Keep updater signature next to the renamed AppImage when present.
for sig in "$APPIMAGE.sig" "${APPIMAGE}.sig"; do
  if [[ -f "$sig" ]]; then
    mv -f "$sig" "${FINAL_APPIMAGE}.sig"
    break
  fi
done
for sig in "$APPIMAGE.tar.gz.sig" "${APPIMAGE}.tar.gz.sig"; do
  if [[ -f "$sig" ]]; then
    # Rare v1-compat path; keep beside any .AppImage.tar.gz if tauri emitted one
    TAR_GZ="$(find "$APPIMAGE_DIR" -maxdepth 1 -type f -name '*.AppImage.tar.gz' | head -1 || true)"
    if [[ -n "$TAR_GZ" ]]; then
      FINAL_TAR="$APPIMAGE_DIR/${SAFE_PREFIX}.AppImage.tar.gz"
      mv -f "$TAR_GZ" "$FINAL_TAR"
      mv -f "$sig" "${FINAL_TAR}.sig"
    fi
    break
  fi
done

echo "==> Desktop bundle -> $DQ_DESKTOP_BUNDLE"
echo "    AppImage: $FINAL_APPIMAGE"
echo "    deb:      $FINAL_DEB"
if has_tauri_signing_key; then
  if [[ -f "${FINAL_APPIMAGE}.sig" ]]; then
    echo "    updater:  ${FINAL_APPIMAGE}.sig"
  else
    echo "WARNING: no *.AppImage.sig — updater latest.json will skip linux-x86_64" >&2
  fi
fi
