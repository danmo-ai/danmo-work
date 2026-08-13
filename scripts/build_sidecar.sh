#!/usr/bin/env bash
# Build Go backend as Tauri sidecar binary
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$ROOT_DIR/desktop/src-tauri/bin"

mkdir -p "$BIN_DIR"

# Detect target triple
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS-$ARCH" in
  Darwin-arm64)
    TRIPLE="aarch64-apple-darwin"
    GOOS=darwin GOARCH=arm64
    ;;
  Darwin-x86_64)
    TRIPLE="x86_64-apple-darwin"
    GOOS=darwin GOARCH=amd64
    ;;
  Linux-x86_64)
    TRIPLE="x86_64-unknown-linux-gnu"
    GOOS=linux GOARCH=amd64
    ;;
  MINGW*-x86_64|MSYS*-x86_64|CYGWIN*-x86_64)
    TRIPLE="x86_64-pc-windows-msvc"
    GOOS=windows GOARCH=amd64
    ;;
  *)
    echo "Unsupported platform: $OS-$ARCH" >&2
    exit 1
    ;;
esac

BINARY_NAME="danmo-work-backend-$TRIPLE"
# Go on Windows always produces .exe; Tauri externalBin expects it
if [[ "$GOOS" == "windows" ]]; then
  BINARY_NAME="${BINARY_NAME}.exe"
fi

# Align sidecar /api/v1/version with desktop release when RELEASE_VERSION is set
VERSION_LDFLAGS="-w -s"
if [[ -n "${RELEASE_VERSION:-}" ]]; then
  VERSION_LDFLAGS="-w -s -X 'danmo-work/server/api/v1.Version=${RELEASE_VERSION}'"
  echo "==> Sidecar version: $RELEASE_VERSION"
fi

echo "==> Building sidecar: $BINARY_NAME ($GOOS/$GOARCH)"

cd "$ROOT_DIR"
GOOS=$GOOS GOARCH=$GOARCH go build -trimpath -ldflags "$VERSION_LDFLAGS" -o "$BIN_DIR/$BINARY_NAME" ./server

if [[ "$GOOS" != "windows" ]]; then
  chmod +x "$BIN_DIR/$BINARY_NAME"
fi
echo "==> Sidecar binary: $BIN_DIR/$BINARY_NAME ($(du -h "$BIN_DIR/$BINARY_NAME" | cut -f1))"
