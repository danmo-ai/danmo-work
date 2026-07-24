#!/usr/bin/env bash
# Fetch Microsoft Coreutils for Windows (multi-call binary) into the desktop resources tree.
# Used by pack_desktop_windows.sh so Windows builds ship POSIX utilities without Git Bash.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=out_paths.sh
source "$SCRIPT_DIR/out_paths.sh"

VERSION="${DQ_COREUTILS_VERSION:-2026.6.16}"
ARCH="${DQ_COREUTILS_ARCH:-x64}" # x64 | arm64
DEST_DIR="${1:-$DQ_ROOT/desktop/src-tauri/resources/coreutils}"
URL="${DQ_COREUTILS_URL:-https://github.com/microsoft/coreutils/releases/download/v${VERSION}/coreutils-${VERSION}-${ARCH}.zip}"

mkdir -p "$DEST_DIR"
DEST_EXE="$DEST_DIR/coreutils.exe"

if [[ -f "$DEST_EXE" && "${DQ_COREUTILS_FORCE:-}" != "1" ]]; then
  SIZE="$(wc -c < "$DEST_EXE" | tr -d ' ')"
  if [[ "${SIZE:-0}" -lt 1000000 ]]; then
    echo "WARNING: existing coreutils.exe looks truncated ($SIZE bytes); re-fetching" >&2
  else
    echo "==> Coreutils already present: $DEST_EXE ($(du -h "$DEST_EXE" | cut -f1))"
    printf '%s\n' "$VERSION" >"$DEST_DIR/VERSION"
    exit 0
  fi
fi

TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

ZIP="$TMP/coreutils.zip"
echo "==> Downloading Coreutils for Windows v${VERSION} (${ARCH})"
echo "    $URL"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$ZIP" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$ZIP" "$URL"
else
  echo "curl or wget required to fetch Coreutils" >&2
  exit 1
fi

echo "==> Extracting coreutils.exe (skipping PDBs)"
unzip -qo "$ZIP" -d "$TMP/out"
FOUND="$(find "$TMP/out" -type f -name 'coreutils.exe' | head -n 1 || true)"
if [[ -z "$FOUND" ]]; then
  echo "coreutils.exe not found in archive" >&2
  exit 1
fi
cp -f "$FOUND" "$DEST_EXE"
# Marker for license attribution / version pinning
printf '%s\n' "$VERSION" >"$DEST_DIR/VERSION"
cat >"$DEST_DIR/README.md" <<EOF
# Microsoft Coreutils for Windows (bundled)

Version: ${VERSION}
Source: https://github.com/microsoft/coreutils/releases/tag/v${VERSION}
License: MIT (see repository NOTICE.md / LICENSE)

**Default-installed** with the Windows desktop NSIS package:
1. \`pack-windows-desktop\` downloads \`coreutils.exe\` into this folder (required).
2. The installer copies it to \`%USERPROFILE%\\.danmo-work\\bin\\coreutils\\\` and runs
   \`coreutils-manager refresh\` to create applet hardlinks (\`ls.exe\`, \`cat.exe\`, …).
3. Desktop / backend prepend that \`bin\\\` directory to \`exec_shell\` PATH.

No Git Bash install is required for POSIX utilities on Windows host / win-token.
EOF

SIZE="$(wc -c < "$DEST_EXE" | tr -d ' ')"
if [[ "${SIZE:-0}" -lt 1000000 ]]; then
  echo "ERROR: downloaded coreutils.exe looks truncated ($SIZE bytes)" >&2
  rm -f "$DEST_EXE"
  exit 1
fi

echo "==> Wrote $DEST_EXE ($(du -h "$DEST_EXE" | cut -f1)) — required for Windows desktop packs"
