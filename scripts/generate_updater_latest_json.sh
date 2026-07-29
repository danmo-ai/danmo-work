#!/usr/bin/env bash
# Generate Tauri updater latest.json from desktop bundle artifacts.
#
# Usage:
#   scripts/generate_updater_latest_json.sh \
#     --version 0.6.8 \
#     --base-url https://github.com/danmo-ai/danmo-work/releases/download/v0.6.8 \
#     --out latest.json \
#     [--notes "release notes"] \
#     [--asset-dir release-assets]
set -euo pipefail

VERSION=""
BASE_URL=""
OUT="latest.json"
NOTES=""
ASSET_DIR="."

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --base-url) BASE_URL="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    --notes) NOTES="$2"; shift 2 ;;
    --asset-dir) ASSET_DIR="$2"; shift 2 ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$VERSION" || -z "$BASE_URL" ]]; then
  echo "Usage: $0 --version X.Y.Z --base-url URL [--out latest.json] [--asset-dir DIR]" >&2
  exit 1
fi

BASE_URL="${BASE_URL%/}"
PUB_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
ASSET_DIR="$(cd "$ASSET_DIR" && pwd)"

python3 - "$VERSION" "$NOTES" "$PUB_DATE" "$BASE_URL" "$ASSET_DIR" "$OUT" <<'PY'
import json, sys
from pathlib import Path

version, notes, pub_date, base_url, asset_dir, out = sys.argv[1:7]
root = Path(asset_dir)
platforms = {}


def read_sig(path):
    sibling = Path(str(path) + ".sig")
    if sibling.is_file():
        return sibling.read_text(encoding="utf-8").strip()
    alt = path.with_suffix(".sig")
    if alt.is_file():
        return alt.read_text(encoding="utf-8").strip()
    return None


def classify(name):
    lower = name.lower()
    if lower.endswith(".app.tar.gz"):
        if "aarch64" in lower or "arm64" in lower:
            return "darwin-aarch64"
        if "x86_64" in lower or "x64" in lower:
            return "darwin-x86_64"
        return "darwin-aarch64"
    # Tauri v2 updater reuses the AppImage (+ .sig); prefer .AppImage over .AppImage.tar.gz
    if lower.endswith(".appimage.tar.gz"):
        if "aarch64" in lower or "arm64" in lower:
            return "linux-aarch64"
        return "linux-x86_64"
    if lower.endswith(".appimage"):
        if "aarch64" in lower or "arm64" in lower:
            return "linux-aarch64"
        return "linux-x86_64"
    if lower.endswith(".nsis.zip"):
        return "windows-x86_64"
    if "setup.exe" in lower or lower.endswith("-setup.exe"):
        return "windows-x86_64"
    return None


# Prefer nsis.zip / AppImage / app.tar.gz over setup.exe
candidates = []
for path in root.rglob("*"):
    if not path.is_file():
        continue
    name = path.name
    platform = classify(name)
    if not platform:
        continue
    # skip signature files themselves
    if name.endswith(".sig"):
        continue
    priority = 0
    lower = name.lower()
    if lower.endswith(".nsis.zip"):
        priority = 2
    elif lower.endswith(".app.tar.gz"):
        priority = 2
    elif lower.endswith(".appimage") and not lower.endswith(".appimage.tar.gz"):
        priority = 2
    elif lower.endswith(".appimage.tar.gz"):
        priority = 1
    elif "setup.exe" in lower:
        priority = 1
    candidates.append((priority, path, platform))

candidates.sort(key=lambda x: x[0], reverse=True)
for priority, path, platform in candidates:
    if platform in platforms and priority < 2:
        continue
    sig = read_sig(path)
    if not sig:
        print(f"WARNING: missing signature for {path.name} — skip {platform}", file=sys.stderr)
        continue
    platforms[platform] = {
        "signature": sig,
        "url": f"{base_url}/{path.name}",
    }

if not platforms:
    print(f"ERROR: no signed updater artifacts found under {asset_dir}", file=sys.stderr)
    sys.exit(1)

doc = {
    "version": version,
    "notes": notes,
    "pub_date": pub_date,
    "platforms": platforms,
}
Path(out).write_text(json.dumps(doc, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
print(f"Wrote {out} with platforms: {', '.join(sorted(platforms))}")
PY
