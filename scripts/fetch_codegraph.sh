#!/usr/bin/env bash
# Fetch Colby CodeGraph CLI into ~/.danmo-work/bin (or a custom DEST).
# Used by desktop/dev packs so the builtin codegraph expert has a local binary.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

VERSION="${CODEGRAPH_VERSION:-1.5.0}"
# Strip leading v if present
VERSION="${VERSION#v}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$OS" in
  linux) CG_OS="linux" ;;
  darwin) CG_OS="darwin" ;;
  mingw*|msys*|cygwin*) CG_OS="win32" ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac
case "$ARCH" in
  x86_64|amd64) CG_ARCH="x64" ;;
  arm64|aarch64) CG_ARCH="arm64" ;;
  *)
    echo "Unsupported arch: $ARCH" >&2
    exit 1
    ;;
esac

DEST_DIR="${1:-$HOME/.danmo-work/bin}"
mkdir -p "$DEST_DIR"
if [[ "$CG_OS" == "win32" ]]; then
  DEST_BIN="$DEST_DIR/codegraph.exe"
  ASSET="codegraph-${CG_OS}-${CG_ARCH}.zip"
else
  DEST_BIN="$DEST_DIR/codegraph"
  ASSET="codegraph-${CG_OS}-${CG_ARCH}.tar.gz"
fi

if [[ -f "$DEST_BIN" && "${CODEGRAPH_FORCE:-}" != "1" ]]; then
  echo "==> CodeGraph already present: $DEST_BIN"
  printf '%s\n' "$VERSION" >"$DEST_DIR/CODEGRAPH_VERSION"
  exit 0
fi

BASE_URL="${CODEGRAPH_BASE_URL:-https://github.com/colbymchenry/codegraph/releases/download/v${VERSION}}"
URL="${BASE_URL}/${ASSET}"
TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

ARCHIVE="$TMP/$ASSET"
echo "==> Downloading CodeGraph v${VERSION} (${CG_OS}/${CG_ARCH})"
echo "    $URL"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$ARCHIVE" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$ARCHIVE" "$URL"
else
  echo "curl or wget required to fetch CodeGraph" >&2
  exit 1
fi

echo "==> Extracting"
if [[ "$ASSET" == *.zip ]]; then
  unzip -qo "$ARCHIVE" -d "$TMP/out"
else
  mkdir -p "$TMP/out"
  tar -xzf "$ARCHIVE" -C "$TMP/out"
fi

FOUND="$(find "$TMP/out" -type f \( -name 'codegraph' -o -name 'codegraph.exe' \) | head -n 1 || true)"
if [[ -z "$FOUND" ]]; then
  echo "codegraph binary not found in archive" >&2
  exit 1
fi
cp -f "$FOUND" "$DEST_BIN"
chmod +x "$DEST_BIN" 2>/dev/null || true
printf '%s\n' "$VERSION" >"$DEST_DIR/CODEGRAPH_VERSION"
echo "==> Wrote $DEST_BIN (CodeGraph v${VERSION}, MIT — https://github.com/colbymchenry/codegraph)"
