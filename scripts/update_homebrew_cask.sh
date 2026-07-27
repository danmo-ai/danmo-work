#!/usr/bin/env bash
# Update Casks/danmo-work.rb version + sha256 for a release DMG.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CASK="$ROOT/Casks/danmo-work.rb"

VERSION=""
SHA256=""
DMG_PATH=""
DMG_URL=""

usage() {
  cat <<'EOF'
Usage:
  scripts/update_homebrew_cask.sh --version X.Y.Z --sha256 HEX
  scripts/update_homebrew_cask.sh --version X.Y.Z --dmg PATH
  scripts/update_homebrew_cask.sh --version X.Y.Z --dmg-url URL
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --sha256) SHA256="${2:-}"; shift 2 ;;
    --dmg) DMG_PATH="${2:-}"; shift 2 ;;
    --dmg-url) DMG_URL="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; usage >&2; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  echo "--version is required" >&2
  exit 1
fi

if [[ -z "$SHA256" && -n "$DMG_PATH" ]]; then
  if [[ ! -f "$DMG_PATH" ]]; then
    echo "DMG not found: $DMG_PATH" >&2
    exit 1
  fi
  SHA256="$(shasum -a 256 "$DMG_PATH" | awk '{print $1}')"
fi

if [[ -z "$SHA256" && -n "$DMG_URL" ]]; then
  TMP="$(mktemp)"
  trap 'rm -f "$TMP"' EXIT
  curl -fsSL -o "$TMP" "$DMG_URL"
  SHA256="$(shasum -a 256 "$TMP" | awk '{print $1}')"
fi

if [[ -z "$SHA256" ]]; then
  echo "Provide --sha256, --dmg, or --dmg-url" >&2
  exit 1
fi

if [[ ! -f "$CASK" ]]; then
  echo "Cask not found: $CASK" >&2
  exit 1
fi

# Portable in-place edit (macOS / Linux)
tmp="$(mktemp)"
sed -E \
  -e "s/^(  version )\"[^\"]+\"/\\1\"${VERSION}\"/" \
  -e "s/^(  sha256 )\"[^\"]+\"/\\1\"${SHA256}\"/" \
  "$CASK" > "$tmp"
mv "$tmp" "$CASK"

echo "Updated $CASK → version=$VERSION sha256=$SHA256"
