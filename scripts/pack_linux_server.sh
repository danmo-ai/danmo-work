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

# Optional bundled OCI env tar (CI: make build-env-tar). Never registry-pulled at runtime.
ENV_TAR=""
for candidate in \
  "${DQ_ENV_DIR}/danmo-work-env-linux-${ARCH}.tar" \
  "${DQ_ENV_DIR}/danmo-work-env.tar" \
  "${DQ_ENV_DIR}/danmo-work-env-linux-amd64.tar" \
  "${DQ_ENV_DIR}/danmo-work-env-linux-arm64.tar"
do
  if [[ -f "$candidate" ]]; then
    ENV_TAR="$candidate"
    break
  fi
done
# uname -m may be x86_64 while file uses amd64
if [[ -z "$ENV_TAR" && "$ARCH" == "x86_64" && -f "${DQ_ENV_DIR}/danmo-work-env-linux-amd64.tar" ]]; then
  ENV_TAR="${DQ_ENV_DIR}/danmo-work-env-linux-amd64.tar"
fi
if [[ -n "$ENV_TAR" ]]; then
  mkdir -p "$STAGE/out/env"
  cp "$ENV_TAR" "$STAGE/out/env/danmo-work-env.tar"
  echo "==> Including env tar: $ENV_TAR"
else
  echo "==> No env tar under out/env (skip); build with: make build-env-tar"
fi

ARCHIVE="$DQ_RELEASE_DIST/${APP_NAME}-${OS}-${ARCH}-${VERSION}.tar.gz"
tar -czf "$ARCHIVE" -C "$STAGE" .
rm -rf "$STAGE"
echo "==> Server archive -> $ARCHIVE"
