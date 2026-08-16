#!/usr/bin/env bash
# Fetch the official ripgrep static binary (BurntSushi/ripgrep) into
# out/rg/<target>/rg and ~/.danmo-work/bin/rg.
# Used by pack_* scripts so release packages bundle a fast grep engine
# without requiring a system rg install.
#
# Usage: fetch_ripgrep.sh [STAGE_DIR]
#   STAGE_DIR: optional extra destination (e.g. desktop/src-tauri/resources/rg)
#
# Overrides:
#   RG_VERSION   ripgrep release tag (default 15.2.0)
#   RG_TARGET    rustc target triple (auto-detected from host unless set,
#                e.g. aarch64-apple-darwin) — set explicitly when staging
#                a foreign platform from CI.
#   RG_DEST_DIR  where to stage the binary (default out/rg/<target>)
#   RG_BASE_URL  alternate download base
#   RG_NO_HOME_BIN=1  skip the ~/.danmo-work/bin copy
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

VERSION="${RG_VERSION:-15.2.0}"
VERSION="${VERSION#v}"

detect_target() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$os" in
    darwin) os="apple-darwin" ;;
    linux) os="unknown-linux-musl" ;;
    mingw*|msys*|cygwin*) os="pc-windows-msvc" ;;
    *)
      echo "Unsupported OS: $os" >&2
      exit 1
      ;;
  esac
  case "$arch" in
    x86_64|amd64) arch="x86_64" ;;
    arm64|aarch64) arch="aarch64" ;;
    *)
      echo "Unsupported arch: $arch" >&2
      exit 1
      ;;
  esac
  TARGET="${arch}-${os}"
}

if [[ -n "${RG_TARGET:-}" ]]; then
  TARGET="$RG_TARGET"
else
  detect_target
fi

case "$TARGET" in
  *windows*)
    EXT="zip"
    BIN_NAME="rg.exe"
    ;;
  *)
    EXT="tar.gz"
    BIN_NAME="rg"
    ;;
esac

DQ_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DEST_DIR="${RG_DEST_DIR:-$DQ_ROOT/out/rg/$TARGET}"
mkdir -p "$DEST_DIR"
DEST_BIN="$DEST_DIR/$BIN_NAME"
VERSION_FILE="$DEST_DIR/VERSION"

if [[ -f "$DEST_BIN" && "${RG_FORCE:-}" != "1" ]]; then
  SIZE="$(wc -c < "$DEST_BIN" | tr -d ' ')"
  if [[ "${SIZE:-0}" -ge 1000000 ]]; then
    printf '%s\n' "$VERSION" >"$VERSION_FILE"
    echo "==> ripgrep already present: $DEST_BIN (v${VERSION}, ${SIZE} bytes)"
    stage_home "${1:-}"
    exit 0
  fi
  echo "WARNING: existing $DEST_BIN looks truncated (${SIZE:-0} bytes); re-fetching" >&2
fi

BASE_URL="${RG_BASE_URL:-https://github.com/BurntSushi/ripgrep/releases/download}"
ASSET="ripgrep-${VERSION}-${TARGET}.${EXT}"
URL="${BASE_URL}/${VERSION}/${ASSET}"
TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

ARCHIVE="$TMP/$ASSET"
echo "==> Downloading ripgrep v${VERSION} (${TARGET})"
echo "    $URL"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$ARCHIVE" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$ARCHIVE" "$URL"
else
  echo "curl or wget required to fetch ripgrep" >&2
  exit 1
fi

if [[ "$EXT" == "zip" ]]; then
  mkdir -p "$TMP/out"
  unzip -qo "$ARCHIVE" -d "$TMP/out"
else
  mkdir -p "$TMP/out"
  tar -xzf "$ARCHIVE" -C "$TMP/out"
fi
FOUND="$(find "$TMP/out" -type f \( -name 'rg' -o -name 'rg.exe' \) | head -n 1 || true)"
if [[ -z "$FOUND" ]]; then
  echo "rg binary not found in archive" >&2
  exit 1
fi

cp -f "$FOUND" "$DEST_BIN"
chmod +x "$DEST_BIN" 2>/dev/null || true
printf '%s\n' "$VERSION" >"$VERSION_FILE"
echo "==> Wrote $DEST_BIN (v${VERSION})"

stage_home() {
  if [[ "${RG_NO_HOME_BIN:-}" != "1" ]]; then
    HOME_BIN="${HOME:-.}/.danmo-work/bin"
    mkdir -p "$HOME_BIN"
    cp -f "$DEST_BIN" "$HOME_BIN/$BIN_NAME"
    chmod +x "$HOME_BIN/$BIN_NAME" 2>/dev/null || true
    echo "==> Copied to $HOME_BIN/$BIN_NAME"
  fi
  if [[ -n "${1:-}" ]]; then
    mkdir -p "$1"
    cp -f "$DEST_BIN" "$1/$BIN_NAME"
    chmod +x "$1/$BIN_NAME" 2>/dev/null || true
    echo "==> Staged into $1/$BIN_NAME"
  fi
}
stage_home "${1:-}"
