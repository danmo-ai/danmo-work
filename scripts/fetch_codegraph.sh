#!/usr/bin/env bash
# Fetch sunerpy/codegraph-rust CLI archive into ~/.danmo-work/bin (or a custom DEST).
# Keeps the compressed release asset (~9–10 MB) — the Go runtime extracts on first use.
# MIT — https://github.com/sunerpy/codegraph-rust
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

VERSION="${CODEGRAPH_VERSION:-0.42.6}"
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

# Map to rustc target triples used by sunerpy release assets.
case "${CG_OS}-${CG_ARCH}" in
  darwin-arm64) TARGET="aarch64-apple-darwin"; EXT="tar.gz" ;;
  darwin-x64) TARGET="x86_64-apple-darwin"; EXT="tar.gz" ;;
  linux-arm64) TARGET="aarch64-unknown-linux-musl"; EXT="tar.gz" ;;
  linux-x64) TARGET="x86_64-unknown-linux-musl"; EXT="tar.gz" ;;
  win32-arm64) TARGET="aarch64-pc-windows-msvc"; EXT="zip" ;;
  win32-x64) TARGET="x86_64-pc-windows-msvc"; EXT="zip" ;;
  *)
    echo "Unsupported platform: ${CG_OS}/${CG_ARCH}" >&2
    exit 1
    ;;
esac

DEST_DIR="${1:-$HOME/.danmo-work/bin}"
mkdir -p "$DEST_DIR"
if [[ "$CG_OS" == "win32" ]]; then
  DEST_BIN="$DEST_DIR/codegraph.exe"
  DEST_ARCHIVE="$DEST_DIR/codegraph.zip"
else
  DEST_BIN="$DEST_DIR/codegraph"
  DEST_ARCHIVE="$DEST_DIR/codegraph.tar.gz"
fi
ASSET="codegraph-${VERSION}-${TARGET}.${EXT}"
VERSION_FILE="$DEST_DIR/CODEGRAPH_VERSION"

need_fetch=0
if [[ "${CODEGRAPH_FORCE:-}" == "1" ]]; then
  need_fetch=1
elif [[ ! -f "$DEST_ARCHIVE" ]]; then
  need_fetch=1
elif [[ ! -f "$VERSION_FILE" ]] || [[ "$(tr -d '[:space:]' <"$VERSION_FILE" 2>/dev/null || true)" != "$VERSION" ]]; then
  need_fetch=1
fi

if [[ "$need_fetch" -eq 0 ]]; then
  BYTES="$(wc -c <"$DEST_ARCHIVE" | tr -d ' ')"
  echo "==> CodeGraph archive already present: $DEST_ARCHIVE (v${VERSION}, ${BYTES} bytes)"
  # Drop an unpacked binary so pack/resources stay archive-only (~9 MB).
  if [[ "${CODEGRAPH_KEEP_BIN:-}" != "1" && -f "$DEST_BIN" ]]; then
    rm -f "$DEST_BIN"
    echo "    removed unpacked $DEST_BIN (extract on first use)"
  fi
  exit 0
fi

BASE_URL="${CODEGRAPH_BASE_URL:-https://github.com/sunerpy/codegraph-rust/releases/download/v${VERSION}}"
URL="${BASE_URL}/${ASSET}"
TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

ARCHIVE="$TMP/$ASSET"
echo "==> Downloading CodeGraph-Rust v${VERSION} (${TARGET})"
echo "    $URL"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$ARCHIVE" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$ARCHIVE" "$URL"
else
  echo "curl or wget required to fetch CodeGraph" >&2
  exit 1
fi

cp -f "$ARCHIVE" "$DEST_ARCHIVE"
printf '%s\n' "$VERSION" >"$VERSION_FILE"
# Prefer archive-only on disk until the runtime extracts (smaller packs / home install).
if [[ "${CODEGRAPH_KEEP_BIN:-}" != "1" && -f "$DEST_BIN" ]]; then
  rm -f "$DEST_BIN"
fi

# Optional: also unpack now (dev convenience).
if [[ "${CODEGRAPH_EXTRACT:-}" == "1" ]]; then
  echo "==> Extracting (CODEGRAPH_EXTRACT=1)"
  mkdir -p "$TMP/out"
  if [[ "$EXT" == "zip" ]]; then
    unzip -qo "$DEST_ARCHIVE" -d "$TMP/out"
  else
    tar -xzf "$DEST_ARCHIVE" -C "$TMP/out"
  fi
  FOUND="$(find "$TMP/out" -type f \( -name 'codegraph' -o -name 'codegraph.exe' \) | head -n 1 || true)"
  if [[ -z "$FOUND" ]]; then
    FOUND="$(find "$TMP" -maxdepth 2 -type f \( -name 'codegraph' -o -name 'codegraph.exe' \) ! -path "$ARCHIVE" ! -path "$DEST_ARCHIVE" | head -n 1 || true)"
  fi
  if [[ -z "$FOUND" ]]; then
    echo "codegraph binary not found in archive" >&2
    exit 1
  fi
  cp -f "$FOUND" "$DEST_BIN"
  chmod +x "$DEST_BIN" 2>/dev/null || true
  echo "==> Wrote $DEST_BIN + $DEST_ARCHIVE (CodeGraph-Rust v${VERSION})"
else
  BYTES="$(wc -c <"$DEST_ARCHIVE" | tr -d ' ')"
  echo "==> Wrote $DEST_ARCHIVE (${BYTES} bytes, CodeGraph-Rust v${VERSION}, MIT — https://github.com/sunerpy/codegraph-rust)"
  echo "    Binary extracts on first expert use (~56 MB unpacked)."
fi
