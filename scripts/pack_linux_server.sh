#!/usr/bin/env bash
# Linux server release tar.gz — preserves out/server + out/frontend/dist layout
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

APP_NAME="${DQ_APP_NAME:-danmo-work}"
VERSION="${RELEASE_VERSION:-$(git -C "$DQ_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
ARCH="$(uname -m)"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

dq_ensure_out_layout

for bin in "$APP_NAME" "$APP_NAME-cli" "$APP_NAME-tui"; do
  if [[ ! -f "$DQ_SERVER_DIR/$bin" ]]; then
    echo "Missing server binary: $DQ_SERVER_DIR/$bin (run make build-go)" >&2
    exit 1
  fi
done

if [[ ! -f "$DQ_FRONTEND_DIST/index.html" ]]; then
  echo "Missing frontend build: $DQ_FRONTEND_DIST (run make frontend-build)" >&2
  exit 1
fi

STAGE="$DQ_RELEASE_DIST/.stage-${APP_NAME}-${OS}-${ARCH}"
rm -rf "$STAGE"
mkdir -p "$STAGE/out/server" "$STAGE/out/frontend/dist"
cp "$DQ_SERVER_DIR/$APP_NAME" "$STAGE/out/server/"
cp "$DQ_SERVER_DIR/$APP_NAME-cli" "$STAGE/out/server/"
cp "$DQ_SERVER_DIR/$APP_NAME-tui" "$STAGE/out/server/"
cp -R "$DQ_FRONTEND_DIST/." "$STAGE/out/frontend/dist/"

# OCI env image tar is NOT packed here — published as a separate GitHub Release
# asset for optional user download (see make build-env-tar / release job env artifact).

ARCHIVE="$DQ_RELEASE_DIST/${APP_NAME}-${OS}-${ARCH}-${VERSION}.tar.gz"
tar -czf "$ARCHIVE" -C "$STAGE" .
rm -rf "$STAGE"
echo "==> Server archive -> $ARCHIVE"
