#!/usr/bin/env bash
# Windows Tauri desktop build — run on Windows x86_64
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

APP_NAME="${DQ_APP_NAME:-danmo-work}"
export CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-$DQ_DESKTOP_CARGO}"
dq_ensure_out_layout

case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN* | Windows*) ;;
  *)
    echo "pack-windows-desktop must run on Windows" >&2
    exit 1
    ;;
esac

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

# Build Go backend as Tauri sidecar binary (with -w to strip DWARF while keeping Go symbols)
echo "==> Building backend sidecar..."
"$SCRIPT_DIR/build_sidecar.sh"

echo "==> Staging Windows first-launch script..."
"$SCRIPT_DIR/stage_first_launch.sh" windows "$HOME/.danmo-work/first_launch" || true
"$SCRIPT_DIR/stage_first_launch.sh" windows "$DQ_ROOT/desktop/src-tauri/resources/first_launch" || true

# Bundle Microsoft Coreutils for Windows (required — default-installed with the NSIS package)
echo "==> Fetching bundled Coreutils for Windows (required)..."
"$SCRIPT_DIR/fetch_windows_coreutils.sh"
CU_EXE="$DQ_ROOT/desktop/src-tauri/resources/coreutils/coreutils.exe"
if [[ ! -f "$CU_EXE" ]]; then
  echo "ERROR: Windows Coreutils is required for pack-windows-desktop but missing:" >&2
  echo "  $CU_EXE" >&2
  echo "  Re-run: scripts/fetch_windows_coreutils.sh" >&2
  exit 1
fi
CU_SIZE="$(wc -c < "$CU_EXE" | tr -d ' ')"
if [[ "${CU_SIZE:-0}" -lt 1000000 ]]; then
  echo "ERROR: coreutils.exe looks truncated ($CU_SIZE bytes): $CU_EXE" >&2
  exit 1
fi
echo "==> Coreutils OK: $CU_EXE ($(du -h "$CU_EXE" | cut -f1))"

# Bundle ripgrep (grep engine) for Windows
echo "==> Fetching bundled ripgrep..."
"$SCRIPT_DIR/fetch_ripgrep.sh" "$DQ_ROOT/desktop/src-tauri/resources/rg"
RG_EXE="$DQ_ROOT/desktop/src-tauri/resources/rg/rg.exe"
if [[ ! -f "$RG_EXE" ]]; then
  echo "ERROR: ripgrep is required for pack-windows-desktop but missing: $RG_EXE" >&2
  exit 1
fi
echo "==> ripgrep OK: $RG_EXE ($(du -h "$RG_EXE" | cut -f1))"

# Ensure only the target-tripled sidecar exists in bin/ to avoid duplicates in the bundle
BIN_DIR="$DQ_ROOT/desktop/src-tauri/bin"
rm -f "$BIN_DIR"/danmo-work-backend.exe
rm -f "$BIN_DIR"/danmo-work-backend

echo "==> Tauri build ($APP_NAME) -> $CARGO_TARGET_DIR"
if has_tauri_signing_key; then
  npm run tauri build -- -b nsis
else
  echo "WARNING: no Tauri signing key — building without updater artifacts"
  npm run tauri build -- -b nsis --config '{"bundle":{"createUpdaterArtifacts":false}}'
fi

BUNDLE_SRC=""
for candidate in \
  "$CARGO_TARGET_DIR/release/bundle" \
  "$CARGO_TARGET_DIR/x86_64-pc-windows-msvc/release/bundle"; do
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
echo "==> Desktop bundle -> $DQ_DESKTOP_BUNDLE"
