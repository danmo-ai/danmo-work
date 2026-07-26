#!/usr/bin/env bash
# Sync release installers + updater artifacts (+ mirror latest.json) to a
# China-reachable object store.
#
# Requires:
#   UPDATE_MIRROR_BASE_URL  e.g. https://releases.danmo.ai/danmo-work
#   And one of:
#     - aws CLI + AWS_* credentials (S3-compatible, including OSS/COS with S3 API)
#     - rclone remote configured via UPDATE_MIRROR_RCLONE_REMOTE
#
# Uploads (flat under the mirror base):
#   - Tauri updater payloads (.app.tar.gz / .nsis.zip / setup.exe + .sig)
#   - macOS .dmg (Homebrew cask)
#   - latest.json (only when updater payloads are present)
#
# Usage:
#   scripts/sync_updater_mirror.sh --asset-dir release-assets --version 0.6.8
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ASSET_DIR="."
VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --asset-dir) ASSET_DIR="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "${UPDATE_MIRROR_BASE_URL:-}" ]]; then
  echo "UPDATE_MIRROR_BASE_URL unset — skip China mirror sync"
  exit 0
fi

if [[ -z "$VERSION" ]]; then
  echo "--version is required" >&2
  exit 1
fi

MIRROR_BASE="${UPDATE_MIRROR_BASE_URL%/}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/upload"

# Updater payloads
find "$ASSET_DIR" \( \
  -name '*.app.tar.gz' -o -name '*.app.tar.gz.sig' -o \
  -name '*.nsis.zip' -o -name '*.nsis.zip.sig' -o \
  -name '*setup.exe' -o -name '*setup.exe.sig' \
\) -exec cp {} "$WORK/upload/" \;

# Desktop / server installers (Homebrew cask uses the arm64 .dmg)
find "$ASSET_DIR" \( \
  -name '*.dmg' -o \
  -name 'danmo-work-linux-*.tar.gz' \
\) -exec cp {} "$WORK/upload/" \;

UPLOAD_COUNT="$(find "$WORK/upload" -type f | wc -l | tr -d ' ')"
if [[ "$UPLOAD_COUNT" -eq 0 ]]; then
  echo "No assets found under $ASSET_DIR — skip mirror sync"
  exit 0
fi

if find "$WORK/upload" \( -name '*.app.tar.gz' -o -name '*.nsis.zip' -o -name '*setup.exe' \) | grep -q .; then
  "$SCRIPT_DIR/generate_updater_latest_json.sh" \
    --version "$VERSION" \
    --base-url "$MIRROR_BASE" \
    --asset-dir "$WORK/upload" \
    --out "$WORK/upload/latest.json" \
    --notes "Danmo Work $VERSION"
else
  echo "No updater artifacts — syncing installers only (leave remote latest.json untouched)"
fi

echo "==> Syncing $UPLOAD_COUNT file(s) to $MIRROR_BASE"
ls -la "$WORK/upload"

if [[ -n "${UPDATE_MIRROR_RCLONE_REMOTE:-}" ]]; then
  rclone copy "$WORK/upload/" "${UPDATE_MIRROR_RCLONE_REMOTE}/" --progress
elif command -v aws >/dev/null 2>&1 && [[ -n "${UPDATE_MIRROR_S3_URI:-}" ]]; then
  aws s3 sync "$WORK/upload/" "${UPDATE_MIRROR_S3_URI%/}/" --acl public-read
else
  echo "ERROR: set UPDATE_MIRROR_RCLONE_REMOTE or UPDATE_MIRROR_S3_URI (+ aws CLI) to sync" >&2
  exit 1
fi

echo "==> Mirror sync complete: $MIRROR_BASE/"
