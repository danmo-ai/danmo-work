#!/usr/bin/env bash
# After Authenticode rewrites the NSIS setup.exe PE, regenerate Tauri updater
# signatures (.sig) so latest.json / auto-update cover the signed bytes.
#
# Usage (from repo root, after SignPath writes the signed installer back):
#   scripts/ci_resign_windows_updater.sh [bundle-dir]
#
# Requires TAURI_SIGNING_PRIVATE_KEY (and optional PASSWORD) in the env —
# same secrets used for the original pack step.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

BUNDLE_DIR="${1:-$DQ_DESKTOP_BUNDLE}"
if [[ ! -d "$BUNDLE_DIR" ]]; then
  echo "ERROR: bundle dir not found: $BUNDLE_DIR" >&2
  exit 1
fi

if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" && -n "${TAURI_SIGNING_PRIVATE_KEY_PATH:-}" && -f "${TAURI_SIGNING_PRIVATE_KEY_PATH}" ]]; then
  TAURI_SIGNING_PRIVATE_KEY="$(tr -d '\r\n' < "$TAURI_SIGNING_PRIVATE_KEY_PATH")"
  export TAURI_SIGNING_PRIVATE_KEY
fi

if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" ]]; then
  echo "WARNING: TAURI_SIGNING_PRIVATE_KEY unset — skip updater resign" >&2
  exit 0
fi

export TAURI_SIGNING_PRIVATE_KEY_PASSWORD="${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"

SETUP_EXE="$(find "$BUNDLE_DIR" -type f -name 'Danmo.Work_*_x64-setup.exe' | head -1 || true)"
if [[ -z "$SETUP_EXE" || ! -f "$SETUP_EXE" ]]; then
  echo "ERROR: no Danmo.Work_*_x64-setup.exe under $BUNDLE_DIR" >&2
  exit 1
fi

cd "$DQ_ROOT/desktop"
if [[ ! -d node_modules ]]; then
  npm install --no-audit --no-fund
fi

echo "==> Re-signing updater payload for Authenticode-touched installer: $SETUP_EXE"
npx tauri signer sign "$SETUP_EXE" -p "${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"

# Tauri auto-update prefers *.nsis.zip. Refresh zip contents from the signed exe
# so the updater downloads the same Authenticode-signed installer.
NSIS_ZIP="$(find "$BUNDLE_DIR" -type f -name '*.nsis.zip' | head -1 || true)"
if [[ -n "$NSIS_ZIP" && -f "$NSIS_ZIP" ]]; then
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  cp "$SETUP_EXE" "$TMP/$(basename "$SETUP_EXE")"
  (
    cd "$TMP"
    # Recreate zip with only the signed setup.exe (Tauri updater expects this shape).
    rm -f "$NSIS_ZIP"
    if command -v zip >/dev/null 2>&1; then
      zip -9 -j "$NSIS_ZIP" "$(basename "$SETUP_EXE")"
    else
      # Windows runners may lack zip; PowerShell Compress-Archive works.
      powershell.exe -NoProfile -Command \
        "Compress-Archive -Path '$(basename "$SETUP_EXE")' -DestinationPath '$NSIS_ZIP' -Force"
    fi
  )
  echo "==> Rebuilt $NSIS_ZIP from signed setup.exe"
  npx tauri signer sign "$NSIS_ZIP" -p "${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}"
fi

echo "==> Updater signatures refreshed after Authenticode"
