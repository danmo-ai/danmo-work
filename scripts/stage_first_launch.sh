#!/usr/bin/env bash
# Stage the platform-specific first-launch script into DEST.
# Usage: stage_first_launch.sh <darwin|linux|windows> [DEST_DIR]
# Default DEST: ~/.danmo-work/first_launch
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

PLATFORM="${1:-}"
case "$PLATFORM" in
  darwin|linux|windows) ;;
  *)
    echo "Usage: $0 <darwin|linux|windows> [DEST_DIR]" >&2
    exit 1
    ;;
esac

DEST="${2:-$HOME/.danmo-work/first_launch}"
mkdir -p "$DEST"

SRC_DIR="$SCRIPT_DIR/first_launch/$PLATFORM"
if [[ "$PLATFORM" == "windows" ]]; then
  SRC="$SRC_DIR/post-install.ps1"
  DST_NAME="post-install.ps1"
else
  SRC="$SRC_DIR/post-install.sh"
  DST_NAME="post-install.sh"
fi

if [[ ! -f "$SRC" ]]; then
  echo "missing first-launch script: $SRC" >&2
  exit 1
fi

# Clear the other platform's script so a shared resources/ dir never ships both.
rm -f "$DEST/post-install.sh" "$DEST/post-install.ps1"
cp -f "$SRC" "$DEST/$DST_NAME"
if [[ "$PLATFORM" != "windows" ]]; then
  chmod +x "$DEST/$DST_NAME"
fi
# Bust / track which platform was staged (consumed by the Go runner stamp).
printf '%s\n' "$PLATFORM" >"$DEST/PLATFORM"
echo "==> Staged first-launch script: $DEST/$DST_NAME ($PLATFORM)"
